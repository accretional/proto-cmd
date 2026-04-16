# Development Rules

**No one-off commands.** All setup, build, test, and run steps go through scripts:

- `./setup.sh` — install proto plugins, `go mod tidy`
- `./build.sh` — runs `setup.sh`, regen proto, `go build`
- `./test.sh` — runs `build.sh`, `go vet` + `go test`
- `./LET_IT_RIP.sh` — runs `test.sh`, then spins up and exercises the server

Every script is idempotent and invokes the one below it. If something needs to be tested or run, it belongs in a script. If it's not in a script, it doesn't count.

**ALWAYS run `./LET_IT_RIP.sh` before EVERY `git commit` and `git push`.** No exceptions.

Quick `go build ./...` or `go vet ./...` during dev is fine for catching compile errors, but final validation before commit must go through the scripts.

## Project rules

- Go version: **go1.26** (in `go.mod`).
- Port is from `github.com/accretional/runrpc/commander`. Keep the proto semantically identical unless there is a good reason to diverge — note any divergence in README.md `### Report`.
- Never edit generated `*.pb.go` / `*_grpc.pb.go` by hand; regenerate via `build.sh`.
- README.md is a living plan: don't condense `# Overview` or `### Report` sections without explicit approval.

## Security note (from runrpc)

The Commander service executes arbitrary shell commands. Do not expose it to any network without a sandbox. Treat it as remote code execution.
