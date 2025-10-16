#!/bin/bash
# Optimized integration test runner using pre-compiled test binary
# Based on recommended Option A from architecture review
set -e

echo "============================================"
echo "SPIRE Integration Tests (Optimized)"
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

# Compile test binary locally (fast, deterministic)
info "Compiling integration test binary..."
go test -tags=integration -c \
  -o /tmp/spire-integration.test \
  ./internal/adapters/outbound/spire || {
    error "Failed to compile test binary"
    exit 1
}

success "Test binary compiled: $(du -h /tmp/spire-integration.test | cut -f1)"

# Create minimal test pod with distroless base
info "Creating minimal test pod..."

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
  restartPolicy: Never
  volumes:
    - name: spire-socket
      hostPath:
        path: /tmp/spire-agent/public
        type: Directory
    - name: work
      emptyDir: {}
  containers:
    - name: test
      # Using debian-slim instead of distroless to support shell for test execution
      image: debian:bookworm-slim
      command: ["sleep", "infinity"]
      env:
        - name: SPIFFE_ENDPOINT_SOCKET
          value: "unix:///spire-socket/api.sock"
        - name: SPIRE_TRUST_DOMAIN
          value: "example.org"
      volumeMounts:
        - name: spire-socket
          mountPath: /spire-socket
          readOnly: true
        - name: work
          mountPath: /work
EOF

# Delete existing test pod if present
kubectl delete pod -n spire-system spire-integration-test --ignore-not-found=true >/dev/null 2>&1

# Create and wait for test pod
kubectl apply -f /tmp/spire-test-pod.yaml >/dev/null
info "Waiting for test pod to be ready..."
kubectl wait --for=condition=Ready pod/spire-integration-test -n spire-system --timeout=60s >/dev/null

success "Test pod ready"

# Copy only the compiled test binary (fast)
info "Copying test binary to pod ($(du -h /tmp/spire-integration.test | cut -f1))..."
kubectl cp /tmp/spire-integration.test spire-system/spire-integration-test:/work/integration.test || {
    error "Failed to copy test binary"
    kubectl delete pod -n spire-system spire-integration-test --ignore-not-found=true
    exit 1
}

# Run tests
echo ""
info "Running integration tests..."
echo ""

if kubectl exec -n spire-system spire-integration-test -- \
    /work/integration.test -test.v; then
    echo ""
    success "Integration tests passed!"
    EXIT_CODE=0
else
    echo ""
    error "Integration tests failed"
    EXIT_CODE=1
fi

# Cleanup
info "Cleaning up..."
kubectl delete pod -n spire-system spire-integration-test --ignore-not-found=true >/dev/null 2>&1
rm -f /tmp/spire-test-pod.yaml /tmp/spire-integration.test

exit $EXIT_CODE
