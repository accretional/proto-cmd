#!/bin/bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
cd "$SCRIPT_DIR"

PORT="${1:-50551}"
ADDR="127.0.0.1:${PORT}"

# --- Pre-run cleanup ---

kill_port() {
    local p="$1"
    local pids
    pids="$(lsof -ti ":$p" 2>/dev/null || true)"
    if [ -n "$pids" ]; then
        echo "killing processes on port $p: $pids"
        echo "$pids" | xargs kill -9 2>/dev/null || true
    fi
}

kill_port "$PORT"

SERVER_PID=""
cleanup() {
    echo ""
    echo "=== cleanup ==="
    if [ -n "$SERVER_PID" ]; then
        kill "$SERVER_PID" 2>/dev/null || true
        wait "$SERVER_PID" 2>/dev/null || true
    fi
    kill_port "$PORT"
}
trap cleanup EXIT

# --- Chain to test (which chains to build, setup) ---

bash "$SCRIPT_DIR/test.sh"

# --- End-to-end: spin up commanderd and drive it via grpcurl or a Go client ---

echo ""
echo "=== end-to-end smoke test ==="
echo "starting commanderd on ${ADDR}..."
./commanderd -addr "$ADDR" >/tmp/commanderd.log 2>&1 &
SERVER_PID=$!

# Wait for port to come up
for i in $(seq 1 30); do
    if lsof -ti ":$PORT" >/dev/null 2>&1; then
        echo "commanderd ready (pid $SERVER_PID)"
        break
    fi
    sleep 0.1
done

# Exercise the service with a minimal Go client built on the fly.
echo ""
echo "--- exercising Commander.Shell ---"
GOBIN="${GOBIN:-$(go env GOPATH)/bin}"
export PATH="$GOBIN:$PATH"

go run ./internal/smoke "$ADDR"

echo ""
echo "=== LET_IT_RIP complete ==="
