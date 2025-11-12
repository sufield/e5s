#!/bin/bash
set -e

# Cleanup script for prerelease testing (Helm-based)

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
TEST_DIR="$PROJECT_ROOT/test-demo"

echo "=== Cleaning Up Pre-Release Test Environment (Helm-based) ==="
echo ""

# Step 1: Uninstall Helm release
echo "1. Uninstalling Helm release..."
if helm list | grep -q e5s-test; then
    helm uninstall e5s-test
    echo "   Helm release 'e5s-test' uninstalled"
else
    echo "   Helm release 'e5s-test' not found"
fi

# Step 2: Delete any remaining Kubernetes resources (cleanup orphaned jobs)
echo "2. Cleaning up any remaining Kubernetes resources..."
kubectl delete job e5s-client 2>/dev/null || echo "   Client job not found"
kubectl delete job e5s-unregistered-client 2>/dev/null || echo "   Unregistered client job not found"

# Step 3: Delete Docker images
echo "3. Deleting Docker images from Minikube..."
eval $(minikube docker-env)
docker rmi e5s-server:dev 2>/dev/null || echo "   Server image not found"
docker rmi e5s-client:dev 2>/dev/null || echo "   Client image not found"

# Step 4: Clean test directory (optional)
echo ""
read -p "Delete test-demo directory? [y/N] " -n 1 -r
echo
if [[ $REPLY =~ ^[Yy]$ ]]; then
    echo "4. Deleting test-demo directory..."
    rm -rf "$TEST_DIR"
    echo "   Test directory deleted"
else
    echo "4. Keeping test-demo directory"
    echo "   To manually clean binaries: rm -rf $TEST_DIR/bin"
    echo "   Helm chart preserved at: $TEST_DIR/charts/e5s-test"
fi

echo ""
echo "=== Cleanup Complete ==="
echo ""
