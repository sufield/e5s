---
type: tutorial
audience: beginner
---

# Quick Start Guide - mTLS with SPIRE

This guide covers getting started with the mTLS identity server and client using SPIRE.

## Prerequisites

- Go 1.25 or later
- `kubectl` and `minikube` (for Kubernetes testing)
- Docker (for building images)

SPIRE is deployed via Kubernetes/Minikube, not as standalone SPIRE binaries. The automated setup deploys SPIRE server and agents using Helm charts.

## What You'll Build

- **mTLS Server**: A server that authenticates clients using SPIFFE identities
- **Test Client**: A client that connects to the server using mTLS

## Related Documentation

- [Testing Guide](TESTING_GUIDE.md) - Test scenarios and verification procedures
- [Troubleshooting Guide](TROUBLESHOOTING.md) - Diagnosing and fixing common issues
- [Example Code](../examples/) - Reference implementations

---

## Quick Start (Minikube)

SPIRE infrastructure is deployed via Kubernetes.

### Step 1: Deploy SPIRE Infrastructure

```bash
# Start Minikube and deploy SPIRE (server + agents)
make minikube-up
```

This command:
- Starts Minikube cluster
- Deploys SPIRE server via Helm chart
- Deploys SPIRE agents on each node
- Configures the trust domain as `example.org`

### Step 2: Verify SPIRE Deployment

```bash
# Check SPIRE components are running
kubectl get pods -n spire-system

# Expected output:
# NAME                            READY   STATUS    RESTARTS   AGE
# spire-server-0                  2/2     Running   0          2m
# spire-agent-xxxxx               1/1     Running   0          2m
```

### Step 3: Deploy Example Workloads

```bash
# Deploy mTLS server example
kubectl apply -f examples/mtls-server.yaml

# Verify pod is running
kubectl get pod -l app=mtls-server

# Build and copy the binary to the pod
go build -o bin/mtls-server ./examples/zeroconfig-example
POD=$(kubectl get pod -l app=mtls-server -o jsonpath='{.items[0].metadata.name}')
kubectl cp bin/mtls-server "$POD":/tmp/mtls-server
kubectl exec "$POD" -- chmod +x /tmp/mtls-server
```

### Step 4: Run the Example Server

```bash
# Execute the server in the pod
kubectl exec -it "$POD" -- /tmp/mtls-server
```

Expected output:
```
========================================
Zero Trust mTLS Server - Starting
========================================
Configuration:
  SPIRE Socket: unix:///spire-socket/api.sock
  Listen Address: :8443
========================================
Connecting to SPIRE agent...
Starting mTLS server...
✓ Configuration detected successfully
  Trust Domain: example.org
  Socket Path: unix:///spire-socket/api.sock
✓ mTLS server created successfully
✓ Routes registered
========================================
🚀 Server listening on :8443
========================================
Waiting for connections... (Press Ctrl+C to stop)
```

### Step 5: Test with Example Client

In a new terminal:

```bash
# Deploy client workload
kubectl apply -f examples/test-client.yaml

# Build the test client binary locally and copy it to the pod
go build -o /tmp/test-client examples/test-client.go
CLIENT_POD=$(kubectl get pod -l app=test-client -o jsonpath='{.items[0].metadata.name}')
kubectl cp /tmp/test-client "$CLIENT_POD":/tmp/test-client
kubectl exec "$CLIENT_POD" -- chmod +x /tmp/test-client

# Run the test client
kubectl exec "$CLIENT_POD" -- /tmp/test-client
```

Expected output:
```
=== Testing: https://mtls-server:8443/ ===
Status: 200
Body: Success! Authenticated as: spiffe://example.org/client

=== Testing: https://mtls-server:8443/api/hello ===
Status: 200
Body: Success! Authenticated as: spiffe://example.org/client

=== Testing: https://mtls-server:8443/api/identity ===
Status: 200
Body: Success! Authenticated as: spiffe://example.org/client

=== Testing: https://mtls-server:8443/health ===
Status: 200
Body: {"status":"ok"}
```

---

## Next Steps

Now that you have the server and client running:

1. **Explore the code**: Review the example implementations in `examples/`
2. **Run tests**: See the [Testing Guide](TESTING_GUIDE.md) for running tests
3. **Troubleshoot issues**: Check the [Troubleshooting Guide](TROUBLESHOOTING.md) if you have problems
4. **Build your application**: Use these examples as a foundation for your own mTLS services

## Understanding SPIFFE Identities

In the example above:
- **Server SPIFFE ID**: `spiffe://example.org/server`
- **Client SPIFFE ID**: `spiffe://example.org/client`
- **Trust Domain**: `example.org`

These identities are automatically provisioned by SPIRE based on Kubernetes workload selectors. Manual certificate management is not required.

## Common Commands

```bash
# Check SPIRE infrastructure status
kubectl get pods -n spire-system

# View server logs
kubectl logs -l app=mtls-server --tail=50 -f

# View client logs
kubectl logs -l app=test-client --tail=50 -f

# Restart a deployment
kubectl rollout restart deployment/mtls-server

# Clean up everything
make minikube-down
```

---

## Related Documentation

- [Testing Guide](TESTING_GUIDE.md) - Detailed test scenarios and verification procedures
- [Troubleshooting Guide](TROUBLESHOOTING.md) - Diagnosing and fixing common issues
- [Port-Based Improvements](PORT_BASED_IMPROVEMENTS.md) - Architecture overview
- [Unified Configuration](UNIFIED_CONFIG_IMPROVEMENTS.md) - Configuration details
- [Example Code](../examples/) - Reference implementations
- [SPIRE Documentation](https://spiffe.io/docs/latest/spire/) - Official SPIRE docs
