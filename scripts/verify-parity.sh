#!/bin/bash
# Verify Parity Script
# Compares current installer output against saved snapshots

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"
SNAPSHOT_DIR="$PROJECT_ROOT/internal/manager/snapshot/testdata"
BINARY_PATH="/tmp/fusionaly-parity-test"
ACTUAL_DIR="/tmp/fusionaly-actual-snapshots"

cd "$PROJECT_ROOT"

echo "========================================="
echo "Verifying UI Parity Against Snapshots"
echo "========================================="
echo

# Check snapshots exist
if [ ! -d "$SNAPSHOT_DIR" ] || [ -z "$(ls -A $SNAPSHOT_DIR/*.golden 2>/dev/null)" ]; then
    echo "ERROR: No snapshots found in $SNAPSHOT_DIR"
    echo "Run ./scripts/snapshot-parity.sh first to capture baseline snapshots."
    exit 1
fi

# Build current binary
echo "Building current binary..."
go build -o "$BINARY_PATH" ./cmd/manager
echo "  ✓ Binary built"
echo

# Create temp dir for actual outputs
rm -rf "$ACTUAL_DIR"
mkdir -p "$ACTUAL_DIR"

run_scenario() {
    local name=$1
    shift
    local inputs="$@"
    local output_file="$ACTUAL_DIR/${name}.actual"

    echo -e "$inputs" | ENV=test SKIP_PORT_CHECKING=1 timeout 30 "$BINARY_PATH" install 2>&1 | \
        sed 's/\x1b\[[0-9;]*[mK]//g' | \
        sed 's/\r//g' | \
        sed -E 's/[0-9]+\.[0-9]+\.[0-9]+\.[0-9]+/X.X.X.X/g' | \
        sed -E 's/[0-9]{4}-[0-9]{2}-[0-9]{2}[T ][0-9]{2}:[0-9]{2}:[0-9]{2}/TIMESTAMP/g' | \
        sed -E 's/ERROR\[[0-9]{2}:[0-9]{2}:[0-9]{2}\]/ERROR[TIMESTAMP]/g' | \
        sed -E 's/in [0-9]+s/in Xs/g' | \
        sed -E 's/[a-f0-9]{32,}/PRIVATE_KEY/g' | \
        sed -E 's/  [⠋⠙⠹⠸⠼⠴⠦⠧⠇⠏]//g' | \
        sed -E 's/Docker[[:space:]]+[⠋⠙⠹⠸⠼⠴⠦⠧⠇⠏][[:space:]]+Docker/Docker/g' | \
        sed 's/[[:space:]]*$//' \
        > "$output_file" || true
}

echo "Running scenarios..."
run_scenario "happy_path_localhost" "localhost\ny"
run_scenario "invalid_domain_spaces" "my domain.com\nlocalhost\ny"
run_scenario "invalid_domain_http" "http://example.com\nlocalhost\ny"
run_scenario "invalid_domain_https" "https://example.com\nlocalhost\ny"
run_scenario "user_cancels" "localhost\nn"
run_scenario "empty_domain" "\nlocalhost\ny"
run_scenario "subdomain_format" "analytics.example.com\ny"
echo "  ✓ All scenarios completed"
echo

# Compare each snapshot
echo "Comparing snapshots..."
FAILED=0
PASSED=0

for golden in "$SNAPSHOT_DIR"/*.golden; do
    name=$(basename "$golden" .golden)
    actual="$ACTUAL_DIR/${name}.actual"

    if [ ! -f "$actual" ]; then
        echo "  ✗ $name: No actual output generated"
        FAILED=$((FAILED + 1))
        continue
    fi

    if diff -q "$golden" "$actual" > /dev/null 2>&1; then
        echo "  ✓ $name"
        PASSED=$((PASSED + 1))
    else
        echo "  ✗ $name: MISMATCH"
        echo
        echo "    Diff (expected vs actual):"
        diff --color=always -u "$golden" "$actual" | head -50 | sed 's/^/    /'
        echo
        FAILED=$((FAILED + 1))
    fi
done

echo
echo "========================================="
echo "Results: $PASSED passed, $FAILED failed"
echo "========================================="

if [ $FAILED -gt 0 ]; then
    echo
    echo "To see full diff for a specific scenario:"
    echo "  diff $SNAPSHOT_DIR/<name>.golden $ACTUAL_DIR/<name>.actual"
    echo
    echo "To update snapshots after intentional changes:"
    echo "  ./scripts/snapshot-parity.sh"
    exit 1
else
    echo
    echo "All snapshots match! UI parity maintained."
    exit 0
fi
