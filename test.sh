#!/bin/bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
cd "$SCRIPT_DIR"

# --- Chain to build (which chains to setup) ---

bash "$SCRIPT_DIR/build.sh"

echo ""
echo "=== proto-cmd tests ==="

echo "--- go vet ---"
go vet ./...
echo "go vet: ok"
echo ""

echo "--- go test ---"
go test -count=1 ./...
echo ""

echo "=== all tests passed ==="
