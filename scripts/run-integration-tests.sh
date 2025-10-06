#!/bin/bash
set -e

echo "============================================"
echo "SPIRE Integration Tests (Kubernetes Pod)"
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

if ! command -v kubectl >/dev/null 2>&1; then
    error "kubectl not found"
    exit 1
fi

if ! kubectl get namespace spire-system >/dev/null 2>&1; then
    error "SPIRE namespace not found. Run: make minikube-up"
    exit 1
fi

# Get SPIRE Agent pod
AGENT_POD=$(kubectl get pods -n spire-system -l app.kubernetes.io/name=agent -o jsonpath='{.items[0].metadata.name}' 2>/dev/null)
if [ -z "$AGENT_POD" ]; then
    error "SPIRE Agent pod not found"
    exit 1
fi

success "Found SPIRE Agent: $AGENT_POD"

# Note: SPIRE agent uses distroless image with no shell, so we skip socket check
# The test pod will verify socket access when it runs
info "SPIRE Agent is running (socket check skipped - distroless image)"

# Create a test pod manifest
info "Creating test pod with socket access..."

cat > /tmp/spire-test-pod.yaml <<EOF
apiVersion: v1
kind: Pod
metadata:
  name: spire-integration-test
  namespace: spire-system
spec:
  serviceAccountName: spire-agent
  hostPID: true
  hostNetwork: true
  containers:
  - name: test
    image: golang:1.23
    command: ["sleep", "infinity"]
    env:
    - name: GOTOOLCHAIN
      value: "auto"
    - name: SPIRE_AGENT_SOCKET
      value: "unix:///spire-agent-socket/api.sock"
    - name: SPIRE_TRUST_DOMAIN
      value: "example.org"
    volumeMounts:
    - name: spire-agent-socket
      mountPath: /spire-agent-socket
      readOnly: true
  volumes:
  - name: spire-agent-socket
    hostPath:
      path: /tmp/spire-agent/public
      type: Directory
  restartPolicy: Never
EOF

# Delete existing test pod if present
kubectl delete pod -n spire-system spire-integration-test --ignore-not-found=true >/dev/null 2>&1

# Create and wait for test pod
kubectl apply -f /tmp/spire-test-pod.yaml >/dev/null
info "Waiting for test pod to be ready..."
kubectl wait --for=condition=Ready pod/spire-integration-test -n spire-system --timeout=60s >/dev/null

success "Test pod is ready"

# Copy project code to test pod
info "Copying project code to test pod..."
kubectl exec -n spire-system spire-integration-test -- mkdir -p /workspace
kubectl cp . spire-system/spire-integration-test:/workspace >/dev/null 2>&1 || {
    error "Failed to copy project"
    kubectl delete pod -n spire-system spire-integration-test --ignore-not-found=true
    exit 1
}

# Run tests in pod
echo ""
info "Running integration tests in pod..."
echo ""

if kubectl exec -n spire-system spire-integration-test -- \
    sh -c "cd /workspace && go test -tags=integration -race -v ./internal/adapters/outbound/spire/..."; then
    echo ""
    success "Integration tests passed!"
    EXIT_CODE=0
else
    echo ""
    error "Integration tests failed"
    EXIT_CODE=1
fi

# Cleanup
info "Cleaning up test pod..."
kubectl delete pod -n spire-system spire-integration-test --ignore-not-found=true >/dev/null 2>&1

rm -f /tmp/spire-test-pod.yaml

exit $EXIT_CODE
