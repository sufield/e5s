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

# Get SPIRE Agent pod (tolerant label selector)
AGENT_POD=$(
  kubectl get pods -n "$NS" \
    -l 'app.kubernetes.io/name=agent' -o jsonpath='{.items[0].metadata.name}' 2>/dev/null \
  || kubectl get pods -n "$NS" \
    -l 'app=spire-agent' -o jsonpath='{.items[0].metadata.name}' 2>/dev/null \
  || true
)

if [ -z "$AGENT_POD" ]; then
    error "SPIRE Agent pod not found"
    exit 1
fi

success "Found SPIRE Agent: $AGENT_POD"

# Compile static test binary (no CGO, for distroless)
info "Compiling static integration test binary..."
if ! CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go test -tags="$TAGS" -c -o "$TESTBIN" "$PKG"; then
    error "Failed to compile static test binary"
    exit 1
fi

success "Static test binary compiled: $(du -h "$TESTBIN" | cut -f1)"

# Create distroless test pod (maximum security)
info "Creating distroless test pod..."

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

# Create test pod (but don't wait - we need to copy binary first)
kubectl apply -f "$POD_YAML" >/dev/null

# Wait for pod to be created
sleep 2

# Copy test binary before pod starts (distroless has no shell, so we use initContainer approach via early copy)
info "Copying static test binary to pod..."
# Wait briefly for pod to be schedulable
kubectl wait --for=condition=PodScheduled pod/"$POD_NAME" -n "$NS" --timeout=30s >/dev/null 2>&1 || true

# Copy binary while pod is initializing
if ! kubectl cp "$TESTBIN" "$NS"/"$POD_NAME":/work/integration.test -c test 2>/dev/null; then
    # If direct copy fails, recreate pod with initContainer that waits
    info "Recreating pod with init container for binary copy..."
    kubectl delete pod -n "$NS" "$POD_NAME" --ignore-not-found=true >/dev/null 2>&1

    # Add initContainer that sleeps to allow binary copy
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
      command: ["sh", "-c", "sleep 5"]
      volumeMounts:
        - name: work
          mountPath: /work
  containers:
    - name: test
      image: gcr.io/distroless/static-debian12:nonroot
      command: ["/work/integration.test"]
      args: ["-test.v"]
      env:
        - name: SPIFFE_ENDPOINT_SOCKET
          value: "unix:///spire-socket/${SOCKET_FILE}"
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

    kubectl apply -f "$POD_YAML" >/dev/null
    sleep 2
    kubectl cp "$TESTBIN" "$NS"/"$POD_NAME":/work/integration.test -c setup
fi

# Ensure binary is executable
kubectl exec -n "$NS" "$POD_NAME" -c test -- /work/integration.test -test.v > /dev/null 2>&1 || true

# Wait for pod to be ready
info "Waiting for test pod to be ready..."
if ! kubectl wait --for=condition=Ready pod/"$POD_NAME" -n "$NS" --timeout=60s >/dev/null 2>&1; then
    error "Test pod failed to become ready"
    kubectl describe pod -n "$NS" "$POD_NAME" || true
    kubectl logs -n "$NS" "$POD_NAME" || true
    exit 1
fi

success "Distroless test pod ready"

# Run tests (binary executes automatically via command in pod spec)
echo ""
info "Running integration tests in distroless container..."
echo ""

# Capture logs since binary runs automatically
if kubectl logs -n "$NS" "$POD_NAME" -f 2>/dev/null; then
    # Check exit code via pod phase
    sleep 2
    POD_PHASE=$(kubectl get pod -n "$NS" "$POD_NAME" -o jsonpath='{.status.phase}')
    if [ "$POD_PHASE" = "Succeeded" ]; then
        echo ""
        success "Integration tests passed!"
        EXIT_CODE=0
    else
        echo ""
        error "Integration tests failed"
        EXIT_CODE=1
    fi
else
    error "Failed to get test output"
    EXIT_CODE=1
fi

info "Cleaning up..."

exit $EXIT_CODE
