# Getting Started Tutorial: mTLS with SPIRE in Development

**Target Audience**: New developers who want to learn how to use this library with SPIRE in a local development environment using Minikube.

**What You'll Learn**:
- Set up SPIRE infrastructure in Minikube
- Register workloads with SPIRE
- Build and run mTLS services with automatic certificate rotation
- Verify mutual TLS authentication is working

**Time Required**: ~30 minutes

---

This is divided into two main parts:

### Part A: Infrastructure Setup (~15 minutes)
Set up the SPIRE infrastructure that provides cryptographic identities for your services.
- Start Minikube cluster
- Install SPIRE Server and Agent
- Register your workloads

### Part B: Application Development (~15 minutes)
Build and run your mTLS applications using the e5s library.
- Write server and client code
- Configure and test locally
- Verify mTLS authentication

Complete Part A before starting Part B. The infrastructure must be running for your applications to obtain certificates.

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

5. **Go** - Programming language (1.25 or higher)
   ```bash
   go version
   # Should output: go version go1.25.0 or higher
   ```

### Installing Prerequisites

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

# Part A: Infrastructure Setup

This part sets up the SPIRE infrastructure that will provide cryptographic identities to applications. You'll install SPIRE Server and Agent in Minikube, then register workloads.

**Goal**: Have a working SPIRE deployment where workloads can request certificates.

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

## Step 2: Install SPIRE

SPIRE has two components:
- **SPIRE Server**: Central authority that issues identities
- **SPIRE Agent**: Runs on each node, provides Workload API to applications

The modern SPIRE Helm chart installs both components together.

### Clean Up Previous Installations (if any)

If you've previously attempted to install SPIRE, clean up first:

```bash
# Clean up any previous installations (safe to run even if nothing exists)
helm uninstall spire -n spire 2>/dev/null || true
helm uninstall spire-server -n spire 2>/dev/null || true
helm uninstall spire-agent -n spire 2>/dev/null || true
helm uninstall spire-crds -n spire 2>/dev/null || true
kubectl delete namespace spire 2>/dev/null || true

# Wait for cleanup to complete
sleep 5
```

### Install SPIRE

```bash
# Add the SPIFFE Helm repository
helm repo add spiffe https://spiffe.github.io/helm-charts-hardened/
helm repo update

# Create namespace for SPIRE
kubectl create namespace spire

# Install SPIRE CRDs (Custom Resource Definitions) first
helm install spire-crds spire-crds \
  --repo https://spiffe.github.io/helm-charts-hardened/ \
  --namespace spire

# Install SPIRE (both server and agent)
helm install spire spire \
  --repo https://spiffe.github.io/helm-charts-hardened/ \
  --namespace spire \
  --set global.spire.trustDomain=example.org \
  --set global.spire.clusterName=minikube-cluster

# Wait for SPIRE Server to be ready (this may take 1-2 minutes)
kubectl wait --for=condition=ready pod \
  -l app.kubernetes.io/name=server \
  -n spire \
  --timeout=120s

# Wait for SPIRE Agent to be ready
kubectl wait --for=condition=ready pod \
  -l app.kubernetes.io/name=agent \
  -n spire \
  --timeout=120s
```

**Expected output**:
```
NAME: spire-crds
...
NAME: spire
...
pod/spire-server-0 condition met
pod/spire-agent-xxxxx condition met
```

**If installation fails**: Clean up and try again:
```bash
helm uninstall spire -n spire
helm uninstall spire-crds -n spire
# Then run the installation commands above again
```

**Verify SPIRE is running**:
```bash
kubectl get pods -n spire
```

**Expected output**:
```
NAME                                         READY   STATUS    RESTARTS   AGE
spire-agent-xxxxx                            3/3     Running   0          1m
spire-server-0                               2/2     Running   0          1m
spiffe-csi-driver-xxxxx                      3/3     Running   0          1m
spiffe-oidc-discovery-provider-xxxxx         1/1     Running   0          1m
```

---

## Step 3: Create Registration Entries

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

# Part B: Application Development

Now that the SPIRE infrastructure is running, you can build applications that use it for mTLS. In this section, you'll write a simple server and client using the e5s library, which abstracts all the complexity of SPIRE integration.

**Goal**: Build and run mTLS applications that automatically obtain certificates from SPIRE.

---

## Step 1: Build Example Application

Now let's build a simple mTLS application using the e5s library.

### Install Dependencies

First, create a Go module and install the required dependencies:

```bash
# Create a directory for your application
mkdir -p ~/mtls-demo
cd ~/mtls-demo

# Initialize Go module
go mod init mtls-demo

# Install e5s library
go get github.com/sufield/e5s@latest

# Install chi router
go get github.com/go-chi/chi/v5
```

### Create Server Application

Create `server/main.go`:

```go
package main

import (
	"fmt"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/sufield/e5s"
)

func main() {
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

	// Run mTLS server (handles config, SPIRE connection, and graceful shutdown)
	e5s.Run(r)
}
```

**What this code does:**
- Creates a chi router with two endpoints: `/healthz` and `/hello`
- The `/hello` endpoint extracts the client's SPIFFE ID using `e5s.PeerID()`
- Calls `e5s.Run(r)` which handles everything:
  - Config file discovery (e5s.yaml or E5S_CONFIG)
  - SPIRE connection and mTLS setup
  - Signal handling (Ctrl+C)
  - Graceful shutdown

**That's it!** Zero boilerplate - just define your routes and call `e5s.Run()`.

**For more control** over signal handling, config paths, or logging, see [ADVANCED.md](ADVANCED.md).

### Create Client Application

Create `client/main.go`:

```go
package main

import (
	"fmt"
	"io"
	"log"

	"github.com/sufield/e5s"
)

func main() {
	// Perform mTLS GET request (handles config, client creation, and cleanup)
	resp, err := e5s.Get("https://localhost:8443/hello")
	if err != nil {
		log.Fatal(err)
	}
	defer resp.Body.Close()  // This also triggers cleanup automatically

	// Read and print response
	body, _ := io.ReadAll(resp.Body)
	fmt.Println(string(body))
}
```

**What this code does:**
- Calls `e5s.Get()` which handles everything:
  - Config file discovery (e5s.yaml or E5S_CONFIG)
  - mTLS client creation
  - Making the request
  - Automatic cleanup when body is closed
- Prints the response from the server

**That's it!** Zero boilerplate - just call `e5s.Get()` and read the response.

**For advanced patterns** like multiple requests with the same client, explicit config paths, context timeouts, or custom headers, see [ADVANCED.md](ADVANCED.md).

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

## Step 2: Create Kubernetes Manifests

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

## Step 3: Run Locally (Easier for Development)

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

## Step 4: Run and Test

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

## Step 5: Verify mTLS is Working

Now let's verify that our mTLS setup is working correctly by testing both success and failure cases.

---

### ✅ **SUCCESS CASE: Registered Client** (`client/main.go`)

**Test: Registered Client Connects Successfully**

The client you ran in Step 4 successfully connected because it was registered with SPIRE.

**Why it worked:**
- Client was registered in Part A, Step 3
- SPIRE issued it a certificate with SPIFFE ID: `spiffe://example.org/client`
- Server accepted it because it's in the allowed trust domain (`example.org`)

**Expected success output** (from Step 4):
```
2024/10/30 10:00:01 Creating mTLS client (config: e5s.yaml)...
2024/10/30 10:00:01 Client created successfully
2024/10/30 10:00:01 Making request to https://localhost:8443/hello...
2024/10/30 10:00:01 Response (status 200): Hello, spiffe://example.org/client!
```

This proves that:
- ✅ Registered workloads can obtain certificates from SPIRE
- ✅ mTLS handshake succeeds when both parties have valid certificates
- ✅ Server can extract and verify client identity

---

### ❌ **FAILURE CASE: Unregistered Client** (`unregistered-client/main.go`)

**Test: Unregistered Client Connection Blocked**

This is the most important security test - it proves that SPIRE enforces identity-based access control. Let's try to connect with a client that is NOT registered with SPIRE.

**Create an unregistered client:**

Create `unregistered-client/main.go`:

```go
package main

import (
	"fmt"
	"io"
	"log"

	"github.com/sufield/e5s"
)

func main() {
	log.Println("Attempting to connect as unregistered workload...")

	resp, err := e5s.Get("https://localhost:8443/hello")
	if err != nil {
		log.Fatalf("Connection failed (expected): %v", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	fmt.Println(string(body))
}
```

**Try to run it:**

```bash
cd ~/mtls-demo
go run unregistered-client/main.go
```

**Expected failure:**
```
Attempting to connect as unregistered workload...
Connection failed (expected): failed to create X509Source: no identity issued
```

**Why it fails:**
1. The unregistered client tries to contact the SPIRE Agent
2. SPIRE Agent checks if this workload has a registration entry
3. No entry exists for this workload
4. SPIRE refuses to issue a certificate
5. Without a certificate, the client cannot establish mTLS connection

This proves that:
- ✅ Only registered workloads can obtain certificates
- ✅ Unregistered workloads cannot communicate with mTLS services
- ✅ SPIRE enforces zero-trust security model
- ✅ Identity must be explicitly granted, not assumed

---

### ❌ **FAILURE CASE: curl** (No SPIFFE Identity)

**Test: Traditional HTTP Client Blocked**

Standard tools like curl also cannot connect because they don't have SPIFFE identities:

```bash
# This also fails because curl doesn't have a SPIFFE identity
curl -k https://localhost:8443/hello
```

**Expected failure:**
```
curl: (35) error:14094410:SSL routines:ssl3_read_bytes:sslv3 alert handshake failure
```

**Why it fails**: The server requires client certificate authentication. curl cannot obtain a certificate from SPIRE without being a registered workload.

This demonstrates that traditional HTTP clients cannot bypass SPIFFE-based mTLS security.

---

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

## Step 6: Understand What Just Happened

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
   - Deploy using the k8s manifests from Step 2
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
helm uninstall spire -n spire
helm uninstall spire-crds -n spire
kubectl delete namespace spire

# Stop Minikube
minikube stop

# (Optional) Delete Minikube cluster
minikube delete
```

---

## Resources

- **Troubleshooting Guide**: See [TROUBLESHOOTING.md](TROUBLESHOOTING.md)
- **Advanced Examples**: See [ADVANCED.md](ADVANCED.md)
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
