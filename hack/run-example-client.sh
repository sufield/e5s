#!/bin/bash
# Run the e5s example client to make a request to the example server.
#
# Usage:
#   ./scripts/run-example-client.sh [SERVER_URL]
#
# This script only orchestrates: sets environment, paths, and runs the Go binary.
# All client logic lives in cmd/example-client.

set -euo pipefail

cd "$(dirname "$0")/.."

# Config paths
APP_CONFIG="examples/highlevel/client-config.yaml"
E5S_CONFIG="examples/highlevel/e5s-client.yaml"

# Allow overriding server URL
SERVER_URL="${1:-https://localhost:8443/time}"

echo "========================================"
echo "e5s Example Client"
echo "========================================"
echo "App config: $APP_CONFIG"
echo "e5s config: $E5S_CONFIG"
echo "Server URL: $SERVER_URL"
echo ""

# Export SERVER_URL for the client to use
export SERVER_URL

# Run client (go run compiles and executes)
exec go run ./cmd/example-client \
    -app-config "$APP_CONFIG" \
    -e5s-config "$E5S_CONFIG"
