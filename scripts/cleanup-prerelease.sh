#!/bin/bash
set -e

# Cleanup script for prerelease testing

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
TEST_DIR="$PROJECT_ROOT/test-demo"

echo "=== Cleaning Up Pre-Release Test Environment ==="
echo ""

# Step 1: Delete Kubernetes resources
echo "1. Deleting Kubernetes resources..."
kubectl delete deployment e5s-server 2>/dev/null || echo "   Server deployment not found"
kubectl delete service e5s-server 2>/dev/null || echo "   Server service not found"
kubectl delete job e5s-client 2>/dev/null || echo "   Client job not found"
kubectl delete job e5s-unregistered-client 2>/dev/null || echo "   Unregistered client job not found"
kubectl delete configmap e5s-config 2>/dev/null || echo "   ConfigMap not found"

# Step 2: Delete Docker images
echo "2. Deleting Docker images from Minikube..."
eval $(minikube docker-env)
docker rmi e5s-server:dev 2>/dev/null || echo "   Server image not found"
docker rmi e5s-client:dev 2>/dev/null || echo "   Client image not found"

# Step 3: Clean test directory (optional)
echo ""
read -p "Delete test-demo directory? [y/N] " -n 1 -r
echo
if [[ $REPLY =~ ^[Yy]$ ]]; then
    echo "3. Deleting test-demo directory..."
    rm -rf "$TEST_DIR"
    echo "   Test directory deleted"
else
    echo "3. Keeping test-demo directory"
    echo "   To manually clean binaries: rm -rf $TEST_DIR/bin"
fi

echo ""
echo "=== Cleanup Complete ==="
echo ""
