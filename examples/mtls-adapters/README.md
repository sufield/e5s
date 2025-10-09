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
- **[README.md](README.md)** - This file

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

See [Kubernetes Deployment](#kubernetes-deployment) section below.

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

### Deployment Manifests

#### Server Deployment

```yaml
# server-deployment.yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: mtls-server
  namespace: default
spec:
  replicas: 1
  selector:
    matchLabels:
      app: mtls-server
  template:
    metadata:
      labels:
        app: mtls-server
    spec:
      containers:
      - name: server
        image: mtls-server:latest
        ports:
        - containerPort: 8443
          name: https
        env:
        - name: SPIRE_AGENT_SOCKET
          value: "unix:///spire-agent-socket/api.sock"
        - name: SERVER_ADDRESS
          value: ":8443"
        volumeMounts:
        - name: spire-agent-socket
          mountPath: /spire-agent-socket
          readOnly: true
      volumes:
      - name: spire-agent-socket
        hostPath:
          path: /run/spire/sockets
          type: Directory
---
apiVersion: v1
kind: Service
metadata:
  name: mtls-server
  namespace: default
spec:
  selector:
    app: mtls-server
  ports:
  - port: 8443
    targetPort: 8443
    name: https
```

#### Client Job

```yaml
# client-job.yaml
apiVersion: batch/v1
kind: Job
metadata:
  name: mtls-client
  namespace: default
spec:
  template:
    metadata:
      labels:
        app: mtls-client
    spec:
      containers:
      - name: client
        image: mtls-client:latest
        env:
        - name: SPIRE_AGENT_SOCKET
          value: "unix:///spire-agent-socket/api.sock"
        - name: SERVER_URL
          value: "https://mtls-server:8443"
        volumeMounts:
        - name: spire-agent-socket
          mountPath: /spire-agent-socket
          readOnly: true
      volumes:
      - name: spire-agent-socket
        hostPath:
          path: /run/spire/sockets
          type: Directory
      restartPolicy: OnFailure
```

### Deploying to Kubernetes

```bash
# Build and load images into Minikube
eval $(minikube docker-env)

# Build server image
docker build -t mtls-server:latest -f examples/mtls-adapters/server/Dockerfile .

# Build client image
docker build -t mtls-client:latest -f examples/mtls-adapters/client/Dockerfile .

# Deploy
kubectl apply -f examples/mtls-adapters/k8s/server-deployment.yaml
kubectl apply -f examples/mtls-adapters/k8s/client-job.yaml

# View logs
kubectl logs -l app=mtls-server
kubectl logs job/mtls-client
```

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

### "Failed to create X509Source: context deadline exceeded"

**Problem**: Cannot connect to SPIRE agent socket.

**Solution**:
1. Check SPIRE agent is running
2. Verify socket path is correct
3. Check file permissions

```bash
# Check socket exists
ls -la /tmp/spire-agent/public/api.sock

# Check SPIRE agent is running
ps aux | grep spire-agent
```

### "No identity issued" or "no such registration entry"

**Problem**: Workload not registered in SPIRE.

**Solution**: Register the workload:
```bash
spire-server entry create \
  -spiffeID spiffe://example.org/myservice \
  -parentID spiffe://example.org/agent \
  -selector unix:uid:$(id -u)

# Verify registration
spire-server entry show
```

### "TLS handshake failed"

**Problem**: mTLS authentication failed.

**Possible causes**:
1. Client and server not in same trust domain
2. Server's `ALLOWED_CLIENT_ID` doesn't match client
3. Client's `EXPECTED_SERVER_ID` doesn't match server
4. SVID expired (check SPIRE server logs)

**Solution**:
```bash
# Check both can fetch SVIDs
SPIRE_AGENT_SOCKET=unix:///tmp/spire-agent/public/api.sock \
  spire-agent api fetch x509

# Check authorizer configuration matches
```

### Connection Refused

**Problem**: Server not reachable.

**Solution**:
1. Verify server is running: `ps aux | grep mtls-server`
2. Check server address: `netstat -tlnp | grep 8443`
3. In Kubernetes: `kubectl get svc mtls-server`

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

## References

- [ITERATION_1_COMPLETE.md](../../docs/ITERATION_1_COMPLETE.md) - Server implementation
- [ITERATION_2_COMPLETE.md](../../docs/ITERATION_2_COMPLETE.md) - Client implementation
- [ITERATION_3_COMPLETE.md](../../docs/ITERATION_3_COMPLETE.md) - Identity utilities
- [MTLS_IMPLEMENTATION.md](../../docs/MTLS_IMPLEMENTATION.md) - Implementation plan
- [SPIFFE/SPIRE Documentation](https://spiffe.io/docs/)
- [go-spiffe SDK](https://github.com/spiffe/go-spiffe)
