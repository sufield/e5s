#!/usr/bin/env bash
# Optimized integration test runner using pre-compiled test binary
# Based on Option A recommendation: compile locally, run in-cluster
#
# Configuration via environment variables:
#   NS              - Kubernetes namespace (default: spire-system)
#   SOCKET_DIR      - Socket directory on node (default: /tmp/spire-agent/public)
#   SOCKET_FILE     - Socket filename (default: api.sock)
#   PKG             - Package to test (default: ./internal/adapters/outbound/spire)
#   TESTBIN         - Test binary path (default: /tmp/spire-integration.test)
#   TAGS            - Build tags (default: integration)
#   KEEP            - Keep pod for faster iteration (default: false)
#   TRUST_DOMAIN    - SPIRE trust domain (default: example.org)

set -Eeuo pipefail

# Configuration
NS="${NS:-spire-system}"
SOCKET_DIR="${SOCKET_DIR:-/tmp/spire-agent/public}"
SOCKET_FILE="${SOCKET_FILE:-api.sock}"
PKG="${PKG:-./internal/adapters/outbound/spire}"
TESTBIN="${TESTBIN:-/tmp/spire-integration.test}"
TAGS="${TAGS:-integration}"
KEEP="${KEEP:-false}"
TRUST_DOMAIN="${TRUST_DOMAIN:-example.org}"

POD_NAME="spire-integration-test"
POD_YAML="/tmp/spire-test-pod.yaml"

# Cleanup on exit (always runs)
cleanup() {
    if [ "$KEEP" != "true" ]; then
        kubectl delete pod -n "$NS" "$POD_NAME" --ignore-not-found=true >/dev/null 2>&1 || true
    fi
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
echo "SPIRE Integration Tests (Optimized)"
echo "============================================"
echo ""
info "Configuration:"
echo "  Namespace: $NS"
echo "  Socket: $SOCKET_DIR/$SOCKET_FILE"
echo "  Package: $PKG"
echo "  Keep pod: $KEEP"
echo ""

# Check prerequisites
info "Checking prerequisites..."

if ! command -v kubectl >/dev/null 2>&1; then
    error "kubectl not found"
    exit 1
fi

if ! kubectl get namespace "$NS" >/dev/null 2>&1; then
    error "SPIRE namespace '$NS' not found. Run: make minikube-up"
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
    error "SPIRE Agent pod not found (tried labels: app.kubernetes.io/name, app, name)"
    exit 1
fi

success "Found SPIRE Agent: $AGENT_POD"

# Verify socket exists
# Note: SPIRE agent may use distroless image without shell, so we check via node or CSI driver
info "Checking Workload API socket availability..."

# Try Minikube node check first (most reliable for local dev)
if command -v minikube >/dev/null 2>&1; then
    info "Minikube detected - verifying socket on node..."
    if ! minikube ssh -- "test -S ${SOCKET_DIR}/${SOCKET_FILE}" >/dev/null 2>&1; then
        error "Socket missing on Minikube node: ${SOCKET_DIR}/${SOCKET_FILE}"
        exit 1
    fi
    success "Workload API socket verified on Minikube node"
else
    # Fallback: Try CSI driver pod (usually has shell) or skip check
    info "Skipping socket verification (agent uses distroless image, no Minikube detected)"
    info "Will verify via test pod connection instead"
fi

# Compile test binary locally (fast, deterministic)
# Auto-detect node architecture for cross-platform support
NODE_ARCH=$(kubectl get nodes -o jsonpath='{.items[0].status.nodeInfo.architecture}')
GOARCH="${GOARCH:-$NODE_ARCH}"

info "Compiling static integration test binary (GOARCH=$GOARCH)..."
if ! CGO_ENABLED=0 GOOS=linux GOARCH="$GOARCH" go test -tags="$TAGS" -c -o "$TESTBIN" "$PKG"; then
    error "Failed to compile static test binary"
    exit 1
fi

success "Test binary compiled: $(du -h "$TESTBIN" | cut -f1)"

# Create minimal test pod (no unnecessary privileges)
info "Creating minimal test pod..."

cat > "$POD_YAML" <<EOF
apiVersion: v1
kind: Pod
metadata:
  name: ${POD_NAME}
  namespace: ${NS}
  labels:
    app: spire-integration-test
    role: test
spec:
  restartPolicy: Never
  volumes:
    - name: spire-socket
      hostPath:
        path: ${SOCKET_DIR}
        type: Directory
    - name: work
      emptyDir: {}
  containers:
    - name: test
      image: debian:bookworm-slim
      command: ["sleep", "infinity"]
      env:
        - name: SPIRE_AGENT_SOCKET
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
        runAsNonRoot: true
        runAsUser: 65532  # nonroot user
        allowPrivilegeEscalation: false
EOF

# Delete existing pod unless KEEP=true
if [ "$KEEP" != "true" ]; then
    kubectl delete pod -n "$NS" "$POD_NAME" --ignore-not-found=true >/dev/null 2>&1 || true
fi

# Create and wait for test pod
if ! kubectl get pod -n "$NS" "$POD_NAME" >/dev/null 2>&1; then
    kubectl apply -f "$POD_YAML" >/dev/null
    info "Waiting for test pod to be scheduled..."
    if ! kubectl wait --for=condition=PodScheduled pod/"$POD_NAME" -n "$NS" --timeout=60s >/dev/null 2>&1; then
        error "Test pod failed to be scheduled"
        kubectl describe pod -n "$NS" "$POD_NAME" || true
        exit 1
    fi
    # Wait for Ready condition (Debian pod with sleep infinity should become ready)
    info "Waiting for test pod to be ready..."
    if ! kubectl wait --for=condition=Ready pod/"$POD_NAME" -n "$NS" --timeout=60s >/dev/null 2>&1; then
        error "Test pod failed to become ready"
        kubectl describe pod -n "$NS" "$POD_NAME" || true
        exit 1
    fi
    success "Test pod ready"
else
    info "Reusing existing test pod"
fi

# Copy test binary to pod
info "Copying test binary to pod ($(du -h "$TESTBIN" | cut -f1))..."
if ! kubectl cp "$TESTBIN" "$NS"/"$POD_NAME":/work/integration.test; then
    error "Failed to copy test binary"
    exit 1
fi

# Ensure binary is executable
kubectl exec -n "$NS" "$POD_NAME" -- chmod +x /work/integration.test >/dev/null 2>&1

# Run tests
echo ""
info "Running integration tests (timeout: 3m)..."
echo ""

if kubectl exec -n "$NS" "$POD_NAME" -- /work/integration.test -test.v -test.timeout=3m; then
    echo ""
    success "Integration tests passed!"
    EXIT_CODE=0
else
    echo ""
    error "Integration tests failed"
    # Surface failure context for debugging
    kubectl describe pod -n "$NS" "$POD_NAME" || true
    EXIT_CODE=1
fi

# Cleanup (or keep for next run)
if [ "$KEEP" = "true" ]; then
    info "Keeping test pod for next run (KEEP=true)"
else
    info "Cleaning up..."
fi

exit $EXIT_CODE
