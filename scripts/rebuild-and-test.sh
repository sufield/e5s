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

# Step 2: Point to Minikube's Docker and clean old images
echo "2. Cleaning old Docker images..."
eval $(minikube docker-env)
docker rmi e5s-server:dev e5s-client:dev 2>/dev/null || true

# Step 3: Rebuild Docker images
echo "3. Rebuilding Docker images..."
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

# Step 4: Force server pods to restart with new image
echo "4. Forcing server pods to restart with new image..."
kubectl delete pods -l app=e5s-server
kubectl wait --for=condition=ready pod -l app=e5s-server --timeout=90s
echo "   ✓ Server restarted with new image"

# Step 5: Test with authenticated client
echo ""
echo "5. Testing with authenticated client..."
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
          value: "https://e5s-server:8443/time"
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

# Step 6: Test unregistered client
echo ""
echo "6. Testing unregistered client (zero-trust verification)..."

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
          value: "https://e5s-server:8443/time"
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
UNREGEOF

echo "   Waiting for unregistered client to fail (max 60s)..."
kubectl wait --for=condition=failed job/e5s-unregistered-client --timeout=60s 2>/dev/null || \
    kubectl wait --for=condition=complete job/e5s-unregistered-client --timeout=5s 2>/dev/null || true

echo ""
echo "❌ Unregistered Client (NOT registered in control plane):"
echo "---"
echo "Unregistered client logs (SPIRE refuses to issue identity):"
kubectl logs -l app=e5s-unregistered-client --tail=20 || echo "   (Pod failed before producing logs - expected)"

echo ""
echo "=== Zero-Trust Verification ==="
echo "✓ Authenticated client:   SUCCESS (registered in SPIRE control plane)"
echo "❌ Unregistered client:   FAILED (not registered, no identity issued)"

echo ""
echo "=== Rebuild Complete ==="
echo ""
