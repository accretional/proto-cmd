#!/bin/bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
cd "$SCRIPT_DIR"

echo "=== proto-cmd setup ==="

# --- Go ---

if ! command -v go &>/dev/null; then
    echo "ERROR: go not found. Install Go 1.26+ from https://go.dev/dl/"
    exit 1
fi
GO_VERSION="$(go version | awk '{print $3}')"
echo "go:     ${GO_VERSION}"

# Require 1.26+ (match go.mod). Accept any go1.26.x or newer.
case "$GO_VERSION" in
    go1.26*|go1.27*|go1.28*|go1.29*|go1.3*|go1.4*|go2*) ;;
    *)
        echo "ERROR: Go 1.26+ required, found ${GO_VERSION}"
        exit 1
        ;;
esac

# --- protoc ---

if ! command -v protoc &>/dev/null; then
    echo "ERROR: protoc not found."
    echo "  macOS: brew install protobuf"
    echo "  Linux: apt install -y protobuf-compiler"
    exit 1
fi
echo "protoc: $(protoc --version)"

# --- protoc plugins ---

GOBIN="${GOBIN:-$(go env GOPATH)/bin}"
export PATH="$GOBIN:$PATH"

install_if_missing() {
    local bin="$1"
    local pkg="$2"
    if command -v "$bin" &>/dev/null || [ -x "$GOBIN/$bin" ]; then
        echo "$bin: found"
    else
        echo "Installing $bin..."
        go install "$pkg"
    fi
}

install_if_missing protoc-gen-go      google.golang.org/protobuf/cmd/protoc-gen-go@latest
install_if_missing protoc-gen-go-grpc google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest

# --- go mod tidy ---

echo ""
echo "go mod tidy..."
go mod tidy

echo ""
echo "=== setup complete ==="
