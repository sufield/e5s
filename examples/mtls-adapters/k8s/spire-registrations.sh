#!/bin/bash
# Script to register SPIRE entries for the mTLS examples in Kubernetes

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Helper functions
error() {
    echo -e "${RED}[ERROR]${NC} $1" >&2
}

success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

info() {
    echo -e "${YELLOW}[INFO]${NC} $1"
}

# Check if kubectl is available
if ! command -v kubectl &> /dev/null; then
    error "kubectl not found. Please install kubectl."
    exit 1
fi

# Check if SPIRE server pod is running
info "Checking for SPIRE server..."
SERVER_POD=$(kubectl get pods -n spire-system -l app.kubernetes.io/name=server -o jsonpath='{.items[0].metadata.name}' 2>/dev/null || echo "")

if [ -z "$SERVER_POD" ]; then
    error "SPIRE server pod not found. Please ensure SPIRE is deployed."
    exit 1
fi

success "Found SPIRE server: $SERVER_POD"

# Get agent SPIFFE ID
info "Getting agent SPIFFE ID..."
AGENT_JSON=$(kubectl exec -n spire-system "$SERVER_POD" -c spire-server -- \
  /opt/spire/bin/spire-server agent list -output json 2>/dev/null)

TRUST_DOMAIN=$(echo "$AGENT_JSON" | grep -o '"trust_domain":"[^"]*"' | head -1 | cut -d'"' -f4)
AGENT_PATH=$(echo "$AGENT_JSON" | grep -o '"path":"[^"]*"' | head -1 | cut -d'"' -f4)

if [ -z "$TRUST_DOMAIN" ] || [ -z "$AGENT_PATH" ]; then
    error "Failed to get agent information"
    exit 1
fi

AGENT_SPIFFE_ID="spiffe://${TRUST_DOMAIN}${AGENT_PATH}"
success "Agent SPIFFE ID: $AGENT_SPIFFE_ID"

# Register server workload
info "Registering server workload..."
SERVER_OUTPUT=$(kubectl exec -n spire-system "$SERVER_POD" -c spire-server -- \
  /opt/spire/bin/spire-server entry create \
    -spiffeID "spiffe://${TRUST_DOMAIN}/server" \
    -parentID "$AGENT_SPIFFE_ID" \
    -selector "k8s:ns:default" \
    -selector "k8s:pod-label:app:mtls-server" \
    -x509SVIDTTL 3600 2>&1)

if echo "$SERVER_OUTPUT" | grep -q "AlreadyExists"; then
    info "Server workload already registered"
elif echo "$SERVER_OUTPUT" | grep -qE "Entry ID.*[a-f0-9-]{36}"; then
    success "Server workload registered: spiffe://${TRUST_DOMAIN}/server"
else
    error "Failed to register server workload"
    echo "$SERVER_OUTPUT"
    exit 1
fi

# Register client workload
info "Registering client workload..."
CLIENT_OUTPUT=$(kubectl exec -n spire-system "$SERVER_POD" -c spire-server -- \
  /opt/spire/bin/spire-server entry create \
    -spiffeID "spiffe://${TRUST_DOMAIN}/client" \
    -parentID "$AGENT_SPIFFE_ID" \
    -selector "k8s:ns:default" \
    -selector "k8s:pod-label:app:mtls-client" \
    -x509SVIDTTL 3600 2>&1)

if echo "$CLIENT_OUTPUT" | grep -q "AlreadyExists"; then
    info "Client workload already registered"
elif echo "$CLIENT_OUTPUT" | grep -qE "Entry ID.*[a-f0-9-]{36}"; then
    success "Client workload registered: spiffe://${TRUST_DOMAIN}/client"
else
    error "Failed to register client workload"
    echo "$CLIENT_OUTPUT"
    exit 1
fi

# Verify registrations
info "Verifying registrations..."
ENTRIES=$(kubectl exec -n spire-system "$SERVER_POD" -c spire-server -- \
  /opt/spire/bin/spire-server entry show 2>/dev/null | grep -E "spiffe://${TRUST_DOMAIN}/(server|client)" || true)

if [ -z "$ENTRIES" ]; then
    error "No entries found. Registration may have failed."
    exit 1
fi

success "Registration complete!"
echo ""
info "Registered workloads:"
echo "$ENTRIES"
echo ""
info "You can now deploy the mTLS examples:"
echo "  kubectl apply -f examples/mtls-adapters/k8s/server-deployment.yaml"
echo "  kubectl apply -f examples/mtls-adapters/k8s/client-job.yaml"
