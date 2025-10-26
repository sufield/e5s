# Testing Guide - SPIRE mTLS

This guide includes scenarios and verification procedures for QA and validation of the SPIRE mTLS server and client functionality.

## Prerequisites

- SPIRE infrastructure deployed (see [Manual Testing Guide](MANUAL_TESTING_GUIDE.md))
- `kubectl` and `minikube` configured
- Test workloads deployed (mtls-server and test-client)

---

## Server Testing

### Test 1: Basic Server Startup

**Goal**: Verify server starts and connects to SPIRE agent

**Setup**:
```bash
# Ensure SPIRE infrastructure is running
make minikube-up

# Deploy the server
kubectl apply -f examples/mtls-server.yaml
POD=$(kubectl get pod -l app=mtls-server -o jsonpath='{.items[0].metadata.name}')
go build -o bin/mtls-server ./examples/zeroconfig-example
kubectl cp bin/mtls-server "$POD":/tmp/mtls-server
kubectl exec "$POD" -- chmod +x /tmp/mtls-server
```

**Execution**:
```bash
kubectl exec -it "$POD" -- /tmp/mtls-server
```

**Expected Results**:
- ✅ "Server created successfully"
- ✅ "Listening on :8443"
- ✅ "Configuration detected successfully"
- ✅ Trust Domain shown as "example.org"
- ❌ No errors about socket connection

**Pass Criteria**: Server starts without errors and displays all success messages.

---

### Test 2: Server Endpoints

**Goal**: Verify all server endpoints are accessible

**Prerequisites**: Server running from Test 1

**Execution**:
```bash
CLIENT_POD=$(kubectl get pod -l app=test-client -o jsonpath='{.items[0].metadata.name}')

# Test health endpoint
kubectl exec "$CLIENT_POD" -- curl -k https://mtls-server:8443/health

# Test authenticated endpoints (requires valid client cert)
go build -o /tmp/test-client examples/test-client.go
kubectl cp /tmp/test-client "$CLIENT_POD":/tmp/test-client
kubectl exec "$CLIENT_POD" -- chmod +x /tmp/test-client
kubectl exec "$CLIENT_POD" -- /tmp/test-client
```

**Expected Results**:
- Health endpoint returns: `{"status":"ok"}`
- Root endpoint (/) returns: `Success! Authenticated as: spiffe://example.org/client`
- /api/hello returns: `Success! Authenticated as: spiffe://example.org/client`
- /api/identity returns client identity details
- All responses have HTTP status 200

**Pass Criteria**: All endpoints return expected responses with status 200.

---

### Test 3: SPIFFE ID Extraction

**Goal**: Verify server correctly extracts and validates SPIFFE ID from client certificates

**Setup**:
Create a test client (or use the example from Quick Start).

**Execution**:
```bash
kubectl exec "$CLIENT_POD" -- /tmp/test-client
```

**Expected Results**:
Server logs should show:
```
Received request from: spiffe://example.org/client
```

Client receives response:
```
=== Client Identity Details ===
SPIFFE ID: spiffe://example.org/client
Trust Domain: example.org
Path: /client
```

**Pass Criteria**:
- Server correctly extracts SPIFFE ID from client certificate
- Client identity information matches registration entry
- No authentication errors

Private keys are managed by the SDK's X509SVID and are not exposed in domain model or HTTP responses.

---

### Test 4: Graceful Shutdown

**Goal**: Verify server shuts down gracefully without resource leaks

**Prerequisites**: Server running

**Execution**:
```bash
# Get the PID of the server process inside the pod
kubectl exec "$POD" -- ps aux | grep mtls-server

# Send SIGTERM
kubectl exec "$POD" -- kill -TERM <PID>
```

**Expected Results**:
```
Shutting down server...
Server resources released
Server stopped gracefully
```

**Pass Criteria**:
- ✅ Graceful shutdown messages appear
- ✅ No panics or errors during shutdown
- ✅ All connections closed properly
- ✅ Process exits cleanly

---

## Client Testing

### Test 1: Basic Client Connection

**Goal**: Verify client can establish mTLS connection to server

**Prerequisites**: Server running

**Setup**:
```bash
kubectl apply -f examples/test-client.yaml
go build -o /tmp/test-client examples/test-client.go
CLIENT_POD=$(kubectl get pod -l app=test-client -o jsonpath='{.items[0].metadata.name}')
kubectl cp /tmp/test-client "$CLIENT_POD":/tmp/test-client
kubectl exec "$CLIENT_POD" -- chmod +x /tmp/test-client
```

**Execution**:
```bash
kubectl exec "$CLIENT_POD" -- /tmp/test-client
```

**Expected Results**:
- ✅ All endpoints return Status: 200
- ✅ Response bodies contain expected content
- ✅ Client successfully authenticates with server
- ❌ No TLS errors or connection failures

**Pass Criteria**: All test requests succeed without errors.

---

### Test 2: Server Identity Verification

**Goal**: Verify client rejects connections to servers with wrong SPIFFE ID

**Setup**:
Modify the test client to expect a different server ID:
```go
cfg.SPIFFE.ExpectedServerID = "spiffe://example.org/wrong-server"
```

**Execution**:
Run the modified client.

**Expected Results**:
- ❌ Connection fails
- Error message contains: "unexpected peer ID" or "identity verification failure"
- Client does not proceed with request

**Pass Criteria**: Client correctly rejects server with mismatched SPIFFE ID.

---

### Test 3: Client Timeout Handling

**Goal**: Verify client respects timeout configuration

**Setup**:
Modify client code to use very short timeout:
```go
cfg.HTTP.Timeout = 1 * time.Millisecond
```

**Execution**:
Run the modified client.

**Expected Results**:
- ❌ Request fails with timeout error
- Error message mentions "context deadline exceeded" or "timeout"
- Client handles timeout gracefully without panic

**Pass Criteria**: Timeout is enforced and error is handled properly.

---

## Error Case Testing

### Error Case 1: Missing SPIRE Agent

**Goal**: Verify graceful error when SPIRE agent is unavailable

**Setup**:
```bash
# Scale down SPIRE agents
kubectl scale daemonset spire-agent -n spire-system --replicas=0
```

**Execution**:
```bash
POD=$(kubectl get pod -l app=mtls-server -o jsonpath='{.items[0].metadata.name}')
kubectl exec -it "$POD" -- /tmp/mtls-server
```

**Expected Results**:
```
Failed to create server: create X509Source: connection error: ...
```

**Cleanup**:
```bash
# Restore agents
kubectl scale daemonset spire-agent -n spire-system --replicas=1
```

**Pass Criteria**: Clear error message indicating SPIRE agent connection failure.

---

### Error Case 2: Invalid SPIFFE ID Format

**Goal**: Verify validation of SPIFFE ID format

**Setup**:
```bash
# Edit deployment with invalid SPIFFE ID
kubectl set env deployment/mtls-server ALLOWED_CLIENT_ID="not-a-valid-spiffe-id"
```

**Execution**:
Wait for pod to restart and check logs:
```bash
kubectl logs -l app=mtls-server --tail=50
```

**Expected Results**:
```
Failed to create server: parse allowed peer ID: scheme is missing or invalid
```

**Cleanup**:
```bash
kubectl set env deployment/mtls-server ALLOWED_CLIENT_ID-
kubectl rollout restart deployment/mtls-server
```

**Pass Criteria**: Server rejects invalid SPIFFE ID format with clear error.

---

### Error Case 3: Unauthorized Client

**Goal**: Verify server rejects clients with wrong SPIFFE ID

**Setup**:
1. Create a pod with different workload selector
2. Register it with a different SPIFFE ID (not the allowed one)
3. Attempt connection from that pod

**Expected Results**:
- ❌ Connection rejected by server
- Server logs show: "unexpected peer ID"
- Client receives authentication error

**Pass Criteria**: Server correctly rejects unauthorized client.

For detailed workload registration, see `examples/README.md`.

---

### Error Case 4: Non-TLS Connection Attempt

**Goal**: Verify server requires TLS

**Note**: This is tested in unit tests. Manual verification:

```go
// Try HTTP instead of HTTPS (will fail)
resp, err := http.Get("http://mtls-server:8443/")
```

**Expected Results**:
- Connection fails (server only accepts HTTPS)
- Error indicates TLS required

**Pass Criteria**: Plain HTTP connections are rejected.

---

## Performance Testing

### Basic Load Test

**Goal**: Verify server can handle concurrent requests

**Setup**:
Create a load test tool:

```go
package main

import (
    "context"
    "fmt"
    "log"
    "sync"
    "time"

    "github.com/pocket/hexagon/spire/internal/adapters/outbound/httpclient"
    "github.com/pocket/hexagon/spire/internal/ports"
)

func main() {
    ctx := context.Background()
    cfg := ports.ClientConfig{
        WorkloadAPI: ports.WorkloadAPIConfig{
            SocketPath: "unix:///spire-socket/api.sock",
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
        log.Fatalf("Failed to create client: %v", err)
    }
    defer client.Close()

    // Run 100 concurrent requests
    var wg sync.WaitGroup
    start := time.Now()
    errors := 0

    for i := 0; i < 100; i++ {
        wg.Add(1)
        go func(id int) {
            defer wg.Done()
            resp, err := client.Get(ctx, "https://mtls-server:8443/health")
            if err != nil {
                log.Printf("Request %d failed: %v", id, err)
                errors++
                return
            }
            resp.Body.Close()
        }(i)
    }

    wg.Wait()
    elapsed := time.Since(start)

    fmt.Printf("100 requests completed in %v\n", elapsed)
    fmt.Printf("Average: %v per request\n", elapsed/100)
    fmt.Printf("Errors: %d\n", errors)
}
```

**Execution**:
Build and run the load test tool from within a client pod.

**Expected Results**:
- All 100 requests complete successfully
- No connection errors
- Average response time under acceptable threshold (e.g., < 100ms for health endpoint)
- Error count: 0

**Pass Criteria**:
- 100% success rate
- Acceptable average response time
- No resource exhaustion

---

## Verification Checklist

Use this checklist to verify full functionality:

### Server Tests
- [ ] Server starts successfully with valid configuration
- [ ] Server connects to SPIRE agent
- [ ] Server listens on configured port (8443)
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
- [ ] Server closes idle connections appropriately
- [ ] Client validates server certificate
- [ ] Client validates server SPIFFE ID

### Integration Tests
- [ ] End-to-end request flow works
- [ ] Certificate rotation handled gracefully
- [ ] Multiple endpoints accessible
- [ ] Error responses formatted correctly
- [ ] Logging provides useful information

---

## Test Reports

After completing tests, document results:

### Test Summary Template

```
Test Run: [Date/Time]
Environment: [Minikube/Production/Staging]
SPIRE Version: [Version]
Application Version: [Commit Hash]

Server Tests: [Pass/Fail Count]
Client Tests: [Pass/Fail Count]
Error Cases: [Pass/Fail Count]
Performance: [Pass/Fail]

Issues Found:
- [Description]
- [Description]

Notes:
[Additional observations]
```

---

## Related Documentation

- [Manual Testing Guide](MANUAL_TESTING_GUIDE.md) - Quick start and setup
- [Troubleshooting Guide](TROUBLESHOOTING.md) - Diagnosing issues
- [Example Code](../examples/) - Reference implementations
- [SPIRE Documentation](https://spiffe.io/docs/latest/spire/) - Official SPIRE docs
