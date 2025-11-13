# Testing Pre-Release

**Target Audience**: Internal developers.

**Purpose**: Test local e5s code changes in a realistic environment before releasing to end users.

**Time Required**: ~20 minutes

---

## When to Use This Guide

When:
- Developing new features for the e5s library
- Testing changes before creating a release
- Validating bug fixes in a real environment
- Testing the tutorial steps before release

---

## ⚡ Quick Start (Automated Scripts)

**For fast iterations**: Use these automated scripts to test your changes in ~30 seconds per iteration.

**Prerequisites**: SPIRE must be running:

```bash
make start-stack
```

```bash
eval $(minikube -p minikube docker-env)
```

### Initial Setup (Once)

```bash
./hack/test-prerelease.sh
```

Creates test environment and deploys both server and client. You should see:
```
Hello, spiffe://example.org/ns/default/sa/default!
```

### Test Your Changes (Repeat)

```bash
# 1. Make code changes
vim e5s.go

# 2. Test your changes
./hack/rebuild-and-test.sh
```

Rebuilds binaries, redeploys, and shows test results (~30 seconds).

### Cleanup

```bash
./hack/cleanup-prerelease.sh
```

**Note**: For understanding what these scripts do or manual step-by-step control, see the detailed guide below.

---

## Prerequisites

1. **SPIRE Infrastructure Running**: Start SPIRE in Minikube (~15 minutes)

   ```bash
   make start-stack
   ```

   This will:
   - Start Minikube cluster (if not running)
   - Install SPIRE Server and Agent via Helm
   - Configure trust domain: `example.org`
   - Wait for all SPIRE components to be ready

   The Makefile uses Helm to install SPIRE infrastructure. This guide deploys test applications using kubectl directly without using Helm.

2. **Local e5s Code**: You should be in the e5s project directory

   Verify you're in the right place:
   ```bash
   ls -la e5s.go spiffehttp/ spire/ examples/
   ```

   Should show the e5s library source code with public packages

3. **Build e5s CLI Tool**: Build the CLI tool for checking versions and managing configurations

   ```bash
   make build-cli
   ```

4. **Verify Required Tools**: Check that all required tools are installed

   ```bash
   make verify-tools
   ```

---

## Step 1: Create Test Application Directory

Create a test application that uses your local e5s code. 

Go to the e5s project root:

```bash
cd /path/to/e5s
```

Create a test directory:

```bash
mkdir -p demo
cd demo
```

Initialize Go module:

```bash
go mod init demo
```

---

## Step 2: Configure Local Dependency

Use the Go `replace` directive to point to your local e5s code instead of the released version:

Add replace directive to point to local e5s code. The '..' means parent directory (where e5s source code is):
```bash
go mod edit -replace github.com/sufield/e5s=..
```

Add chi router dependency:
```bash
go get github.com/go-chi/chi/v5
```

Add e5s to require section (will use local code due to replace directive):
```bash
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

- The `replace` directive tells Go to use the parent directory instead of downloading from GitHub
- Any `import "github.com/sufield/e5s"` in code will use local e5s code
- You can modify e5s code and immediately see changes in test application
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
    log.Printf("--------------Nov 10, 2025------(server main.go running...)")

		// Get current server time
		serverTime := time.Now().Format(time.RFC3339)
		response := fmt.Sprintf("Server time: %s", serverTime)
		log.Printf("→ Sending response: %s", response)
		fmt.Fprintf(w, "%s\n", response)
	})

	// Start server with automatic signal handling and graceful shutdown
	if err := e5s.Serve("e5s-server.yaml", r); err != nil {
		log.Fatal(err)
	}
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
	"net/http"
	"os"

	"github.com/sufield/e5s"
)

func main() {
  log.Printf("--------------Nov 10, 2025------(client main.go running...)")
	log.Println("Starting e5s mTLS client...")

	// Get server URL from environment variable
	// This follows the 12-factor app pattern: config in environment
	serverURL := os.Getenv("SERVER_URL")
	if serverURL == "" {
		log.Fatalf("❌ SERVER_URL environment variable not set")
	}

	// Perform mTLS GET request with automatic client lifecycle management
	err := e5s.WithClient("e5s-client.yaml", func(client *http.Client) error {
		resp, err := client.Get(serverURL)
		if err != nil {
			return err
		}
		defer resp.Body.Close()

		// Read and print response
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return err
		}
		log.Printf("← Server response: %s", string(body))
		fmt.Print(string(body))
		return nil
	})
	if err != nil {
		log.Fatal(err)
	}
}
```

**Configuration Approach:**

This follows the 12-factor app pattern:

1. **e5s Server Config** (`e5s-server.yaml`) - SPIRE/mTLS configuration for server processes
2. **e5s Client Config** (`e5s-client.yaml`) - SPIRE/mTLS configuration for client processes
3. **Environment Variables** - Application-specific settings (like SERVER_URL) come from environment

Each binary gets its own configuration file that describes only what that process does. This keeps infrastructure config separate from application config.

---

## Step 5: Create e5s Configuration Files

Use the e5s CLI tool to construct the SPIFFE ID to avoid mistakes.

The SPIFFE ID format for Kubernetes is: `spiffe://{trust-domain}/ns/{namespace}/sa/{service-account}`

**Where do these values come from?**

These values come from your Kubernetes deployment configuration. You need to know:

1. **Namespace**: The Kubernetes namespace where your client will run (e.g., `default`, `production`)
2. **Service Account**: The Kubernetes service account your client pod will use (e.g., `default`, `api-client`)

**How to find these values:**

From your Kubernetes deployment YAML:
```yaml
apiVersion: batch/v1
kind: Job
metadata:
  namespace: default          # <-- This is your namespace
spec:
  template:
    spec:
      serviceAccountName: default  # <-- This is your service account
```

**For this tutorial, we're using:**
- Namespace: `default` (standard Kubernetes namespace)
- Service Account: `default` (standard Kubernetes service account)

Since we know these values from our deployment configuration (defined in Step 8), we can construct the SPIFFE ID now using the CLI tool:

> **Note**: Discovery commands (`e5s discover pod/label/deployment`) require running pods and are shown later in **Step 11** after deployment. For initial configuration, we use the construction approach below since we already know the namespace and service account from our deployment YAML.

```bash
CLIENT_SPIFFE_ID=$(./bin/e5s spiffe-id k8s default default)
echo "Client SPIFFE ID: $CLIENT_SPIFFE_ID"
```

**What this command does:**
- `spiffe-id k8s` - construct a Kubernetes-style SPIFFE ID
- `default` (first) - Kubernetes namespace
- `default` (second) - Kubernetes service account name
- Trust domain is auto-detected from SPIRE installation

The trust domain is auto-detected from your SPIRE Helm installation or ConfigMap. If you need to see what trust domain was detected, run:

```bash
./bin/e5s discover trust-domain
```

If auto-detection fails (e.g., SPIRE not installed via Helm), you can explicitly specify the trust domain:

```bash
CLIENT_SPIFFE_ID=$(./bin/e5s spiffe-id k8s default default --trust-domain=example.org)
```

Output: `Client SPIFFE ID: spiffe://example.org/ns/default/sa/default`

Now create separate configuration files for server and client:

**Server configuration** (`e5s-server.yaml`):

```bash
cat > e5s-server.yaml <<EOF
spire:
  # Path to SPIRE Agent's Workload API socket
  workload_socket: unix:///tmp/spire-agent/public/api.sock

server:
  listen_addr: ":8443"
  # Only accept the specific registered client SPIFFE ID
  # This demonstrates zero-trust: even if SPIRE issues other identities,
  # the server only accepts the explicitly authorized client
  allowed_client_spiffe_id: "$CLIENT_SPIFFE_ID"
EOF
```

**Client configuration** (`e5s-client.yaml`):

```bash
cat > e5s-client.yaml <<EOF
spire:
  # Path to SPIRE Agent's Workload API socket
  workload_socket: unix:///tmp/spire-agent/public/api.sock

client:
  # Connect to any server in this trust domain
  expected_server_trust_domain: "example.org"
EOF
```

Each binary gets its own configuration file describing only that process's role.

---

## Step 6: Build Test Binaries

Build test applications. These builds will use LOCAL e5s code (due to replace directive) from test-demo directory.

Build server:
```bash
go build -o bin/server ./server
```

Build client:
```bash
go build -o bin/client ./client
```

Verify the binaries were created:

```bash
ls -lh bin/
```

Every time e5s library code is modified, rebuild these binaries to see the changes.

---

## Step 7: Create Kubernetes Configuration

**Why Kubernetes?** The SPIRE Workload API socket is only accessible inside Kubernetes pods, not from your local machine. You must deploy your test applications to Kubernetes.

For now, we'll use the default service account's SPIFFE ID. See **Step 11** for SPIFFE ID discovery instructions.

Create separate ConfigMaps for server and client configurations:

```bash
cat > k8s-configs.yaml <<'EOF'
apiVersion: v1
kind: ConfigMap
metadata:
  name: e5s-server-config
  namespace: default
data:
  e5s-server.yaml: |
    spire:
      # Path to SPIRE Agent socket inside Kubernetes pods
      workload_socket: unix:///spire/agent-socket/spire-agent.sock

    server:
      listen_addr: ":8443"
      # Only accept the specific registered client SPIFFE ID
      # This demonstrates zero-trust: even if SPIRE auto-registers other service accounts,
      # the server only accepts the explicitly authorized client identity
      allowed_client_spiffe_id: "spiffe://example.org/ns/default/sa/default"
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: e5s-client-config
  namespace: default
data:
  e5s-client.yaml: |
    spire:
      # Path to SPIRE Agent socket inside Kubernetes pods
      workload_socket: unix:///spire/agent-socket/spire-agent.sock

    client:
      expected_server_trust_domain: "example.org"
EOF
```

Apply the configuration:

```bash
kubectl apply -f k8s-configs.yaml
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
          mountPath: /app/e5s-server.yaml
          subPath: e5s-server.yaml
      volumes:
      - name: spire-agent-socket
        csi:
          driver: "csi.spiffe.io"
          readOnly: true
      - name: config
        configMap:
          name: e5s-server-config
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

Deploy the e5s server Deployment and Service to Kubernetes to run the server pod with SPIRE integration:
```bash
kubectl apply -f k8s-server.yaml
```

Wait for server to be ready:
```bash
kubectl wait --for=condition=ready pod -l app=e5s-server --timeout=60s
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
        command: ["/app/client"]
        env:
        - name: SERVER_URL
          value: "https://e5s-server:8443/time"
        volumeMounts:
        - name: spire-agent-socket
          mountPath: /spire/agent-socket
          readOnly: true
        - name: e5s-config
          mountPath: /app/e5s-client.yaml
          subPath: e5s-client.yaml
      volumes:
      - name: spire-agent-socket
        csi:
          driver: "csi.spiffe.io"
          readOnly: true
      - name: e5s-config
        configMap:
          name: e5s-client-config
EOF
```

**Configuration Approach:**
- Server URL comes from `SERVER_URL` environment variable (12-factor app pattern)
- SPIRE/mTLS config comes from role-specific ConfigMaps (`e5s-server-config` for server, `e5s-client-config` for client)
- Clean separation: infrastructure config vs application config, and server config vs client config

Run the test client:

```bash
# Run the client job (replace if it exists)
kubectl replace --force -f k8s-client-job.yaml

# Wait for job to complete
sleep 10

# Check the logs
kubectl logs -l app=e5s-client
```

**Note**: Kubernetes Jobs don't automatically restart. To re-run the client with fresh logs, use:

```bash
kubectl replace --force -f k8s-client-job.yaml
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

```
kubectl logs -l app=e5s-server
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

Make changes to e5s library code (go to e5s project root):
```bash
cd ..
vim e5s.go
```

Return to test-demo and rebuild binaries:
```bash
cd test-demo
CGO_ENABLED=0 go build -o bin/server ./server
CGO_ENABLED=0 go build -o bin/client ./client
```

Point to Minikube's Docker daemon:
```bash
eval $(minikube docker-env)
```

Remove old Docker images to force clean rebuild:
```bash
docker rmi e5s-server:dev e5s-client:dev 2>/dev/null || true
```

Rebuild Docker images with updated binaries:
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

Force server pods to restart with new image:
```bash
kubectl delete pods -l app=e5s-server
kubectl wait --for=condition=ready pod -l app=e5s-server --timeout=60s
```

Test with client using new image:
```bash
kubectl replace --force -f k8s-client-job.yaml
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

Check client logs - should show successful response:
```bash
kubectl logs -l app=e5s-client
```

Check server logs - should show authenticated request:
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

## Step 11: Discovering SPIFFE IDs for Configuration

**Why this matters:** For zero-trust security, you need to configure the exact SPIFFE ID that your client presents. This section shows you how to discover actual SPIFFE IDs from running workloads.

### Understanding SPIFFE ID Format

The SPIRE CSI driver automatically creates SPIFFE IDs following this format:

```
spiffe://{trust-domain}/ns/{namespace}/sa/{serviceaccount}
```

**Components:**
- `spiffe://` - SPIFFE URI scheme (always)
- `example.org` - Trust domain (configured in SPIRE)
- `ns/default` - Kubernetes namespace
- `sa/default` - Kubernetes service account name

**Example:** If your pod uses service account `my-client` in namespace `production`:
```
spiffe://example.org/ns/production/sa/my-client
```

---

### Method 1: Use the e5s CLI Tool ⭐ RECOMMENDED

The e5s CLI tool prevents manual errors by automatically discovering and constructing SPIFFE IDs.

**Quick Start:**

Discover SPIFFE ID using label selector (works for Jobs, Deployments, etc.):
```bash
./bin/e5s discover label app=e5s-client
```

Output: spiffe://example.org/ns/default/sa/default

Use in configuration. Copy the output and paste into your e5s-server.yaml:
```bash
allowed_client_spiffe_id: "spiffe://example.org/ns/default/sa/default"
```

**All Discovery Methods:**

**Discover from label selector:** ⭐ RECOMMENDED

This works for any resource type (Jobs, Deployments, StatefulSets, etc.):
```bash
./bin/e5s discover label app=e5s-client
```

Output: spiffe://example.org/ns/default/sa/default

**Discover from deployment:**

This works for Deployments (like our server):
```bash
./bin/e5s discover deployment e5s-server
```

Output: spiffe://example.org/ns/default/sa/default

**Discover from pod name:**

For Jobs, get the full pod name first (Job pods have generated names):
```bash
POD_NAME=$(kubectl get pods -l app=e5s-client -o name | head -1 | cut -d/ -f2)
./bin/e5s discover pod $POD_NAME
```

Output: spiffe://example.org/ns/default/sa/default

**Use in scripts to automatically configure server:**

Discover the client's actual SPIFFE ID:
```bash
CLIENT_ID=$(./bin/e5s discover label app=e5s-client)
```

Update server config with the discovered ID:
```bash
cat > k8s-configs.yaml <<EOF
apiVersion: v1
kind: ConfigMap
metadata:
  name: e5s-server-config
data:
  e5s-server.yaml: |
    spire:
      workload_socket: unix:///spire/agent-socket/spire-agent.sock
    server:
      listen_addr: ":8443"
      allowed_client_spiffe_id: "$CLIENT_ID"
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: e5s-client-config
data:
  e5s-client.yaml: |
    spire:
      workload_socket: unix:///spire/agent-socket/spire-agent.sock
    client:
      expected_server_trust_domain: "example.org"
EOF
```

Apply the configuration:
```bash
kubectl apply -f k8s-configs.yaml
```

**Construct SPIFFE IDs manually:**

If you know the namespace and service account:
```bash
./bin/e5s spiffe-id k8s example.org default api-client
```

Output: spiffe://example.org/ns/default/sa/api-client

This prevents manual typos when constructing SPIFFE IDs.

### Method 2: Check Server Logs

The e5s server logs show the SPIFFE ID of every authenticated client:

```bash
kubectl logs -l app=e5s-server --tail=20 | grep "Authenticated"
```

**Example output:**

```
2025/11/05 18:26:39 ✓ Authenticated request from: spiffe://example.org/ns/default/sa/default
```

This shows you what SPIFFE ID your client is presenting.

### Method 3: Query SPIRE Registration Entries

List all SPIFFE IDs that SPIRE has registered:

```bash
kubectl exec -n spire spire-server-0 -c spire-server -- spire-server entry show
```

**Example output:**

```
Entry ID         : minikube-cluster.7d3be6ed-190a-4cc9-9325-b851804cc00a
SPIFFE ID        : spiffe://example.org/ns/default/sa/default
Parent ID        : spiffe://example.org/spire/agent/k8s_psat/minikube-cluster/...
Selector         : k8s:pod-uid:007fa962-5ac0-4397-b4cc-96bd9b025874
Hint             : default
```

This shows:
- What SPIFFE IDs exist
- Which selectors (namespace, service account, pod labels) map to which IDs
- Auto-registered entries (from CSI driver) vs manually registered entries

### Method 4: Check Your Pod's Service Account

Find out what service account your pod is using:

```bash
kubectl get pod -l app=e5s-client -o jsonpath='{.spec.serviceAccountName}'
```

**Example output:**

```
default
```

Then construct the SPIFFE ID using the pattern:
```
spiffe://example.org/ns/default/sa/default
           └─────┬─────┘    └─┬──┘    └─┬──┘
           trust domain  namespace  service account
```

### Practical Example: Finding Your Client's SPIFFE ID

Consider a client pod with label `app=my-api-client`:

**Step 1:** Check what service account it uses:

```bash
kubectl get pod -l app=my-api-client -o jsonpath='{.spec.serviceAccountName}'
```

Output: api-client-sa

**Step 2:** Check what namespace it's in:

```bash
kubectl get pod -l app=my-api-client -o jsonpath='{.items[0].metadata.namespace}'
```

Output: production

**Step 3:** Check your SPIRE trust domain:

```bash
kubectl exec -n spire spire-server-0 -c spire-server -- spire-server entry show | grep "SPIFFE ID" | head -1
```

Output: SPIFFE ID        : spiffe://example.org/...

**Step 4:** Construct the SPIFFE ID:

```
spiffe://example.org/ns/production/sa/api-client-sa
```

**Step 5:** Verify by checking server logs after the client connects:

```bash
kubectl logs -l app=your-server | grep "Authenticated"
```

Should show: spiffe://example.org/ns/production/sa/api-client-sa

### Using SPIFFE IDs in Configuration

Once you know the SPIFFE ID, configure your server to allow only that specific identity:

**For exact identity matching (recommended for production):**

```yaml
server:
  listen_addr: ":8443"
  # Only allow this specific client identity
  allowed_client_spiffe_id: "spiffe://example.org/ns/default/sa/default"
```

**For trust domain matching (less secure, useful for development):**

```yaml
server:
  listen_addr: ":8443"
  # Allow ANY identity in this trust domain
  allowed_client_trust_domain: "example.org"
```

### Why This Matters

❌ **Wrong approach:** Guessing the SPIFFE ID or using trust domain matching in production

✅ **Right approach:** Discover actual SPIFFE IDs using observability, then configure explicit authorization

Your configuration should match reality (what identities are actually being used), not assumptions.

---

## Step 12: Zero-Trust Security Demonstration

This section demonstrates zero-trust authorization at the application layer. Even though SPIRE's CSI driver auto-registers all service accounts in the namespace, your application enforces explicit authorization.

**What's happening:**

- ✅ SPIRE CSI driver auto-creates identities for all service accounts: `spiffe://example.org/ns/{namespace}/sa/{serviceaccount}`
- ✅ Both the registered and unregistered clients can obtain SPIFFE identities from SPIRE
- 🔒 But the server only accepts ONE specific identity: `spiffe://example.org/ns/default/sa/default`
- ❌ Any other identity (like `unregistered-client`) is rejected at the application level

This is defense in depth: SPIRE provides the identity infrastructure, your application enforces the authorization policy.

We'll test both scenarios:

1. ✅ **Authorized client** (`serviceAccountName: default`) - allowed by server
2. ❌ **Unauthorized client** (`serviceAccountName: unregistered-client`) - rejected by server

### Create Unauthorized Client Job

An unauthorized client is one that:
- **HAS access to the SPIRE Workload API socket** (same infrastructure access)
- **DOES obtain a SPIFFE identity from SPIRE** (CSI driver auto-registers it)
- **Uses a different service account** (`unregistered-client` vs `default`)
- Gets identity: `spiffe://example.org/ns/default/sa/unregistered-client`
- **Is REJECTED by the server** because it's not in the allowed list
- Demonstrates application-level authorization enforcement

First, create a service account that is NOT registered with SPIRE:

```bash
kubectl create serviceaccount unregistered-client -n default
```

Then create the unregistered client job using that service account:

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
      serviceAccountName: unregistered-client
      restartPolicy: Never
      containers:
      - name: client
        image: e5s-client:dev
        imagePullPolicy: Never
        command: ["/app/client"]
        env:
        - name: SERVER_URL
          value: "https://e5s-server:8443/time"
        volumeMounts:
        - name: spire-agent-socket
          mountPath: /spire/agent-socket
          readOnly: true
        - name: e5s-config
          mountPath: /app/e5s-client.yaml
          subPath: e5s-client.yaml
      volumes:
      - name: spire-agent-socket
        csi:
          driver: "csi.spiffe.io"
          readOnly: true
      - name: e5s-config
        configMap:
          name: e5s-client-config
EOF
```

**Difference**: This client uses `serviceAccountName: unregistered-client` which has no SPIRE registration entry. The authenticated client uses `serviceAccountName: default` which is registered with SPIRE. Even though both have socket access, only the registered one will get a SPIFFE identity.

### Run Both Tests

Create service account for unregistered client (if not already created):
```bash
kubectl create serviceaccount unregistered-client -n default 2>/dev/null || true
```

Clean up any previous jobs:
```bash
kubectl delete job e5s-client e5s-unregistered-client 2>/dev/null || true
sleep 2
```

Run both tests:
```bash
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "TEST 1: Authenticated Client (HAS SPIRE identity)"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""
kubectl replace --force -f k8s-client-job.yaml 2>/dev/null || kubectl apply -f k8s-client-job.yaml
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

**Test 2: Unauthorized Client (FAILURE)** ❌

```
Client Logs:
2025/11/05 18:22:36 Starting e5s mTLS client...
2025/11/05 18:22:36 → Requesting server time from: https://e5s-server:8443/time
2025/11/05 18:22:36 → Initializing SPIRE client and fetching SPIFFE identity...
2025/11/05 18:22:37 ❌ Request failed: Get "https://e5s-server:8443/time": remote error: tls: bad certificate

Server Logs:
2025/11/05 18:22:37 ❌ Unauthorized request from 10.244.0.12:34567
```

**What happened:**

1. Client connected to SPIRE Workload API socket successfully
2. **SPIRE auto-registered the service account** via CSI driver
3. Client obtained SPIFFE identity: `spiffe://example.org/ns/default/sa/unregistered-client`
4. Client initiated mTLS connection to server with valid certificate
5. **Server verified the certificate but rejected the identity** - not in allowed list
6. Server expects: `spiffe://example.org/ns/default/sa/default`
7. Server got: `spiffe://example.org/ns/default/sa/unregistered-client`
8. **Server rejected the connection during TLS handshake**
9. Client received "bad certificate" error
10. **Zero-trust enforced: valid SPIFFE identity is not enough - explicit authorization required**

### Security Analysis

This demonstrates application-layer zero-trust authorization:

| Component | Authorized Client | Unauthorized Client |
|-----------|-------------------|---------------------|
| SPIRE Socket | ✅ Mounted via CSI | ✅ Mounted via CSI |
| SPIRE Auto-Registration | ✅ Auto-registered | ✅ Auto-registered |
| SPIFFE Identity | ✅ `...sa/default` | ✅ `...sa/unregistered-client` |
| mTLS Certificate | ✅ Valid cert | ✅ Valid cert |
| Server Authorization | ✅ In allowed list | ❌ NOT in allowed list |
| Server Communication | ✅ Allowed | ❌ Rejected |

**Security Principles:**

1. **Defense in Depth**: SPIRE provides identity, your application enforces authorization
2. **Explicit Authorization**: Having a valid SPIFFE identity is NOT enough - server explicitly allows specific identities
3. **Application Control**: Even though SPIRE auto-registers workloads, the application decides who can communicate
4. **No Implicit Trust**: Network access + valid certificate + same namespace ≠ authorization
5. **Zero-Trust**: Trust is based on explicit allow-lists, not infrastructure or identity provider alone

### Clean Up Test Jobs

```bash
kubectl delete job e5s-client e5s-unregistered-client
```

---

## Step 12: Debug and Monitoring

### Check Pod Status

List all pods:
```bash
kubectl get pods
```

Describe server pod for details:
```bash
kubectl describe pod -l app=e5s-server
```

Check if SPIRE socket is mounted:
```bash
kubectl exec -l app=e5s-server -- ls -la /spire/agent-socket/
```

### View Server Logs

Follow server logs in real-time:
```bash
kubectl logs -l app=e5s-server -f
```

View last 100 lines:
```bash
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
        "mountPath": "/app/e5s-client.yaml",
        "subPath": "e5s-client.yaml"
      }]
    }],
    "volumes": [{
      "name": "spire-agent-socket",
      "csi": {"driver": "csi.spiffe.io", "readOnly": true}
    }, {
      "name": "config",
      "configMap": {"name": "e5s-client-config"}
    }]
  }
}
'
```
Inside the pod, run:
```bash
/app/client
```

---

## Common Testing Scenarios

### Testing Config Changes

If you modify `internal/config/`:

1. Update k8s-configs.yaml with new config:
```bash
vim k8s-configs.yaml
```

2. Apply updated ConfigMap:
```bash
kubectl apply -f k8s-configs.yaml
```

3. Restart deployments to pick up new config:
```bash
kubectl rollout restart deployment/e5s-server
kubectl replace --force -f k8s-client-job.yaml
```

4. Check results:
```bash
kubectl logs -l app=e5s-client
```

### Restarting Server to Pick Up Code Changes

After making changes to the e5s library code, use this single command to rebuild and restart:

```bash
make restart-server
```

This command:
1. Rebuilds the server binary
2. Rebuilds the Docker image in Minikube
3. Deletes the server pod (Kubernetes automatically recreates it with the new image)
4. Waits for the new pod to be ready

Then test with a fresh client run:
```bash
make test-client
```

Or manually:
```bash
kubectl replace --force -f k8s-client-job.yaml
sleep 10
kubectl logs -l app=e5s-client
```

<details>
<summary>Manual steps (if make is not available)</summary>

If you can't use make, run these steps manually:

```bash
# 1. Rebuild server binary
CGO_ENABLED=0 go build -o bin/example-server ./examples/basic-server

# 2. Point to Minikube's Docker daemon
eval $(minikube docker-env)

# 3. Remove old Docker image
docker rmi e5s-server:dev 2>/dev/null || true

# 4. Rebuild Docker image
docker build -t e5s-server:dev -f - . <<'EOF'
FROM alpine:latest
WORKDIR /app
COPY bin/example-server .
ENTRYPOINT ["/app/example-server"]
EOF

# 5. Delete pod to force recreation
kubectl delete pod -l app=e5s-server

# 6. Wait for new pod
kubectl wait --for=condition=ready pod -l app=e5s-server --timeout=30s
```

</details>

### Testing SPIRE Integration Changes

If you modify `spire/`:

Rebuild binaries:
```bash
CGO_ENABLED=0 go build -o bin/server ./server
CGO_ENABLED=0 go build -o bin/client ./client
```

Point to Minikube's Docker and clean old images:
```bash
eval $(minikube docker-env)
docker rmi e5s-server:dev e5s-client:dev 2>/dev/null || true
```

Rebuild Docker images:
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

Force pods to use new images:
```bash
kubectl delete pods -l app=e5s-server
kubectl wait --for=condition=ready pod -l app=e5s-server --timeout=60s
```

Watch SPIRE logs while testing:
```bash
kubectl logs -n spire -l app.kubernetes.io/name=server -c spire-server --follow
```

Test certificate rotation. SVIDs rotate every ~30 minutes - server should handle automatically.

### Testing TLS Config Changes

If you modify `spiffehttp/`:

Rebuild and redeploy (see Step 9 workflow).

Use port-forward to inspect TLS from local machine:
```bash
kubectl port-forward svc/e5s-server 8443:8443
```

In another terminal, inspect TLS handshake:
```bash
openssl s_client -connect localhost:8443 -showcerts
```

Verify TLS 1.3 is enforced.

Verify client certificate is required (should fail without client cert).

---

## Clean Up

After testing, delete Kubernetes resources:

```bash
kubectl delete deployment e5s-server
kubectl delete service e5s-server
kubectl delete job e5s-client e5s-unregistered-client
kubectl delete configmap e5s-config
kubectl delete serviceaccount unregistered-client
```

Clean up test directory files:

```bash
cd test-demo
rm -rf bin/
rm -f k8s-*.yaml
```

Remove entire test directory: (Optional)

```bash
cd ..
rm -rf test-demo
```

Clean up Docker images from Minikube:

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

Verify replace directive is in go.mod:
```bash
cat go.mod | grep replace
```

Should show:
```
replace github.com/sufield/e5s => ..
```

Re-run go mod tidy:
```bash
go mod tidy
```

Verify e5s.go exists in parent directory:
```bash
ls -la ../e5s.go
```

**Issue: "changes not reflected in build"**

Always rebuild after changing e5s code:
```bash
go build -o bin/server ./server
go build -o bin/client ./client
```

Or use go run (rebuilds automatically):
```bash
go run ./server/main.go
```

**Issue: "import cycle detected"**

This means test code is imported into the library. Keep test code separate from library code.

**For other issues**: See [../../docs/reference/troubleshooting.md](../../docs/reference/troubleshooting.md)

---

## Resources

- **End User Tutorial**: See [TUTORIAL.md](TUTORIAL.md) for the published library tutorial
- **SPIRE Setup**: See [SPIRE_SETUP.md](SPIRE_SETUP.md) for infrastructure setup
- **Troubleshooting**: See [../../docs/reference/troubleshooting.md](../../docs/reference/troubleshooting.md) for common issues
- **Advanced Usage**: See [ADVANCED.md](ADVANCED.md) for advanced usage
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
- The `replace` directive lets you test library changes locally before release
- SPIRE Workload API is only accessible inside Kubernetes pods, requiring containerized deployment
- Kubernetes is used to test in a realistic production environment
- Helm is used only for SPIRE infrastructure installation (prerequisite step)
- kubectl is used directly to deploy and test your applications (no Helm charts needed)

**Next Step**: Once testing is complete, follow the release checklist above to release a new version.
