# SPIRE Infrastructure Setup

This guide covers how to set up SPIRE infrastructure in Minikube for local development and testing.

**Time Required**: ~15 minutes

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

### Installing Prerequisites (if needed)

**macOS**:
```bash
brew install docker minikube kubectl helm
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

## Step 2: Install SPIRE

SPIRE has two main components:
- **SPIRE Server**: Central authority that issues identities
- **SPIRE Agent**: Runs on each node, provides Workload API to applications

The modern SPIRE Helm chart installs both components together.

### Clean Up Previous Installations (if any)

If you've previously attempted to install SPIRE, clean up first:

Clean up any previous installations (safe to run even if nothing exists)

```bash 
helm uninstall spire -n spire 2>/dev/null || true
helm uninstall spire-server -n spire 2>/dev/null || true
helm uninstall spire-agent -n spire 2>/dev/null || true
helm uninstall spire-crds -n spire 2>/dev/null || true
```

Delete namespace-scoped resources

```bash
kubectl delete namespace spire 2>/dev/null || true
```

Delete cluster-scoped resources (these can cause conflicts)

```bash
kubectl delete clusterrole spire-agent spire-server spire-controller-manager 2>/dev/null || true
kubectl delete clusterrolebinding spire-agent spire-server spire-controller-manager 2>/dev/null || true
kubectl delete csidriver csi.spiffe.io 2>/dev/null || true
kubectl delete validatingwebhookconfiguration spire-server 2>/dev/null || true
kubectl delete mutatingwebhookconfiguration spire-controller-manager 2>/dev/null || true
```

Delete CRDs (Custom Resource Definitions)

```bash
kubectl delete crd clusterspiffeids.spire.spiffe.io 2>/dev/null || true
kubectl delete crd clusterstaticentries.spire.spiffe.io 2>/dev/null || true
kubectl delete crd clusterfederatedtrustdomains.spire.spiffe.io 2>/dev/null || true
kubectl delete crd controllermanagerconfigs.spire.spiffe.io 2>/dev/null || true
```

Wait for cleanup to complete

```bash
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

**Verify SPIRE is running**:
```bash
kubectl get pods -n spire
```

**Expected output**:
```
NAME                                         READY   STATUS    RESTARTS   AGE
spire-agent-xxxxx                            1/1     Running   0          1m
spire-server-0                               2/2     Running   0          1m
spire-spiffe-csi-driver-xxxxx                2/2     Running   0          1m
spiffe-oidc-discovery-provider-xxxxx         2/2     Running   0          1m
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

## Next Steps

After completing this SPIRE setup:
- **For end users**: Follow the [TUTORIAL.md](TUTORIAL.md) to build and run mTLS applications
- **For internal testing**: Follow the [TESTING_PRERELEASE.md](TESTING_PRERELEASE.md) to test with local e5s code

---

## Clean Up

When you're done:

```bash
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

## Troubleshooting

See [TROUBLESHOOTING.md](TROUBLESHOOTING.md) for common SPIRE installation and configuration issues.
