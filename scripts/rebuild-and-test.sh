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
sleep 10

echo ""
echo "=== Test Results ==="
kubectl logs -l app=e5s-client --tail=20

echo ""
echo "=== Rebuild Complete ==="
echo ""
