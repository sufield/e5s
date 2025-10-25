# Manual Testing Guide - mTLS Identity Server & Client

This guide walks you through manual testing of the mTLS identity server and client functionality with SPIRE.

## Prerequisites

- Go 1.25 or later
- `kubectl` and `minikube` (for Kubernetes testing)
- Docker (for building images)

This project uses SPIRE deployed via Kubernetes/Minikube, not standalone SPIRE binaries. The automated setup deploys SPIRE server and agents using Helm charts.

## Table of Contents

1. [Quick Start (Minikube)](#quick-start-minikube)
2. [Testing the Identity Server](#testing-the-identity-server)
3. [Testing the Identity Client](#testing-the-identity-client)
4. [Testing Error Cases](#testing-error-cases)
5. [Manual Testing with curl/openssl](#manual-testing-with-curlopenssl)

---

## Quick Start (Minikube)

This project's SPIRE infrastructure is deployed via Kubernetes. Follow these steps to set up a complete testing environment.

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
Creating mTLS server with configuration:
  Socket: unix:///spire-socket/api.sock
  Address: :8443
  Allowed client: spiffe://example.org/workload/client
✓ Server created successfully
Listening on :8443 with mTLS authentication
```

### Step 5: Test with Example Client

In a new terminal:

```bash
# Deploy client workload
kubectl apply -f examples/mtls-client.yaml

# Build and copy client binary
go build -o bin/mtls-client ./examples/zeroconfig-example
CLIENT_POD=$(kubectl get pod -l app=mtls-client -o jsonpath='{.items[0].metadata.name}')
kubectl cp bin/mtls-client "$CLIENT_POD":/tmp/mtls-client
kubectl exec "$CLIENT_POD" -- chmod +x /tmp/mtls-client

# Run the client
kubectl exec -it "$CLIENT_POD" -- /tmp/mtls-client
```

Expected output:
```
✓ Connection successful
Response: Success! Authenticated as: spiffe://example.org/workload/client
```

---

## Testing the Identity Server

### Test 1: Basic Server Startup

**Goal**: Verify server starts and connects to SPIRE agent

This test runs inside a Kubernetes pod with access to the SPIRE agent socket.

```bash
# Ensure SPIRE infrastructure is running
make minikube-up

# Deploy and run the server (see Quick Start section)
kubectl apply -f examples/mtls-server.yaml
POD=$(kubectl get pod -l app=mtls-server -o jsonpath='{.items[0].metadata.name}')
go build -o bin/mtls-server ./examples/zeroconfig-example
kubectl cp bin/mtls-server "$POD":/tmp/mtls-server
kubectl exec "$POD" -- chmod +x /tmp/mtls-server
kubectl exec -it "$POD" -- /tmp/mtls-server
```

**Expected**:
- ✅ "Server created successfully"
- ✅ "Listening on :8443"
- ❌ No errors about socket connection

### Test 2: Server Endpoints

With the server running, test each endpoint:

```bash
# Health endpoint (should work without mTLS if configured)
curl -k https://localhost:8443/health

# Root endpoint (requires client cert)
# See section on curl/openssl testing
```

### Test 3: SPIFFE ID Extraction

**Goal**: Verify server correctly extracts and validates SPIFFE ID

Create a test client:

```go
package main

import (
    "context"
    "fmt"
    "io"
    "log"
    "time"

    "github.com/pocket/hexagon/spire/internal/adapters/outbound/httpclient"
    "github.com/pocket/hexagon/spire/internal/ports"
)

func main() {
    ctx := context.Background()

    cfg := ports.ClientConfig{
        WorkloadAPI: ports.WorkloadAPIConfig{
            SocketPath: "unix:///tmp/spire-agent/public/api.sock",
        },
        SPIFFE: ports.SPIFFEClientConfig{
            ExpectedServerID: "spiffe://example.org/server",
        },
        HTTP: ports.HTTPClientConfig{
            Timeout: 30 * time.Second,
        },
    }

    client, err := httpclient.NewSPIFFEClient(ctx, cfg)
    if err != nil {
        log.Fatal(err)
    }
    defer client.Close()

    resp, err := client.Get(ctx, "https://localhost:8443/api/identity")
    if err != nil {
        log.Fatal(err)
    }
    defer resp.Body.Close()

    body, _ := io.ReadAll(resp.Body)
    fmt.Println(string(body))
}
```

**Expected output**:
```
=== Client Identity Details ===
SPIFFE ID: spiffe://example.org/client
Trust Domain: example.org
Path: /client
```

Private keys are managed by the SDK's X509SVID and are not exposed in the domain model or HTTP responses.

### Test 4: Graceful Shutdown

**Goal**: Verify server shuts down gracefully

With server running:
```bash
# Send SIGTERM
pkill -TERM -f zeroconfig-example
```

**Expected**:
- ✅ "Shutting down server..."
- ✅ "Server resources released"
- ✅ "Server stopped gracefully"
- ❌ No panics or errors

---

## Testing the Identity Client

### Test 1: Basic Client Connection

This test runs inside a Kubernetes pod with access to the SPIRE agent socket.

```bash
# Deploy client (see Quick Start section)
kubectl apply -f examples/mtls-client.yaml
CLIENT_POD=$(kubectl get pod -l app=mtls-client -o jsonpath='{.items[0].metadata.name}')
go build -o bin/mtls-client ./examples/zeroconfig-example
kubectl cp bin/mtls-client "$CLIENT_POD":/tmp/mtls-client
kubectl exec "$CLIENT_POD" -- chmod +x /tmp/mtls-client
kubectl exec -it "$CLIENT_POD" -- /tmp/mtls-client
```

**Expected**:
- ✅ Connection successful
- ✅ Response received from server
- ❌ No TLS errors

### Test 2: Server Identity Verification

**Goal**: Verify client rejects connections to servers with wrong SPIFFE ID

```bash
# Set wrong server ID
export EXPECTED_SERVER_ID="spiffe://example.org/wrong"
go run main.go
```

**Expected**:
- ❌ Connection should fail
- Error message should mention identity verification failure

### Test 3: Client Timeout

**Goal**: Verify client respects timeout configuration

Modify client code to use short timeout:
```go
cfg.HTTP.Timeout = 1 * time.Millisecond
```

**Expected**:
- ❌ Request fails with timeout error
- Error message mentions "context deadline exceeded"

---

## Testing Error Cases

### Error Case 1: Missing SPIRE Agent

**Goal**: Verify graceful error when SPIRE agent is unavailable

```bash
# Scale down SPIRE agents
kubectl scale daemonset spire-agent -n spire-system --replicas=0

# Try to start server (will fail to connect to agent socket)
POD=$(kubectl get pod -l app=mtls-server -o jsonpath='{.items[0].metadata.name}')
kubectl exec -it "$POD" -- /tmp/mtls-server

# Restore agents
kubectl scale daemonset spire-agent -n spire-system --replicas=1
```

**Expected error**:
```
Failed to create server: create X509Source: connection error: ...
```

### Error Case 2: Invalid SPIFFE ID

**Goal**: Verify validation of SPIFFE ID format

Modify the server deployment to use an invalid SPIFFE ID:

```bash
# Edit the deployment and add invalid environment variable
kubectl set env deployment/mtls-server -n default ALLOWED_CLIENT_ID="not-a-valid-spiffe-id"

# Check pod logs for error
kubectl logs -l app=mtls-server --tail=50
```

**Expected error**:
```
Failed to create server: parse allowed peer ID: scheme is missing or invalid
```

### Error Case 3: Unauthorized Client

**Goal**: Verify server rejects clients with wrong SPIFFE ID

Registration entries are managed automatically by the Kubernetes setup. To test unauthorized access, you would need to:

1. Create a pod with a different workload selector
2. Register it with a different SPIFFE ID
3. Attempt connection from that pod

**Expected**:
- ❌ Connection rejected by server
- Server logs: "unexpected peer ID"

For detailed workload registration, see `examples/README.md`.

### Error Case 4: No TLS Connection

**Goal**: Verify server requires TLS

This is tested automatically in unit tests, but you can verify:

```go
// Try to connect without TLS (will fail at TCP level)
resp, err := http.Get("http://localhost:8443/")
// This will fail because server only accepts HTTPS
```

---

## Manual Testing with curl/openssl

### Extract Certificates from SPIRE

SPIRE provides certificates via the Workload API. To test with curl, you need to extract them from a pod:

```bash
# Exec into a workload pod with SPIRE access
POD=$(kubectl get pod -l app=mtls-server -o jsonpath='{.items[0].metadata.name}')

# Install spire-agent CLI in the pod (if not already present)
kubectl exec -it "$POD" -- sh -c "
  # Fetch certificates using the workload API socket
  # Note: This requires spire-agent CLI tools in the pod
  # For this example, certificates are managed by the go-spiffe SDK
"

# Alternative: Use go-spiffe SDK's debug endpoint if enabled
# Or extract from application logs in debug mode
```

**Note**: In production usage, certificates are managed automatically by the go-spiffe SDK and don't need manual extraction. This section is primarily for debugging TLS handshake issues.

### Test with curl (from within cluster)

Testing mTLS endpoints requires valid SPIFFE certificates, which are easiest to use from within the cluster:

```bash
# Port-forward the server to access from outside
kubectl port-forward svc/mtls-server 8443:8443 &

# Test from client pod (which has valid certificates via SPIRE)
CLIENT_POD=$(kubectl get pod -l app=mtls-client -o jsonpath='{.items[0].metadata.name}')

kubectl exec -it "$CLIENT_POD" -- sh -c '
  # Test health endpoint
  curl -k https://mtls-server:8443/health

  # Authenticated endpoints require certificates managed by application
  # Use the built binary instead of curl for proper mTLS
'
```

### Test with openssl s_client (TLS handshake inspection)

```bash
# Port-forward the service
kubectl port-forward svc/mtls-server 8443:8443

# In another terminal, inspect TLS handshake
# Note: This will fail at mTLS verification without valid client cert
openssl s_client -connect localhost:8443 -showcerts

# To see full mTLS with valid certs, exec into a pod with SPIRE access
kubectl exec -it "$CLIENT_POD" -- openssl s_client \
  -connect mtls-server:8443 -showcerts
```

---

## Verification Checklist

Use this checklist to verify full functionality:

### Server Tests
- [ ] Server starts successfully with valid configuration
- [ ] Server connects to SPIRE agent
- [ ] Server listens on configured port
- [ ] Server handles graceful shutdown (SIGTERM/SIGINT)
- [ ] Server closes all resources on shutdown
- [ ] Server rejects invalid SPIFFE ID configuration
- [ ] Server handles missing SPIRE agent gracefully

### Client Tests
- [ ] Client connects successfully with valid configuration
- [ ] Client sends requests with mTLS
- [ ] Client verifies server identity
- [ ] Client respects timeout configuration
- [ ] Client rejects servers with wrong SPIFFE ID
- [ ] Client handles connection errors gracefully

### Authentication Tests
- [ ] Server extracts SPIFFE ID from client certificate
- [ ] Server rejects clients with wrong SPIFFE ID
- [ ] Server allows clients with correct SPIFFE ID
- [ ] SPIFFE ID is available in request context
- [ ] Multiple concurrent clients work correctly

### Security Tests
- [ ] Server requires TLS (no plain HTTP)
- [ ] Server enforces ReadHeaderTimeout
- [ ] Server enforces WriteTimeout
- [ ] Server closes idle connections
- [ ] Client validates server certificate
- [ ] Client validates server SPIFFE ID

---

## Troubleshooting

### Issue: "connection refused"

**Cause**: Server not running or wrong port

**Fix**:
```bash
# Check if pod is running
kubectl get pod -l app=mtls-server

# Check pod logs
kubectl logs -l app=mtls-server --tail=50

# Check what's listening inside the pod
kubectl exec -it <pod-name> -- netstat -tlnp | grep 8443
```

### Issue: "create X509Source: connection error"

**Cause**: SPIRE agent not running or socket not mounted

**Fix**:
```bash
# Check SPIRE agent status
kubectl get pods -n spire-system -l app=spire-agent

# Verify socket is mounted in pod
kubectl exec -it <pod-name> -- ls -l /spire-socket/api.sock

# Check volume mount in deployment
kubectl describe pod <pod-name> | grep -A 5 "Mounts:"
```

### Issue: "unexpected peer ID"

**Cause**: Client has different SPIFFE ID than configured

**Fix**:
```bash
# List registration entries using kubectl exec into spire-server pod
kubectl exec -n spire-system spire-server-0 -c spire-server -- \
  /opt/spire/bin/spire-server entry show

# Check workload registration in examples/README.md
# Verify SPIFFE IDs match between server config and registration
```

### Issue: "certificate signed by unknown authority"

**Cause**: Client and server have different trust domains

**Fix**:
```bash
# Verify both pods connect to same SPIRE infrastructure
kubectl exec -n spire-system spire-server-0 -c spire-server -- \
  /opt/spire/bin/spire-server bundle show

# Check trust domain in SPIFFE IDs (should be example.org)
```

### Issue: Server panics on startup

**Cause**: Usually configuration validation failed

**Fix**:
```bash
# Check pod logs for detailed error
kubectl logs -l app=mtls-server --tail=100

# Check environment variables in pod
kubectl exec -it <pod-name> -- env | grep -E "(SPIRE|ALLOWED|SERVER)"

# Verify SPIFFE ID format in deployment YAML
# Format: spiffe://trust-domain/path
```

---

## Performance Testing

### Basic Load Test

```bash
# Install hey (HTTP load testing tool)
go install github.com/rakyll/hey@latest

# Run load test (requires client cert support)
# Note: hey doesn't support client certs easily, better to write custom Go tool
```

### Custom Load Test Tool

```go
package main

import (
    "context"
    "fmt"
    "sync"
    "time"

    "github.com/pocket/hexagon/spire/internal/adapters/outbound/httpclient"
    "github.com/pocket/hexagon/spire/internal/ports"
)

func main() {
    ctx := context.Background()
    cfg := ports.ClientConfig{
        // ... configuration
    }

    client, err := httpclient.NewSPIFFEClient(ctx, cfg)
    if err != nil {
        log.Fatalf("Failed to create client: %v", err)
    }
    defer client.Close()

    // Run 100 concurrent requests
    var wg sync.WaitGroup
    start := time.Now()

    for i := 0; i < 100; i++ {
        wg.Add(1)
        go func() {
            defer wg.Done()
            resp, err := client.Get(ctx, "https://localhost:8443/health")
            if err != nil {
                log.Printf("Request failed: %v", err)
                return
            }
            resp.Body.Close()
        }()
    }

    wg.Wait()
    elapsed := time.Since(start)

    fmt.Printf("100 requests completed in %v\n", elapsed)
    fmt.Printf("Average: %v per request\n", elapsed/100)
}
```

---

## Related Documentation

- [Port-Based Improvements](PORT_BASED_IMPROVEMENTS.md) - Architecture overview
- [Unified Configuration](UNIFIED_CONFIG_IMPROVEMENTS.md) - Configuration details
- [Example Server](../examples/zeroconfig-example/main.go) - Complete server example
- [SPIRE Documentation](https://spiffe.io/docs/latest/spire/) - Official SPIRE docs
