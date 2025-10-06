#!/bin/bash
set -e

echo "============================================"
echo "SPIRE Integration Tests (Minikube)"
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

# Check Minikube is running
info "Checking Minikube status..."
if ! minikube status >/dev/null 2>&1; then
    error "Minikube is not running"
    echo "Run: make minikube-up"
    exit 1
fi
success "Minikube is running"

# Check SPIRE is deployed
info "Checking SPIRE deployment..."
if ! kubectl get pods -n spire-system >/dev/null 2>&1; then
    error "SPIRE is not deployed"
    echo "Run: make minikube-up"
    exit 1
fi

# Check SPIRE Agent is ready
AGENT_POD=$(kubectl get pods -n spire-system -l app=spire-agent -o jsonpath='{.items[0].metadata.name}' 2>/dev/null)
if [ -z "$AGENT_POD" ]; then
    error "SPIRE Agent pod not found"
    exit 1
fi

AGENT_STATUS=$(kubectl get pod -n spire-system "$AGENT_POD" -o jsonpath='{.status.phase}' 2>/dev/null)
if [ "$AGENT_STATUS" != "Running" ]; then
    error "SPIRE Agent is not running (status: $AGENT_STATUS)"
    exit 1
fi
success "SPIRE Agent is ready: $AGENT_POD"

# Check socket exists in Minikube
info "Checking SPIRE Agent socket in Minikube..."
if minikube ssh "test -S /tmp/spire-agent/public/api.sock" 2>/dev/null; then
    success "Socket exists in Minikube: /tmp/spire-agent/public/api.sock"
else
    error "Socket not found in Minikube"
    echo "Expected: /tmp/spire-agent/public/api.sock"
    exit 1
fi

echo ""
info "Running integration tests inside Minikube..."
echo ""

# Create a test script to run inside Minikube
TEST_SCRIPT=$(cat <<'EOF'
#!/bin/bash
set -e

# Set environment for tests
export SPIRE_AGENT_SOCKET="unix:///tmp/spire-agent/public/api.sock"
export SPIRE_TRUST_DOMAIN="example.org"

# Change to project directory
cd /mnt/spire

# Run integration tests
go test -tags=integration -v ./internal/adapters/outbound/spire/...
EOF
)

# Copy project to Minikube and run tests
info "Copying project to Minikube..."
minikube ssh "sudo rm -rf /tmp/spire-test && sudo mkdir -p /tmp/spire-test"
minikube mount "$(pwd):/mnt/spire" --9p-version=9p2000.L &
MOUNT_PID=$!
sleep 2

# Run tests inside Minikube
info "Executing tests..."
if minikube ssh "cd /mnt/spire && SPIRE_AGENT_SOCKET=unix:///tmp/spire-agent/public/api.sock SPIRE_TRUST_DOMAIN=example.org go test -tags=integration -v ./internal/adapters/outbound/spire/..." 2>&1; then
    success "Integration tests passed!"
    EXIT_CODE=0
else
    error "Integration tests failed"
    EXIT_CODE=1
fi

# Cleanup
kill $MOUNT_PID 2>/dev/null || true

exit $EXIT_CODE
