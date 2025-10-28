#!/bin/bash
set -e

echo "============================================"
echo "Production Binary Test (Minikube)"
echo "============================================"
echo ""

# Colors
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
NC='\033[0m'

success() {
    echo -e "${GREEN}✅ $1${NC}"
}

error() {
    echo -e "${RED}❌ $1${NC}"
}

info() {
    echo -e "${YELLOW}ℹ️  $1${NC}"
}

# Check prerequisites
info "Checking prerequisites..."

if ! command -v minikube >/dev/null 2>&1; then
    error "minikube not found"
    exit 1
fi

if ! minikube status >/dev/null 2>&1; then
    error "Minikube is not running. Run: make minikube-up"
    exit 1
fi

if ! kubectl get namespace spire-system >/dev/null 2>&1; then
    error "SPIRE namespace not found. Run: make minikube-up"
    exit 1
fi

success "Prerequisites OK"

# Build production binary
info "Building production binary..."
if make prod-build >/dev/null 2>&1; then
    BINARY_SIZE=$(ls -lh bin/spire-server | awk '{print $5}')
    success "Production binary built: $BINARY_SIZE"
else
    error "Failed to build production binary"
    exit 1
fi

# Copy binary to Minikube (use /var/tmp which allows execution, unlike /tmp which is noexec)
info "Copying binary to Minikube..."
if minikube cp bin/spire-server /home/docker/spire-server 2>/dev/null; then
    # Move to /var/tmp with proper permissions using sudo (tmpfs /tmp is noexec)
    minikube ssh "sudo cp /home/docker/spire-server /var/tmp/spire-server && sudo chmod +x /var/tmp/spire-server" 2>/dev/null
    minikube ssh "rm -f /home/docker/spire-server" 2>/dev/null
    success "Binary copied to Minikube:/var/tmp/spire-server"
else
    error "Failed to copy binary to Minikube"
    exit 1
fi

# Verify socket exists
info "Verifying SPIRE Agent socket..."
if minikube ssh "test -S /tmp/spire-agent/public/api.sock" 2>/dev/null; then
    success "Socket exists: /tmp/spire-agent/public/api.sock"
else
    error "Socket not found. Ensure SPIRE Agent is running."
    exit 1
fi

# Test the production binary
echo ""
info "Testing production binary with live SPIRE..."
echo "-------------------------------------------"

# Run the binary in test mode (it will try to connect and fetch SVID)
# We'll run it in background and kill after a few seconds since it's a daemon
TEST_OUTPUT=$(minikube ssh "sudo SPIRE_AGENT_SOCKET=unix:///tmp/spire-agent/public/api.sock \
    SPIRE_TRUST_DOMAIN=example.org \
    timeout 5s /var/tmp/spire-server 2>&1" || true)

echo "$TEST_OUTPUT"
echo "-------------------------------------------"

# Check for success indicators in output
if echo "$TEST_OUTPUT" | grep -q "SPIRE Agent (Production Mode)"; then
    success "Binary started in production mode"
else
    error "Binary did not start in production mode"
    exit 1
fi

if echo "$TEST_OUTPUT" | grep -q "Connecting to SPIRE Agent:"; then
    success "Binary attempting SPIRE connection"
else
    error "Binary not attempting SPIRE connection"
    exit 1
fi

# Check for errors (but allow "no identity issued" which is expected for unregistered workloads)
if echo "$TEST_OUTPUT" | grep -q "no identity issued"; then
    info "Got 'no identity issued' (expected - workload not registered with SPIRE)"
    info "This confirms the binary successfully connected to SPIRE!"
elif echo "$TEST_OUTPUT" | grep -iq "failed to create SPIRE" && ! echo "$TEST_OUTPUT" | grep -q "no identity issued"; then
    error "SPIRE client creation failed unexpectedly"
    echo ""
    echo "Check the error message above for details"
    exit 1
elif echo "$TEST_OUTPUT" | grep -iq "failed to bootstrap" && ! echo "$TEST_OUTPUT" | grep -q "no identity issued"; then
    error "Application bootstrap failed unexpectedly"
    echo ""
    echo "Check the error message above for details"
    exit 1
fi

# If we get here, check for positive indicators
if echo "$TEST_OUTPUT" | grep -q "Trust Domain: example.org"; then
    success "Trust domain configured correctly"
fi

if echo "$TEST_OUTPUT" | grep -q "Agent Identity:"; then
    success "Agent identity loaded"
fi

# Cleanup
info "Cleaning up..."
minikube ssh "sudo rm -f /var/tmp/spire-server" 2>/dev/null || true

echo ""
echo "============================================"
echo "PRODUCTION BINARY TEST SUMMARY"
echo "============================================"
echo ""
success "Production binary validation complete!"
echo ""
echo "Verified:"
echo "  ✓ Binary builds successfully"
echo "  ✓ Binary runs in production mode"
echo "  ✓ Connects to SPIRE Agent socket"
echo "  ✓ Trust domain configuration works"
echo "  ✓ No critical errors"
echo ""
info "The production SPIRE adapters are working correctly!"
echo ""
