#!/usr/bin/env bash
# Run CI integration tests locally
# This script simulates the GitHub Actions workflow for local testing

set -e

echo "=========================================="
echo "Running CI Integration Tests Locally"
echo "=========================================="
echo

# Colors for output
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m' # No Color

# Change to repository root
cd "$(dirname "$0")/.."

echo -e "${YELLOW}Step 1: Starting SPIRE server in Docker...${NC}"
docker compose -f docker-compose.test.yml up -d spire-server

echo "Waiting for SPIRE server to be healthy..."
timeout 60 bash -c 'until docker exec spire-server-test /opt/spire/bin/spire-server healthcheck >/dev/null 2>&1; do sleep 2; done'
echo -e "${GREEN}✓ SPIRE server is healthy${NC}"
echo

echo -e "${YELLOW}Step 2: Checking for SPIRE agent binaries...${NC}"
SPIRE_VERSION="1.13.0"
SPIRE_DIR="./tmp/spire-${SPIRE_VERSION}"

if [ ! -d "$SPIRE_DIR" ]; then
    echo "Downloading SPIRE ${SPIRE_VERSION}..."
    mkdir -p ./tmp
    cd ./tmp
    wget -q "https://github.com/spiffe/spire/releases/download/v${SPIRE_VERSION}/spire-${SPIRE_VERSION}-linux-amd64-musl.tar.gz"
    tar xzf "spire-${SPIRE_VERSION}-linux-amd64-musl.tar.gz"
    rm "spire-${SPIRE_VERSION}-linux-amd64-musl.tar.gz"
    cd ..
    echo -e "${GREEN}✓ SPIRE binaries downloaded${NC}"
else
    echo -e "${GREEN}✓ SPIRE binaries already present${NC}"
fi
echo

echo -e "${YELLOW}Step 3: Configuring and starting SPIRE agent on host...${NC}"

# Create agent config
mkdir -p /tmp/spire-agent/data /tmp/spire-agent/public
cat > /tmp/spire-agent.conf <<EOF
agent {
    data_dir = "/tmp/spire-agent/data"
    log_level = "DEBUG"
    trust_domain = "example.org"
    server_address = "127.0.0.1"
    server_port = "8081"
    insecure_bootstrap = true
}

plugins {
    KeyManager "memory" {
        plugin_data {}
    }

    NodeAttestor "join_token" {
        plugin_data {}
    }

    WorkloadAttestor "unix" {
        plugin_data {}
    }
}
EOF

# Generate join token
echo "Generating join token..."
JOIN_TOKEN=$(docker exec spire-server-test /opt/spire/bin/spire-server token generate -spiffeID spiffe://example.org/agent | grep "Token:" | awk '{print $2}')
echo -e "${GREEN}✓ Join token generated${NC}"

# Start agent in background
echo "Starting SPIRE agent..."
"${SPIRE_DIR}/bin/spire-agent" run \
    -config /tmp/spire-agent.conf \
    -socketPath /tmp/spire-agent/public/api.sock \
    -joinToken "${JOIN_TOKEN}" > /tmp/spire-agent.log 2>&1 &
AGENT_PID=$!

# Wait for agent socket
echo "Waiting for agent socket..."
timeout 30 bash -c 'until [ -S /tmp/spire-agent/public/api.sock ]; do sleep 1; done'
echo -e "${GREEN}✓ SPIRE agent is running (PID: ${AGENT_PID})${NC}"
echo

# Cleanup function
cleanup() {
    echo
    echo -e "${YELLOW}Cleaning up...${NC}"
    if [ ! -z "$AGENT_PID" ]; then
        echo "Stopping SPIRE agent (PID: ${AGENT_PID})..."
        kill $AGENT_PID 2>/dev/null || true
        wait $AGENT_PID 2>/dev/null || true
    fi
    echo "Stopping Docker containers..."
    docker compose -f docker-compose.test.yml down
    echo -e "${GREEN}✓ Cleanup complete${NC}"
}

trap cleanup EXIT INT TERM

echo -e "${YELLOW}Step 4: Running integration tests...${NC}"
echo "=========================================="
export SPIFFE_ENDPOINT_SOCKET=unix:///tmp/spire-agent/public/api.sock

# Run the tests
if go test -v -tags=integration -timeout=5m ./spire; then
    echo
    echo -e "${GREEN}=========================================="
    echo "✓ All integration tests passed!"
    echo "==========================================${NC}"
    exit 0
else
    echo
    echo -e "${RED}=========================================="
    echo "✗ Integration tests failed"
    echo "==========================================${NC}"
    echo
    echo "Check logs at: /tmp/spire-agent.log"
    exit 1
fi
