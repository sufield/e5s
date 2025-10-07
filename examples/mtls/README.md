# mTLS Service-to-Service Example

This example demonstrates two services communicating using mTLS with X.509 SVIDs for authentication.

## Architecture

```
┌──────────────────┐           mTLS           ┌──────────────────┐
│  Client Service  │ ───────────────────────> │  Server Service  │
│                  │   (X.509 SVID auth)      │                  │
│  - Fetches SVID  │                          │  - Fetches SVID  │
│  - Presents cert │                          │  - Presents cert │
│  - Verifies srv  │                          │  - Verifies cli  │
└────────┬─────────┘                          └────────┬─────────┘
         │                                             │
         │           ┌──────────────────┐             │
         └──────────>│   SPIRE Agent    │<────────────┘
                     │  (Unix Socket)   │
                     └──────────────────┘
```

## Components

### Server ([server/main.go](server/main.go))
- Creates mTLS HTTP server using `identityserver.Server`
- Authenticates clients using X.509 SVIDs
- Exposes endpoints:
  - `GET /api/hello` - Returns greeting with client identity
  - `GET /api/echo` - Echoes request details
  - `GET /health` - Health check

### Client ([client/main.go](client/main.go))
- Creates mTLS HTTP client using `httpclient.Client`
- Fetches its own X.509 SVID from SPIRE agent
- Makes authenticated requests to server
- Verifies server identity

## Running the Example

### Prerequisites

1. **SPIRE running in Minikube**:
   ```bash
   make minikube-up
   ```

2. **Register server and client workloads**:
   ```bash
   # Register server workload
   kubectl exec -n spire-system spire-server-0 -c spire-server -- \
     /opt/spire/bin/spire-server entry create \
       -spiffeID spiffe://example.org/server \
       -parentID spiffe://example.org/spire/agent/k8s_psat/minikube/...(agent-id) \
       -selector k8s:ns:default \
       -selector k8s:pod-label:app:mtls-server

   # Register client workload
   kubectl exec -n spire-system spire-server-0 -c spire-server -- \
     /opt/spire/bin/spire-server entry create \
       -spiffeID spiffe://example.org/client \
       -parentID spiffe://example.org/spire/agent/k8s_psat/minikube/...(agent-id) \
       -selector k8s:ns:default \
       -selector k8s:pod-label:app:mtls-client
   ```

### Option 1: Run Locally with SPIRE in Minikube

1. **Get SPIRE agent socket from Minikube**:
   ```bash
   # Forward agent socket to local machine
   kubectl port-forward -n spire-system daemonset/spire-agent 8081:8081

   # In another terminal, proxy the socket
   # (This requires socat or similar tool)
   ```

2. **Build binaries**:
   ```bash
   go build -o bin/mtls-server ./examples/mtls/server
   go build -o bin/mtls-client ./examples/mtls/client
   ```

3. **Run server**:
   ```bash
   SPIRE_AGENT_SOCKET=unix:///tmp/spire-agent/public/api.sock \
   SERVER_ADDRESS=:8443 \
   ./bin/mtls-server

   # Output:
   # Starting mTLS server with configuration:
   #   Socket: unix:///tmp/spire-agent/public/api.sock
   #   Address: :8443
   #   Allowed client: any from trust domain
   # ✓ Server started successfully on :8443
   ```

4. **Run client** (in another terminal):
   ```bash
   SPIRE_AGENT_SOCKET=unix:///tmp/spire-agent/public/api.sock \
   SERVER_URL=https://localhost:8443 \
   ./bin/mtls-client

   # Output:
   # Creating mTLS client with configuration:
   #   Socket: unix:///tmp/spire-agent/public/api.sock
   #   Server URL: https://localhost:8443
   # ✓ Client created successfully
   # === Making GET request to /api/hello ===
   # GET https://localhost:8443/api/hello
   # Status: 200 OK
   # Response:
   # Hello from mTLS server!
   # Authenticated client: spiffe://example.org/client
   ```

### Option 2: Run in Kubernetes

1. **Create Kubernetes deployments**:

```yaml
# server-deployment.yaml
apiVersion: v1
kind: Pod
metadata:
  name: mtls-server
  namespace: default
  labels:
    app: mtls-server
spec:
  containers:
  - name: server
    image: mtls-server:latest
    ports:
    - containerPort: 8443
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
      path: /run/spire/agent-sockets
      type: Directory
```

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
          path: /run/spire/agent-sockets
          type: Directory
      restartPolicy: OnFailure
```

2. **Deploy**:
   ```bash
   kubectl apply -f server-deployment.yaml
   kubectl apply -f client-job.yaml
   ```

3. **View logs**:
   ```bash
   # Server logs
   kubectl logs mtls-server

   # Client logs
   kubectl logs job/mtls-client
   ```

## Configuration

### Server Configuration

Environment variables:
- `SPIRE_AGENT_SOCKET` - Path to SPIRE agent socket (default: `unix:///tmp/spire-agent/public/api.sock`)
- `SERVER_ADDRESS` - Address to listen on (default: `:8443`)
- `ALLOWED_CLIENT_ID` - Restrict to specific client SPIFFE ID (optional)

### Client Configuration

Environment variables:
- `SPIRE_AGENT_SOCKET` - Path to SPIRE agent socket (default: `unix:///tmp/spire-agent/public/api.sock`)
- `SERVER_URL` - Server URL to connect to (default: `https://localhost:8443`)
- `EXPECTED_SERVER_ID` - Expected server SPIFFE ID (optional)

## Security Notes

### Authentication vs Authorization

This example demonstrates **authentication only**:
- ✅ Server verifies client identity using X.509 SVID
- ✅ Client verifies server identity using X.509 SVID
- ✅ mTLS ensures both parties are authenticated

Authorization (access control) is **out of scope** for this library:
- ❌ No role-based access control (RBAC)
- ❌ No resource-level permissions
- ❌ No policy enforcement

For authorization, implement in your application layer:
```go
func handler(w http.ResponseWriter, r *http.Request) {
    // Authentication already done by identityserver
    clientID, _ := spiffetls.PeerIDFromConnectionState(*r.TLS)

    // Application implements authorization
    if !myAuthzService.IsAllowed(clientID, "read", "resource") {
        http.Error(w, "Forbidden", http.StatusForbidden)
        return
    }

    // Handle request...
}
```

### Certificate Rotation

Both server and client automatically rotate their X.509 SVIDs:
- SVIDs fetched from SPIRE agent have short TTL (default: 1 hour)
- `workloadapi.X509Source` automatically fetches new SVIDs before expiry
- Zero-downtime rotation - no service interruption

### Trust Domain

Both server and client must be in the same trust domain (or have trust bundle federation configured in SPIRE).

The default configuration allows:
- Server: Any client from same trust domain
- Client: Any server from same trust domain

For stricter control, specify exact SPIFFE IDs:
```bash
# Server: Only allow specific client
ALLOWED_CLIENT_ID=spiffe://example.org/client ./bin/mtls-server

# Client: Only connect to specific server
EXPECTED_SERVER_ID=spiffe://example.org/server ./bin/mtls-client
```

## Troubleshooting

### "Failed to create X509Source"

**Problem**: Cannot connect to SPIRE agent socket.

**Solution**:
1. Check socket path is correct
2. Verify SPIRE agent is running
3. Check file permissions on socket

```bash
# Check socket exists
ls -la /tmp/spire-agent/public/api.sock

# Check SPIRE agent is running
kubectl get pods -n spire-system -l app.kubernetes.io/name=agent
```

### "No identity issued"

**Problem**: Workload not registered in SPIRE.

**Solution**: Register workload with correct selectors:
```bash
kubectl exec -n spire-system spire-server-0 -c spire-server -- \
  /opt/spire/bin/spire-server entry show
```

### "TLS handshake failed"

**Problem**: Client and server cannot establish mTLS connection.

**Solution**:
1. Verify both are in same trust domain
2. Check both can fetch valid SVIDs
3. Verify authorizer configuration matches

```bash
# Check server SVID
SPIRE_AGENT_SOCKET=unix:///tmp/spire-agent/public/api.sock \
  spire-agent api fetch x509

# Check client SVID
SPIRE_AGENT_SOCKET=unix:///tmp/spire-agent/public/api.sock \
  spire-agent api fetch x509
```

## Next Steps

- Add more endpoints to demonstrate different use cases
- Implement application-level authorization
- Add metrics and logging
- Deploy to production environment
- Configure trust bundle federation for multi-cluster

## References

- [SPIFFE/SPIRE Documentation](https://spiffe.io/docs/)
- [go-spiffe SDK](https://github.com/spiffe/go-spiffe)
- [mTLS Implementation Guide](../../docs/MTLS_IMPLEMENTATION.md)
- [Architecture Comparison](../../docs/MTLS_ARCHITECTURE_COMPARISON.md)
