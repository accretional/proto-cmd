#!/bin/bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
cd "$SCRIPT_DIR"

# --- Chain to setup (idempotent) ---

bash "$SCRIPT_DIR/setup.sh"

echo ""
echo "=== proto-cmd build ==="

GOBIN="${GOBIN:-$(go env GOPATH)/bin}"
export PATH="$GOBIN:$PATH"

# --- Regenerate proto bindings if sources are newer than generated files ---

needs_gen() {
    local proto="$1"
    local pb="${proto%.proto}.pb.go"
    local grpc_pb="${proto%.proto}_grpc.pb.go"
    if [ ! -f "$pb" ] || [ ! -f "$grpc_pb" ]; then
        return 0
    fi
    if [ "$proto" -nt "$pb" ] || [ "$proto" -nt "$grpc_pb" ]; then
        return 0
    fi
    return 1
}

PROTOS=(commander/commander.proto)

for proto in "${PROTOS[@]}"; do
    if needs_gen "$proto"; then
        echo "protoc: generating $proto"
        protoc \
            --proto_path=. \
            --go_out=. --go_opt=paths=source_relative \
            --go-grpc_out=. --go-grpc_opt=paths=source_relative \
            "$proto"
    else
        echo "protoc: $proto up-to-date"
    fi
done

# --- Compile ---

echo ""
echo "go build ./..."
go build ./...

# --- Build commanderd binary ---

echo "go build -o commanderd ./cmd/commanderd"
go build -o commanderd ./cmd/commanderd

echo ""
echo "=== build complete ==="
