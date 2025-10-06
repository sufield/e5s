#!/bin/bash
set -e

echo "============================================"
echo "Registering Test Workload in SPIRE"
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

# Get SPIRE server pod
info "Finding SPIRE server pod..."
SERVER_POD=$(kubectl get pods -n spire-system -l app.kubernetes.io/name=server -o jsonpath='{.items[0].metadata.name}' 2>/dev/null)
if [ -z "$SERVER_POD" ]; then
    error "SPIRE server pod not found"
    exit 1
fi
success "Found SPIRE server: $SERVER_POD"

# Get node name for the test pod
info "Getting node information..."
NODE_NAME=$(kubectl get nodes -o jsonpath='{.items[0].metadata.name}')
success "Node: $NODE_NAME"

# Get agent SPIFFE ID from JSON output
info "Getting agent SPIFFE ID..."
AGENT_JSON=$(kubectl exec -n spire-system "$SERVER_POD" -c spire-server -- \
  /opt/spire/bin/spire-server agent list -output json 2>/dev/null)

# Parse trust_domain and path from JSON
TRUST_DOMAIN=$(echo "$AGENT_JSON" | grep -o '"trust_domain":"[^"]*"' | head -1 | cut -d'"' -f4)
AGENT_PATH=$(echo "$AGENT_JSON" | grep -o '"path":"[^"]*"' | head -1 | cut -d'"' -f4)

if [ -z "$TRUST_DOMAIN" ] || [ -z "$AGENT_PATH" ]; then
    error "Could not parse agent SPIFFE ID from JSON"
    exit 1
fi

AGENT_SPIFFE_ID="spiffe://${TRUST_DOMAIN}${AGENT_PATH}"
success "Agent SPIFFE ID: $AGENT_SPIFFE_ID"

# Create workload registration entry (idempotent - will skip if already exists)
info "Registering workload (or verifying existing registration)..."

# Try to create entry - capture output to check result
kubectl exec -n spire-system "$SERVER_POD" -c spire-server -- \
  /opt/spire/bin/spire-server entry create \
    -spiffeID "spiffe://example.org/test/integration-test" \
    -parentID "$AGENT_SPIFFE_ID" \
    -selector "k8s:ns:spire-system" \
    -selector "k8s:pod-name:spire-integration-test" \
    -x509SVIDTTL 3600 2>&1 | tee /tmp/spire-entry-create.log >/dev/null || true

# Check if entry was created or already exists
if grep -q "AlreadyExists" /tmp/spire-entry-create.log; then
  info "Workload already registered (OK)"
elif grep -qE "Entry ID.*[a-f0-9-]{36}" /tmp/spire-entry-create.log; then
  success "Workload registered successfully"
else
  error "Unexpected error during registration"
  cat /tmp/spire-entry-create.log
  rm -f /tmp/spire-entry-create.log
  exit 1
fi
rm -f /tmp/spire-entry-create.log

# List all entries to verify
info "Listing all registration entries..."
kubectl exec -n spire-system "$SERVER_POD" -c spire-server -- \
  /opt/spire/bin/spire-server entry show

echo ""
success "Test workload registered successfully!"
info "Now run: make test-integration"
