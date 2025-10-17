# SPIRE mTLS Examples

This directory contains examples demonstrating mTLS authentication using SPIFFE/SPIRE.

## Available Examples

### 1. Zero-Config Example (Recommended for Quick Start)

**Location**: `examples/zeroconfig-example/`

The simplest way to get started - a minimal server that auto-detects everything:

```go
func main() {
    ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
    defer stop()

    err := zerotrustserver.Serve(ctx, map[string]http.Handler{
        "/": http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            id, ok := zerotrustserver.Identity(r.Context())
            if !ok {
                http.Error(w, "unauthorized", http.StatusUnauthorized)
                return
            }
            fmt.Fprintf(w, "Success! Authenticated as: %s\n", id.SPIFFEID)
        }),
    })
    if err != nil {
        log.Fatal(err)
    }
}
```

**What it provides**:
- ✅ Zero configuration required
- ✅ Auto-detects SPIRE socket and trust domain
- ✅ Built-in health endpoint
- ✅ Production-ready defaults

**Requirements**: SPIRE agent must be running locally or accessible via standard paths.

### 2. Kubernetes/Minikube Deployment (Full Infrastructure)

**Location**: This guide below + `examples/zeroconfig-example/`

Complete guide for deploying to Kubernetes with SPIRE infrastructure. This demonstrates:
- Deploying SPIRE server and agent on Minikube
- Workload registration with Kubernetes selectors
- mTLS communication between pods
- Zero-config mTLS server deployment

Use this example to understand how to deploy in a real Kubernetes environment.

---

## Kubernetes/Minikube Deployment Guide

This section shows how to deploy and test the zero-config mTLS server example on Kubernetes using Minikube with SPIRE infrastructure.

### Overview

This example demonstrates:
- Deploying the zero-config mTLS server (`examples/zeroconfig-example`) to Kubernetes
- Automatic SPIFFE identity issuance via SPIRE Workload API
- Automatic socket and trust domain detection
- Mutual TLS authentication between workloads
- Identity extraction in HTTP handlers

**Architecture**: The example uses the zero-config API which wraps the production `identityserver` adapter with intelligent defaults.

## Prerequisites

### Required Tools

| Tool | Version | Installation |
|------|---------|--------------|
| Go | 1.25.1+ | https://go.dev/dl/ |
| Minikube | 1.32.0+ | https://minikube.sigs.k8s.io/docs/start/ |
| kubectl | 1.28.0+ | https://kubernetes.io/docs/tasks/tools/ |
| Helm | 3.13.0+ | https://helm.sh/docs/intro/install/ |

### Verify Installation

```bash
# Set kubectl context to minikube
kubectl config use-context minikube

# Verify tools (optional - runs make target)
make check-prereqs-k8s
```

---

## Quick Start

### 1. Start SPIRE Infrastructure

Start Minikube and deploy SPIRE (server + agent):

```bash
# Start Minikube and deploy SPIRE
make minikube-up

# Verify SPIRE is running
make minikube-status

# Check pods are ready
kubectl get pods -n spire-system
```

**Expected output**:
```
NAMESPACE      NAME               READY   STATUS
spire-system   spire-server-0     2/2     Running
spire-system   spire-agent-xxxxx  1/1     Running
```

---

### 2. Build the Example Server

Build the zero-config mTLS server binary:

```bash
# Run tests first (optional but recommended)
make test

# Build the zero-config example server
go build -o bin/mtls-server ./examples/zeroconfig-example

# Verify binary was created
ls -lh bin/mtls-server
```

---

### 3. Register Workloads with SPIRE

Create SPIRE registration entries for the server and client workloads.

#### Get Agent SPIFFE ID

Fetch the actual agent SPIFFE ID dynamically. Do not hardcode it.

```bash
# Get the agent's SPIFFE ID
AGENT_ID=$(kubectl exec -n spire-system statefulset/spire-server -c spire-server -- \
    /opt/spire/bin/spire-server agent list | grep "SPIFFE ID" | awk -F': ' '{print $2}')

  echo "Agent SPIFFE ID: $AGENT_ID"
```

#### Register Server Workload

```bash
kubectl exec -n spire-system statefulset/spire-server -c spire-server -- \
  /opt/spire/bin/spire-server entry create \
    -spiffeID spiffe://example.org/server \
    -parentID "$AGENT_ID" \
    -selector k8s:ns:default \
    -selector k8s:sa:default \
    -selector k8s:container-name:mtls-server \
    -dns mtls-server \
    -dns mtls-server.default \
    -dns mtls-server.default.svc \
    -dns mtls-server.default.svc.cluster.local \
    -dns localhost
```

**Why DNS SANs?** The server certificate includes these DNS names for TLS hostname verification. Include all DNS names you'll use to reach the service.

#### Register Client Workload

```bash
kubectl exec -n spire-system statefulset/spire-server -c spire-server -- \
  /opt/spire/bin/spire-server entry create \
    -spiffeID spiffe://example.org/client \
    -parentID "$AGENT_ID" \
    -selector k8s:ns:default \
    -selector k8s:sa:default \
    -selector k8s:container-name:test-client
```

#### Verify Entries

```bash
kubectl exec -n spire-system statefulset/spire-server -c spire-server -- \
  /opt/spire/bin/spire-server entry show
```

You should see both `spiffe://example.org/server` and `spiffe://example.org/client` entries.

---

### 4. Deploy the mTLS Server

Deploy the server to Kubernetes:

```bash
# Deploy the server deployment and service
kubectl apply -f examples/mtls-server.yaml

# Wait for pod to be ready
kubectl wait --for=condition=Ready deploy/mtls-server --timeout=60s

# Get the pod name
POD=$(kubectl get pod -l app=mtls-server -o jsonpath='{.items[0].metadata.name}')
echo "Server pod: $POD"
```

#### Copy Binary to Pod

```bash
# Copy the compiled binary into the pod
kubectl cp bin/mtls-server "$POD":/tmp/mtls-server

# Make it executable
kubectl exec "$POD" -- chmod +x /tmp/mtls-server
```

#### Run the Server

```bash
# Run the server (this will block - use a separate terminal or use screen/tmux)
kubectl exec -it "$POD" -- /tmp/mtls-server
```

**Expected output**:
```
Server starting on :8443 with zero-trust mTLS
Auto-detected socket: unix:///spire-socket/api.sock
Auto-detected trust domain: example.org
Server listening on :8443
Press Ctrl+C to stop
```

**What's happening?**
1. The zero-config server auto-detects the SPIRE agent socket (checks `SPIFFE_ENDPOINT_SOCKET` env var and common paths)
2. Connects to SPIRE agent via mounted socket at `/spire-socket/api.sock`
3. Obtains its X.509 SVID and extracts the trust domain automatically
4. Starts HTTPS server on port 8443 with mTLS authentication
5. Authorizes any client in the same trust domain (`example.org`)

---

### 5. Test the Server

Deploy a client pod and make requests to the server.

#### Deploy Client Pod

```bash
# Deploy the test client
kubectl apply -f examples/test-client.yaml

# Wait for pod to be ready
kubectl wait --for=condition=Ready deploy/test-client --timeout=120s

# Get the client pod name
CLIENT_POD=$(kubectl get pod -l app=test-client -o jsonpath='{.items[0].metadata.name}')
echo "Client pod: $CLIENT_POD"
```

#### Run Test Client

Copy the test client from the repository and run it:

```bash
# Copy the test client code
kubectl cp examples/test-client.go "$CLIENT_POD":/workspace/testclient/test-client.go -c test-client

# Run the test client
kubectl exec -it "$CLIENT_POD" -c test-client -- bash -c "cd /workspace/testclient && go mod tidy && go run test-client.go"
```

**Expected output**:
```
=== Testing: https://mtls-server:8443/ ===
Status: 200
Body: Success! Authenticated as: spiffe://example.org/client

=== Testing: https://mtls-server:8443/api/hello ===
Status: 200
Body: {"message":"Hello from mTLS server!","identity":"spiffe://example.org/client"}

=== Testing: https://mtls-server:8443/api/identity ===
Status: 200
Body: {"identity":{"path":"/client","spiffe_id":"spiffe://example.org/client","trust_domain":"example.org"},...}

=== Testing: https://mtls-server:8443/health ===
Status: 200
Body: {"status":"healthy"}
```

**What's happening?**
1. Client gets its own X.509 SVID (`spiffe://example.org/client`) from SPIRE
2. Establishes mTLS connection to server
3. Both client and server verify each other's SPIFFE IDs
4. Server extracts client identity and includes it in responses

---

## Understanding the Setup

### SPIRE Socket Mounting

Both server and client pods mount the SPIRE agent socket using `hostPath`:

```yaml
volumes:
  - name: spire-socket
    hostPath:
      path: /tmp/spire-agent/public   # On Minikube node
      type: Directory

volumeMounts:
  - name: spire-socket
    mountPath: /spire-socket           # Inside pod
    readOnly: true
```

The workload connects via:
```
SPIFFE_ENDPOINT_SOCKET=unix:///spire-socket/api.sock
```

### SPIFFE ID Attestation

SPIRE agent attests workload identity using Kubernetes selectors:

```bash
-selector k8s:ns:default           # Namespace
-selector k8s:sa:default           # ServiceAccount
-selector k8s:container-name:mtls-server  # Container name
```

All selectors must match for the workload to receive the registered SPIFFE ID.

### mTLS Authentication Flow

1. **Server startup**:
   - Connects to SPIRE agent via socket
   - Receives X.509 SVID (`spiffe://example.org/server`)
   - Starts HTTPS server with this certificate

2. **Client request**:
   - Client gets its own X.509 SVID (`spiffe://example.org/client`)
   - Initiates TLS handshake
   - Both present certificates
   - Both verify peer SPIFFE ID

3. **Authorization**:
   - Server auto-detects trust domain from its own SVID
   - Authorizes any client in the same trust domain
   - Request is accepted/rejected based on client's trust domain membership

---

## Troubleshooting

### Check SPIRE Status

```bash
# Check SPIRE pods
kubectl get pods -n spire-system

# View server logs
kubectl logs -n spire-system statefulset/spire-server -c spire-server --tail=100

# View agent logs
kubectl logs -n spire-system ds/spire-agent -c spire-agent --tail=100
```

### Verify Socket Exists

```bash
# Get agent pod name
AGENT_POD=$(kubectl get pods -n spire-system -l app.kubernetes.io/name=agent -o jsonpath='{.items[0].metadata.name}')

# Check socket exists inside agent
kubectl exec -n spire-system "$AGENT_POD" -- ls -la /tmp/spire-agent/public/api.sock
```

### Workload Can't Get SVID

**Symptoms**:
- `no such file or directory` for socket
- `no identity issued`
- `permission denied`

**Solutions**:
1. **Verify selectors match**:
   ```bash
   kubectl exec -n spire-system statefulset/spire-server -c spire-server -- \
     /opt/spire/bin/spire-server entry show
   ```
   - Check namespace, serviceAccount, container-name match your pod

2. **Verify parentID**:
   - Must match the agent SPIFFE ID exactly
   - Don't hardcode - always fetch dynamically

3. **Check socket mount**:
   ```bash
   kubectl exec $POD -- ls -la /spire-socket/api.sock
   ```
   - Should show socket file
   - If missing, check hostPath and volumeMount in YAML

4. **Wait for cache refresh**:
   - After creating/updating entries, wait 5-10 seconds
   - SPIRE caches registration entries

### TLS Handshake Errors

**Symptoms**:
- `certificate verify failed`
- `bad certificate`
- `unknown authority`

**Solutions**:
1. **Check DNS SANs**:
   - Server entry must include `-dns` for all hostnames you use
   - Example: `-dns mtls-server -dns mtls-server.default.svc`

2. **Verify trust domain**:
   - Client and server must be in same trust domain
   - Check SPIFFE IDs: `spiffe://example.org/...`

3. **Check trust domain**:
   - The zero-config server auto-detects trust domain and authorizes all clients in that domain
   - Client and server must be in the same trust domain

### Server Won't Start

**Check logs**:
```bash
kubectl logs $POD
```

**Common issues**:
- Socket not found in standard locations (the zero-config server checks common paths automatically)
- Socket path wrong or not mounted
- Unable to fetch SVID from SPIRE agent

---

## Cleanup

```bash
# Delete deployments
kubectl delete -f examples/test-client.yaml
kubectl delete -f examples/mtls-server.yaml

# Stop SPIRE but keep data
make minikube-down

# Or destroy everything
make minikube-delete
```

---

## Advanced Usage

### Using Custom Trust Domain

The zero-config server automatically detects and uses the trust domain from its SVID. To use a custom trust domain:

1. Configure SPIRE server with your custom trust domain
2. Register workloads with that trust domain:

```bash
# Register with custom trust domain
kubectl exec -n spire-system statefulset/spire-server -c spire-server -- \
  /opt/spire/bin/spire-server entry create \
    -spiffeID spiffe://mycompany.com/server \
    -parentID "$AGENT_ID" \
    -selector k8s:ns:default \
    -selector k8s:sa:default \
    -selector k8s:container-name:mtls-server
```

The server will automatically detect `mycompany.com` as the trust domain and authorize all clients in that domain.

### Production Deployment

For production:

1. **Build container image**:
   ```dockerfile
   FROM golang:1.25 AS builder
   WORKDIR /app
   COPY . .
   RUN go build -o mtls-server ./examples/zeroconfig-example

   FROM debian:bookworm-slim
   COPY --from=builder /app/mtls-server /usr/local/bin/
   CMD ["/usr/local/bin/mtls-server"]
   ```

2. **Update deployment**:
   ```yaml
   containers:
     - name: mtls-server
       image: your-registry/mtls-server:v1.0.0  # Use your image
       # Remove "sleep infinity" command
   ```

3. **Use real certificates**:
   - Don't use `-dns localhost` in production
   - Use actual service DNS names
   - Consider using Ingress with SPIFFE authentication

---

## Next Steps

- Try [zeroconfig-example/](zeroconfig-example/) for a simpler, zero-configuration approach
- Read [docs/TEST_ARCHITECTURE.md](../docs/TEST_ARCHITECTURE.md) for integration testing patterns
- Explore [internal/adapters/inbound/identityserver/](../internal/adapters/inbound/identityserver/) to understand the adapter implementation
- Review [docs/CONTROL_PLANE.md](../docs/CONTROL_PLANE.md) for SPIRE deployment architecture
- See [main README](../README.md) for API reference and design patterns

---

## Summary

This guide covers how to:
- ✅ Deploy SPIRE infrastructure on Minikube
- ✅ Register workload identities with SPIRE
- ✅ Deploy an mTLS server using SPIFFE authentication
- ✅ Test mutual TLS between workloads
- ✅ Troubleshoot common issues

1. Always mount the SPIRE agent socket into workload pods
2. Registration selectors must exactly match pod metadata
3. Never hardcode agent parentID - fetch it dynamically
4. DNS SANs are required for TLS hostname verification
5. SPIFFE authentication is identity-based, not DNS-based
