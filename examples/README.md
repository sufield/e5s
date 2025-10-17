# SPIRE mTLS Examples - Quick Start Guide

This guide shows how to run the mTLS server examples using the project's automated Minikube infrastructure.

## Table of Contents

1. [Prerequisites](#prerequisites)
2. [Start SPIRE Infrastructure](#start-spire-infrastructure)
3. [Build the Example Server](#build-the-example-server)
4. [Register Workloads](#register-workloads)
5. [Run the Example Server](#run-the-example-server)
6. [Test the Server](#test-the-server)
7. [Cleanup](#cleanup)
8. [Troubleshooting](#troubleshooting)

---

## Prerequisites

### Required Tools

You need the following tools installed:

| Tool | Minimum Version | Installation |
|------|----------------|--------------|
| Go | 1.25+ | https://go.dev/dl/ |
| kubectl | 1.28+ | https://kubernetes.io/docs/tasks/tools/ |
| minikube | 1.32+ | https://minikube.sigs.k8s.io/docs/start/ |
| helm | 3.12+ | https://helm.sh/docs/intro/install/ |

### Quick Installation (Ubuntu 24.04)

```bash
# Install Go 1.25+
wget https://go.dev/dl/go1.25.3.linux-amd64.tar.gz
sudo rm -rf /usr/local/go
sudo tar -C /usr/local -xzf go1.25.3.linux-amd64.tar.gz
export PATH=$PATH:/usr/local/go/bin

# Install kubectl
curl -LO "https://dl.k8s.io/release/$(curl -L -s https://dl.k8s.io/release/stable.txt)/bin/linux/amd64/kubectl"
sudo install -o root -g root -m 0755 kubectl /usr/local/bin/kubectl

# Install minikube
curl -LO https://storage.googleapis.com/minikube/releases/latest/minikube-linux-amd64
sudo install minikube-linux-amd64 /usr/local/bin/minikube

# Install helm
curl https://raw.githubusercontent.com/helm/helm/main/scripts/get-helm-3 | bash

# Verify installations
go version
kubectl version --client
minikube version
helm version
```

### Verify Prerequisites

```bash
cd ~/hexagon/spire  # Or wherever you cloned the repo
make check-prereqs-k8s
```

**Expected output:**
```
Checking Kubernetes tools...
✓ helm found
✓ kubectl found
✓ minikube found
✓ Kubernetes tools satisfied
```

---

## Start SPIRE Infrastructure   

```bash
cd ~/hexagon/spire

# Start Minikube cluster and deploy SPIRE
make minikube-up
```

**This single command:**
- ✅ Starts a Minikube Kubernetes cluster
- ✅ Deploys SPIRE Server using Helm
- ✅ Deploys SPIRE Agent as a DaemonSet
- ✅ Waits for all components to be ready
- ✅ Creates the Workload API socket

**Expected output:**
```
Starting Minikube infrastructure...
→ Checking prerequisites...
✓ Prerequisites check passed
→ Starting Minikube cluster 'minikube'...
✓ Minikube cluster 'minikube' started successfully
→ Deploying SPIRE using helmfile...
✓ SPIRE deployed successfully
→ Waiting for SPIRE deployments to be ready...
✓ All SPIRE components are ready
✓ SPIRE server is healthy
```

### Verify SPIRE is Running

```bash
# Check status
make minikube-status

# Or manually check pods
kubectl get pods -n spire-system
```

**Expected output:**
```
NAME                            READY   STATUS    RESTARTS   AGE
spire-agent-xxxxx               1/1     Running   0          2m
spire-server-0                  2/2     Running   0          2m
```

---

## Build the Example Server

```bash
cd ~/hexagon/spire

# Run tests to verify everything works
make test

# Build the example server
go build -o bin/mtls-server ./examples/identityserver-example

# Verify binary was created
ls -lh bin/mtls-server
```

---

## Register Workloads

Create SPIRE registration entries for the server and client workloads:

```bash
# Register the server workload
# This grants the server the SPIFFE ID spiffe://example.org/server
kubectl exec -n spire-system spire-server-0 -c spire-server -- \
    /opt/spire/bin/spire-server entry create \
    -spiffeID spiffe://example.org/server \
    -parentID spiffe://example.org/spire/agent/k8s_psat/minikube/default \
    -selector k8s:ns:default \
    -selector k8s:sa:default \
    -selector k8s:container-name:mtls-server \
    -dns localhost \
    -dns mtls-server

# Register a client workload for testing
kubectl exec -n spire-system spire-server-0 -c spire-server -- \
    /opt/spire/bin/spire-server entry create \
    -spiffeID spiffe://example.org/client \
    -parentID spiffe://example.org/spire/agent/k8s_psat/minikube/default \
    -selector k8s:ns:default \
    -selector k8s:sa:default \
    -selector k8s:container-name:test-client

# Verify entries were created
kubectl exec -n spire-system spire-server-0 -c spire-server -- \
    /opt/spire/bin/spire-server entry show
```

The selectors above assume running workloads in Kubernetes pods (see next section).

---

## Run the Example Server

The example server needs to access the SPIRE agent socket. Since the socket is inside the Minikube node, you have two options:

### Option 1: Run in Kubernetes (Recommended)

Create a Kubernetes deployment that runs the example server:

```bash
# Create the server deployment
kubectl create deployment mtls-server \
    --image=golang:1.25 \
    -- sleep infinity

# Wait for pod to be ready
kubectl wait --for=condition=Ready pod -l app=mtls-server --timeout=60s

# Copy the binary to the pod
POD=$(kubectl get pod -l app=mtls-server -o jsonpath='{.items[0].metadata.name}')
kubectl cp bin/mtls-server $POD:/usr/local/bin/mtls-server

# Run the server in the pod
kubectl exec -it $POD -- /usr/local/bin/mtls-server
```

The server will start and connect to the SPIRE agent socket at `/var/run/spire/sockets/agent.sock` (default K8s location).

### Option 2: Run from Host (Alternative)

If you want to run the server directly on your host machine:

```bash
# SSH into minikube and run the server there
minikube ssh

# Inside minikube
cd /tmp
# You'll need to copy the binary and run it
# The socket is at /tmp/spire-agent/public/api.sock
```

---

## Test the Server

### Option 1: Create a Test Client Pod

```bash
# Create test client deployment
kubectl create deployment test-client \
    --image=golang:1.25 \
    -- sleep infinity

# Wait for pod
kubectl wait --for=condition=Ready pod -l app=test-client --timeout=60s

# Get pod name
CLIENT_POD=$(kubectl get pod -l app=test-client -o jsonpath='{.items[0].metadata.name}')

# Exec into the pod to create and run the test client
kubectl exec -it $CLIENT_POD -- /bin/bash
```

Inside the client pod:

```bash
# Initialize Go module and download dependencies
cd /tmp
go mod init test-client
go get github.com/spiffe/go-spiffe/v2@latest

# Create the test client program
cat > test-client.go <<'EOF'
package main

import (
    "context"
    "fmt"
    "io"
    "log"
    "net/http"
    "time"

    "github.com/spiffe/go-spiffe/v2/spiffeid"
    "github.com/spiffe/go-spiffe/v2/spiffetls/tlsconfig"
    "github.com/spiffe/go-spiffe/v2/workloadapi"
)

func main() {
    ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
    defer cancel()

    // Create X.509 source from SPIRE agent
    source, err := workloadapi.NewX509Source(ctx)
    if err != nil {
        log.Fatalf("Failed to create X.509 source: %v", err)
    }
    defer source.Close()

    // Configure TLS to accept any SPIFFE ID from example.org
    trustDomain := spiffeid.RequireTrustDomainFromString("example.org")
    tlsConfig := tlsconfig.MTLSClientConfig(source, source, tlsconfig.AuthorizeMemberOf(trustDomain))

    // Create HTTP client with mTLS
    client := &http.Client{
        Transport: &http.Transport{
            TLSClientConfig: tlsConfig,
        },
        Timeout: 10 * time.Second,
    }

    // Test endpoints
    endpoints := []string{
        "https://mtls-server:8443/",
        "https://mtls-server:8443/api/hello",
        "https://mtls-server:8443/health",
    }

    for _, url := range endpoints {
        fmt.Printf("\n=== Testing %s ===\n", url)
        resp, err := client.Get(url)
        if err != nil {
            log.Printf("Request failed: %v", err)
            continue
        }

        body, _ := io.ReadAll(resp.Body)
        resp.Body.Close()

        fmt.Printf("Status: %d\n", resp.StatusCode)
        fmt.Printf("Body: %s\n", string(body))
    }
}
EOF

# Run the test client
go run test-client.go
```

### Option 2: Use kubectl port-forward

```bash
# Forward the server port to your localhost
kubectl port-forward deployment/mtls-server 8443:8443
```

Then access from your host machine at `https://localhost:8443`.

---

## Cleanup

```bash
# Delete deployments
kubectl delete deployment mtls-server test-client

# Stop and delete Minikube cluster
make minikube-delete

# Or just stop it (keep data)
make minikube-down
```

---

## Troubleshooting

### Check SPIRE Status

```bash
# Quick status check
make minikube-status

# Detailed pod status
kubectl get pods -n spire-system

# Check server logs
kubectl logs -n spire-system spire-server-0 -c spire-server

# Check agent logs
kubectl logs -n spire-system daemonset/spire-agent
```

### Verify Workload Registration

```bash
# List all registration entries
kubectl exec -n spire-system spire-server-0 -c spire-server -- \
    /opt/spire/bin/spire-server entry show

# Check if workload can fetch SVID
kubectl exec $POD -- \
    /opt/spire/bin/spire-agent api fetch x509
```

### Server Won't Start

**Problem:** Server can't connect to SPIRE agent socket

**Solution:**
```bash
# Verify SPIRE agent is running
kubectl get pods -n spire-system -l app=spire-agent

# Check agent logs
kubectl logs -n spire-system -l app=spire-agent --tail=50

# Verify socket path
kubectl exec -n spire-system -l app=spire-agent -- \
    ls -la /var/run/spire/sockets/agent.sock
```

### Registration Entry Not Working

**Problem:** Workload can't get SVID

**Solution:**
```bash
# Check the parent ID matches your cluster
kubectl exec -n spire-system spire-server-0 -c spire-server -- \
    /opt/spire/bin/spire-server agent list

# Use the actual agent SPIFFE ID as parentID
# Example: spiffe://example.org/spire/agent/k8s_psat/minikube/default

# Delete wrong entry
kubectl exec -n spire-system spire-server-0 -c spire-server -- \
    /opt/spire/bin/spire-server entry delete -entryID <id>

# Create with correct parentID
kubectl exec -n spire-system spire-server-0 -c spire-server -- \
    /opt/spire/bin/spire-server entry create \
    -spiffeID spiffe://example.org/server \
    -parentID <correct-agent-spiffe-id> \
    -selector k8s:ns:default \
    -selector k8s:sa:default \
    -selector k8s:container-name:mtls-server
```

### Reset Everything

```bash
# Complete reset
make minikube-delete
make minikube-up

# Rebuild and redeploy
make test
go build -o bin/mtls-server ./examples/identityserver-example
# ... then follow deployment steps again
```

---

## Available Makefile Targets

```bash
make help                    # Show all available targets
make minikube-up             # Start Minikube + deploy SPIRE
make minikube-status         # Check SPIRE infrastructure status
make minikube-down           # Stop Minikube (keep data)
make minikube-delete         # Delete cluster completely
make test                    # Run all tests
make test-integration        # Run integration tests
make check-spire-ready       # Verify SPIRE is ready
```

---

## Next Steps

- **Customize handlers**: Modify `examples/identityserver-example/main.go` to add your own endpoints
- **Add authorization**: Implement application-level access control based on SPIFFE IDs
- **Production deployment**: See `docs/CONTROL_PLANE.md` for production Kubernetes deployment
- **Integration testing**: Run `make test-integration` to test against live SPIRE

---

## Why Minikube Instead of Manual SPIRE?

The automated Minikube approach provides:

✅ **Consistency**: Same SPIRE setup for examples and integration tests
✅ **Reproducibility**: One command (`make minikube-up`) vs 20+ manual steps
✅ **Production-like**: Tests run in Kubernetes like real deployments
✅ **Automated**: Scripts handle cluster setup, SPIRE deployment, health checks
✅ **Clean**: `make minikube-delete` removes everything

---

## References

- [SPIRE Documentation](https://spiffe.io/docs/latest/spire/)
- [Minikube Documentation](https://minikube.sigs.k8s.io/docs/)
- [Project Architecture](../docs/ARCHITECTURE_REVIEW.md)
- [Integration Test Infrastructure](../docs/INTEGRATION_TEST_OPTIMIZATION.md)
