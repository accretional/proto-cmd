# proto-cmd

## Instructions

Make sure you create a setup.sh, build.sh, test.sh, and LET_IT_RIP.sh that contain all project setup scripts/commands used - NEVER build/test/run the code in this repo outside of these scripts, NEVER commit or push without running these either. Make them idempotent so that each build.sh can run setup.sh and skip things already set up, each test.sh can run build.sh, each LET_IT_RIP runs test.sh

use go1.26

Port github.com/accretional/runrpc's commander service to here and set it up with tests. Go through its code, impl, docs, etc. and determine if there is anything of interest/concern, report it here in ### Report. Write a quick doc/overview in this file in # Overview

# Overview

`proto-cmd` is a standalone Go module that packages the `Commander` gRPC service — originally part of [`github.com/accretional/runrpc`](https://github.com/accretional/runrpc) — as its own tiny repo.

The service exposes a single streaming RPC:

```proto
service Commander {
    rpc Shell(Command) returns (stream Output);
}
```

`Shell` takes a `Command` (shell string, optional argv, working dir, env, timeout, shell path) and streams back `Output` chunks tagged as stdout or stderr. Non-zero exit is not a gRPC error — the server writes a final `exit status N` chunk to stderr and closes the stream cleanly.

## Layout

- `commander/` — `.proto`, server implementation, generated bindings, tests
- `cmd/commanderd/` — standalone gRPC server binary
- `setup.sh` / `build.sh` / `test.sh` / `LET_IT_RIP.sh` — idempotent pipeline (each chains into the one above it)

## Running

```bash
./LET_IT_RIP.sh          # setup → build → test → boot server → end-to-end smoke test
./LET_IT_RIP.sh 50552    # use a different port
```

Never run `go build` / `go test` as final validation — everything must go through the scripts. See `AGENTS.md` and `CLAUDE.md`.

## Security

The service executes arbitrary shell commands. Do not expose `commanderd` to a network without a sandbox. Treat it as remote code execution. See `### Report` below.

### Report

Notes from reading through `runrpc`'s `commander` package and surrounding code:

**Implementation correctness**
- Server impl at `commander/commander.go` is clean: it sequences the two readers through a single send goroutine, so `stream.Send` is never called concurrently (gRPC forbids that). The `done` channel plus an extra goroutine that closes `ch` once both readers exit is the idiomatic fan-in.
- `exec.CommandContext` is used correctly so timeouts cancel the child, and a non-zero exit is converted into a terminal stderr chunk rather than a gRPC error. That is a deliberate design choice — clients must inspect the stream for `exit status N` rather than rely on `status.FromError`.

**Concerns**

1. **Remote code execution by design.** The service literally runs whatever string (or argv) the client sends, under the server's uid. The upstream README calls this out explicitly ("Yes, that's remote code execution"). `commanderd` in this repo binds to `127.0.0.1` by default for that reason. Anything broader needs a sandbox.
2. **`stream.Send` return value ignored on the final exit-status chunk.** In the non-zero-exit branch, the server calls `stream.Send(&Output{…"exit status N"…})` but discards the error. If the client has already gone away, that write is silently dropped. Low-impact (the stream is about to close anyway) but worth knowing.
3. **No max-message-size / backpressure considerations.** 4 KiB read buffer on each pipe is reasonable, but a chatty command can still produce a large stream. Callers that care should set gRPC `MaxCallRecvMsgSize` and/or wrap with their own cancel.
4. **Env merging replaces with `os.Environ()` + user-supplied keys.** If a client supplies `PATH=""`, that overrides the server's `PATH`. This is the upstream behavior; preserved here for parity. Server operators should be aware.
5. **Working-dir fallback uses the server's CWD.** When `working_dir` is empty, the server calls `os.Getwd()` — meaning behavior changes if you `cd` before launching the server. Consider passing `working_dir` explicitly in production callers.
6. **Shell default is `/bin/sh` and is only consulted when `args` is empty.** When `args` is set, the `shell` field is ignored and the command is exec'd directly. This dual-mode behavior matches upstream but deserves documentation for callers.
7. **No auth.** The upstream `runrpc` repo has an `auth/` package that gates services; the commander package itself does not enforce anything. This port does not include auth either — it's expected to be layered in by the caller via gRPC interceptors.
8. **Upstream is on go1.25.5; this repo requires go1.26.** Bindings regenerate cleanly with modern `protoc-gen-go` / `protoc-gen-go-grpc`.

**Divergences from upstream**
- Module path: `github.com/accretional/proto-cmd` (new repo).
- `go_package` option in `.proto` updated to match the new module path.
- Tiny ordering cleanup in `Shell`: `ctx`/timeout setup moved above the exec-vs-shell branch so both paths pick up the timeout identically. (Upstream already worked — this is just more linear to read.)
- Adds `cmd/commanderd/` so the service is runnable standalone, plus `commander_test.go` with in-process bufconn tests covering stdout, stderr, exec-mode args, non-zero exit, env/working-dir, and timeout.
