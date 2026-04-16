package commander

import (
	"bytes"
	"context"
	"io"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
)

const bufSize = 1 << 20

// startServer spins up an in-process Commander server over bufconn and
// returns a client plus a cleanup function.
func startServer(t *testing.T) (CommanderClient, func()) {
	t.Helper()

	lis := bufconn.Listen(bufSize)
	srv := grpc.NewServer()
	RegisterCommanderServer(srv, NewCommanderServer())

	serveErr := make(chan error, 1)
	go func() { serveErr <- srv.Serve(lis) }()

	dialer := func(context.Context, string) (net.Conn, error) { return lis.Dial() }
	conn, err := grpc.NewClient(
		"passthrough:///bufnet",
		grpc.WithContextDialer(dialer),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		srv.Stop()
		t.Fatalf("dial bufconn: %v", err)
	}

	cleanup := func() {
		conn.Close()
		srv.GracefulStop()
		<-serveErr
	}
	return NewCommanderClient(conn), cleanup
}

// drain collects every Output chunk the server sends until EOF or error.
func drain(t *testing.T, stream grpc.ServerStreamingClient[Output]) (stdout, stderr []byte, err error) {
	t.Helper()
	var outBuf, errBuf bytes.Buffer
	for {
		msg, rerr := stream.Recv()
		if rerr == io.EOF {
			return outBuf.Bytes(), errBuf.Bytes(), nil
		}
		if rerr != nil {
			return outBuf.Bytes(), errBuf.Bytes(), rerr
		}
		if msg.Stdout {
			outBuf.Write(msg.Data)
		} else {
			errBuf.Write(msg.Data)
		}
	}
}

func TestShell_EmptyCommand(t *testing.T) {
	client, cleanup := startServer(t)
	defer cleanup()

	stream, err := client.Shell(context.Background(), &Command{})
	if err != nil {
		t.Fatalf("Shell: %v", err)
	}
	_, _, err = drain(t, stream)
	st, ok := status.FromError(err)
	if !ok || st.Code() != codes.InvalidArgument {
		t.Fatalf("expected InvalidArgument, got %v", err)
	}
}

func TestShell_StdoutCapture(t *testing.T) {
	client, cleanup := startServer(t)
	defer cleanup()

	stream, err := client.Shell(context.Background(), &Command{
		Command: "echo hello-stdout",
	})
	if err != nil {
		t.Fatalf("Shell: %v", err)
	}
	stdout, stderr, err := drain(t, stream)
	if err != nil {
		t.Fatalf("drain: %v", err)
	}
	if !bytes.Contains(stdout, []byte("hello-stdout")) {
		t.Fatalf("stdout missing expected output: %q", stdout)
	}
	if len(stderr) != 0 {
		t.Fatalf("unexpected stderr: %q", stderr)
	}
}

func TestShell_StderrCapture(t *testing.T) {
	client, cleanup := startServer(t)
	defer cleanup()

	stream, err := client.Shell(context.Background(), &Command{
		Command: "echo oops 1>&2",
	})
	if err != nil {
		t.Fatalf("Shell: %v", err)
	}
	stdout, stderr, err := drain(t, stream)
	if err != nil {
		t.Fatalf("drain: %v", err)
	}
	if !bytes.Contains(stderr, []byte("oops")) {
		t.Fatalf("stderr missing expected output: %q", stderr)
	}
	if len(stdout) != 0 {
		t.Fatalf("unexpected stdout: %q", stdout)
	}
}

// Direct-exec path: when Args are supplied, the server runs argv[0] directly
// rather than routing through /bin/sh -c.
func TestShell_ExecArgsMode(t *testing.T) {
	client, cleanup := startServer(t)
	defer cleanup()

	stream, err := client.Shell(context.Background(), &Command{
		Command: "echo",
		Args:    []string{"direct-exec"},
	})
	if err != nil {
		t.Fatalf("Shell: %v", err)
	}
	stdout, _, err := drain(t, stream)
	if err != nil {
		t.Fatalf("drain: %v", err)
	}
	if !bytes.Contains(stdout, []byte("direct-exec")) {
		t.Fatalf("stdout missing: %q", stdout)
	}
}

func TestShell_NonZeroExit(t *testing.T) {
	client, cleanup := startServer(t)
	defer cleanup()

	stream, err := client.Shell(context.Background(), &Command{
		Command: "exit 7",
	})
	if err != nil {
		t.Fatalf("Shell: %v", err)
	}
	_, stderr, err := drain(t, stream)
	if err != nil {
		t.Fatalf("drain: %v", err)
	}
	// Non-zero exit is surfaced as a final stderr chunk, not a gRPC error.
	if !bytes.Contains(stderr, []byte("exit status 7")) {
		t.Fatalf("stderr missing exit status: %q", stderr)
	}
}

func TestShell_EnvAndWorkingDir(t *testing.T) {
	client, cleanup := startServer(t)
	defer cleanup()

	tmp := t.TempDir()
	marker := filepath.Join(tmp, "marker")
	if err := os.WriteFile(marker, []byte("present"), 0o644); err != nil {
		t.Fatalf("write marker: %v", err)
	}

	stream, err := client.Shell(context.Background(), &Command{
		Command:    `echo "$CMD_ENV_VAR" && ls marker`,
		WorkingDir: tmp,
		Env:        map[string]string{"CMD_ENV_VAR": "env-ok"},
	})
	if err != nil {
		t.Fatalf("Shell: %v", err)
	}
	stdout, _, err := drain(t, stream)
	if err != nil {
		t.Fatalf("drain: %v", err)
	}
	if !bytes.Contains(stdout, []byte("env-ok")) {
		t.Fatalf("env not propagated, stdout=%q", stdout)
	}
	if !bytes.Contains(stdout, []byte("marker")) {
		t.Fatalf("working_dir not applied, stdout=%q", stdout)
	}
}

func TestShell_Timeout(t *testing.T) {
	client, cleanup := startServer(t)
	defer cleanup()

	start := time.Now()
	stream, err := client.Shell(context.Background(), &Command{
		Command:        "sleep 5",
		TimeoutSeconds: 1,
	})
	if err != nil {
		t.Fatalf("Shell: %v", err)
	}
	// Drain to completion — CommandContext kills the child, server reports the
	// kill signal back as a non-error stream close or as a final stderr chunk.
	_, _, _ = drain(t, stream)
	if elapsed := time.Since(start); elapsed > 4*time.Second {
		t.Fatalf("timeout did not fire within expected window (elapsed=%s)", elapsed)
	}
}
