# Manual Testing Guide - mTLS Identity Server & Client

This guide walks you through manually testing the mTLS identity server and client functionality with SPIRE.

## Prerequisites

Before testing, ensure you have:

- Go 1.21 or later
- SPIRE server and agent installed
- `kubectl` (for Minikube testing)
- `minikube` (optional, for local Kubernetes testing)

## Table of Contents

1. [Quick Start (Local SPIRE)](#quick-start-local-spire)
2. [Minikube Testing](#minikube-testing)
3. [Testing the Identity Server](#testing-the-identity-server)
4. [Testing the Identity Client](#testing-the-identity-client)
5. [Testing Error Cases](#testing-error-cases)
6. [Manual Testing with curl/openssl](#manual-testing-with-curlopenssl)

---

## Quick Start (Local SPIRE)

### Step 1: Start SPIRE Server and Agent

```bash
# Start SPIRE server (in terminal 1)
spire-server run -config /path/to/server.conf

# Start SPIRE agent (in terminal 2)
spire-agent run -config /path/to/agent.conf
```

### Step 2: Create Registration Entries

```bash
# Register the server workload
spire-server entry create \
  -spiffeID spiffe://example.org/server \
  -parentID spiffe://example.org/agent \
  -selector unix:uid:$(id -u)

# Register the client workload
spire-server entry create \
  -spiffeID spiffe://example.org/client \
  -parentID spiffe://example.org/agent \
  -selector unix:uid:$(id -u)
```

### Step 3: Run the Example Server

```bash
cd examples/identityserver-example

# Set environment variables
export SPIRE_AGENT_SOCKET="unix:///tmp/spire-agent/public/api.sock"
export ALLOWED_CLIENT_ID="spiffe://example.org/client"
export SERVER_ADDRESS=":8443"

# Run the server
go run main.go
```

You should see:
```
Creating mTLS server with configuration:
  Socket: unix:///tmp/spire-agent/public/api.sock
  Address: :8443
  Allowed client: spiffe://example.org/client
✓ Server created successfully
Listening on :8443 with mTLS authentication
```

### Step 4: Test with Example Client

In a new terminal:

```bash
cd examples/mtls-adapters/client

# Set environment variables
export SPIRE_AGENT_SOCKET="unix:///tmp/spire-agent/public/api.sock"
export SERVER_URL="https://localhost:8443"
export EXPECTED_SERVER_ID="spiffe://example.org/server"

# Run the client
go run main.go
```

Expected output:
```
✓ Connection successful
Response: Success! Authenticated as: spiffe://example.org/client
```

---

## Minikube Testing

### Step 1: Start Minikube and Deploy SPIRE

```bash
# Start Minikube
minikube start

# Deploy SPIRE using the dev bootstrap
cd /path/to/spire
IDP_MODE=inmem go run ./cmd/console

# In the console, run:
bootstrap-minikube-infra
```

### Step 2: Deploy Test Workloads

```bash
# Build and deploy the server
kubectl apply -f infra/dev/minikube/test-server.yaml

# Build and deploy the client
kubectl apply -f infra/dev/minikube/test-client.yaml
```

### Step 3: Verify Deployment

```bash
# Check server logs
kubectl logs -n spire-system deployment/test-server -f

# Check client logs
kubectl logs -n spire-system deployment/test-client -f
```

### Step 4: Test Connectivity

```bash
# Port-forward to access server
kubectl port-forward -n spire-system svc/test-server 8443:8443

# In another terminal, test with curl (requires client cert)
# See "Manual Testing with curl/openssl" section below
```

---

## Testing the Identity Server

### Test 1: Basic Server Startup

**Goal**: Verify server starts and connects to SPIRE agent

```bash
cd examples/identityserver-example
export SPIRE_AGENT_SOCKET="unix:///tmp/spire-agent/public/api.sock"
export ALLOWED_CLIENT_ID="spiffe://example.org/client"
go run main.go
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
    "log"
    "net/http"

    "github.com/pocket/hexagon/spire/internal/adapters/outbound/httpclient"
    "github.com/pocket/hexagon/spire/internal/ports"
    "github.com/spiffe/go-spiffe/v2/spiffeid"
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

### Test 4: Graceful Shutdown

**Goal**: Verify server shuts down gracefully

With server running:
```bash
# Send SIGTERM
pkill -TERM -f identityserver-example
```

**Expected**:
- ✅ "Shutting down server..."
- ✅ "Server resources released"
- ✅ "Server stopped gracefully"
- ❌ No panics or errors

---

## Testing the Identity Client

### Test 1: Basic Client Connection

```bash
cd examples/mtls-adapters/client
export SPIRE_AGENT_SOCKET="unix:///tmp/spire-agent/public/api.sock"
export SERVER_URL="https://localhost:8443"
export EXPECTED_SERVER_ID="spiffe://example.org/server"
go run main.go
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
# Stop SPIRE agent
pkill spire-agent

# Try to start server
cd examples/identityserver-example
go run main.go
```

**Expected error**:
```
Failed to create server: create X509Source: connection error: ...
```

### Error Case 2: Invalid SPIFFE ID

**Goal**: Verify validation of SPIFFE ID format

```bash
export ALLOWED_CLIENT_ID="not-a-valid-spiffe-id"
go run main.go
```

**Expected error**:
```
Failed to create server: parse allowed peer ID: scheme is missing or invalid
```

### Error Case 3: Unauthorized Client

**Goal**: Verify server rejects clients with wrong SPIFFE ID

1. Register a different client:
```bash
spire-server entry create \
  -spiffeID spiffe://example.org/unauthorized \
  -parentID spiffe://example.org/agent \
  -selector unix:uid:$(id -u)
```

2. Update client to use unauthorized ID (modify registration entry)

3. Try to connect

**Expected**:
- ❌ Connection rejected by server
- Server logs: "unexpected peer ID"

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

SPIRE provides certificates via the Workload API. To test with curl, you need to extract them:

```bash
# Get the X.509 SVID and key
spire-agent api fetch x509 -socketPath /tmp/spire-agent/public/api.sock -write /tmp/certs/

# This creates:
# /tmp/certs/svid.0.pem        (certificate)
# /tmp/certs/svid.0.key        (private key)
# /tmp/certs/bundle.0.pem      (trust bundle)
```

### Test with curl

```bash
# Test health endpoint (may not require client cert depending on config)
curl -k https://localhost:8443/health

# Test authenticated endpoint
curl -k \
  --cert /tmp/certs/svid.0.pem \
  --key /tmp/certs/svid.0.key \
  --cacert /tmp/certs/bundle.0.pem \
  https://localhost:8443/

# Test API endpoints
curl -k \
  --cert /tmp/certs/svid.0.pem \
  --key /tmp/certs/svid.0.key \
  https://localhost:8443/api/hello

curl -k \
  --cert /tmp/certs/svid.0.pem \
  --key /tmp/certs/svid.0.key \
  https://localhost:8443/api/identity
```

### Test with openssl s_client

```bash
# Connect and see TLS handshake details
openssl s_client \
  -connect localhost:8443 \
  -cert /tmp/certs/svid.0.pem \
  -key /tmp/certs/svid.0.key \
  -CAfile /tmp/certs/bundle.0.pem

# Once connected, type:
GET / HTTP/1.1
Host: localhost:8443

# Press Enter twice to send request
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
# Check if server is running
ps aux | grep identityserver-example

# Check what's listening on port 8443
lsof -i :8443
```

### Issue: "create X509Source: connection error"

**Cause**: SPIRE agent not running or wrong socket path

**Fix**:
```bash
# Check SPIRE agent status
ps aux | grep spire-agent

# Verify socket exists
ls -l /tmp/spire-agent/public/api.sock

# Check socket path in configuration
echo $SPIRE_AGENT_SOCKET
```

### Issue: "unexpected peer ID"

**Cause**: Client has different SPIFFE ID than configured

**Fix**:
```bash
# List registration entries
spire-server entry show

# Verify client's SPIFFE ID matches ALLOWED_CLIENT_ID
# Update registration or environment variable
```

### Issue: "certificate signed by unknown authority"

**Cause**: Client and server have different trust domains

**Fix**:
```bash
# Verify both use same SPIRE server
# Check trust domain matches in SPIFFE IDs
```

### Issue: Server panics on startup

**Cause**: Usually configuration validation failed

**Fix**:
```bash
# Check all required environment variables are set
env | grep SPIRE
env | grep ALLOWED

# Verify SPIFFE ID format
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

    client, _ := httpclient.NewSPIFFEClient(ctx, cfg)
    defer client.Close()

    // Run 100 concurrent requests
    var wg sync.WaitGroup
    start := time.Now()

    for i := 0; i < 100; i++ {
        wg.Add(1)
        go func() {
            defer wg.Done()
            client.Get(ctx, "https://localhost:8443/health")
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
- [Example Server](../examples/identityserver-example/main.go) - Complete server example
- [SPIRE Documentation](https://spiffe.io/docs/latest/spire/) - Official SPIRE docs
