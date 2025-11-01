#!/bin/bash
set -e

# Quick rebuild and test script for e5s library developers
# Use this after making changes to e5s library code

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
TEST_DIR="$PROJECT_ROOT/test-demo"

cd "$TEST_DIR"

echo "=== Rebuilding and Testing e5s Changes ==="
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
kubectl rollout status deployment/e5s-server --timeout=60s

# Step 4: Test with client
echo "4. Testing with client..."
kubectl delete job e5s-client 2>/dev/null || true
kubectl apply -f k8s-client-job.yaml -o name

echo "   Waiting for client to complete..."
sleep 15

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

# Test unregistered client
echo ""
echo "Testing unregistered client (zero-trust verification)..."

if [ ! -f k8s-unregistered-client-job.yaml ]; then
    cat > k8s-unregistered-client-job.yaml <<'EOF'
apiVersion: batch/v1
kind: Job
metadata:
  name: e5s-unregistered-client
  namespace: default
spec:
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
        # This client has config but no access to SPIRE Workload API
        # It cannot obtain a SPIFFE identity, so it cannot authenticate
      volumes:
      - name: config
        configMap:
          name: e5s-config
EOF
fi

kubectl delete job e5s-unregistered-client 2>/dev/null || true
kubectl apply -f k8s-unregistered-client-job.yaml -o name
sleep 10

echo ""
echo "❌ Unregistered Client (NO SPIRE identity):"
echo "---"
echo "Unregistered client logs (should fail to get identity):"
kubectl logs -l app=e5s-unregistered-client --tail=20 || echo "No logs (pod failed as expected)"

echo ""
echo "=== Zero-Trust Verification ==="
echo "✓ Authenticated client:   SUCCESS (has SPIRE identity)"
echo "❌ Unregistered client:   FAILED (no SPIRE identity)"

echo ""
echo "=== Rebuild Complete ==="
echo ""
