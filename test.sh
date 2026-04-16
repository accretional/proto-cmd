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

# --- Fuzz (short by default; opt out with FUZZTIME=0) ---

FUZZTIME="${FUZZTIME:-5s}"
if [ "$FUZZTIME" = "0" ] || [ "$FUZZTIME" = "0s" ]; then
    echo "--- fuzz (skipped, FUZZTIME=0) ---"
else
    echo "--- fuzz (FUZZTIME=$FUZZTIME per target) ---"
    # Each target runs for $FUZZTIME. Corpus / failures land in commander/testdata.
    # -run=^$ skips unit tests so we don't re-run them; crashing inputs in
    # testdata/fuzz are still replayed by the earlier `go test` line above.
    for target in FuzzShell_EchoArgs FuzzCommand_Roundtrip FuzzOutput_Roundtrip; do
        echo "  -> $target"
        go test -run=^$ -fuzz="^${target}$" -fuzztime="$FUZZTIME" ./commander
    done
fi
echo ""

echo "=== all tests passed ==="
