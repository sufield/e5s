#!/usr/bin/env bash
# CI-optimized integration test runner with static binary and distroless image
# For production CI pipelines - maximum security and determinism
#
# Configuration via environment variables:
#   NS              - Kubernetes namespace (default: spire-system)
#   SOCKET_DIR      - Socket directory on node (default: /tmp/spire-agent/public)
#   SOCKET_FILE     - Socket filename (default: api.sock)
#   PKG             - Package to test (default: ./pkg/spire)
#   TESTBIN         - Test binary path (default: /tmp/spire-integration.test)
#   TAGS            - Build tags (default: integration)
#   KEEP            - Keep pod for inspection (default: false)
#   TRUST_DOMAIN    - SPIRE trust domain (default: example.org)

set -Eeuo pipefail

# Configuration
NS="${NS:-spire-system}"
SOCKET_DIR="${SOCKET_DIR:-/tmp/spire-agent/public}"
SOCKET_FILE="${SOCKET_FILE:-api.sock}"
PKG="${PKG:-./pkg/spire}"
TESTBIN="${TESTBIN:-/tmp/spire-integration.test}"
TAGS="${TAGS:-integration}"
KEEP="${KEEP:-false}"
TRUST_DOMAIN="${TRUST_DOMAIN:-example.org}"

POD_NAME="spire-integration-test-ci"
POD_YAML="/tmp/spire-test-pod-ci.yaml"

# Cleanup on exit (honors KEEP flag)
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
echo "SPIRE Integration Tests (CI/Distroless)"
echo "============================================"
echo ""
info "Configuration:"
echo "  Namespace: $NS"
echo "  Socket: $SOCKET_DIR/$SOCKET_FILE"
echo "  Package: $PKG"
echo "  Keep pod: $KEEP"
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

# Setup SPIRE registration entries (if not already done)
info "Setting up SPIRE workload registration entries..."
if [ -f "./scripts/setup-spire-registrations.sh" ]; then
    # Run registration setup and capture exit code
    SETUP_OUTPUT=$(NS="$NS" TRUST_DOMAIN="$TRUST_DOMAIN" bash ./scripts/setup-spire-registrations.sh 2>&1) || SETUP_EXIT=$?
    SETUP_EXIT=${SETUP_EXIT:-0}

    # Show key output
    echo "$SETUP_OUTPUT" | grep -E "(✅|❌|Entry ID|SPIFFE ID|Quick fix)" || true

    if [ "$SETUP_EXIT" -eq 0 ]; then
        success "Registration entries verified/created"
    else
        echo ""
        error "Registration setup failed - this will likely cause test failures"
        echo ""
        info "Common issue: SPIRE server using distroless image"
        info "Quick fix (development only):"
        echo ""
        echo "  # Switch to non-distroless SPIRE server image"
        echo "  kubectl set image statefulset/spire-server -n $NS \\"
        echo "    spire-server=ghcr.io/spiffe/spire-server:1.9.0"
        echo ""
        echo "  # Wait for rollout"
        echo "  kubectl rollout status statefulset/spire-server -n $NS"
        echo ""
        echo "  # Re-run this script"
        echo "  $0"
        echo ""
        info "Continuing anyway - tests will fail if registrations don't exist"
        sleep 5
    fi
else
    info "Registration setup script not found, assuming entries exist"
    info "If tests fail with 'context deadline exceeded', create workload entries manually"
fi
echo ""

# Verify socket exists
# Note: SPIRE agent may be distroless (no shell/test command), so we verify via node instead
info "Checking Workload API socket availability..."

# Try agent pod check first (works if agent has shell/test)
if kubectl exec -n "$NS" "$AGENT_POD" -- test -S /tmp/spire-agent/public/api.sock >/dev/null 2>&1; then
    success "Workload API socket verified via agent pod"
elif command -v minikube >/dev/null 2>&1; then
    # Fallback to node check for Minikube (works with distroless agent)
    info "Agent pod check failed (likely distroless), checking via Minikube node..."
    if ! minikube ssh -- "test -S ${SOCKET_DIR}/${SOCKET_FILE}" >/dev/null 2>&1; then
        error "Socket missing on Minikube node: ${SOCKET_DIR}/${SOCKET_FILE}"
        exit 1
    fi
    success "Workload API socket verified via Minikube node"
else
    # For non-Minikube clusters, assume socket is available (hostPath should work)
    info "Cannot verify socket (distroless agent, not Minikube), assuming hostPath is configured correctly"
fi

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
    # First init container: Wait for SPIRE Workload API socket to be available
    - name: wait-for-socket
      image: busybox:stable-musl
      command:
        - sh
        - -c
        - |
          echo "Waiting for SPIRE Workload API socket..."
          WAIT_TIME=0
          MAX_WAIT=120
          until [ -S /spire-socket/${SOCKET_FILE} ]; do
            if [ \$WAIT_TIME -ge \$MAX_WAIT ]; then
              echo "ERROR: Socket not found after \${MAX_WAIT}s"
              exit 1
            fi
            echo "Socket not found yet (waited \${WAIT_TIME}s), retrying in 2s..."
            sleep 2
            WAIT_TIME=\$((WAIT_TIME + 2))
          done
          echo "✅ Socket found: /spire-socket/${SOCKET_FILE}"
          # Give agent a moment to fully initialize
          echo "Waiting 5s for agent initialization..."
          sleep 5
          echo "✅ Ready for workload attestation"
      volumeMounts:
        - name: spire-socket
          mountPath: /spire-socket
          readOnly: true
      securityContext:
        allowPrivilegeEscalation: false
        capabilities:
          drop: ["ALL"]
    # Second init container: Wait for test binary and prepare it
    - name: setup
      image: busybox:stable-musl
      # Wait for binary to be copied with file size stability check
      command:
        - sh
        - -c
        - |
          echo "Waiting for test binary..."
          # Wait for file to appear
          while [ ! -f /work/integration.test ]; do sleep 0.2; done
          # Wait for file size to stabilize (copy complete)
          PREV_SIZE=0
          STABLE_COUNT=0
          while [ \$STABLE_COUNT -lt 3 ]; do
            CURR_SIZE=\$(stat -c%s /work/integration.test 2>/dev/null || echo 0)
            if [ "\$CURR_SIZE" = "\$PREV_SIZE" ] && [ "\$CURR_SIZE" -gt 0 ]; then
              STABLE_COUNT=\$((STABLE_COUNT + 1))
            else
              STABLE_COUNT=0
            fi
            PREV_SIZE=\$CURR_SIZE
            sleep 0.3
          done
          chmod +x /work/integration.test
          echo "✅ Binary ready: \$(stat -c%s /work/integration.test) bytes"
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
      args: ["-test.v", "-test.timeout=3m"]
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

# Wait for socket-wait initContainer to complete
info "Waiting for socket availability check to complete..."
for i in {1..120}; do
    SOCKET_INIT_STATE=$(kubectl get pod -n "$NS" "$POD_NAME" \
        -o jsonpath='{.status.initContainerStatuses[?(@.name=="wait-for-socket")].state}' 2>/dev/null || true)

    if echo "$SOCKET_INIT_STATE" | grep -q "terminated"; then
        EXIT_CODE=$(kubectl get pod -n "$NS" "$POD_NAME" \
            -o jsonpath='{.status.initContainerStatuses[?(@.name=="wait-for-socket")].state.terminated.exitCode}' 2>/dev/null || echo "")
        if [ "$EXIT_CODE" = "0" ]; then
            success "Socket is available and ready"
            break
        else
            error "Socket wait failed (exit code: $EXIT_CODE)"
            kubectl logs -n "$NS" "$POD_NAME" -c wait-for-socket || true
            exit 1
        fi
    fi

    if [ $i -eq 120 ]; then
        error "Socket wait timed out after 120s"
        kubectl describe pod -n "$NS" "$POD_NAME" || true
        kubectl logs -n "$NS" "$POD_NAME" -c wait-for-socket 2>/dev/null || true
        exit 1
    fi
    sleep 1
done

# Wait for setup initContainer to be running
info "Waiting for setup initContainer to start..."
for i in {1..30}; do
    INIT_STATE=$(kubectl get pod -n "$NS" "$POD_NAME" \
        -o jsonpath='{.status.initContainerStatuses[?(@.name=="setup")].state}' 2>/dev/null || true)

    if echo "$INIT_STATE" | grep -q "running"; then
        success "Setup init container is running"
        break
    fi

    if [ $i -eq 30 ]; then
        error "Setup init container failed to start after 30s"
        kubectl describe pod -n "$NS" "$POD_NAME" || true
        exit 1
    fi
    sleep 1
done

# Copy test binary to initContainer (which is waiting for it)
# Retry a few times in case kubectl cp timing issue
info "Copying static test binary to pod..."
COPY_SUCCESS=false
for i in {1..5}; do
    if kubectl cp "$TESTBIN" "$NS"/"$POD_NAME":/work/integration.test -c setup 2>&1; then
        COPY_SUCCESS=true
        break
    fi
    info "Copy attempt $i failed, retrying..."
    sleep 1
done

if [ "$COPY_SUCCESS" != "true" ]; then
    error "Failed to copy test binary to initContainer after 5 attempts"
    kubectl describe pod -n "$NS" "$POD_NAME" || true
    kubectl logs -n "$NS" "$POD_NAME" -c setup 2>/dev/null || true
    exit 1
fi

success "Binary copied, initContainer will chmod +x and test container will start"

# Run tests by streaming logs
echo ""
info "Running integration tests in distroless container..."
echo ""

# Wait for test container to start running
for i in {1..30}; do
    TEST_STATE=$(kubectl get pod -n "$NS" "$POD_NAME" \
        -o jsonpath='{.status.containerStatuses[?(@.name=="test")].state}' 2>/dev/null || true)
    if echo "$TEST_STATE" | grep -q "running"; then
        break
    fi
    sleep 1
done

# Stream logs from test container (blocks until container exits)
kubectl logs -n "$NS" "$POD_NAME" -c test -f 2>&1 || true

# Wait for container to terminate completely
echo ""
info "Waiting for test container to terminate..."
if ! kubectl wait --for=condition=ContainersReady=false pod/"$POD_NAME" -n "$NS" --timeout=180s >/dev/null 2>&1; then
    info "Wait timed out or pod deleted"
fi

# Check exit code from terminated container state (with retry to avoid race)
info "Checking test results..."
EXIT_CODE=""
for i in {1..20}; do
    EXIT_CODE="$(kubectl get pod -n "$NS" "$POD_NAME" \
        -o jsonpath='{.status.containerStatuses[?(@.name=="test")].state.terminated.exitCode}' 2>/dev/null || true)"
    [ -n "$EXIT_CODE" ] && break
    sleep 0.5
done

# If exit code is still empty, container might have failed to start
if [ -z "$EXIT_CODE" ]; then
    error "Could not determine test exit code"
    kubectl describe pod -n "$NS" "$POD_NAME" || true
    kubectl logs -n "$NS" "$POD_NAME" -c test --previous 2>/dev/null || true
    EXIT_CODE=1
elif [ "$EXIT_CODE" = "0" ]; then
    success "Integration tests passed!"
else
    error "Integration tests failed (exit code: $EXIT_CODE)"
    # Surface failure context for debugging
    kubectl describe pod -n "$NS" "$POD_NAME" || true
fi

# Cleanup (or keep for inspection)
if [ "$KEEP" = "true" ]; then
    info "Keeping test pod for inspection (KEEP=true)"
    info "To delete manually: kubectl delete pod -n $NS $POD_NAME"
else
    info "Cleaning up..."
fi

exit $EXIT_CODE
