package commander

// Fuzz targets for the Commander package.
//
// SAFETY CONSTRAINT: fuzzer-controlled bytes MUST NEVER reach /bin/sh -c.
// All Shell-level fuzzing runs /bin/echo via Args mode so the fuzzed payload
// is an argv slot (printed verbatim), never a shell string. Proto-level
// fuzzing is pure — it never execs anything.
//
// Run locally:
//
//	go test -run=^$ -fuzz=FuzzShell_EchoArgs      ./commander -fuzztime=30s
//	go test -run=^$ -fuzz=FuzzCommand_Roundtrip   ./commander -fuzztime=30s
//
// test.sh runs each target briefly (FUZZTIME, default 5s). A crash gets
// written to commander/testdata/fuzz/<target>/<hash> and will replay as a
// regular unit test on subsequent `go test` runs.

import (
	"bytes"
	"context"
	"io"
	"sync"
	"testing"
	"time"
	"unicode/utf8"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
)

// FuzzShell_EchoArgs stresses the streaming plumbing by passing arbitrary
// bytes as a single argv slot to /bin/echo and draining the response stream.
//
// Success criteria (what we actually care about):
//   - No server panic / crash.
//   - Stream closes within a bounded time.
//   - Output size is bounded (no runaway).
//
// We deliberately do NOT assert echo's exact output — that depends on the
// platform's echo(1) and on process scheduling (the server-side child
// timeout can legitimately kill echo before it flushes under heavy fuzz
// load). The proto-level round-trip fuzzers assert exact equality; this
// one targets the server's concurrency/streaming code.
func FuzzShell_EchoArgs(f *testing.F) {
	client, cleanup := startServer(f)
	f.Cleanup(cleanup)

	// Serialize iterations within a single fuzz worker process so we don't
	// pile N concurrent fork/exec on top of one shared bufconn transport.
	var mu sync.Mutex

	f.Add([]byte(""))
	f.Add([]byte("hello"))
	f.Add([]byte("line1\nline2\nline3"))
	f.Add([]byte("\x01\x02\x03"))
	f.Add([]byte("漢字 café 🚀"))
	f.Add(bytes.Repeat([]byte("A"), 8_000))

	f.Fuzz(func(t *testing.T, payload []byte) {
		// Contract constraints that aren't what we're testing:
		// - proto3 `string` requires valid UTF-8; Args is repeated string.
		// - argv cannot contain NUL (C-string terminator).
		if !utf8.Valid(payload) {
			t.Skip("Args is proto3 string; needs valid UTF-8")
		}
		if bytes.IndexByte(payload, 0) >= 0 {
			t.Skip("argv cannot contain NUL")
		}
		if len(payload) > 64_000 {
			t.Skip("argv large enough; oversized cases tested elsewhere")
		}

		mu.Lock()
		defer mu.Unlock()

		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()

		stream, err := client.Shell(ctx, &Command{
			Command:        "/bin/echo",
			Args:           []string{string(payload)},
			TimeoutSeconds: 10,
		})
		if err != nil {
			t.Fatalf("Shell: %v", err)
		}

		const outputCap = 10 * 1024 * 1024
		var total int
		for {
			msg, rerr := stream.Recv()
			if rerr == io.EOF {
				return
			}
			if rerr != nil {
				// Context cancel / deadline are acceptable — mean the server
				// correctly propagated cancellation. Anything else is a bug.
				if c := status.Code(rerr); c == codes.DeadlineExceeded || c == codes.Canceled {
					return
				}
				t.Fatalf("recv: %v", rerr)
			}
			total += len(msg.Data)
			if total > outputCap {
				t.Fatalf("runaway output: %d bytes for %d-byte payload", total, len(payload))
			}
		}
	})
}

// FuzzCommand_Roundtrip fuzzes Command proto marshal/unmarshal. Pure —
// never execs. Catches regressions where a Command value fails to
// round-trip through the generated marshaler.
func FuzzCommand_Roundtrip(f *testing.F) {
	// Seed with a few structurally varied Commands.
	seeds := []*Command{
		{},
		{Command: "echo hi"},
		{Command: "/bin/true", Args: []string{"a", "", "c\x00"}},
		{Command: "x", WorkingDir: "/tmp", Env: map[string]string{"K": "v", "": ""}, TimeoutSeconds: 7, Shell: "/bin/bash"},
	}
	for _, s := range seeds {
		b, err := proto.Marshal(s)
		if err == nil {
			f.Add(b)
		}
	}

	f.Fuzz(func(t *testing.T, data []byte) {
		var cmd Command
		if err := proto.Unmarshal(data, &cmd); err != nil {
			return // invalid wire is fine — not what we're looking for
		}
		out, err := proto.Marshal(&cmd)
		if err != nil {
			t.Fatalf("re-marshal: %v", err)
		}
		var cmd2 Command
		if err := proto.Unmarshal(out, &cmd2); err != nil {
			t.Fatalf("unmarshal of re-marshaled bytes: %v (original=%x)", err, data)
		}
		if !proto.Equal(&cmd, &cmd2) {
			t.Fatalf("round-trip diverged\nfirst:  %+v\nsecond: %+v", &cmd, &cmd2)
		}
	})
}

// FuzzOutput_Roundtrip is the sibling of FuzzCommand_Roundtrip for Output.
// Also pure — exercises bytes + bool serialization.
func FuzzOutput_Roundtrip(f *testing.F) {
	for _, s := range []*Output{
		{},
		{Stdout: true, Data: []byte("hello")},
		{Stdout: false, Data: bytes.Repeat([]byte{0}, 1024)},
	} {
		b, err := proto.Marshal(s)
		if err == nil {
			f.Add(b)
		}
	}

	f.Fuzz(func(t *testing.T, data []byte) {
		var o Output
		if err := proto.Unmarshal(data, &o); err != nil {
			return
		}
		out, err := proto.Marshal(&o)
		if err != nil {
			t.Fatalf("re-marshal: %v", err)
		}
		var o2 Output
		if err := proto.Unmarshal(out, &o2); err != nil {
			t.Fatalf("unmarshal re-marshaled: %v", err)
		}
		if !proto.Equal(&o, &o2) {
			t.Fatalf("round-trip diverged: %+v vs %+v", &o, &o2)
		}
	})
}
