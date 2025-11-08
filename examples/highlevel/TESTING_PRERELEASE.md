# Testing Pre-Release

**Target Audience**: Internal developers.

**Purpose**: Test local e5s code changes in a realistic environment before releasing to end users.

**Time Required**: ~20 minutes

---

## When to Use This Guide

When you:
- Are developing new features for the e5s library
- Need to test changes before creating a release
- Want to validate bug fixes in a real environment
- Are testing the tutorial steps before release

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
   docker version
   minikube version
   kubectl version --client
   helm version
   ```

3. **Go** - Programming language (1.25.3 or higher)
   ```bash
   go version
   ```

4. **Local e5s Code**: You should be in the e5s project directory

   Verify you're in the right place

   ```bash
   ls -la e5s.go spiffehttp/ spire/ examples/
   ```

   Should show the e5s library source code with public packages

---

## Step 1: Create Test Application Directory

Create a test application that uses your local e5s code:

```bash
# Navigate to the e5s project root
cd /path/to/e5s  # Where your e5s code is located

# Create a test directory
mkdir -p demo
cd demo

# Initialize Go module
go mod init demo
```

---

## Step 2: Configure Local Dependency

Use the Go `replace` directive to point to your local e5s code instead of the released version:

Add replace directive to point to local e5s code. The '..' means parent directory (where e5s source code is)

```bash
go mod edit -replace github.com/sufield/e5s=..
```

Add chi router dependency

```bash
go get github.com/go-chi/chi/v5
```

Add e5s to require section (will use local code due to replace directive)

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
	"os"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/sufield/e5s"
)

func main() {
	// Set config path for e5s to use
	os.Setenv("E5S_CONFIG", "e5s.yaml")

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
    log.Printf("--------------Nov 6, 2025------(server main.go running...)")

		// Get current server time
		serverTime := time.Now().Format(time.RFC3339)
		response := fmt.Sprintf("Server time: %s", serverTime)
		log.Printf("→ Sending response: %s", response)
		fmt.Fprintf(w, "%s\n", response)
	})

	// Serve handles SPIRE connection, mTLS setup, signal handling, and graceful shutdown
	if err := e5s.Serve(r); err != nil {
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
	"os"

	"gopkg.in/yaml.v3"
	"github.com/sufield/e5s"
)

// AppConfig represents the client application configuration
// In a real application, you would define your own config structure
type AppConfig struct {
	ServerURL string `yaml:"server_url"`
}

func main() {
  log.Printf("--------------Nov 6, 2025------(client main.go running...)")
	log.Println("Starting e5s mTLS client...")

	// Load application-specific configuration
	// This demonstrates the real-world use: your app manages its own config
	cfg, err := loadAppConfig("client-config.yaml")
	if err != nil {
		log.Fatalf("❌ Failed to load app config: %v", err)
	}

	// Allow SERVER_URL environment variable to override config
	// This is common for Kubernetes deployments
	serverURL := os.Getenv("SERVER_URL")
	if serverURL == "" {
		serverURL = cfg.ServerURL
	}
	if serverURL == "" {
		log.Fatalf("❌ server_url not set in config and SERVER_URL environment variable not set")
	}

	// Set E5S_CONFIG for the library to use
	os.Setenv("E5S_CONFIG", "e5s.yaml")

	// Perform mTLS GET using high-level API
	// e5s.Get() handles client creation, SPIRE connection, and cleanup
	resp, err := e5s.Get(serverURL)
	if err != nil {
		log.Fatal(err)
	}
	defer resp.Body.Close()

	// Read and print response
	body, _ := io.ReadAll(resp.Body)
	log.Printf("← Server response: %s", string(body))
	fmt.Print(string(body))
}

// loadAppConfig loads the application-specific configuration
// This demonstrates the real-world pattern: applications manage their own config files
func loadAppConfig(path string) (*AppConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var cfg AppConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}

	return &cfg, nil
}
```

**Real-World Approach:**

This approach is for production applications:

1. **Application Config** (`client-config.yaml`) - Your app manages its own application-specific configuration including server URLs, timeouts, etc.
2. **e5s Config** (`e5s.yaml`) - SPIRE/mTLS configuration managed by the e5s library
3. **Environment Overrides** - Allow environment variables to override config values for deployment flexibility

This separation of concerns is standard practice.

---

## Step 5: Create Configuration Files

### Create Application Configuration

Create `client-config.yaml`:

```bash
cat > client-config.yaml <<'EOF'
# Application-specific configuration
# This is YOUR application's config - not part of e5s library
server_url: "https://localhost:8443/time"
EOF
```

This file contains settings specific to your application (like server URLs, timeouts, etc.).

### Create e5s Library Configuration

The configuration below includes hardcoded SPIFFE IDs for simplicity in this initial setup. In production, you should discover SPIFFE IDs using the e5s CLI tool to avoid typos:

>
> ```bash
> # Build the CLI tool
> go build -o bin/e5s ./cmd/e5s
>
> # Discover SPIFFE ID from running pod
> ./bin/e5s discover pod e5s-client
> # Output: spiffe://example.org/ns/default/sa/default
>
> # Or construct SPIFFE ID from known values
> ./bin/e5s spiffe-id k8s example.org default default
> # Output: spiffe://example.org/ns/default/sa/default
> ```
>
> See **Step 11: Discovering SPIFFE IDs** for instructions on using the CLI tool.

Create `e5s.yaml`:

```bash
cat > e5s.yaml <<'EOF'
spire:
  # Path to SPIRE Agent's Workload API socket
  workload_socket: unix:///tmp/spire-agent/public/api.sock

  # (Optional) How long to wait for identity from SPIRE before failing startup
  # If not set, defaults to 30s
  # initial_fetch_timeout: 30s

server:
  listen_addr: ":8443"
  # Only accept the specific registered client SPIFFE ID
  # This demonstrates zero-trust: even if SPIRE issues other identities,
  # the server only accepts the explicitly authorized client
  allowed_client_spiffe_id: "spiffe://example.org/ns/default/sa/default"

client:
  # Connect to any server in this trust domain
  expected_server_trust_domain: "example.org"
EOF
```

This file contains SPIRE and mTLS settings used by the e5s library.

### Verify Configuration Files

Check that both files were created:

```bash
ls -la *.yaml
```

You should see:
```
client-config.yaml  # Your application's config
e5s.yaml           # e5s library config
```

### Add YAML Dependency

The client application needs a YAML parser to read `client-config.yaml`:

```bash
go get gopkg.in/yaml.v3
```

---

## Step 6: Build Test Binaries

Build test applications. These builds will use LOCAL e5s code (due to replace directive)
From test-demo directory:

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

Create ConfigMaps for both application and e5s configuration:

```bash
cat > k8s-configs.yaml <<'EOF'
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
      # Only accept the specific registered client SPIFFE ID
      # This demonstrates zero-trust: even if SPIRE auto-registers other service accounts,
      # the server only accepts the explicitly authorized client identity
      allowed_client_spiffe_id: "spiffe://example.org/ns/default/sa/default"

    client:
      expected_server_trust_domain: "example.org"
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: client-config
  namespace: default
data:
  client-config.yaml: |
    # Application-specific configuration
    # Server URL for Kubernetes service discovery
    server_url: "https://e5s-server:8443/time"
EOF
```

Apply the configuration:

```bash
kubectl apply -f k8s-configs.yaml
```

Both `e5s-config` ConfigMap and `client-config` ConfigMap get mounted into the client pod.

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
        command: ["/app/client"]
        volumeMounts:
        - name: spire-agent-socket
          mountPath: /spire/agent-socket
          readOnly: true
        - name: e5s-config
          mountPath: /app/e5s.yaml
          subPath: e5s.yaml
        - name: client-config
          mountPath: /app/client-config.yaml
          subPath: client-config.yaml
      volumes:
      - name: spire-agent-socket
        csi:
          driver: "csi.spiffe.io"
          readOnly: true
      - name: e5s-config
        configMap:
          name: e5s-config
      - name: client-config
        configMap:
          name: client-config
EOF
```

**Configuration-Driven:**
- Client reads server URL from `client-config.yaml` (no hardcoded values)
- SPIRE/mTLS config comes from `e5s.yaml`
- Environment variables can override config if needed (optional, not required)
- All configuration is explicit and visible in ConfigMaps

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

Build the CLI tool (one time)

```bash 
go build -o bin/e5s ./cmd/e5s
```

Discover SPIFFE ID from running pod

```bash
./bin/e5s discover pod e5s-client
```

Output: spiffe://example.org/ns/default/sa/default

Use in configuration. Copy the output and paste into your e5s.yaml:

```bash
allowed_client_spiffe_id: "spiffe://example.org/ns/default/sa/default"
```

**All Discovery Methods:**

**Discover from pod name:**
```bash
./bin/e5s discover pod e5s-client
```

Output: spiffe://example.org/ns/default/sa/default

**Discover from label selector:**

```bash
./bin/e5s discover label app=e5s-client
```

Output: spiffe://example.org/ns/default/sa/default

**Discover from deployment:**
```bash
./bin/e5s discover deployment e5s-client
```

Output: spiffe://example.org/ns/default/sa/default

**Use in scripts to automatically configure server:**

Discover the client's actual SPIFFE ID

```bash
CLIENT_ID=$(./bin/e5s discover pod e5s-client)

# Update server config with the discovered ID
cat > k8s-configs.yaml <<EOF
apiVersion: v1
kind: ConfigMap
metadata:
  name: e5s-config
data:
  e5s.yaml: |
    spire:
      workload_socket: unix:///spire/agent-socket/spire-agent.sock
    server:
      listen_addr: ":8443"
      allowed_client_spiffe_id: "$CLIENT_ID"
EOF
```

Apply the configuration

```bash
kubectl apply -f k8s-configs.yaml
```

**Construct SPIFFE IDs manually:**

If you know the namespace and service account

Output: spiffe://example.org/ns/default/sa/api-client

```bash
./bin/e5s spiffe-id k8s example.org default api-client
```

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

**principle:** Your configuration should match reality (what identities are actually being used), not assumptions.

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
        volumeMounts:
        - name: spire-agent-socket
          mountPath: /spire/agent-socket
          readOnly: true
        - name: e5s-config
          mountPath: /app/e5s.yaml
          subPath: e5s.yaml
        - name: client-config
          mountPath: /app/client-config.yaml
          subPath: client-config.yaml
      volumes:
      - name: spire-agent-socket
        csi:
          driver: "csi.spiffe.io"
          readOnly: true
      - name: e5s-config
        configMap:
          name: e5s-config
      - name: client-config
        configMap:
          name: client-config
EOF
```

**Difference**: This client uses `serviceAccountName: unregistered-client` which has no SPIRE registration entry. The authenticated client uses `serviceAccountName: default` which is registered with SPIRE. Even though both have socket access, only the registered one will get a SPIFFE identity.

### Run Both Tests

```bash
# Create service account for unregistered client (if not already created)
kubectl create serviceaccount unregistered-client -n default 2>/dev/null || true

# Clean up any previous jobs
kubectl delete job e5s-client e5s-unregistered-client 2>/dev/null || true
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

List all pods

```bash 
kubectl get pods
```

# Describe server pod for details

```bash
kubectl describe pod -l app=e5s-server
```

# Check if SPIRE socket is mounted

```bash
kubectl exec -l app=e5s-server -- ls -la /spire/agent-socket/
```

### View Server Logs

Follow server logs in real-time

```bash 
kubectl logs -l app=e5s-server -f
```

# View last 100 lines

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
```
Inside the pod, run:

```bash
/app/client
```

---

## Common Testing Scenarios

### Testing Config Changes

If you modify `internal/config/`:

```bash
# 1. Update k8s-configs.yaml with new config
vim k8s-configs.yaml

# 2. Apply updated ConfigMap
kubectl apply -f k8s-configs.yaml

# 3. Restart deployments to pick up new config
kubectl rollout restart deployment/e5s-server
kubectl delete job e5s-client
kubectl apply -f k8s-client-job.yaml

# 4. Check results
kubectl logs -l app=e5s-client
```

### Testing SPIRE Integration Changes

If you modify `spire/`:

1. Rebuild binaries

```bash
CGO_ENABLED=0 go build -o bin/server ./server
CGO_ENABLED=0 go build -o bin/client ./client
```

# 2. Point to Minikube's Docker and clean old images

```bash
eval $(minikube docker-env)
```

```bash
docker rmi e5s-server:dev e5s-client:dev 2>/dev/null || true
```

3. Rebuild Docker images

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

4. Force pods to use new images

```bash
kubectl delete pods -l app=e5s-server
```

```bash
kubectl wait --for=condition=ready pod -l app=e5s-server --timeout=60s
```

# 5. Watch SPIRE logs while testing

```bash
kubectl logs -n spire -l app.kubernetes.io/name=server -c spire-server --follow
```

6. Test certificate rotation
SVIDs rotate every ~30 minutes - server should handle automatically

### Testing TLS Config Changes

If you modify `spiffehttp/`:

1. Rebuild and redeploy (see Step 9 workflow)

2. Use port-forward to inspect TLS from local machine

```bash
kubectl port-forward svc/e5s-server 8443:8443
```

3. In another terminal, inspect TLS handshake

```bash
openssl s_client -connect localhost:8443 -showcerts
```

4. Verify TLS 1.3 is enforced
5. Verify client certificate is required (should fail without client cert)

---

## Clean Up

After testing, delete Kubernetes resources:

```bash
kubectl delete deployment e5s-server
kubectl delete service e5s-server
kubectl delete job e5s-client e5s-unregistered-client
kubectl delete configmap e5s-config client-config
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

This means test code is imported into the library. Keep test code separate from library code.

**For other issues**: See [TROUBLESHOOTING.md](TROUBLESHOOTING.md)

---

## Resources

- **End User Tutorial**: See [TUTORIAL.md](TUTORIAL.md) for the published library tutorial
- **SPIRE Setup**: See [SPIRE_SETUP.md](SPIRE_SETUP.md) for infrastructure setup
- **Troubleshooting**: See [TROUBLESHOOTING.md](TROUBLESHOOTING.md) for common issues
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
