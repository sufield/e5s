# Testing Pre-Release: Internal Development Guide

**Target Audience**: Internal developers testing e5s library changes before publishing to GitHub.

**Purpose**: Test local e5s code changes in a realistic environment before releasing to end users.

**Time Required**:
- **Manual Setup**: ~20 minutes (step-by-step)

If you prefer step-by-step control or need to understand the process:

---

## When to Use This Guide

Use this guide when you:
- Are developing new features for the e5s library
- Need to test changes before creating a release
- Want to validate bug fixes in a real environment
- Are testing the tutorial steps before publishing

---

## Prerequisites

1. **SPIRE Infrastructure Running**: Follow [SPIRE_SETUP.md](SPIRE_SETUP.md) to set up SPIRE in Minikube (~15 minutes)
   - Minikube cluster running
   - SPIRE Server and Agent installed via Helm
   - Server and client workloads registered

   The setup uses Helm to install SPIRE infrastructure. This guide deploys test applications using kubectl directly without using Helm.

2. **Required Tools**:
   - **Docker** - For building container images
   - **Minikube** - For Kubernetes cluster (installed via SPIRE_SETUP.md)
   - **kubectl** - For deploying applications (installed via SPIRE_SETUP.md)
   - **Helm** - For SPIRE installation only (installed via SPIRE_SETUP.md)

  Verify tools are installed

   ```bash 
   docker --version
   minikube version
   kubectl version --client
   helm version
   ```

3. **Go** - Programming language (1.25.3 or higher)
   ```bash
   go version
   # Should output: go version go1.25.3 or higher
   ```

4. **Local e5s Code**: You should be in the e5s project directory

   Verify you're in the right place

   ```bash
   ls -la e5s.go pkg/ examples/
   ```

   Should show the e5s library source code

---

## Step 1: Create Test Application Directory

Create a test application that uses your local e5s code:

```bash
# Navigate to the e5s project root
cd /path/to/e5s  # Where your e5s code is located

# Create a test directory
mkdir -p test-demo
cd test-demo

# Initialize Go module
go mod init test-demo
```

---

## Step 2: Configure Local Dependency

Use the Go `replace` directive to point to your local e5s code instead of the released version:

```bash
# Add replace directive to point to local e5s code
# The '..' means parent directory (where e5s source code is)
go mod edit -replace github.com/sufield/e5s=..

# Add chi router dependency
go get github.com/go-chi/chi/v5

# Add e5s to require section (will use local code due to replace directive)
go get github.com/sufield/e5s
```

**Verify `go.mod`:**

```bash
cat go.mod
```

It should look like:

```
module test-demo

go 1.25.3

require (
    github.com/go-chi/chi/v5 v5.2.3
    github.com/sufield/e5s v0.0.0
)

replace github.com/sufield/e5s => ..
```

**What this does**:
- The `replace` directive tells Go to use the parent directory instead of downloading from GitHub
- Any `import "github.com/sufield/e5s"` in your code will use your local e5s code
- You can modify e5s code and immediately see changes in your test application
- Perfect for iterating on library changes

---

## Step 3: Create Server Application

Create `server/main.go`:

```go
package main

import (
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/sufield/e5s"
)

func main() {
	log.Println("Starting e5s mTLS server...")

	// Create HTTP router
	r := chi.NewRouter()

	// Health check endpoint
	r.Get("/healthz", func(w http.ResponseWriter, req *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})

	// Authenticated endpoint that returns server time
	r.Get("/time", func(w http.ResponseWriter, req *http.Request) {
		// Extract peer identity from mTLS connection
		id, ok := e5s.PeerID(req)
		if !ok {
			log.Printf("❌ Unauthorized request from %s", req.RemoteAddr)
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		log.Printf("✓ Authenticated request from: %s", id)

		// Get current server time
		serverTime := time.Now().Format(time.RFC3339)
		response := fmt.Sprintf("Server time: %s", serverTime)
		log.Printf("→ Sending response: %s", response)
		fmt.Fprintf(w, "%s\n", response)
	})

	log.Println("Server configured, initializing mTLS with SPIRE...")
	// Run mTLS server (uses local e5s code)
	e5s.Run(r)
}
```

---

## Step 4: Create Client Application

Create `client/main.go`:

```go
package main

import (
	"fmt"
	"io"
	"log"
	"os"

	"github.com/sufield/e5s"
)

func main() {
	log.Println("Starting e5s mTLS client...")

	// Get server URL from environment variable, default to localhost
	// This allows the client to work both locally and in Kubernetes
	serverURL := os.Getenv("SERVER_URL")
	if serverURL == "" {
		serverURL = "https://localhost:8443/time"
	}

	log.Printf("→ Requesting server time from: %s", serverURL)
	log.Println("→ Initializing SPIRE client and fetching SPIFFE identity...")

	// Perform mTLS GET request (uses local e5s code)
	resp, err := e5s.Get(serverURL)
	if err != nil {
		log.Fatalf("❌ Request failed: %v", err)
	}
	defer resp.Body.Close()

	log.Printf("✓ Received response: HTTP %d %s", resp.StatusCode, resp.Status)

	// Read and print response
	body, _ := io.ReadAll(resp.Body)
	log.Printf("← Server response: %s", string(body))
	fmt.Print(string(body))
}
```

We include environment variable support from the start because we'll deploy to Kubernetes where the client needs to connect to a service name (e.g., `https://e5s-server:8443/time`) instead of localhost.

---

## Step 5: Create Configuration File

Create `e5s.yaml` in the test-demo directory:

```yaml
spire:
  # Path to SPIRE Agent's Workload API socket
  workload_socket: unix:///tmp/spire-agent/public/api.sock

  # (Optional) How long to wait for identity from SPIRE before failing startup
  # If not set, defaults to 30s
  # initial_fetch_timeout: 30s

server:
  listen_addr: ":8443"
  # Accept any client in this trust domain
  allowed_client_trust_domain: "example.org"

client:
  # Connect to any server in this trust domain
  expected_server_trust_domain: "example.org"
```

---

## Step 6: Build Test Binaries

Build your test applications:

```bash
# From your test-demo directory
# These builds will use your LOCAL e5s code (due to replace directive)

# Build server
go build -o bin/server ./server

# Build client
go build -o bin/client ./client

# Verify the binaries were created
ls -lh bin/
```

Every time you modify e5s library code, you need to rebuild these binaries to see the changes.

---

## Step 7: Create Kubernetes Configuration

**Why Kubernetes?** The SPIRE Workload API socket is only accessible inside Kubernetes pods, not from your local machine. You must deploy your test applications to Kubernetes.

Create a ConfigMap for e5s.yaml with the correct socket path for Kubernetes:

```bash
cat > k8s-e5s-config.yaml <<'EOF'
apiVersion: v1
kind: ConfigMap
metadata:
  name: e5s-config
  namespace: default
data:
  e5s.yaml: |
    spire:
      # Path to SPIRE Agent socket inside Kubernetes pods
      workload_socket: unix:///spire/agent-socket/spire-agent.sock

      # (Optional) How long to wait for identity from SPIRE before failing startup
      # If not set, defaults to 30s
      # initial_fetch_timeout: 30s

    server:
      listen_addr: ":8443"
      allowed_client_trust_domain: "example.org"

    client:
      expected_server_trust_domain: "example.org"
EOF
```

Create k8s ConfigMap containing e5s configuration that gets mounted into server and client pods.

```bash
kubectl apply -f k8s-e5s-config.yaml
```

---

## Step 8: Build and Deploy to Kubernetes

### Build Static Binaries

This is required for Alpine containers.

```bash
CGO_ENABLED=0 go build -o bin/server ./server
CGO_ENABLED=0 go build -o bin/client ./client
```

### Build Docker Images in Minikube

Point Docker to Minikube's Docker daemon:

```bash 
eval $(minikube docker-env)
```

Build server image:

```bash
docker build -t e5s-server:dev -f - . <<'EOF'
FROM alpine:latest
WORKDIR /app
COPY bin/server .
ENTRYPOINT ["/app/server"]
EOF
```

Build client image:

```bash
docker build -t e5s-client:dev -f - . <<'EOF'
FROM alpine:latest
WORKDIR /app
COPY bin/client .
ENTRYPOINT ["/app/client"]
EOF
```

### Deploy Server

```bash
cat > k8s-server.yaml <<'EOF'
apiVersion: apps/v1
kind: Deployment
metadata:
  name: e5s-server
  namespace: default
spec:
  replicas: 1
  selector:
    matchLabels:
      app: e5s-server
  template:
    metadata:
      labels:
        app: e5s-server
    spec:
      serviceAccountName: default
      containers:
      - name: server
        image: e5s-server:dev
        imagePullPolicy: Never  # Use local image from Minikube
        volumeMounts:
        - name: spire-agent-socket
          mountPath: /spire/agent-socket
          readOnly: true
        - name: config
          mountPath: /app/e5s.yaml
          subPath: e5s.yaml
      volumes:
      - name: spire-agent-socket
        csi:
          driver: "csi.spiffe.io"
          readOnly: true
      - name: config
        configMap:
          name: e5s-config
---
apiVersion: v1
kind: Service
metadata:
  name: e5s-server
  namespace: default
spec:
  selector:
    app: e5s-server
  ports:
  - protocol: TCP
    port: 8443
    targetPort: 8443
EOF
```

Deploy the e5s server Deployment and Service to Kubernetes to run the server pod with SPIRE integration.

```bash
kubectl apply -f k8s-server.yaml
```

Wait for server to be ready

```bash
kubectl wait --for=condition=ready pod -l app=e5s-server -n default --timeout=60s
```

### Test with Client Job

```bash
cat > k8s-client-job.yaml <<'EOF'
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
      serviceAccountName: default
      restartPolicy: Never
      containers:
      - name: client
        image: e5s-client:dev
        imagePullPolicy: Never
        env:
        - name: SERVER_URL
          value: "https://e5s-server:8443/time"
        command: ["/app/client"]
        volumeMounts:
        - name: spire-agent-socket
          mountPath: /spire/agent-socket
          readOnly: true
        - name: config
          mountPath: /app/e5s.yaml
          subPath: e5s.yaml
      volumes:
      - name: spire-agent-socket
        csi:
          driver: "csi.spiffe.io"
          readOnly: true
      - name: config
        configMap:
          name: e5s-config
EOF
```

Run the test client:

```
kubectl apply -f k8s-client-job.yaml
```

Wait a few seconds:

```
sleep 10
```

Check logs:

```
kubectl logs -l app=e5s-client
```

**Expected client output**:
```
2025/01/15 10:15:23 Starting e5s mTLS client...
2025/01/15 10:15:23 → Requesting server time from: https://e5s-server:8443/time
2025/01/15 10:15:23 → Initializing SPIRE client and fetching SPIFFE identity...
2025/01/15 10:15:24 ✓ Received response: HTTP 200 OK
2025/01/15 10:15:24 ← Server response: Server time: 2025-01-15T10:15:24Z
Server time: 2025-01-15T10:15:24Z
```

**Expected server logs**:
```
2025/01/15 10:15:23 Starting e5s mTLS server...
2025/01/15 10:15:23 Server configured, initializing mTLS with SPIRE...
2025/01/15 10:15:24 ✓ Authenticated request from: spiffe://example.org/ns/default/sa/default
2025/01/15 10:15:24 → Sending response: Server time: 2025-01-15T10:15:24Z
```

**Success!** Your local e5s code is working in Kubernetes with SPIRE. The client requested the server time using mTLS, and the server responded with its current time.

---

## Step 9: Iterate on Code Changes

Now you can quickly test e5s library changes:

### Steps

Make changes to e5s library code

```bash
cd ..  # Go to e5s project root
vim e5s.go  # Edit any e5s files
```

Return to test-demo and rebuild binaries

```bash
cd test-demo
CGO_ENABLED=0 go build -o bin/server ./server
CGO_ENABLED=0 go build -o bin/client ./client
```

Point to Minikube's Docker daemon

```bash
eval $(minikube docker-env)
```

Remove old Docker images to force clean rebuild

```bash
docker rmi e5s-server:dev e5s-client:dev 2>/dev/null || true
```

Rebuild Docker images with updated binaries

```bash
docker build -t e5s-server:dev -f - . <<'EOF'
FROM alpine:latest
WORKDIR /app
COPY bin/server .
ENTRYPOINT ["/app/server"]
EOF
```

```bash
docker build -t e5s-client:dev -f - . <<'EOF'
FROM alpine:latest
WORKDIR /app
COPY bin/client .
ENTRYPOINT ["/app/client"]
EOF
```

Force server pods to restart with new image

```bash
kubectl delete pods -l app=e5s-server
kubectl wait --for=condition=ready pod -l app=e5s-server --timeout=60s
```

Test with client using new image

```bash
kubectl delete job e5s-client 2>/dev/null || true
kubectl apply -f k8s-client-job.yaml
sleep 10
kubectl logs -l app=e5s-client
```

**Summary**:
1. Make changes to e5s library code
2. Rebuild binaries with updated e5s code
3. Delete old Docker images to ensure clean rebuild
4. Rebuild Docker images with new binaries
5. Force pods to restart and use new images
6. Test immediately

**Why delete Docker images?**
- Kubernetes with `imagePullPolicy: Never` uses local Minikube images
- Rebuilding with same tag doesn't guarantee Kubernetes sees the change
- Deleting old images forces a complete rebuild
- Deleting pods forces Kubernetes to load the freshly built images

These steps test local e5s changes in a real Kubernetes environment before release.

---

## Step 10: Verify mTLS Authentication

Check that your server and client are using proper mTLS with SPIRE identities:

Check client logs - should show successful response

```bash
kubectl logs -l app=e5s-client
```

Check server logs - should show authenticated request

```bash
kubectl logs -l app=e5s-server
```

**Expected client logs**:
```
2025/01/15 10:15:23 Starting e5s mTLS client...
2025/01/15 10:15:23 → Requesting server time from: https://e5s-server:8443/time
2025/01/15 10:15:23 → Initializing SPIRE client and fetching SPIFFE identity...
2025/01/15 10:15:24 ✓ Received response: HTTP 200 OK
2025/01/15 10:15:24 ← Server response: Server time: 2025-01-15T10:15:24Z
Server time: 2025-01-15T10:15:24Z
```

**Expected server logs**:
```
2025/01/15 10:15:24 ✓ Authenticated request from: spiffe://example.org/ns/default/sa/default
2025/01/15 10:15:24 → Sending response: Server time: 2025-01-15T10:15:24Z
```

This confirms:
- ✓ Client successfully obtained SPIFFE identity from SPIRE
- ✓ Client sent GET request to fetch server time using mTLS
- ✓ Server verified client's certificate during TLS handshake
- ✓ Server responded with its current time
- ✓ Complete request/response flow is visible in the logs

**View SPIRE server logs to see certificate issuance:**

```bash
kubectl logs -n spire -l app.kubernetes.io/name=server -c spire-server --tail=50
```

This confirms your local e5s code properly implements zero-trust mTLS with SPIRE.

---

## Step 11: Zero-Trust Security Demonstration

This section demonstrates that **only workloads with SPIRE identities can communicate**. We'll test both scenarios:
1. ✅ **Authenticated client** (has SPIRE identity) - succeeds
2. ❌ **Unregistered client** (no SPIRE identity) - fails

### Create Unregistered Client Job

An unregistered client is one that:
- **HAS access to the SPIRE Workload API socket** (same infrastructure access)
- **NOT registered in SPIRE control plane** (no workload registration entry)
- Cannot obtain a SPIFFE identity (SPIRE refuses to issue one)
- Cannot perform mTLS handshake
- Will be rejected even though it uses the e5s library and has socket access

```bash
cat > k8s-unregistered-client-job.yaml <<'EOF'
apiVersion: batch/v1
kind: Job
metadata:
  name: e5s-unregistered-client
  namespace: default
spec:
  backoffLimit: 0
  template:
    metadata:
      labels:
        app: e5s-unregistered-client
    spec:
      serviceAccountName: default
      restartPolicy: Never
      containers:
      - name: client
        image: e5s-client:dev
        imagePullPolicy: Never
        env:
        - name: SERVER_URL
          value: "https://e5s-server:8443/time"
        command: ["/app/client"]
        volumeMounts:
        - name: spire-agent-socket
          mountPath: /spire/agent-socket
          readOnly: true
        - name: config
          mountPath: /app/e5s.yaml
          subPath: e5s.yaml
      volumes:
      - name: spire-agent-socket
        csi:
          driver: "csi.spiffe.io"
          readOnly: true
      - name: config
        configMap:
          name: e5s-config
EOF
```

**Difference**: This client has the SPIRE socket mounted and can communicate with the SPIRE Agent, but it's not registered in the SPIRE control plane. SPIRE will refuse to issue it a SPIFFE identity.

### Run Both Tests

```bash
# Clean up any previous jobs
kubectl delete job e5s-client e5s-unregistered-client 2>/dev/null
sleep 2

echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "TEST 1: Authenticated Client (HAS SPIRE identity)"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""

# Deploy authenticated client
kubectl apply -f k8s-client-job.yaml
echo "Waiting for authenticated client..."
sleep 10

echo "Client Output:"
kubectl logs -l app=e5s-client
echo ""

echo "Server Logs (authenticated request):"
kubectl logs -l app=e5s-server --tail=3 | grep "Authenticated"
echo ""

echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "TEST 2: Unregistered Client (NO SPIRE identity)"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""

# Deploy unregistered client
kubectl apply -f k8s-unregistered-client-job.yaml
echo "Waiting for unregistered client to fail..."
sleep 30

echo "Unregistered Client Output (will fail):"
kubectl logs -l app=e5s-unregistered-client 2>&1
echo ""
```

### Expected Results

**Test 1: Authenticated Client (SUCCESS)** ✅

```
Client Logs:
2025/01/15 10:20:15 Starting e5s mTLS client...
2025/01/15 10:20:15 → Requesting server time from: https://e5s-server:8443/time
2025/01/15 10:20:15 → Initializing SPIRE client and fetching SPIFFE identity...
2025/01/15 10:20:16 ✓ Received response: HTTP 200 OK
2025/01/15 10:20:16 ← Server response: Server time: 2025-01-15T10:20:16Z
Server time: 2025-01-15T10:20:16Z

Server Logs:
2025/01/15 10:20:16 ✓ Authenticated request from: spiffe://example.org/ns/default/sa/default
2025/01/15 10:20:16 → Sending response: Server time: 2025-01-15T10:20:16Z
```

**What happened:**
1. Client connected to SPIRE Workload API via CSI volume
2. SPIRE issued a SPIFFE identity: `spiffe://example.org/ns/default/sa/default`
3. Client sent `GET /time` request using mTLS
4. Server verified client's certificate during TLS handshake
5. Server responded with its current time
6. Client received and printed the response
7. **All communication steps are visible in the logs with timestamps**

**Test 2: Unregistered Client (FAILURE)** ❌

```
Client Logs:
2025/01/15 10:20:45 Starting e5s mTLS client...
2025/01/15 10:20:45 → Requesting server time from: https://e5s-server:8443/time
2025/01/15 10:20:45 → Initializing SPIRE client and fetching SPIFFE identity...
2025/01/15 10:21:15 ❌ Request failed: error fetching X.509 SVID: initial_fetch_timeout expired

Server Logs:
(no logs - server never received request)
```

**What happened:**
1. Client attempted to initialize e5s library
2. Client tried to request server time
3. Client connected to SPIRE Workload API socket successfully
4. Client requested a SPIFFE identity from SPIRE
5. **SPIRE refused** - this workload has no registration entry in the control plane
6. Client waited for identity for 30 seconds (default initial_fetch_timeout)
7. Client timed out and failed with error
8. **Client failed during startup** - never obtained certificate, never sent HTTP request
9. Server never receives the time request
10. **Zero-trust enforced: no communication without valid identity**

### Security Analysis

This demonstrates **zero-trust mTLS security**:

| Component | Authenticated Client | Unregistered Client |
|-----------|---------------------|---------------------|
| SPIRE Socket | ✅ Mounted via CSI | ✅ Mounted via CSI |
| SPIRE Registration | ✅ Registered | ❌ Not registered |
| SPIFFE Identity | ✅ Obtained | ❌ SPIRE refused |
| mTLS Handshake | ✅ Successful | ❌ Cannot initiate |
| Server Communication | ✅ Allowed | ❌ Blocked |

**Key Insights:**

1. **Registration Required**: Even with socket access, workloads must be registered in SPIRE to get identities
2. **Control Plane Authorization**: SPIRE control plane enforces which workloads can obtain identities
3. **No Bypass**: Both clients have identical infrastructure access and code, but only registered clients work
4. **Defense in Depth**: Network access + code + configuration isn't enough - explicit registration required
5. **Zero-Trust**: Trust is based on cryptographic identity issued by SPIRE, not infrastructure access

### Clean Up Test Jobs

```bash
kubectl delete job e5s-client e5s-unregistered-client
```

---

## Step 12: Debug and Monitoring

### Check Pod Status

```bash
# List all pods
kubectl get pods

# Describe server pod for details
kubectl describe pod -l app=e5s-server

# Check if SPIRE socket is mounted
kubectl exec -l app=e5s-server -- ls -la /spire/agent-socket/
```

### View Server Logs

```bash
# Follow server logs in real-time
kubectl logs -l app=e5s-server -f

# View last 100 lines
kubectl logs -l app=e5s-server --tail=100
```

### Interactive Testing

Create an interactive pod to test manually:

```bash
kubectl run -it test-client --rm --restart=Never \
  --image=e5s-client:dev \
  --overrides='
{
  "spec": {
    "containers": [{
      "name": "test-client",
      "image": "e5s-client:dev",
      "command": ["/bin/sh"],
      "stdin": true,
      "tty": true,
      "env": [{"name": "SERVER_URL", "value": "https://e5s-server:8443/hello"}],
      "volumeMounts": [{
        "name": "spire-agent-socket",
        "mountPath": "/spire/agent-socket",
        "readOnly": true
      }, {
        "name": "config",
        "mountPath": "/app/e5s.yaml",
        "subPath": "e5s.yaml"
      }]
    }],
    "volumes": [{
      "name": "spire-agent-socket",
      "csi": {"driver": "csi.spiffe.io", "readOnly": true}
    }, {
      "name": "config",
      "configMap": {"name": "e5s-config"}
    }]
  }
}
'

# Inside the pod, run:
/app/client
```

---

## Common Testing Scenarios

### Testing Config Changes

If you modify `internal/config/`:

```bash
# 1. Update k8s-e5s-config.yaml with new config
vim k8s-e5s-config.yaml

# 2. Apply updated ConfigMap
kubectl apply -f k8s-e5s-config.yaml

# 3. Restart deployments to pick up new config
kubectl rollout restart deployment/e5s-server
kubectl delete job e5s-client
kubectl apply -f k8s-client-job.yaml

# 4. Check results
kubectl logs -l app=e5s-client
```

### Testing SPIRE Integration Changes

If you modify `pkg/spire/`:

```bash
# 1. Rebuild binaries
CGO_ENABLED=0 go build -o bin/server ./server
CGO_ENABLED=0 go build -o bin/client ./client

# 2. Point to Minikube's Docker and clean old images
eval $(minikube docker-env)
docker rmi e5s-server:dev e5s-client:dev 2>/dev/null || true

# 3. Rebuild Docker images
docker build -t e5s-server:dev -f - . <<'EOF'
FROM alpine:latest
WORKDIR /app
COPY bin/server .
ENTRYPOINT ["/app/server"]
EOF

docker build -t e5s-client:dev -f - . <<'EOF'
FROM alpine:latest
WORKDIR /app
COPY bin/client .
ENTRYPOINT ["/app/client"]
EOF

# 4. Force pods to use new images
kubectl delete pods -l app=e5s-server
kubectl wait --for=condition=ready pod -l app=e5s-server --timeout=60s

# 5. Watch SPIRE logs while testing
kubectl logs -n spire -l app.kubernetes.io/name=server -c spire-server --follow

# 6. Test certificate rotation
# SVIDs rotate every ~30 minutes - server should handle automatically
```

### Testing TLS Config Changes

If you modify `pkg/spiffehttp/`:

```bash
# 1. Rebuild and redeploy (see Step 9 workflow)

# 2. Use port-forward to inspect TLS from local machine
kubectl port-forward svc/e5s-server 8443:8443

# 3. In another terminal, inspect TLS handshake
openssl s_client -connect localhost:8443 -showcerts

# 4. Verify TLS 1.3 is enforced
# 5. Verify client certificate is required (should fail without client cert)
```

---

## Clean Up

After testing, delete Kubernetes resources.

```bash 
kubectl delete deployment e5s-server
kubectl delete service e5s-server
kubectl delete job e5s-client
kubectl delete configmap e5s-config
```

Clean up test directory files.

```bash
cd test-demo
rm -rf bin/
rm -f k8s-*.yaml
```

Remove entire test directory (Optional)

```bash
cd ..
rm -rf test-demo
```

Clean up Docker images from Minikube

```bash
eval $(minikube docker-env)
docker rmi e5s-server:dev e5s-client:dev
```

**To clean up SPIRE infrastructure**, follow the cleanup instructions in [SPIRE_SETUP.md](SPIRE_SETUP.md).

---

## Release Checklist

Before releasing a new version of e5s:

- [ ] All tests pass: `make test`
- [ ] Security checks pass: `make sec-all`
- [ ] Examples build: `make examples`
- [ ] Tutorial tested with local code (this guide)
- [ ] Documentation updated (README.md, TUTORIAL.md, ADVANCED.md)
- [ ] CHANGELOG updated
- [ ] Version bumped in code
- [ ] Git tag created
- [ ] Published to GitHub

After release, verify:

- [ ] Tutorial works with released version: `go get github.com/sufield/e5s@latest`
- [ ] Examples work for end users

---

## Troubleshooting

**Issue: "replace directive not working"**

```bash
# Verify replace directive is in go.mod
cat go.mod | grep replace

# Should show:
# replace github.com/sufield/e5s => ..

# Re-run go mod tidy
go mod tidy

# Verify e5s.go exists in parent directory
ls -la ../e5s.go
```

**Issue: "changes not reflected in build"**

```bash
# Always rebuild after changing e5s code
go build -o bin/server ./server
go build -o bin/client ./client

# Or use go run (rebuilds automatically)
go run ./server/main.go
```

**Issue: "import cycle detected"**

This means you're importing test code into the library. Keep test code separate from library code.

**For other issues**: See [TROUBLESHOOTING.md](TROUBLESHOOTING.md)

---

## Resources

- **End User Tutorial**: See [TUTORIAL.md](TUTORIAL.md) for the published library tutorial
- **SPIRE Setup**: See [SPIRE_SETUP.md](SPIRE_SETUP.md) for infrastructure setup
- **Troubleshooting**: See [TROUBLESHOOTING.md](TROUBLESHOOTING.md) for common issues
- **Advanced Patterns**: See [ADVANCED.md](ADVANCED.md) for advanced usage
- **Library Docs**: See [main README](../../README.md)

---

## Summary

You've successfully:
- Set up local development environment for e5s library
- Used `replace` directive to test local code changes
- Built static binaries and containerized them for Kubernetes
- Deployed and tested mTLS applications with local e5s code in Kubernetes
- Verified mTLS authentication works correctly with SPIRE
- Demonstrated zero-trust security by testing both authenticated and unregistered clients
- Learned how to iterate quickly on library changes using the Kubernetes workflow

**Notes**:
- The `replace` directive lets you test library changes locally before publishing
- SPIRE Workload API is only accessible inside Kubernetes pods, requiring containerized deployment
- The Kubernetes workflow ensures you test in a realistic environment matching production use
- **Helm** is used only for SPIRE infrastructure installation (prerequisite step)
- **kubectl** is used directly to deploy and test your applications (no Helm charts needed)

**Next Step**: Once testing is complete, follow the release checklist above to release a new version.
