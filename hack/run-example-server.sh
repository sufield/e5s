#!/bin/bash
# Run the e5s example server in either normal or debug mode.
#
# Usage:
#   ./scripts/run-example-server.sh              # Normal mode (multi-threaded)
#   ./scripts/run-example-server.sh --debug      # Debug mode (single-threaded)
#
# This script only orchestrates: sets environment, paths, and runs the Go binary.
# All server logic lives in examples/basic-server.

set -euo pipefail

cd "$(dirname "$0")/.."

# Parse mode
MODE="normal"
if [[ "${1:-}" == "--debug" ]]; then
    MODE="debug"
fi

# Config paths
CONFIG="examples/highlevel/e5s-server.yaml"

echo "========================================"
echo "e5s Example Server"
echo "========================================"
echo "Mode: $MODE"
echo "Config: $CONFIG"
echo ""

if [[ "$MODE" == "debug" ]]; then
    echo "Running in SINGLE-THREADED mode for debugging"
    echo "This eliminates e5s's HTTP server goroutine."
    echo "Useful for:"
    echo "  - Step debugging without goroutine interference"
    echo "  - Race detector with GOMAXPROCS=1"
    echo "  - Isolating concurrency issues"
    echo ""

    # Set debug mode environment variable
    export E5S_DEBUG_SINGLE_THREAD=1
fi

# Run server (go run compiles and executes)
exec go run ./examples/basic-server -config "$CONFIG"
