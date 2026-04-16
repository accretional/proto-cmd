# Agents

## What this repo is

A port of [`github.com/accretional/runrpc`](https://github.com/accretional/runrpc)'s `commander` service into a standalone module. The `Commander` gRPC service executes shell commands and streams back stdout/stderr chunks.

## Go version

go1.26 (enforced via `go.mod`).

## Scripts

All build/test/run actions go through scripts. **Never build, test, or run outside these scripts. Never `git commit` or `git push` without running them.**

- `./setup.sh` — install `protoc-gen-go` + `protoc-gen-go-grpc`, run `go mod tidy`
- `./build.sh` — runs `setup.sh`, regenerates `.pb.go`/`_grpc.pb.go`, `go build ./...`
- `./test.sh` — runs `build.sh`, then `go vet ./...` and `go test ./...`
- `./LET_IT_RIP.sh` — runs `test.sh`, then boots the commander server and exercises it end-to-end

Scripts are idempotent: each stage skips work already done and each higher-level script invokes the lower-level one. Re-running is always safe.

## Layout

- `commander/commander.proto` — service definition (ported verbatim from runrpc)
- `commander/commander.go` — server implementation
- `commander/commander_test.go` — in-process gRPC test using `bufconn`
- `cmd/commanderd/` — standalone gRPC server binary (used by `LET_IT_RIP.sh`)

## Important

Never edit generated `*.pb.go` / `*_grpc.pb.go` — regenerate via `build.sh`.
