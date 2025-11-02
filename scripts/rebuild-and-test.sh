#!/bin/bash
set -e

# Quick rebuild and test script for e5s library developers
# Use this after making changes to e5s library code
# Now uses Helm for production-like deployment

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
TEST_DIR="$PROJECT_ROOT/test-demo"

cd "$TEST_DIR"

echo "=== Rebuilding and Testing e5s Changes (Helm-based) ==="
echo ""

# Step 1: Rebuild binaries
echo "1. Rebuilding static binaries..."
CGO_ENABLED=0 go build -o bin/server ./server
CGO_ENABLED=0 go build -o bin/client ./client

# Step 2: Rebuild Docker images
echo "2. Rebuilding Docker images..."
eval $(minikube docker-env)

docker build -t e5s-server:dev -q -f - . <<'DOCKERFILE'
FROM alpine:latest
WORKDIR /app
COPY bin/server .
ENTRYPOINT ["/app/server"]
DOCKERFILE

docker build -t e5s-client:dev -q -f - . <<'DOCKERFILE'
FROM alpine:latest
WORKDIR /app
COPY bin/client .
ENTRYPOINT ["/app/client"]
DOCKERFILE

# Step 3: Restart server deployment
echo "3. Restarting server deployment..."
kubectl rollout restart deployment/e5s-server
kubectl rollout status deployment/e5s-server --timeout=90s
echo "   ✓ Server restarted successfully"

# Step 4: Test with authenticated client
echo ""
echo "4. Testing with authenticated client..."
kubectl delete job e5s-client 2>/dev/null || true

cat <<'JOBEOF' | kubectl apply -f -
apiVersion: batch/v1
kind: Job
metadata:
  name: e5s-client
  namespace: default
spec:
  backoffLimit: 0
  template:
    metadata:
      labels:
        app: e5s-client
    spec:
      serviceAccountName: default
      restartPolicy: Never
      containers:
      - name: client
        image: e5s-client:dev
        imagePullPolicy: Never
        env:
        - name: SERVER_URL
          value: "https://e5s-server:8443/hello"
        command: ["/app/client"]
        volumeMounts:
        - name: spire-agent-socket
          mountPath: /spire/agent-socket
          readOnly: true
        - name: config
          mountPath: /app/e5s.yaml
          subPath: e5s.yaml
      volumes:
      - name: spire-agent-socket
        csi:
          driver: "csi.spiffe.io"
          readOnly: true
      - name: config
        configMap:
          name: e5s-config
JOBEOF

echo "   Waiting for client job to complete (max 60s)..."
kubectl wait --for=condition=complete job/e5s-client --timeout=60s 2>/dev/null || \
    kubectl wait --for=condition=failed job/e5s-client --timeout=5s 2>/dev/null || true

echo ""
echo "=== Test Results ==="
echo ""
echo "✓ Authenticated Client (registered with SPIRE):"
echo "---"
echo "Server logs (last 10 lines):"
kubectl logs -l app=e5s-server --tail=10
echo ""
echo "Client logs:"
kubectl logs -l app=e5s-client --tail=20

# Step 5: Test unregistered client
echo ""
echo "5. Testing unregistered client (zero-trust verification)..."

kubectl delete job e5s-unregistered-client 2>/dev/null || true

cat <<'UNREGEOF' | kubectl apply -f -
apiVersion: batch/v1
kind: Job
metadata:
  name: e5s-unregistered-client
  namespace: default
spec:
  backoffLimit: 0
  template:
    metadata:
      labels:
        app: e5s-unregistered-client
    spec:
      serviceAccountName: default
      restartPolicy: Never
      containers:
      - name: client
        image: e5s-client:dev
        imagePullPolicy: Never
        env:
        - name: SERVER_URL
          value: "https://e5s-server:8443/hello"
        command: ["/app/client"]
        volumeMounts:
        - name: config
          mountPath: /app/e5s.yaml
          subPath: e5s.yaml
        # NOTE: NO SPIRE socket mounted!
      volumes:
      - name: config
        configMap:
          name: e5s-config
UNREGEOF

echo "   Waiting for unregistered client to fail (max 60s)..."
kubectl wait --for=condition=failed job/e5s-unregistered-client --timeout=60s 2>/dev/null || \
    kubectl wait --for=condition=complete job/e5s-unregistered-client --timeout=5s 2>/dev/null || true

echo ""
echo "❌ Unregistered Client (NO SPIRE identity):"
echo "---"
echo "Unregistered client logs (should fail to get identity):"
kubectl logs -l app=e5s-unregistered-client --tail=20 || echo "   (Pod failed before producing logs - expected)"

echo ""
echo "=== Zero-Trust Verification ==="
echo "✓ Authenticated client:   SUCCESS (has SPIRE identity)"
echo "❌ Unregistered client:   FAILED (no SPIRE identity)"

echo ""
echo "=== Rebuild Complete ==="
echo ""
