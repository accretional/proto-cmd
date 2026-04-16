# Fuzz testing

Go native fuzz targets live in `commander/fuzz_test.go`. `test.sh` runs each one briefly on every pipeline invocation so regressions surface automatically; longer burns are a one-word env change.

## TL;DR

```bash
./test.sh                                     # each target: 2s, 2 workers (default)
FUZZTIME=30s ./test.sh                        # longer per-target burn
FUZZTIME=5m FUZZPARALLEL=4 ./test.sh          # deeper burn, more workers
FUZZTIME=0 ./test.sh                          # skip fuzz entirely

# target one explicitly
go test -run=^$ -fuzz=^FuzzCommand_Roundtrip$ -fuzztime=2m -parallel=2 ./commander
```

Defaults are intentionally conservative. `FuzzShell_EchoArgs` forks `/bin/echo` for every iteration, and 10 parallel workers × fork/exec will saturate a laptop (this tipped one dev machine over). Bump `FUZZPARALLEL` when you know the box can take it.

A failing input gets written to `commander/testdata/fuzz/<target>/<hash>` and **replays as a regular unit test** on every subsequent `go test` run. Commit those files — they're the regression suite.

## Targets

| Target                    | Target type | What it exercises                                                                     |
|---------------------------|-------------|---------------------------------------------------------------------------------------|
| `FuzzShell_EchoArgs`      | End-to-end  | Bytes → `/bin/echo` argv → gRPC stream. Catches crashes, hangs, runaway output.       |
| `FuzzCommand_Roundtrip`   | Pure proto  | `Command` marshal → unmarshal → re-marshal equality.                                  |
| `FuzzOutput_Roundtrip`    | Pure proto  | Same for `Output`.                                                                    |

Approximate throughput on a current MacBook (single 5s run):

- `FuzzShell_EchoArgs` — ~1.5k execs/s (limited by fork/exec + stream round-trip)
- `FuzzCommand_Roundtrip` — ~50k execs/s
- `FuzzOutput_Roundtrip` — ~60k execs/s

## Safety rule

> **Fuzzer-controlled bytes must never reach `/bin/sh -c`.**

The `Commander.Shell` RPC is a remote-code-execution surface by design. If we wired the fuzzer's input directly into `cmd.Command` we'd be running arbitrary generated shell on the dev machine.

`FuzzShell_EchoArgs` enforces this structurally by using **Args mode** (`Command: "/bin/echo"`, `Args: []string{fuzzedBytes}`). In Args mode the server calls `exec.CommandContext("/bin/echo", fuzzedBytes)` — argv[0] is hardcoded to a known-safe binary, and argv[1] is handed to echo as a literal string, never interpreted.

If you add a new `Shell`-level fuzz target, keep to the same pattern: hardcode argv[0] to something that can't do damage even if handed bizarre input.

## What `FuzzShell_EchoArgs` does and doesn't assert

**Does assert:**
- No server panic.
- Stream closes within a bounded context deadline.
- Total output bytes are bounded (10 MiB cap).
- Non-cancel gRPC errors fail the test.

**Does NOT assert** exact echo output. Reasons:

1. Platform echo(1) behavior varies — BSD echo (macOS) and GNU echo (Linux) differ in flag handling (`-n`, `-e`, `-E`) and some edge cases.
2. Under heavy fuzz load, the server-side child timeout can legitimately fire before echo flushes, leaving us with a killed child and empty stdout. That's correct server behavior, not a bug.

The proto-level round-trip targets (`FuzzCommand_Roundtrip`, `FuzzOutput_Roundtrip`) are the exact-equality ones — they're pure, deterministic, and ~100× faster per exec.

## Known non-bug inputs (skipped in the fuzz body)

The fuzzer found these early on. All three are legitimate contract constraints, not server bugs, so the target skips them via `t.Skip`:

| Constraint                                  | Why                                                                                  |
|---------------------------------------------|--------------------------------------------------------------------------------------|
| `Args` string must be valid UTF-8           | proto3 `string` requires UTF-8; the gRPC marshaler rejects invalid sequences.        |
| `Args` string cannot contain NUL (`0x00`)   | POSIX argv is NUL-terminated C strings; the kernel rejects at `execve`.              |
| `len(payload) > 64_000`                     | Keeps iterations fast; oversized inputs are covered elsewhere.                       |

If a future change adds a new such constraint, add it to the skip list *with a comment explaining why* rather than working around it.

## Concurrency model

Go's fuzz harness runs workers in parallel. Three details matter:

1. **Worker count is capped via `-parallel`**, wired from the `FUZZPARALLEL` env var in `test.sh` (default `2`). Without this, Go defaults to `GOMAXPROCS`, and 10 workers × fork/exec on the Shell target is enough to crash a laptop.
2. **Across worker processes**, each worker calls `FuzzShell_EchoArgs(f)` independently and gets its own in-process gRPC server via `startServer(f)`. No cross-process state.
3. **Within a single worker**, a `sync.Mutex` serializes iterations. The shared bufconn transport doesn't love having many concurrent streams with fork/exec behind them, so we lock for the duration of one Shell round-trip. Fuzz still explores the input space plenty fast.

`bufSize` in the test helper is 16 MiB — sized for the current fuzz payload cap plus gRPC framing overhead.

## Adding a new fuzz target

1. Drop a `FuzzXxx(f *testing.F)` in `commander/fuzz_test.go`.
2. Seed with `f.Add(...)` values that represent real-world shapes.
3. In the `f.Fuzz` body, `t.Skip` inputs that violate documented contracts — don't rewrite them to be valid.
4. If the target calls `Shell`, follow the safety rule above.
5. Add its name to the list `test.sh` iterates over.
6. Document its purpose in this file's **Targets** table.

## Reproducing a reported crash

1. The crash report shows a `testdata/fuzz/<target>/<hash>` path. Commit that file.
2. Rerun just that case: `go test -run=FuzzXxx/<hash> ./commander`.
3. If it doesn't reproduce sequentially, it's a concurrency-with-the-test-harness issue, not a server bug — see the Shell target comment for precedent.

## Where to look next

- `commander/fuzz_test.go` — source of truth; the safety rule is encoded there.
- `commander/commander_test.go` — the `startServer` / `drain` helpers the fuzz targets reuse.
- `test.sh` — fuzz wiring (the `FUZZTIME` block).
- `AGENTS.md` — top-level summary of the fuzz workflow for future agents.
