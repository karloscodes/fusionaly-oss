#!/usr/bin/env bash
set -euo pipefail

# Production Load Test Script for Fusionaly
# Inspired by analytics-bench Rails version, adapted for Go/Fusionaly
#
# Usage:
#   ./run_production_loadtest.sh                    # Run all scenarios
#   SCENARIOS="heavy extreme" ./run_production_loadtest.sh  # Run specific scenarios
#
# Environment Variables:
#   SCENARIOS    - Space-separated list: light medium heavy extreme (default: all)
#   SERVER_URL   - Override server URL (default: http://localhost:3000)
#   SKIP_BUILD   - Set to true to skip building (default: false)
#   SKIP_MIGRATE - Set to true to skip migrations (default: false)

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

# Configuration
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
PERFTEST_BIN="$PROJECT_ROOT/cmd/tools/perftest/main.go"
SERVER_BIN="$PROJECT_ROOT/cmd/fusionaly/main.go"
RESULTS_DIR="$PROJECT_ROOT/tmp/loadtest-results"
SERVER_URL="${SERVER_URL:-http://localhost:3000}"
SKIP_BUILD="${SKIP_BUILD:-false}"
SKIP_MIGRATE="${SKIP_MIGRATE:-false}"

# Test scenarios: concurrency duration rate
SCENARIOS="${SCENARIOS:-light medium heavy extreme}"
LIGHT_SCENARIO="10 30s 50"      # 10 concurrent, 30s, 50 events/sec
MEDIUM_SCENARIO="50 60s 200"    # 50 concurrent, 60s, 200 events/sec
HEAVY_SCENARIO="100 90s 500"    # 100 concurrent, 90s, 500 events/sec
EXTREME_SCENARIO="200 60s 1000" # 200 concurrent, 60s, 1000 events/sec

# Process IDs
SERVER_PID=""

# Cleanup function
cleanup() {
    echo ""
    echo -e "${YELLOW}Cleaning up...${NC}"

    if [ ! -z "$SERVER_PID" ]; then
        echo "Stopping server (PID: $SERVER_PID)..."
        kill $SERVER_PID 2>/dev/null || true
        wait $SERVER_PID 2>/dev/null || true
    fi

    echo -e "${GREEN}Cleanup complete${NC}"
}

# Print configuration
print_config() {
    echo -e "${BLUE}╔════════════════════════════════════════════════════════╗${NC}"
    echo -e "${BLUE}║         Fusionaly Production Load Test                ║${NC}"
    echo -e "${BLUE}╠════════════════════════════════════════════════════════╣${NC}"
    echo -e "${BLUE}║${NC} Server URL:    $SERVER_URL"
    echo -e "${BLUE}║${NC} Scenarios:     $SCENARIOS"
    echo -e "${BLUE}║${NC} Results Dir:   $RESULTS_DIR"
    echo -e "${BLUE}║${NC}"
    echo -e "${BLUE}║${NC} Test Scenarios:"
    echo -e "${BLUE}║${NC}   Light:    $(echo $LIGHT_SCENARIO | awk '{print $1" concurrent, "$2" duration, "$3" req/s"}')"
    echo -e "${BLUE}║${NC}   Medium:   $(echo $MEDIUM_SCENARIO | awk '{print $1" concurrent, "$2" duration, "$3" req/s"}')"
    echo -e "${BLUE}║${NC}   Heavy:    $(echo $HEAVY_SCENARIO | awk '{print $1" concurrent, "$2" duration, "$3" req/s"}')"
    echo -e "${BLUE}║${NC}   Extreme:  $(echo $EXTREME_SCENARIO | awk '{print $1" concurrent, "$2" duration, "$3" req/s"}')"
    echo -e "${BLUE}╚════════════════════════════════════════════════════════╝${NC}"
    echo ""
}

# Show usage
show_usage() {
    cat <<EOF
Fusionaly Production Load Test Script

Usage:
  $0 [options]

Options:
  -h, --help              Show this help message
  -u, --url URL           Server URL (default: http://localhost:3000)
  -s, --skip-build        Skip building binaries
  -m, --skip-migrate      Skip database migrations

Environment Variables:
  SCENARIOS               Scenarios to run: light medium heavy extreme
                         (default: "light medium heavy extreme")
  SERVER_URL             Override server URL
  SKIP_BUILD             Set to true to skip building
  SKIP_MIGRATE           Set to true to skip migrations

Test Scenarios:
  light    - Light load:    10 concurrent, 30s, 50 req/s
  medium   - Medium load:   50 concurrent, 60s, 200 req/s
  heavy    - Heavy load:    100 concurrent, 90s, 500 req/s
  extreme  - Extreme load:  200 concurrent, 60s, 1000 req/s

Examples:
  $0                                    # Run all scenarios
  SCENARIOS="heavy extreme" $0          # Run only heavy and extreme
  SERVER_URL=http://prod:3000 $0        # Test production server
  SKIP_BUILD=true $0                    # Skip build, use existing binaries

Output:
  Results: $RESULTS_DIR
EOF
}

# Check dependencies
check_dependencies() {
    echo -e "${YELLOW}Checking dependencies...${NC}"

    if ! command -v go &> /dev/null; then
        echo -e "${RED}Error: go not found${NC}"
        exit 1
    fi

    # Check Go version
    GO_VERSION=$(go version | awk '{print $3}' | sed 's/go//')
    echo "Go version: $GO_VERSION"

    echo -e "${GREEN}Dependencies OK${NC}"
    echo ""
}

# Build binaries
build_binaries() {
    if [ "$SKIP_BUILD" = "true" ]; then
        echo -e "${YELLOW}Skipping build (SKIP_BUILD=true)${NC}"
        echo ""
        return
    fi

    echo -e "${YELLOW}Building binaries...${NC}"
    
    cd "$PROJECT_ROOT"
    
    # Build server
    echo "Building server..."
    go build -o tmp/fusionaly cmd/fusionaly/main.go
    
    # Build perftest
    echo "Building perftest..."
    go build -o tmp/perftest cmd/tools/perftest/main.go
    
    echo -e "${GREEN}Binaries built successfully${NC}"
    echo ""
}

# Prepare test environment
prepare_environment() {
    echo -e "${YELLOW}Preparing test environment...${NC}"
    
    # Create results directory
    mkdir -p "$RESULTS_DIR"
    
    # Database setup
    if [ "$SKIP_MIGRATE" != "true" ]; then
        echo "Running database migrations..."
        cd "$PROJECT_ROOT"
        make db-migrate || {
            echo -e "${RED}Migration failed${NC}"
            exit 1
        }
    else
        echo -e "${YELLOW}Skipping migrations (SKIP_MIGRATE=true)${NC}"
    fi
    
    echo -e "${GREEN}Environment ready${NC}"
    echo ""
}

# Start server
start_server() {
    echo -e "${YELLOW}Starting Fusionaly server...${NC}"
    
    cd "$PROJECT_ROOT"
    
    # Start server in background
    ./tmp/fusionaly > "$RESULTS_DIR/server.log" 2>&1 &
    SERVER_PID=$!
    
    echo "Server starting (PID: $SERVER_PID)..."
    echo "Server logs: $RESULTS_DIR/server.log"
    
    # Wait for server to be ready
    echo -n "Waiting for server"
    for i in {1..30}; do
        if curl -s "$SERVER_URL/health" > /dev/null 2>&1; then
            echo ""
            echo -e "${GREEN}Server is ready!${NC}"
            echo ""
            return 0
        fi
        echo -n "."
        sleep 1
    done
    
    echo ""
    echo -e "${RED}Server failed to start. Check $RESULTS_DIR/server.log${NC}"
    tail -20 "$RESULTS_DIR/server.log"
    exit 1
}

# Run single test scenario
run_single_test() {
    local scenario_name="$1"
    local concurrency="$2"
    local duration="$3"
    local rate="$4"
    
    echo -e "${BLUE}╔════════════════════════════════════════════════════════╗${NC}"
    echo -e "${BLUE}║  Running ${scenario_name} Scenario${NC}"
    echo -e "${BLUE}╠════════════════════════════════════════════════════════╣${NC}"
    echo -e "${BLUE}║${NC} Concurrency: $concurrency"
    echo -e "${BLUE}║${NC} Duration:    $duration"
    echo -e "${BLUE}║${NC} Rate:        $rate events/sec"
    echo -e "${BLUE}╚════════════════════════════════════════════════════════╝${NC}"
    echo ""
    
    local result_file="$RESULTS_DIR/${scenario_name}_results.json"
    local log_file="$RESULTS_DIR/${scenario_name}_test.log"
    
    # Run the test
    cd "$PROJECT_ROOT"
    ./tmp/perftest \
        -url "$SERVER_URL" \
        -c "$concurrency" \
        -d "$duration" \
        -rate "$rate" \
        -timeout 30s 2>&1 | tee "$log_file"
    
    # Move results to scenario-specific file
    if [ -f "perf_results.json" ]; then
        mv perf_results.json "$result_file"
        echo ""
        echo -e "${GREEN}Results saved to: $result_file${NC}"
    fi
    
    # Brief pause between tests
    echo ""
    echo -e "${YELLOW}Cooling down for 5 seconds...${NC}"
    sleep 5
    echo ""
}

# Run all test scenarios
run_tests() {
    echo ""
    echo -e "${BLUE}╔════════════════════════════════════════════════════════╗${NC}"
    echo -e "${BLUE}║           Starting Performance Tests                   ║${NC}"
    echo -e "${BLUE}╚════════════════════════════════════════════════════════╝${NC}"
    echo ""
    
    # Run each scenario
    for scenario in $SCENARIOS; do
        case $scenario in
            "light")
                run_single_test "LIGHT" $(echo $LIGHT_SCENARIO)
                ;;
            "medium")
                run_single_test "MEDIUM" $(echo $MEDIUM_SCENARIO)
                ;;
            "heavy")
                run_single_test "HEAVY" $(echo $HEAVY_SCENARIO)
                ;;
            "extreme")
                run_single_test "EXTREME" $(echo $EXTREME_SCENARIO)
                ;;
            *)
                echo -e "${YELLOW}Unknown scenario: $scenario${NC}"
                ;;
        esac
    done
}

# Show final results
show_results() {
    echo ""
    echo -e "${GREEN}╔════════════════════════════════════════════════════════╗${NC}"
    echo -e "${GREEN}║              LOAD TEST SUMMARY                         ║${NC}"
    echo -e "${GREEN}╠════════════════════════════════════════════════════════╣${NC}"
    
    # Parse and display key metrics from each test
    for scenario in $SCENARIOS; do
        local scenario_upper=$(echo "$scenario" | tr '[:lower:]' '[:upper:]')
        local result_file="$RESULTS_DIR/${scenario_upper}_results.json"
        
        if [ -f "$result_file" ]; then
            echo -e "${GREEN}║${NC}"
            echo -e "${GREEN}║${NC} ${BLUE}$scenario_upper Scenario:${NC}"
            
            # Extract key metrics using jq if available, otherwise basic parsing
            if command -v jq &> /dev/null; then
                local total_req=$(jq -r '.summary.totalRequests' "$result_file")
                local success_rate=$(jq -r '.summary.successRate' "$result_file")
                local rps=$(jq -r '.summary.requestsPerSecond' "$result_file")
                local p50=$(jq -r '.summary.p50LatencyMs' "$result_file")
                local p95=$(jq -r '.summary.p95LatencyMs' "$result_file")
                
                echo -e "${GREEN}║${NC}   Total Requests:    $total_req"
                echo -e "${GREEN}║${NC}   Success Rate:      ${success_rate}%"
                echo -e "${GREEN}║${NC}   Requests/sec:      ${rps}"
                echo -e "${GREEN}║${NC}   P50 Latency:       ${p50}ms"
                echo -e "${GREEN}║${NC}   P95 Latency:       ${p95}ms"
            else
                echo -e "${GREEN}║${NC}   Results file: $result_file"
            fi
        fi
    done
    
    echo -e "${GREEN}╠════════════════════════════════════════════════════════╣${NC}"
    echo -e "${GREEN}║${NC} Results Directory: $RESULTS_DIR"
    echo -e "${GREEN}║${NC} Server Logs:       $RESULTS_DIR/server.log"
    echo -e "${GREEN}╚════════════════════════════════════════════════════════╝${NC}"
    echo ""
    
    # Show database stats
    echo -e "${BLUE}Database Statistics:${NC}"
    cd "$PROJECT_ROOT"
    ./tmp/fusionaly -c "SELECT COUNT(*) FROM events" 2>/dev/null || echo "  Unable to query database"
    
    echo ""
    echo -e "${GREEN}All tests completed successfully!${NC}"
    echo ""
    echo -e "${YELLOW}TIP: View detailed results:${NC}"
    echo "  cat $RESULTS_DIR/LIGHT_results.json | jq '.summary'"
    echo "  cat $RESULTS_DIR/server.log | tail -50"
}

# Parse command line arguments
while [[ $# -gt 0 ]]; do
    case $1 in
        -h|--help)
            show_usage
            exit 0
            ;;
        -u|--url)
            SERVER_URL="$2"
            shift 2
            ;;
        -s|--skip-build)
            SKIP_BUILD=true
            shift
            ;;
        -m|--skip-migrate)
            SKIP_MIGRATE=true
            shift
            ;;
        *)
            echo "Unknown option: $1"
            echo "Use -h or --help for usage information"
            exit 1
            ;;
    esac
done

# Main execution
main() {
    # Set trap for cleanup on exit
    trap cleanup EXIT
    
    print_config
    check_dependencies
    build_binaries
    prepare_environment
    start_server
    
    echo -e "${GREEN}System ready. Starting load tests...${NC}"
    echo ""
    sleep 2
    
    run_tests
    
    # Brief pause for final stats
    echo -e "${YELLOW}Collecting final statistics...${NC}"
    sleep 3
    
    show_results
}

# Run main function
main
