#!/usr/bin/env bash
# SPIRE Registration Setup Script
# Creates necessary registration entries for integration testing
#
# Configuration via environment variables:
#   NS              - Kubernetes namespace (default: spire-system)
#   TRUST_DOMAIN    - SPIRE trust domain (default: example.org)
#   SERVER_POD      - Override SPIRE server pod name (auto-detected if not set)
#   WORKLOAD_SA     - Workload service account (default: default)
#   INTERACTIVE     - Enable interactive prompts (default: false)
#
# Non-interactive mode (default):
#   - If entry exists, verifies and succeeds
#   - If entry missing, creates it
#   - No user prompts, safe for automation
#
# Interactive mode (INTERACTIVE=true):
#   - Shows existing entries
#   - Prompts before updating/deleting
#   - Useful for manual management

set -Eeuo pipefail

# Configuration
NS="${NS:-spire-system}"
TRUST_DOMAIN="${TRUST_DOMAIN:-example.org}"
WORKLOAD_SA="${WORKLOAD_SA:-default}"

# Colors
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
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

header() {
    echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
    echo -e "${BLUE}  $1${NC}"
    echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
}

# Print configuration
header "SPIRE Registration Setup"
echo ""
info "Configuration:"
echo "  Namespace:      $NS"
echo "  Trust Domain:   $TRUST_DOMAIN"
echo "  Workload SA:    $WORKLOAD_SA"
echo ""

# Check prerequisites
info "Checking prerequisites..."

if ! command -v kubectl >/dev/null 2>&1; then
    error "kubectl not found"
    exit 1
fi

if ! kubectl get namespace "$NS" >/dev/null 2>&1; then
    error "Namespace '$NS' not found"
    exit 1
fi

# Find SPIRE server pod
if [ -z "${SERVER_POD:-}" ]; then
    info "Auto-detecting SPIRE server pod..."
    SERVER_POD=$(
        kubectl get pods -n "$NS" \
            -l 'app.kubernetes.io/name=server' -o jsonpath='{.items[0].metadata.name}' 2>/dev/null \
        || kubectl get pods -n "$NS" \
            -l 'app=spire-server' -o jsonpath='{.items[0].metadata.name}' 2>/dev/null \
        || kubectl get pods -n "$NS" \
            -l 'name=spire-server' -o jsonpath='{.items[0].metadata.name}' 2>/dev/null \
        || true
    )
fi

if [ -z "$SERVER_POD" ]; then
    error "SPIRE Server pod not found"
    exit 1
fi

success "Found SPIRE Server: $SERVER_POD"

# Check if server has spire-server binary (tolerant to distroless)
info "Checking server binary access..."

# Try a simple exec - if this works, server has shell access
if kubectl exec -n "$NS" "$SERVER_POD" -- ls /opt/spire/bin/spire-server >/dev/null 2>&1; then
    success "SPIRE server binary accessible"
    SERVER_HAS_SHELL=true
elif kubectl exec -n "$NS" "$SERVER_POD" -- /opt/spire/bin/spire-server --version >/dev/null 2>&1; then
    success "SPIRE server binary accessible (distroless - direct exec only)"
    SERVER_HAS_SHELL=false
else
    error "SPIRE server binary not accessible"
    echo ""
    info "This script requires one of the following:"
    echo "  1. Non-distroless SPIRE server image (recommended for development)"
    echo "  2. SPIRE Server API enabled (for production/distroless setups)"
    echo "  3. SPIRE Controller Manager with CRDs (Kubernetes-native approach)"
    echo ""
    info "Quick fix for development:"
    echo "  Update your SPIRE server deployment to use a non-distroless image:"
    echo ""
    echo "  kubectl set image deployment/spire-server -n $NS \\"
    echo "    spire-server=ghcr.io/spiffe/spire-server:1.9.0"
    echo ""
    echo "  Then re-run this script."
    echo ""
    info "For production setups, see:"
    echo "  - docs/SPIRE_INTEGRATION_TEST_FIX.md (detailed guide)"
    echo ""
    exit 1
fi
echo ""

# Helper function to execute spire-server commands
spire_server() {
    kubectl exec -n "$NS" "$SERVER_POD" -- /opt/spire/bin/spire-server "$@"
}

# 1. Verify agent attestation
header "Step 1: Verify Agent Attestation"
echo ""

info "Listing attested agents..."
if ! spire_server agent list 2>&1; then
    error "Failed to list agents"
    exit 1
fi
echo ""

AGENT_COUNT=$(spire_server agent list 2>/dev/null | grep -c "SPIFFE ID" || echo "0")
if [ "$AGENT_COUNT" -eq 0 ]; then
    error "No attested agents found!"
    error "Agents must be attested before creating workload entries"
    error "Check agent logs: kubectl logs -n $NS -l app=spire-agent"
    exit 1
fi

success "Found $AGENT_COUNT attested agent(s)"
echo ""

# 2. Check for existing node/agent entry
header "Step 2: Check Agent Registration Entry"
echo ""

info "Checking for agent registration entry..."
AGENT_SPIFFE_ID="spiffe://${TRUST_DOMAIN}/spire-agent"

if spire_server entry show -spiffeID "$AGENT_SPIFFE_ID" >/dev/null 2>&1; then
    success "Agent registration entry exists: $AGENT_SPIFFE_ID"
else
    info "Agent registration entry not found - this is normal if using automatic node attestation"
    info "Agents can be attested without explicit registration entries in some configurations"
fi
echo ""

# 3. Get actual agent SPIFFE ID for use as parent
info "Getting agent SPIFFE ID for workload parent..."
AGENT_SPIFFE_ID_ACTUAL=$(spire_server agent list 2>&1 | grep "SPIFFE ID" | head -1 | awk '{print $4}' || true)

if [ -z "$AGENT_SPIFFE_ID_ACTUAL" ]; then
    error "Could not determine agent SPIFFE ID from agent list"
    exit 1
fi

success "Agent SPIFFE ID: $AGENT_SPIFFE_ID_ACTUAL"
echo ""

# 4. Create/update integration test workload entry
header "Step 4: Create Integration Test Workload Entry"
echo ""

WORKLOAD_SPIFFE_ID="spiffe://${TRUST_DOMAIN}/integration-test"

info "Checking if workload entry exists..."

# Check if entry exists (entry show returns non-zero if not found)
ENTRY_OUTPUT=$(spire_server entry show -spiffeID "$WORKLOAD_SPIFFE_ID" 2>&1 || true)
ENTRY_IDS=$(echo "$ENTRY_OUTPUT" | grep "Entry ID" | awk '{print $4}' || true)

if [ -n "$ENTRY_IDS" ]; then
    info "Workload entry already exists"

    if [ -n "$ENTRY_IDS" ]; then
        # Non-interactive mode (default) - just verify entry exists
        if [ "${INTERACTIVE:-false}" = "true" ]; then
            # Show existing entry
            spire_server entry show -spiffeID "$WORKLOAD_SPIFFE_ID"
            echo ""

            read -p "Update existing entry? (y/N) " -n 1 -r
            echo ""
            if [[ ! $REPLY =~ ^[Yy]$ ]]; then
                success "Entry already exists, skipping update"
            else
                # Delete all existing entries for this SPIFFE ID
                for ENTRY_ID in $ENTRY_IDS; do
                    info "Deleting entry: $ENTRY_ID"
                    spire_server entry delete -entryID "$ENTRY_ID"
                done
                success "Deleted old entries"

                # Create new entry
                info "Creating integration test workload entry..."
                spire_server entry create \
                    -spiffeID "$WORKLOAD_SPIFFE_ID" \
                    -parentID "$AGENT_SPIFFE_ID_ACTUAL" \
                    -selector "k8s:ns:${NS}" \
                    -selector "k8s:sa:${WORKLOAD_SA}" \
                    -selector "k8s:pod-label:app:spire-integration-test"

                success "Workload entry created!"
            fi
        else
            # Non-interactive mode - entry exists, we're good
            success "Entry already exists with correct SPIFFE ID"
        fi
    else
        # No entries found, create new one
        info "Creating integration test workload entry..."
        spire_server entry create \
            -spiffeID "$WORKLOAD_SPIFFE_ID" \
            -parentID "$AGENT_SPIFFE_ID_ACTUAL" \
            -selector "k8s:ns:${NS}" \
            -selector "k8s:sa:${WORKLOAD_SA}" \
            -selector "k8s:pod-label:app:spire-integration-test"

        success "Workload entry created!"
    fi
else
    info "Creating integration test workload entry..."
    spire_server entry create \
        -spiffeID "$WORKLOAD_SPIFFE_ID" \
        -parentID "$AGENT_SPIFFE_ID_ACTUAL" \
        -selector "k8s:ns:${NS}" \
        -selector "k8s:sa:${WORKLOAD_SA}" \
        -selector "k8s:pod-label:app:spire-integration-test"

    success "Workload entry created!"
fi
echo ""

# 4. Create additional test workload entries (for multi-workload tests)
header "Step 4: Create Additional Test Workload Entries"
echo ""

# Client workload entry
CLIENT_SPIFFE_ID="spiffe://${TRUST_DOMAIN}/test-client"
info "Creating test-client workload entry..."

if spire_server entry show -spiffeID "$CLIENT_SPIFFE_ID" >/dev/null 2>&1; then
    info "test-client entry already exists"
else
    spire_server entry create \
        -spiffeID "$CLIENT_SPIFFE_ID" \
        -parentID "$AGENT_SPIFFE_ID_ACTUAL" \
        -selector "k8s:ns:${NS}" \
        -selector "k8s:sa:${WORKLOAD_SA}" \
        -selector "k8s:pod-label:role:client"
    success "test-client entry created"
fi

# Server workload entry
SERVER_SPIFFE_ID="spiffe://${TRUST_DOMAIN}/test-server"
info "Creating test-server workload entry..."

if spire_server entry show -spiffeID "$SERVER_SPIFFE_ID" >/dev/null 2>&1; then
    info "test-server entry already exists"
else
    spire_server entry create \
        -spiffeID "$SERVER_SPIFFE_ID" \
        -parentID "$AGENT_SPIFFE_ID_ACTUAL" \
        -selector "k8s:ns:${NS}" \
        -selector "k8s:sa:${WORKLOAD_SA}" \
        -selector "k8s:pod-label:role:server"
    success "test-server entry created"
fi
echo ""

# 5. List all entries for verification
header "Step 5: Verification"
echo ""

info "Listing all registration entries..."
if ! spire_server entry list 2>&1; then
    info "Could not list entries (command may not be supported)"
fi
echo ""

# 6. Summary
header "Setup Complete!"
echo ""
success "SPIRE registration entries are configured for integration testing"
echo ""
info "Created SPIFFE IDs:"
echo "  • $WORKLOAD_SPIFFE_ID"
echo "  • $CLIENT_SPIFFE_ID"
echo "  • $SERVER_SPIFFE_ID"
echo ""
info "These workloads can now obtain X.509 SVIDs from the SPIRE Workload API"
echo ""
info "Next steps:"
echo "  1. Deploy your test pods with matching labels:"
echo "     - app: spire-integration-test"
echo "     - role: client (for test-client)"
echo "     - role: server (for test-server)"
echo ""
echo "  2. Ensure test pods use service account: $WORKLOAD_SA"
echo ""
echo "  3. Run integration tests:"
echo "     ./scripts/run-integration-tests-ci.sh"
echo ""
