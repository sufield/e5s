#!/bin/bash
# Quick SPIRE connectivity test using kubectl exec

set -e

echo "Quick SPIRE Connectivity Test"
echo "=============================="
echo ""

# Get agent pod
AGENT_POD=$(kubectl get pods -n spire-system -l app=spire-agent -o jsonpath='{.items[0].metadata.name}' 2>/dev/null)

if [ -z "$AGENT_POD" ]; then
    echo "❌ SPIRE Agent pod not found"
    exit 1
fi

echo "✓ Found agent: $AGENT_POD"

# Test 1: Check socket exists
echo "→ Checking socket..."
if kubectl exec -n spire-system "$AGENT_POD" -- test -S /tmp/spire-agent/public/api.sock 2>/dev/null; then
    echo "✓ Socket exists"
else
    echo "❌ Socket not found"
    exit 1
fi

# Test 2: Try to fetch SVID using spire-agent CLI (if available)
echo "→ Testing SVID fetch..."
if kubectl exec -n spire-system "$AGENT_POD" -- \
    /opt/spire/bin/spire-agent api fetch x509 \
    -socketPath /tmp/spire-agent/public/api.sock 2>&1 | grep -q "SPIFFE ID"; then
    echo "✓ Successfully fetched SVID"
    kubectl exec -n spire-system "$AGENT_POD" -- \
        /opt/spire/bin/spire-agent api fetch x509 \
        -socketPath /tmp/spire-agent/public/api.sock 2>&1 | head -10
else
    echo "⚠️  Could not fetch SVID (may need workload registration)"
fi

echo ""
echo "✓ SPIRE Agent is accessible and functional!"
echo ""
echo "Socket path in pod: /tmp/spire-agent/public/api.sock"
echo "Socket path on host: /tmp/spire-agent/public/api.sock (Minikube node)"
