# Getting Started Tutorial: mTLS with SPIRE in Development

**Target Audience**: New developers who want to learn how to use this library with SPIRE in a local development environment using Minikube.

**What You'll Learn**:
- Set up SPIRE infrastructure in Minikube
- Register workloads with SPIRE
- Build and run mTLS services with automatic certificate rotation
- Verify mutual TLS authentication is working

**Time Required**: ~30 minutes

---

## Prerequisites

Before starting, ensure you have these tools installed:

### Required Tools

1. **Docker** - Container runtime
   ```bash
   docker --version
   # Should output: Docker version 20.x or higher
   ```

2. **Minikube** - Local Kubernetes cluster
   ```bash
   minikube version
   # Should output: minikube version: v1.30.0 or higher
   ```

3. **kubectl** - Kubernetes CLI
   ```bash
   kubectl version --client
   # Should output: Client Version: v1.27.0 or higher
   ```

4. **Helm** - Kubernetes package manager
   ```bash
   helm version
   # Should output: version.BuildInfo{Version:"v3.12.0" or higher
   ```

5. **Go** - Programming language (1.21 or higher)
   ```bash
   go version
   # Should output: go version go1.21.0 or higher
   ```

### Installing Prerequisites (if needed)

**macOS**:
```bash
brew install docker minikube kubectl helm go
```

**Ubuntu/Debian**:
```bash
# Docker
sudo apt-get update
sudo apt-get install docker.io

# Minikube
curl -LO https://storage.googleapis.com/minikube/releases/latest/minikube-linux-amd64
sudo install minikube-linux-amd64 /usr/local/bin/minikube

# kubectl
curl -LO "https://dl.k8s.io/release/$(curl -L -s https://dl.k8s.io/release/stable.txt)/bin/linux/amd64/kubectl"
sudo install kubectl /usr/local/bin/kubectl

# Helm
curl https://raw.githubusercontent.com/helm/helm/main/scripts/get-helm-3 | bash

# Go
sudo apt-get install golang-go
```

---

## Step 1: Start Minikube

Start a local Kubernetes cluster with enough resources for SPIRE:

```bash
# Start minikube with appropriate resources
minikube start --cpus=4 --memory=8192 --driver=docker

# Verify cluster is running
minikube status
```

**Expected output**:
```
minikube
type: Control Plane
host: Running
kubelet: Running
apiserver: Running
kubeconfig: Configured
```

**Troubleshooting**:
- If minikube fails to start, try: `minikube delete && minikube start`
- On Linux, you may need to add your user to the docker group: `sudo usermod -aG docker $USER`

---

## Step 2: Install SPIRE Server

SPIRE has two components:
- **SPIRE Server**: Central authority that issues identities
- **SPIRE Agent**: Runs on each node, provides Workload API to applications

Let's install the SPIRE Server first:

```bash
# Add the SPIFFE Helm repository
helm repo add spiffe https://spiffe.github.io/helm-charts-hardened/
helm repo update

# Create namespace for SPIRE
kubectl create namespace spire

# Install SPIRE Server
helm install spire-server spiffe/spire \
  --namespace spire \
  --set global.spire.trustDomain=example.org \
  --set global.spire.clusterName=minikube-cluster

# Wait for SPIRE Server to be ready (this may take 1-2 minutes)
kubectl wait --for=condition=ready pod \
  -l app.kubernetes.io/name=server \
  -n spire \
  --timeout=120s
```

**Expected output**:
```
pod/spire-server-0 condition met
```

**Verify SPIRE Server is running**:
```bash
kubectl get pods -n spire
```

**Expected output**:
```
NAME              READY   STATUS    RESTARTS   AGE
spire-server-0    2/2     Running   0          1m
```

---

## Step 3: Install SPIRE Agent

The SPIRE Agent runs on each Kubernetes node and provides the Workload API:

```bash
# Install SPIRE Agent as a DaemonSet (runs on every node)
helm install spire-agent spiffe/spire \
  --namespace spire \
  --set global.spire.trustDomain=example.org \
  --set spire-agent.enabled=true \
  --set spire-server.enabled=false

# Wait for SPIRE Agent to be ready
kubectl wait --for=condition=ready pod \
  -l app.kubernetes.io/name=agent \
  -n spire \
  --timeout=120s
```

**Verify SPIRE Agent is running**:
```bash
kubectl get pods -n spire -l app.kubernetes.io/name=agent
```

**Expected output**:
```
NAME                READY   STATUS    RESTARTS   AGE
spire-agent-xxxxx   1/1     Running   0          1m
```

---

## Step 4: Create Registration Entries

SPIRE uses "registration entries" to map workload identities to SPIFFE IDs. Let's register two workloads: a server and a client.

### Register Server Workload

```bash
# Get SPIRE Server pod name
SERVER_POD=$(kubectl get pod -n spire -l app.kubernetes.io/name=server -o jsonpath='{.items[0].metadata.name}')

# Create server registration entry
kubectl exec -n spire $SERVER_POD -c spire-server -- \
  /opt/spire/bin/spire-server entry create \
  -spiffeID spiffe://example.org/server \
  -parentID spiffe://example.org/spire/agent/k8s_psat/minikube-cluster/default \
  -selector k8s:ns:default \
  -selector k8s:sa:default \
  -selector k8s:pod-label:app:e5s-server
```

**Expected output**:
```
Entry ID         : 01234567-89ab-cdef-0123-456789abcdef
SPIFFE ID        : spiffe://example.org/server
Parent ID        : spiffe://example.org/spire/agent/k8s_psat/minikube-cluster/default
Revision         : 0
X509-SVID TTL    : default
JWT-SVID TTL     : default
Selector         : k8s:ns:default
Selector         : k8s:sa:default
Selector         : k8s:pod-label:app:e5s-server
```

### Register Client Workload

```bash
# Create client registration entry
kubectl exec -n spire $SERVER_POD -c spire-server -- \
  /opt/spire/bin/spire-server entry create \
  -spiffeID spiffe://example.org/client \
  -parentID spiffe://example.org/spire/agent/k8s_psat/minikube-cluster/default \
  -selector k8s:ns:default \
  -selector k8s:sa:default \
  -selector k8s:pod-label:app:e5s-client
```

### Verify Registration Entries

```bash
# List all registration entries
kubectl exec -n spire $SERVER_POD -c spire-server -- \
  /opt/spire/bin/spire-server entry show
```

**Expected output**: You should see both entries (server and client) listed.

---

## Step 5: Build Example Application

Now let's build a simple mTLS application using the e5s library.

### Create Server Application

Create `server/main.go`:

```go
package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/go-chi/chi/v5"
	"github.com/sufield/e5s"
)

func main() {
	// Create context that listens for interrupt signals
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// Create HTTP router
	r := chi.NewRouter()

	// Health check endpoint
	r.Get("/healthz", func(w http.ResponseWriter, req *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})

	// Authenticated endpoint that requires mTLS
	r.Get("/hello", func(w http.ResponseWriter, req *http.Request) {
		// Extract peer identity from mTLS connection
		id, ok := e5s.PeerID(req)
		if !ok {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		fmt.Fprintf(w, "Hello, %s!\n", id)
	})

	// Get config file path from environment or use default
	configFile := os.Getenv("E5S_CONFIG")
	if configFile == "" {
		configFile = "e5s.yaml"
	}

	// Start mTLS server
	log.Printf("Starting mTLS server (config: %s)...", configFile)
	shutdown, err := e5s.Start(configFile, r)
	if err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
	defer shutdown()

	log.Println("Server running - press Ctrl+C to stop")

	// Wait for interrupt signal for graceful shutdown
	<-ctx.Done()
	stop()
	log.Println("Shutting down gracefully...")
}
```

### Create Client Application

Create `client/main.go`:

```go
package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/sufield/e5s"
)

func main() {
	// Get config file path from environment or use default
	configFile := os.Getenv("E5S_CONFIG")
	if configFile == "" {
		configFile = "e5s.yaml"
	}

	// Get server address from environment or use default
	serverAddr := os.Getenv("SERVER_ADDR")
	if serverAddr == "" {
		serverAddr = "https://localhost:8443"
	}

	// Create mTLS client
	log.Printf("Creating mTLS client (config: %s)...", configFile)
	client, shutdown, err := e5s.Client(configFile)
	if err != nil {
		log.Fatalf("Failed to create client: %v", err)
	}
	defer shutdown()

	log.Println("Client created successfully")

	// Create context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Make request to server
	url := fmt.Sprintf("%s/hello", serverAddr)
	log.Printf("Making request to %s...", url)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		log.Fatalf("Failed to create request: %v", err)
	}

	resp, err := client.Do(req)
	if err != nil {
		log.Fatalf("Request failed: %v", err)
	}
	defer resp.Body.Close()

	// Read and print response
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Fatalf("Failed to read response: %v", err)
	}

	log.Printf("Response (status %d): %s", resp.StatusCode, string(body))
}
```

### Create Configuration File

Use the existing `e5s.yaml` from this directory, or create one:

```yaml
spire:
  # Path to SPIRE Agent's Workload API socket
  workload_socket: unix:///tmp/spire-agent/public/api.sock

  # (Optional) How long to wait for identity from SPIRE before failing startup
  # Format: Go duration (e.g. "5s", "30s", "1m")
  # Default: 30s if not specified
  initial_fetch_timeout: 30s

server:
  listen_addr: ":8443"
  # Accept any client in this trust domain
  allowed_client_trust_domain: "example.org"

client:
  # Connect to any server in this trust domain
  expected_server_trust_domain: "example.org"
```

### Initialize Go Module

```bash
# Create go.mod if not exists
go mod init example/e5s-demo

# Add dependencies
go get github.com/sufield/e5s@latest
go get github.com/go-chi/chi/v5@latest
```

### Build Binaries

```bash
# Build server
go build -o bin/server ./server

# Build client
go build -o bin/client ./client
```

---

## Step 6: Create Kubernetes Manifests

### Server Deployment

Create `k8s-server.yaml`:

```yaml
apiVersion: v1
kind: Pod
metadata:
  name: e5s-server
  namespace: default
  labels:
    app: e5s-server
spec:
  containers:
  - name: server
    image: golang:1.21
    command: ["/app/bin/server"]
    volumeMounts:
    - name: app
      mountPath: /app
    - name: spire-agent-socket
      mountPath: /tmp/spire-agent/public
      readOnly: true
    env:
    - name: E5S_CONFIG
      value: "/app/e5s.yaml"
    ports:
    - containerPort: 8443
      name: https
  volumes:
  - name: app
    hostPath:
      path: /path/to/your/app  # Update this path
      type: Directory
  - name: spire-agent-socket
    hostPath:
      path: /var/run/spire/sockets
      type: Directory
```

### Client Job

Create `k8s-client.yaml`:

```yaml
apiVersion: batch/v1
kind: Job
metadata:
  name: e5s-client
  namespace: default
spec:
  template:
    metadata:
      labels:
        app: e5s-client
    spec:
      restartPolicy: Never
      containers:
      - name: client
        image: golang:1.21
        command: ["/app/bin/client"]
        volumeMounts:
        - name: app
          mountPath: /app
        - name: spire-agent-socket
          mountPath: /tmp/spire-agent/public
          readOnly: true
        env:
        - name: E5S_CONFIG
          value: "/app/e5s.yaml"
        - name: SERVER_ADDR
          value: "https://e5s-server.default.svc.cluster.local:8443"
      volumes:
      - name: app
        hostPath:
          path: /path/to/your/app  # Update this path
          type: Directory
      - name: spire-agent-socket
        hostPath:
          path: /var/run/spire/sockets
          type: Directory
```

---

## Step 7: Run Locally (Easier for Development)

Instead of deploying to Kubernetes initially, let's run locally and connect to SPIRE in Minikube:

### Port Forward SPIRE Agent Socket

```bash
# In a separate terminal, keep this running
kubectl port-forward -n spire \
  $(kubectl get pod -n spire -l app.kubernetes.io/name=agent -o jsonpath='{.items[0].metadata.name}') \
  8081:8081
```

### Update e5s.yaml for Local Development

Temporarily modify `e5s.yaml` to use the port-forwarded socket:

```yaml
spire:
  workload_socket: unix:///tmp/spire-agent.sock
  initial_fetch_timeout: 30s

server:
  listen_addr: ":8443"
  allowed_client_trust_domain: "example.org"

client:
  expected_server_trust_domain: "example.org"
```

### Create Symlink to Agent Socket

```bash
# Create symlink from local machine to forwarded socket
ln -sf ~/.minikube/profiles/minikube/apiserver.sock /tmp/spire-agent.sock
```

---

## Step 8: Run and Test

### Terminal 1: Run Server

```bash
./bin/server
```

**Expected output**:
```
2024/10/30 10:00:00 Starting mTLS server (config: e5s.yaml)...
2024/10/30 10:00:00 Server running - press Ctrl+C to stop
```

### Terminal 2: Run Client

```bash
SERVER_ADDR=https://localhost:8443 ./bin/client
```

**Expected output**:
```
2024/10/30 10:00:01 Creating mTLS client (config: e5s.yaml)...
2024/10/30 10:00:01 Client created successfully
2024/10/30 10:00:01 Making request to https://localhost:8443/hello...
2024/10/30 10:00:01 Response (status 200): Hello, spiffe://example.org/client!
```

**Success!** You've established mutual TLS authentication:
- Client authenticated to server
- Server authenticated to client
- Server extracted client's SPIFFE ID

---

## Step 9: Verify mTLS is Working

### Test Without Valid Certificate

Try connecting with curl (which doesn't have a SPIFFE identity):

```bash
curl -k https://localhost:8443/hello
```

**Expected output**: Connection should fail or be rejected because curl doesn't present a valid SPIFFE certificate.

### Check Certificate Details

You can inspect the certificates using openssl:

```bash
# View server certificate
openssl s_client -connect localhost:8443 -showcerts
```

Look for:
- Subject Alternative Name (SAN): `URI:spiffe://example.org/server`
- Issuer: SPIRE Server

---

## Step 10: Understand What Just Happened

Let's break down the mTLS flow:

1. **Server Startup**:
   - Connects to SPIRE Agent via Unix socket
   - Requests its SVID (SPIFFE Verifiable Identity Document)
   - Receives X.509 certificate with SPIFFE ID: `spiffe://example.org/server`
   - Starts HTTPS listener with mTLS enabled

2. **Client Startup**:
   - Connects to SPIRE Agent via Unix socket
   - Requests its SVID
   - Receives X.509 certificate with SPIFFE ID: `spiffe://example.org/client`
   - Configures HTTP client with mTLS

3. **TLS Handshake**:
   - Client presents its certificate to server
   - Server verifies client certificate was issued by SPIRE
   - Server checks client is in allowed trust domain (`example.org`)
   - Server presents its certificate to client
   - Client verifies server certificate was issued by SPIRE
   - Client checks server is in expected trust domain (`example.org`)

4. **Request Processing**:
   - TLS handshake succeeds (mutual authentication)
   - e5s extracts client SPIFFE ID from certificate
   - Server handler receives authenticated identity
   - Server responds to authenticated client

5. **Certificate Rotation** (automatic):
   - SPIRE automatically rotates certificates before expiry
   - e5s library automatically picks up new certificates
   - No downtime, no manual intervention

---

## Troubleshooting

### Issue: "failed to create X509Source: workload endpoint socket address is not configured"

**Solution**: Ensure the SPIRE Agent socket path in `e5s.yaml` matches the actual socket location.

```bash
# Check where SPIRE Agent socket actually is
kubectl exec -n spire \
  $(kubectl get pod -n spire -l app.kubernetes.io/name=agent -o jsonpath='{.items[0].metadata.name}') \
  -- ls -la /tmp/spire-agent/public/

# Update e5s.yaml with correct path
```

### Issue: "initial SPIRE fetch timed out after 30s"

**Causes**:
1. SPIRE Agent is not running
2. Socket path is wrong
3. Workload not registered in SPIRE

**Solution**:
```bash
# Check SPIRE Agent is running
kubectl get pods -n spire -l app.kubernetes.io/name=agent

# Check registration entries exist
kubectl exec -n spire $SERVER_POD -c spire-server -- \
  /opt/spire/bin/spire-server entry show

# Increase timeout in e5s.yaml
initial_fetch_timeout: 60s
```

### Issue: "tls: failed to verify certificate: x509: certificate signed by unknown authority"

**Cause**: Client doesn't trust the server's certificate authority (SPIRE).

**Solution**: Ensure both client and server are getting certificates from the same SPIRE deployment and both specify the same trust domain in `e5s.yaml`.

### Issue: "unauthorized" response from server

**Cause**: Client presented a certificate, but it doesn't match the server's authorization policy.

**Solution**: Check `e5s.yaml`:
```yaml
server:
  # Must allow the client's trust domain or specific SPIFFE ID
  allowed_client_trust_domain: "example.org"  # Client must be in this TD
```

---

## Next Steps

Now that you have mTLS working:

1. **Try Specific SPIFFE ID Authorization**:
   Update `e5s.yaml` to require specific client IDs:
   ```yaml
   server:
     allowed_client_spiffe_id: "spiffe://example.org/client"
   ```

2. **Observe Certificate Rotation**:
   Certificates rotate automatically. Watch logs to see rotation happen:
   ```bash
   # Certificates typically rotate every hour
   # Watch for new certificates being fetched
   ```

3. **Deploy to Kubernetes**:
   - Build container images for server and client
   - Deploy using the k8s manifests from Step 6
   - Watch them communicate over mTLS in the cluster

4. **Add More Endpoints**:
   - Add authenticated API endpoints
   - Extract identity in middleware
   - Implement authorization based on SPIFFE ID

5. **Production Hardening**:
   - Use specific SPIFFE IDs instead of trust domains
   - Add request logging
   - Set up monitoring and alerting
   - Review [security documentation](../../security/)

---

## Clean Up

When you're done:

```bash
# Stop local applications (Ctrl+C in each terminal)

# Uninstall SPIRE from Minikube
helm uninstall spire-server -n spire
helm uninstall spire-agent -n spire
kubectl delete namespace spire

# Stop Minikube
minikube stop

# (Optional) Delete Minikube cluster
minikube delete
```

---

## Additional Resources

- **SPIRE Documentation**: https://spiffe.io/docs/latest/spire/
- **e5s Library Docs**: See [main README](../../README.md)
- **SPIFFE Standard**: https://github.com/spiffe/spiffe
- **Minikube Docs**: https://minikube.sigs.k8s.io/docs/

---

## Summary

You've successfully:
- Set up SPIRE infrastructure in Minikube
- Registered workloads with SPIRE
- Built mTLS applications using e5s
- Verified mutual TLS authentication
- Understood automatic certificate rotation

The e5s library handles all the complexity:
- SPIRE Workload API connection
- Certificate fetching and rotation
- TLS 1.3 configuration
- mTLS handshake setup
- SPIFFE ID verification

You just write `e5s.Start()`, `e5s.Client()`, and `e5s.PeerID()` - the library does the rest!
