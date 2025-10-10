# mTLS Service-to-Service Communication Examples

This directory demonstrates end-to-end mTLS communication between services using the **adapter pattern** with `httpapi` (server) and `httpclient` (client).

## Overview

These examples show how to:
- Create an mTLS server that authenticates clients using X.509 SVIDs
- Create an mTLS client that authenticates servers using X.509 SVIDs
- Extract and use client identity in handlers
- Use identity utilities for path-based verification
- Handle graceful shutdown and proper resource cleanup

## Architecture

```
┌─────────────────────────────────────────────────────────────────────┐
│                     Client Application                               │
│                                                                      │
│  httpclient.NewSPIFFEHTTPClient(ctx, ClientConfig{...})             │
│       ↓                                                              │
│  client.Get(ctx, "https://server:8443/api/hello")                   │
└───────────────────────────┬─────────────────────────────────────────┘
                            │
                            │ 1. Fetch client SVID
                            ↓
        ┌─────────────────────────────────────────┐
        │    SPIRE Agent (Workload API)           │
        │    unix:///tmp/spire-agent/public/api.sock│
        └─────────────────────────────────────────┘
                            │
                            │ 2. mTLS handshake (mutual auth)
                            ↓
┌─────────────────────────────────────────────────────────────────────┐
│                     Server Application                               │
│                                                                      │
│  httpapi.NewHTTPServer(ctx, ServerConfig{...})                      │
│       ↓                                                              │
│  server.RegisterHandler("/api/hello", handler)                      │
│       ↓                                                              │
│  func handler(w, r) {                                                │
│      clientID, _ := httpapi.GetSPIFFEID(r)  // Extract identity     │
│      // Handle authenticated request                                │
│  }                                                                   │
└─────────────────────────────────────────────────────────────────────┘
```

## Files

- **[server/main.go](server/main.go)** - mTLS HTTP server using `httpapi` adapter
- **[client/main.go](client/main.go)** - mTLS HTTP client using `httpclient` adapter
- **[README.md](README.md)** - This file (overview and local development)
- **[KUBERNETES.md](KUBERNETES.md)** - Kubernetes deployment guide
- **[TROUBLESHOOTING.md](TROUBLESHOOTING.md)** - Troubleshooting guide

## Prerequisites

### Option 1: Local Development (Requires SPIRE)

1. **SPIRE running locally**:
   ```bash
   # Install SPIRE: https://spiffe.io/docs/latest/spire/installing/
   # Or use Docker:
   docker run -d --name spire-server spiffe/spire-server:latest
   docker run -d --name spire-agent spiffe/spire-agent:latest
   ```

2. **Register workloads**:
   ```bash
   # Register server workload
   spire-server entry create \
     -spiffeID spiffe://example.org/server \
     -parentID spiffe://example.org/agent \
     -selector unix:uid:$(id -u)

   # Register client workload
   spire-server entry create \
     -spiffeID spiffe://example.org/client \
     -parentID spiffe://example.org/agent \
     -selector unix:uid:$(id -u)
   ```

### Option 2: Kubernetes with Minikube

```bash
# Start Minikube with SPIRE
make minikube-up

# Register test workloads
make register-mtls-workloads
```

## Running the Examples

### Option 1: Run Locally

#### Terminal 1: Start Server

```bash
# Build and run server
go build -o bin/mtls-server ./examples/mtls-adapters/server
./bin/mtls-server

# Or run directly
go run ./examples/mtls-adapters/server
```

**Output**:
```
Starting mTLS server with configuration:
  Socket: unix:///tmp/spire-agent/public/api.sock
  Address: :8443
  Allowed client: any from trust domain
✓ Server started successfully on :8443
Waiting for requests (Ctrl+C to stop)...
```

#### Terminal 2: Run Client

```bash
# Build and run client
go build -o bin/mtls-client ./examples/mtls-adapters/client
./bin/mtls-client

# Or run directly
go run ./examples/mtls-adapters/client
```

**Output**:
```
Creating mTLS client with configuration:
  Socket: unix:///tmp/spire-agent/public/api.sock
  Server URL: https://localhost:8443
  Expected server: any from trust domain
✓ Client created successfully

=== Making GET request to /api/hello ===
GET https://localhost:8443/api/hello
Status: 200 OK
Response:
Hello from mTLS server!
Authenticated client: spiffe://example.org/client

✓ Request succeeded
...
```

### Option 2: Run in Kubernetes

See the [Kubernetes Deployment Guide](KUBERNETES.md) for detailed instructions on deploying to Kubernetes with Minikube.

## Configuration

### Server Configuration

Environment variables:
- `SPIRE_AGENT_SOCKET` - Path to SPIRE agent socket (default: `unix:///tmp/spire-agent/public/api.sock`)
- `SERVER_ADDRESS` - Address to listen on (default: `:8443`)
- `ALLOWED_CLIENT_ID` - Restrict to specific client SPIFFE ID (optional, default: any from trust domain)

Example:
```bash
SPIRE_AGENT_SOCKET="unix:///var/run/spire/agent.sock" \
SERVER_ADDRESS=":9443" \
ALLOWED_CLIENT_ID="spiffe://example.org/client" \
./bin/mtls-server
```

### Client Configuration

Environment variables:
- `SPIRE_AGENT_SOCKET` - Path to SPIRE agent socket (default: `unix:///tmp/spire-agent/public/api.sock`)
- `SERVER_URL` - Server URL to connect to (default: `https://localhost:8443`)
- `EXPECTED_SERVER_ID` - Expected server SPIFFE ID (optional, default: any from trust domain)

Example:
```bash
SPIRE_AGENT_SOCKET="unix:///var/run/spire/agent.sock" \
SERVER_URL="https://api-server:8443" \
EXPECTED_SERVER_ID="spiffe://example.org/server" \
./bin/mtls-client
```

## Endpoints

The server exposes the following endpoints:

### GET /api/hello

Returns a greeting with the authenticated client's identity.

**Example**:
```bash
curl --cert client.crt --key client.key https://localhost:8443/api/hello
```

**Response**:
```
Hello from mTLS server!
Authenticated client: spiffe://example.org/client
```

### GET /api/echo

Echoes back request details including client identity.

**Response**:
```
Echo from server
Client: spiffe://example.org/client
Method: GET
Path: /api/echo
```

### GET /api/identity

Returns detailed identity information using identity extraction utilities.

**Response**:
```
Identity Details
================
Full ID: spiffe://example.org/client
Trust Domain: example.org
Path: /client
Path Segments: [client]

Path Checks:
  Has /service/ prefix: false
  Has /admin suffix: false
  Matches trust domain 'example.org': true
```

### GET /health

Health check endpoint (still requires mTLS).

**Response**:
```
OK
```

## Identity Extraction Examples

The server demonstrates various identity extraction utilities from Iteration 3:

### Basic Extraction

```go
// Get SPIFFE ID
clientID, ok := httpapi.GetSPIFFEID(r)
if !ok {
    http.Error(w, "No client identity", http.StatusUnauthorized)
    return
}
```

### Trust Domain Verification

```go
// Check trust domain
if !httpapi.MatchesTrustDomain(r, "example.org") {
    http.Error(w, "Must be from example.org", http.StatusForbidden)
    return
}
```

### Path-Based Checks

```go
// Check if client is a service
if httpapi.HasPathPrefix(r, "/service/") {
    // Handle service-to-service request
}

// Check for admin role (application-defined)
if httpapi.HasPathSuffix(r, "/admin") {
    // Handle admin request
}
```

### Path Segments

```go
// Get path components
segments, ok := httpapi.GetPathSegments(r)
// For spiffe://example.org/ns/prod/service/api
// segments = []string{"ns", "prod", "service", "api"}

if len(segments) >= 2 {
    namespace := segments[0]
    environment := segments[1]
    // Use for routing decisions
}
```

## Kubernetes Deployment

For complete Kubernetes deployment instructions including:
- Deployment manifests (server and client)
- Building images for Minikube and remote registries
- Workload registration with SPIRE
- Advanced configuration (ConfigMaps, Secrets, replicas)
- Verification and cleanup

See the **[Kubernetes Deployment Guide](KUBERNETES.md)**.

## Testing Identity Mismatch

To test that the server rejects clients with wrong SPIFFE ID:

```bash
# Start server requiring specific client
ALLOWED_CLIENT_ID="spiffe://example.org/specific-client" ./bin/mtls-server

# Run client with different identity (will fail with TLS error)
./bin/mtls-client
```

**Expected**: TLS handshake failure with "bad certificate" or "certificate verify failed"

## Troubleshooting

Having issues? See the **[Troubleshooting Guide](TROUBLESHOOTING.md)** for detailed solutions to common problems:

- Connection issues (socket access, connection refused)
- SPIRE agent issues (registration, permissions)
- TLS handshake failures (authentication, certificate validation)
- Kubernetes-specific issues (pod access, image pull errors)
- Debugging tips and tools

## Security Considerations

### Authentication vs Authorization

These examples demonstrate **authentication only**:
- ✅ Server verifies client identity using mTLS
- ✅ Client verifies server identity using mTLS
- ✅ Identity exposed to application layer

Authorization (access control) is **application responsibility**:
- ❌ No role-based access control (RBAC)
- ❌ No resource-level permissions
- ❌ No policy enforcement

**Example authorization** (implement in your application):
```go
func handler(w http.ResponseWriter, r *http.Request) {
    clientID, _ := httpapi.GetSPIFFEID(r)

    // Application implements authorization
    if !myAuthzService.IsAllowed(clientID, "read", "resource") {
        http.Error(w, "Forbidden", http.StatusForbidden)
        return
    }

    // Handle request...
}
```

### Certificate Rotation

- SVIDs automatically rotate (default TTL: 1 hour)
- Zero-downtime rotation via `workloadapi.X509Source`
- No manual certificate management required

### Production Considerations

1. **Logging**: Add structured logging for security events
2. **Metrics**: Monitor mTLS handshake failures, latency
3. **Rate Limiting**: Implement at application layer
4. **Error Handling**: Don't leak identity information in errors
5. **Timeouts**: Configure appropriate timeouts for your use case

## Next Steps

- Add application-level authorization
- Implement metrics and monitoring
- Add more complex multi-service examples
- Configure SPIRE federation for multi-cluster

## Additional Documentation

- **[KUBERNETES.md](KUBERNETES.md)** - Complete Kubernetes deployment guide with manifests, building images, and workload registration
- **[TROUBLESHOOTING.md](TROUBLESHOOTING.md)** - Comprehensive troubleshooting guide for common issues

## References

- [ITERATION_1_COMPLETE.md](../../docs/ITERATION_1_COMPLETE.md) - Server implementation
- [ITERATION_2_COMPLETE.md](../../docs/ITERATION_2_COMPLETE.md) - Client implementation
- [ITERATION_3_COMPLETE.md](../../docs/ITERATION_3_COMPLETE.md) - Identity utilities
- [MTLS_IMPLEMENTATION.md](../../docs/MTLS_IMPLEMENTATION.md) - Implementation plan
- [SPIFFE/SPIRE Documentation](https://spiffe.io/docs/)
- [go-spiffe SDK](https://github.com/spiffe/go-spiffe)
