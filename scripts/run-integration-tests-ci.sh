#!/usr/bin/env bash
# CI-optimized integration test runner with static binary and distroless image
# For production CI pipelines - maximum security and determinism
#
# Configuration via environment variables:
#   NS              - Kubernetes namespace (default: spire-system)
#   SOCKET_DIR      - Socket directory on node (default: /tmp/spire-agent/public)
#   SOCKET_FILE     - Socket filename (default: api.sock)
#   PKG             - Package to test (default: ./internal/adapters/outbound/spire)
#   TESTBIN         - Test binary path (default: /tmp/spire-integration.test)
#   TAGS            - Build tags (default: integration)
#   TRUST_DOMAIN    - SPIRE trust domain (default: example.org)

set -Eeuo pipefail

# Configuration
NS="${NS:-spire-system}"
SOCKET_DIR="${SOCKET_DIR:-/tmp/spire-agent/public}"
SOCKET_FILE="${SOCKET_FILE:-api.sock}"
PKG="${PKG:-./internal/adapters/outbound/spire}"
TESTBIN="${TESTBIN:-/tmp/spire-integration.test}"
TAGS="${TAGS:-integration}"
TRUST_DOMAIN="${TRUST_DOMAIN:-example.org}"

POD_NAME="spire-integration-test-ci"
POD_YAML="/tmp/spire-test-pod-ci.yaml"

# Cleanup on exit
cleanup() {
    kubectl delete pod -n "$NS" "$POD_NAME" --ignore-not-found=true >/dev/null 2>&1 || true
    rm -f "$POD_YAML" "$TESTBIN" || true
}
trap cleanup EXIT

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

echo "============================================"
echo "SPIRE Integration Tests (CI/Distroless)"
echo "============================================"
echo ""
info "Configuration:"
echo "  Namespace: $NS"
echo "  Socket: $SOCKET_DIR/$SOCKET_FILE"
echo "  Package: $PKG"
echo "  Mode: CI (static binary + distroless)"
echo ""

# Check prerequisites
info "Checking prerequisites..."

if ! command -v kubectl >/dev/null 2>&1; then
    error "kubectl not found"
    exit 1
fi

if ! kubectl get namespace "$NS" >/dev/null 2>&1; then
    error "SPIRE namespace '$NS' not found"
    exit 1
fi

# Get SPIRE Agent pod (tolerant label selector - tries 3 common patterns)
AGENT_POD=$(
  kubectl get pods -n "$NS" \
    -l 'app.kubernetes.io/name=agent' -o jsonpath='{.items[0].metadata.name}' 2>/dev/null \
  || kubectl get pods -n "$NS" \
    -l 'app=spire-agent' -o jsonpath='{.items[0].metadata.name}' 2>/dev/null \
  || kubectl get pods -n "$NS" \
    -l 'name=spire-agent' -o jsonpath='{.items[0].metadata.name}' 2>/dev/null \
  || true
)

if [ -z "$AGENT_POD" ]; then
    error "SPIRE Agent pod not found"
    exit 1
fi

success "Found SPIRE Agent: $AGENT_POD"

# Verify socket exists on node
info "Checking socket existence on node..."
NODE="$(kubectl get nodes -o jsonpath='{.items[0].metadata.name}')"
if ! minikube ssh -- "test -S ${SOCKET_DIR}/${SOCKET_FILE} -o -d ${SOCKET_DIR}" >/dev/null 2>&1; then
    error "Socket or directory not present on node: ${SOCKET_DIR}/${SOCKET_FILE}"
    exit 1
fi
success "Socket verified on node: $NODE"

# Compile static test binary (no CGO, for distroless)
# Auto-detect node architecture for cross-platform support
NODE_ARCH=$(kubectl get nodes -o jsonpath='{.items[0].status.nodeInfo.architecture}')
GOARCH="${GOARCH:-$NODE_ARCH}"

info "Compiling static integration test binary (GOARCH=$GOARCH)..."
if ! CGO_ENABLED=0 GOOS=linux GOARCH="$GOARCH" \
    go test -tags="$TAGS" -c -o "$TESTBIN" "$PKG"; then
    error "Failed to compile static test binary"
    exit 1
fi

success "Static test binary compiled: $(du -h "$TESTBIN" | cut -f1)"

# Create distroless test pod with initContainer (ALWAYS use this pattern for distroless)
info "Creating distroless test pod with initContainer..."

cat > "$POD_YAML" <<EOF
apiVersion: v1
kind: Pod
metadata:
  name: ${POD_NAME}
  namespace: ${NS}
  labels:
    app: spire-integration-test
    role: ci-test
    security: distroless
spec:
  restartPolicy: Never
  securityContext:
    runAsNonRoot: true
    runAsUser: 65532
    fsGroup: 65532
    seccompProfile:
      type: RuntimeDefault
  volumes:
    - name: spire-socket
      hostPath:
        path: ${SOCKET_DIR}
        type: Directory
    - name: work
      emptyDir: {}
  initContainers:
    - name: setup
      image: busybox:stable-musl
      # Wait for binary to be copied, then make it executable
      command: ["sh", "-c", "while [ ! -f /work/integration.test ]; do sleep 0.2; done; chmod +x /work/integration.test"]
      volumeMounts:
        - name: work
          mountPath: /work
      securityContext:
        allowPrivilegeEscalation: false
        capabilities:
          drop: ["ALL"]
  containers:
    - name: test
      # Distroless: minimal attack surface, no shell, no package manager
      image: gcr.io/distroless/static-debian12:nonroot
      # Direct execution - no shell available in distroless
      command: ["/work/integration.test"]
      args: ["-test.v"]
      env:
        - name: SPIFFE_ENDPOINT_SOCKET
          value: "unix:///spire-socket/${SOCKET_FILE}"
        - name: SPIRE_TRUST_DOMAIN
          value: "${TRUST_DOMAIN}"
      volumeMounts:
        - name: spire-socket
          mountPath: /spire-socket
          readOnly: true
        - name: work
          mountPath: /work
      resources:
        requests:
          cpu: "100m"
          memory: "128Mi"
        limits:
          cpu: "500m"
          memory: "256Mi"
      securityContext:
        allowPrivilegeEscalation: false
        readOnlyRootFilesystem: true
        capabilities:
          drop: ["ALL"]
EOF

# Delete existing pod
kubectl delete pod -n "$NS" "$POD_NAME" --ignore-not-found=true >/dev/null 2>&1 || true

# Create test pod
kubectl apply -f "$POD_YAML" >/dev/null

# Wait for pod to be scheduled
info "Waiting for test pod to be scheduled..."
if ! kubectl wait --for=condition=PodScheduled pod/"$POD_NAME" -n "$NS" --timeout=60s >/dev/null 2>&1; then
    error "Test pod failed to be scheduled"
    kubectl describe pod -n "$NS" "$POD_NAME" || true
    exit 1
fi

# Copy test binary to initContainer (which is waiting for it)
info "Copying static test binary to pod..."
if ! kubectl cp "$TESTBIN" "$NS"/"$POD_NAME":/work/integration.test -c setup 2>/dev/null; then
    error "Failed to copy test binary to initContainer"
    kubectl describe pod -n "$NS" "$POD_NAME" || true
    exit 1
fi

success "Binary copied, initContainer will chmod +x and test container will start"

# Run tests by streaming logs
echo ""
info "Running integration tests in distroless container..."
echo ""

# Stream logs from test container (it runs automatically)
kubectl logs -n "$NS" "$POD_NAME" -c test -f 2>/dev/null || true

# Check exit code from terminated container state
echo ""
info "Checking test results..."
EXIT_CODE="$(kubectl get pod -n "$NS" "$POD_NAME" -o jsonpath='{.status.containerStatuses[?(@.name=="test")].state.terminated.exitCode}' 2>/dev/null || echo "")"

# If exit code is empty, container might still be running or failed to start
if [ -z "$EXIT_CODE" ]; then
    error "Could not determine test exit code"
    kubectl describe pod -n "$NS" "$POD_NAME" || true
    EXIT_CODE=1
elif [ "$EXIT_CODE" = "0" ]; then
    success "Integration tests passed!"
else
    error "Integration tests failed (exit code: $EXIT_CODE)"
fi

info "Cleaning up..."

exit $EXIT_CODE
