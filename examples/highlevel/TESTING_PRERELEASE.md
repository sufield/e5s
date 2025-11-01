# Testing Pre-Release: Internal Development Guide

**Target Audience**: Internal developers testing e5s library changes before publishing to GitHub.

**Purpose**: Test local e5s code changes in a realistic environment before releasing to end users.

**Time Required**:
- **Quick Start**: ~5 minutes (automated)
- **Manual Setup**: ~20 minutes (step-by-step)

---

## 🚀 Quick Start (Recommended)

**For fast testing with minimal steps:**

```bash
# 1. Ensure SPIRE is running (one-time setup)
#    Follow SPIRE_SETUP.md if not already done

# 2. Run automated setup (from project root)
./scripts/test-prerelease.sh

# 3. After making code changes, rebuild and test
./scripts/rebuild-and-test.sh

# 4. Clean up when done
./scripts/cleanup-prerelease.sh
```

**That's it!** The scripts handle all the setup, building, and deployment automatically.

**What the scripts do:**
- `test-prerelease.sh` - Creates test apps, builds binaries, builds Docker images, deploys to Kubernetes
- `rebuild-and-test.sh` - Rebuilds after code changes, redeploys, and shows test results
- `cleanup-prerelease.sh` - Removes all test resources

---

## 📖 Manual Setup (Optional)

**If you prefer step-by-step control or need to understand the process:**

Continue reading below for detailed manual instructions.

---

## When to Use This Guide

Use this guide when you:
- Are developing new features for the e5s library
- Need to test changes before creating a release
- Want to validate bug fixes in a real environment
- Are testing the tutorial steps before publishing

**For end users**: See [TUTORIAL.md](TUTORIAL.md) instead - this guide is for internal testing only.

---

## Prerequisites

Before starting, you must have:

1. **SPIRE Infrastructure Running**: Follow [SPIRE_SETUP.md](SPIRE_SETUP.md) to set up SPIRE in Minikube (~15 minutes)
   - Minikube cluster running
   - SPIRE Server and Agent installed via Helm
   - Server and client workloads registered

   **Note**: SPIRE_SETUP.md uses Helm to install SPIRE infrastructure. This guide deploys test applications using kubectl directly (not Helm).

2. **Required Tools**:
   - **Docker** - For building container images
   - **Minikube** - For Kubernetes cluster (installed via SPIRE_SETUP.md)
   - **kubectl** - For deploying applications (installed via SPIRE_SETUP.md)
   - **Helm** - For SPIRE installation only (installed via SPIRE_SETUP.md)

   ```bash
   # Verify tools are installed
   docker --version
   minikube version
   kubectl version --client
   helm version
   ```

3. **Go** - Programming language (1.21 or higher)
   ```bash
   go version
   # Should output: go version go1.21.0 or higher
   ```

4. **Local e5s Code**: You should be in the e5s project directory
   ```bash
   # Verify you're in the right place
   ls -la e5s.go pkg/ examples/
   # Should show the e5s library source code
   ```

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

**Verify your `go.mod`:**

```bash
cat go.mod
```

**Your `go.mod` should look like:**
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

You don't need to run `go mod tidy` until after you create the source files in Steps 3 and 4.

---

## Step 3: Create Server Application

Create `server/main.go`:

```go
package main

import (
	"fmt"
	"log"
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
			log.Printf("❌ Unauthorized request from %s", req.RemoteAddr)
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		log.Printf("✓ Authenticated request from: %s", id)
		fmt.Fprintf(w, "Hello, %s!\n", id)
	})

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
	// Get server URL from environment variable, default to localhost
	// This allows the client to work both locally and in Kubernetes
	serverURL := os.Getenv("SERVER_URL")
	if serverURL == "" {
		serverURL = "https://localhost:8443/hello"
	}

	// Perform mTLS GET request (uses local e5s code)
	resp, err := e5s.Get(serverURL)
	if err != nil {
		log.Fatal(err)
	}
	defer resp.Body.Close()

	// Read and print response
	body, _ := io.ReadAll(resp.Body)
	fmt.Println(string(body))
}
```

We include environment variable support from the start because we'll deploy to Kubernetes where the client needs to connect to a service name (e.g., `https://e5s-server:8443/hello`) instead of localhost.

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

kubectl apply -f k8s-e5s-config.yaml
```

---

## Step 8: Build and Deploy to Kubernetes

### Build Static Binaries

```bash
# Build static binaries (required for Alpine containers)
CGO_ENABLED=0 go build -o bin/server ./server
CGO_ENABLED=0 go build -o bin/client ./client
```

### Build Docker Images in Minikube

```bash
# Point Docker to Minikube's Docker daemon
eval $(minikube docker-env)

# Build server image
docker build -t e5s-server:dev -f - . <<'EOF'
FROM alpine:latest
WORKDIR /app
COPY bin/server .
ENTRYPOINT ["/app/server"]
EOF

# Build client image
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

kubectl apply -f k8s-server.yaml

# Wait for server to be ready
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
          value: "https://e5s-server:8443/hello"
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

kubectl apply -f k8s-client-job.yaml

# Wait a few seconds and check logs
sleep 10
kubectl logs -l app=e5s-client
```

**Expected output**:
```
Hello, spiffe://example.org/ns/default/sa/default!
```

**Success!** Your local e5s code is working in Kubernetes with SPIRE.

---

## Step 9: Iterate on Code Changes

Now you can quickly test e5s library changes:

### Workflow

```bash
# 1. Make changes to e5s library code
cd ..  # Go to e5s project root
vim e5s.go  # Edit any e5s files

# 2. Return to test-demo and rebuild binaries
cd test-demo
CGO_ENABLED=0 go build -o bin/server ./server
CGO_ENABLED=0 go build -o bin/client ./client

# 3. Rebuild Docker images
eval $(minikube docker-env)

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

# 4. Restart server deployment
kubectl rollout restart deployment/e5s-server

# 5. Test with client
kubectl delete job e5s-client
kubectl apply -f k8s-client-job.yaml
sleep 10
kubectl logs -l app=e5s-client
```

**Summary**:
1. Make changes to e5s library code
2. Rebuild binaries and Docker images
3. Redeploy to Kubernetes
4. Test immediately
5. Iterate quickly

This workflow lets you test local e5s changes in a real Kubernetes environment before publishing!

---

## Step 10: Verify mTLS Authentication

Check that your server and client are using proper mTLS with SPIRE identities:

```bash
# Check server logs
kubectl logs -l app=e5s-server

# Check client logs - should show successful response
kubectl logs -l app=e5s-client
```

**Expected client output**:
```
Hello, spiffe://example.org/ns/default/sa/default!
```

The response confirms:
- ✓ Client successfully obtained SPIFFE identity from SPIRE
- ✓ Client authenticated to server using mTLS
- ✓ Server verified client's certificate
- ✓ Server extracted and returned client's SPIFFE ID

**View SPIRE server logs to see certificate issuance:**

```bash
kubectl logs -n spire -l app.kubernetes.io/name=server -c spire-server --tail=50
```

This confirms your local e5s code properly implements zero-trust mTLS with SPIRE.

---

## Step 11: Debug and Monitoring

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
# 1. Rebuild binaries and images
CGO_ENABLED=0 go build -o bin/server ./server
CGO_ENABLED=0 go build -o bin/client ./client
eval $(minikube docker-env)
docker build -t e5s-server:dev -f - . <<'EOF'
FROM alpine:latest
WORKDIR /app
COPY bin/server .
ENTRYPOINT ["/app/server"]
EOF

# 2. Restart deployment
kubectl rollout restart deployment/e5s-server

# 3. Watch SPIRE logs while testing
kubectl logs -n spire -l app.kubernetes.io/name=server -c spire-server --follow

# 4. Test certificate rotation
# SVIDs rotate every ~30 minutes - server should handle automatically
```

### Testing TLS Config Changes

If you modify `pkg/spiffehttp/`:

```bash
# 1. Rebuild and redeploy (see Step 10 workflow)

# 2. Use port-forward to inspect TLS from local machine
kubectl port-forward svc/e5s-server 8443:8443

# 3. In another terminal, inspect TLS handshake
openssl s_client -connect localhost:8443 -showcerts

# 4. Verify TLS 1.3 is enforced
# 5. Verify client certificate is required (should fail without client cert)
```

---

## Clean Up

When you're done testing:

```bash
# 1. Delete Kubernetes resources
kubectl delete deployment e5s-server
kubectl delete service e5s-server
kubectl delete job e5s-client
kubectl delete configmap e5s-config

# 2. Clean up test directory files
cd test-demo
rm -rf bin/
rm -f k8s-*.yaml

# 3. (Optional) Remove entire test directory
cd ..
rm -rf test-demo

# 4. Clean up Docker images from Minikube
eval $(minikube docker-env)
docker rmi e5s-server:dev e5s-client:dev
```

**To clean up SPIRE infrastructure**, follow the cleanup instructions in [SPIRE_SETUP.md](SPIRE_SETUP.md).

---

## Publishing Checklist

Before publishing a new version of e5s:

- [ ] All tests pass: `make test`
- [ ] Security checks pass: `make sec-all`
- [ ] Examples build: `make examples`
- [ ] Tutorial tested with local code (this guide)
- [ ] Documentation updated (README.md, TUTORIAL.md, ADVANCED.md)
- [ ] CHANGELOG updated
- [ ] Version bumped in code
- [ ] Git tag created
- [ ] Published to GitHub

After publishing, verify:

- [ ] Tutorial works with published version: `go get github.com/sufield/e5s@latest`
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
- Learned how to iterate quickly on library changes using the Kubernetes workflow

**Key Takeaways**:
- The `replace` directive lets you test library changes locally before publishing
- SPIRE Workload API is only accessible inside Kubernetes pods, requiring containerized deployment
- The Kubernetes workflow ensures you test in a realistic environment matching production use
- **Helm** is used only for SPIRE infrastructure installation (prerequisite step)
- **kubectl** is used directly to deploy and test your applications (no Helm charts needed)

**Next Step**: Once testing is complete, follow the publishing checklist above to release a new version.
