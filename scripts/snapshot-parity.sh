#!/bin/bash
# Snapshot Parity Test Script
# Ensures matcha integration maintains UI parity with current installer

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"
SNAPSHOT_DIR="$PROJECT_ROOT/internal/manager/snapshot/testdata"
BINARY_PATH="/tmp/fusionaly-snapshot-test"

cd "$PROJECT_ROOT"

echo "========================================="
echo "Fusionaly Installer Snapshot Parity Test"
echo "========================================="
echo

# Step 1: Build current binary
echo "Step 1: Building current installer..."
go build -o "$BINARY_PATH" ./cmd/manager
echo "  ✓ Binary built at $BINARY_PATH"
echo

# Step 2: Capture snapshots for all scenarios
echo "Step 2: Capturing current UI snapshots..."
mkdir -p "$SNAPSHOT_DIR"

run_scenario() {
    local name=$1
    shift
    local inputs="$@"
    local output_file="$SNAPSHOT_DIR/${name}.golden"

    echo "  Running scenario: $name"

    # Run with test environment
    echo -e "$inputs" | ENV=test SKIP_PORT_CHECKING=1 timeout 30 "$BINARY_PATH" install 2>&1 | \
        # Remove ANSI codes and normalize
        sed 's/\x1b\[[0-9;]*[mK]//g' | \
        sed 's/\r//g' | \
        # Normalize IPs
        sed -E 's/[0-9]+\.[0-9]+\.[0-9]+\.[0-9]+/X.X.X.X/g' | \
        # Normalize timestamps
        sed -E 's/[0-9]{4}-[0-9]{2}-[0-9]{2}[T ][0-9]{2}:[0-9]{2}:[0-9]{2}/TIMESTAMP/g' | \
        # Normalize ERROR timestamps like ERROR[22:37:59]
        sed -E 's/ERROR\[[0-9]{2}:[0-9]{2}:[0-9]{2}\]/ERROR[TIMESTAMP]/g' | \
        # Normalize durations
        sed -E 's/in [0-9]+s/in Xs/g' | \
        # Normalize private keys (long hex strings)
        sed -E 's/[a-f0-9]{32,}/PRIVATE_KEY/g' | \
        # Remove spinner animation characters
        sed -E 's/  [⠋⠙⠹⠸⠼⠴⠦⠧⠇⠏]//g' | \
        # Remove spinner artifacts like "Docker          ⠋  Docker"
        sed -E 's/Docker[[:space:]]+[⠋⠙⠹⠸⠼⠴⠦⠧⠇⠏][[:space:]]+Docker/Docker/g' | \
        # Remove trailing whitespace
        sed 's/[[:space:]]*$//' \
        > "$output_file" || true

    echo "    ✓ Saved to ${name}.golden"
}

# Run all scenarios
run_scenario "happy_path_localhost" "localhost\ny"
run_scenario "invalid_domain_spaces" "my domain.com\nlocalhost\ny"
run_scenario "invalid_domain_http" "http://example.com\nlocalhost\ny"
run_scenario "invalid_domain_https" "https://example.com\nlocalhost\ny"
run_scenario "user_cancels" "localhost\nn"
run_scenario "empty_domain" "\nlocalhost\ny"
run_scenario "subdomain_format" "analytics.example.com\ny"

echo
echo "  ✓ All snapshots captured"
echo

# Count snapshots
SNAPSHOT_COUNT=$(ls -1 "$SNAPSHOT_DIR"/*.golden 2>/dev/null | wc -l | tr -d ' ')
echo "Step 3: Snapshot summary"
echo "  Total scenarios: $SNAPSHOT_COUNT"
echo "  Snapshot directory: $SNAPSHOT_DIR"
echo

# List snapshots with sizes
echo "  Snapshots:"
for f in "$SNAPSHOT_DIR"/*.golden; do
    if [ -f "$f" ]; then
        size=$(wc -l < "$f" | tr -d ' ')
        name=$(basename "$f" .golden)
        echo "    - $name ($size lines)"
    fi
done

echo
echo "========================================="
echo "Snapshots captured successfully!"
echo "========================================="
echo
echo "Next steps:"
echo "  1. Integrate matcha into cmd/manager/main.go"
echo "  2. Run: ./scripts/verify-parity.sh"
echo "  3. Fix any differences until all snapshots match"
