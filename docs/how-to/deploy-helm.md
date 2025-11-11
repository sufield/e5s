# Helm Chart Deployment Guide

Deploy the e5s SPIFFE/SPIRE mTLS demo using Helm with production-ready Docker images from GitHub Container Registry.

## Overview

The e5s Helm chart provides a simple way to deploy the demo server and client to Kubernetes using pre-built multi-arch Docker images from GoReleaser. This eliminates the need to build images locally and enables versioned deployments.

Docker images are automatically published to GitHub Container Registry (GHCR) when a release is created using GoReleaser. See [examples/minikube-lowlevel/](../examples/minikube-lowlevel/) for local development setup.

## Prerequisites

- Kubernetes cluster (1.20+)
- SPIRE Server and Agent installed
- SPIRE CSI Driver installed (`csi.spiffe.io`)
- Helm 3.x
- Configured SPIRE registration entries for the demo workloads

### Verify SPIRE Installation

```bash
# Check SPIRE components are running
kubectl get pods -n spire-system

# Verify CSI driver is installed
kubectl get csidriver csi.spiffe.io
```

## Finding Available Versions

Before deploying, determine which version to use:

### Option 1: Check GitHub Releases

```bash
# View available releases
curl -s https://api.github.com/repos/sufield/e5s/releases | grep '"tag_name"' | head -5

# Or visit: https://github.com/sufield/e5s/releases
```

### Option 2: Check Docker Image Tags

```bash
# List available server image tags (available after first GoReleaser release)
curl -s https://api.github.com/users/sufield/packages/container/e5s-demo-server/versions | grep '"name"' | head -10

# Or visit GitHub Packages (will be available after release):
# https://github.com/sufield?tab=packages&repo_name=e5s
```

### Option 3: Use Latest Release Tag

```bash
# Get the latest release version programmatically
LATEST_VERSION=$(curl -s https://api.github.com/repos/sufield/e5s/releases/latest | grep '"tag_name"' | cut -d'"' -f4)
echo "Latest version: $LATEST_VERSION"
```

**Current released version:** v0.1.0

## Quick Start

### Install with Latest Images

Use `latest` tag for the most recent build (suitable for development):

```bash
# Clone repository
git clone https://github.com/sufield/e5s.git
cd e5s

# Install using latest images
helm install e5s-demo ./chart/e5s-demo
```

### Install with Specific Version

For production, always pin to a specific released version:

```bash
# First, find the latest release (see "Finding Available Versions" above)
LATEST_VERSION=$(curl -s https://api.github.com/repos/sufield/e5s/releases/latest | grep '"tag_name"' | cut -d'"' -f4)

# Install using that version
helm install e5s-demo ./chart/e5s-demo \
  --set server.image.tag=$LATEST_VERSION \
  --set client.image.tag=$LATEST_VERSION
```

**Example with current release:**

```bash
# Install version 0.1.0 (current released version)
helm install e5s-demo ./chart/e5s-demo \
  --set server.image.tag=v0.1.0 \
  --set client.image.tag=v0.1.0
```

### Verify Deployment

```bash
# Check deployment status
kubectl get pods -l app.kubernetes.io/name=e5s-demo

# View server logs
kubectl logs -l app.kubernetes.io/component=server -f

# View client job logs
kubectl logs -l app.kubernetes.io/component=client -f
```

## Configuration

### Basic Configuration Options

| Parameter | Description | Default |
|-----------|-------------|---------|
| `namespace` | Kubernetes namespace | `default` |
| `server.enabled` | Enable server deployment | `true` |
| `server.image.repository` | Server image repository | `ghcr.io/sufield/e5s-demo-server` |
| `server.image.tag` | Server image tag | `latest` |
| `server.image.pullPolicy` | Image pull policy | `IfNotPresent` |
| `server.replicas` | Number of server replicas | `1` |
| `server.service.type` | Service type | `ClusterIP` |
| `server.service.port` | Service port | `8443` |
| `client.enabled` | Enable client job | `true` |
| `client.image.repository` | Client image repository | `ghcr.io/sufield/e5s-demo-client` |
| `client.image.tag` | Client image tag | `latest` |
| `client.job.enabled` | Run client as Kubernetes Job | `true` |
| `client.job.backoffLimit` | Job retry limit | `3` |

### e5s Configuration Options

| Parameter | Description | Default |
|-----------|-------------|---------|
| `server.config.spire.workload_socket` | SPIRE Workload API socket path | `unix:///spire/agent-socket/spire-agent.sock` |
| `server.config.server.listen_addr` | Server listen address | `:8443` |
| `server.config.server.allowed_client_trust_domain` | Allowed client trust domain | `example.org` |
| `server.config.server.allowed_client_spiffe_id` | Allowed client SPIFFE ID (optional) | `` |
| `client.config.spire.workload_socket` | SPIRE Workload API socket path | `unix:///spire/agent-socket/spire-agent.sock` |
| `client.config.client.server_address` | Server address to connect to | `e5s-demo-server:8443` |
| `client.config.client.expected_server_trust_domain` | Expected server trust domain | `example.org` |

### Advanced Configuration

#### Custom Values File

Create a `custom-values.yaml` file:

```yaml
# Namespace to deploy into
namespace: production

# Server configuration
server:
  image:
    tag: v0.1.0  # Use actual released version (see "Finding Available Versions")
  replicas: 3
  service:
    type: LoadBalancer
  config:
    server:
      listen_addr: ":8443"
      # Zero-trust: Allow only specific SPIFFE ID
      allowed_client_spiffe_id: "spiffe://example.org/ns/production/sa/api-client"

# Client configuration
client:
  image:
    tag: v0.1.0  # Use actual released version
  job:
    enabled: true
    backoffLimit: 5
  config:
    client:
      server_address: "e5s-demo-server.production.svc.cluster.local:8443"
      expected_server_trust_domain: "example.org"
```

Deploy with custom values:

```bash
helm install e5s-demo ./chart/e5s-demo -f custom-values.yaml
```

#### Trust Domain Configuration

For trust domain-based authorization (permissive):

```yaml
server:
  config:
    server:
      allowed_client_trust_domain: "example.org"
      # Do not set allowed_client_spiffe_id
```

For SPIFFE ID-based authorization (zero-trust):

```yaml
server:
  config:
    server:
      allowed_client_spiffe_id: "spiffe://example.org/ns/default/sa/api-client"
      # Do not set allowed_client_trust_domain
```

## Deployment Scenarios

### Development Environment

Quick deployment with latest images for testing:

```bash
helm install e5s-demo ./chart/e5s-demo \
  --set server.config.server.allowed_client_trust_domain=example.org
```

### Staging Environment

Versioned deployment with specific image tags:

```bash
# Get latest version
VERSION=$(curl -s https://api.github.com/repos/sufield/e5s/releases/latest | grep '"tag_name"' | cut -d'"' -f4)

helm install e5s-demo ./chart/e5s-demo \
  --namespace staging \
  --create-namespace \
  --set server.image.tag=$VERSION \
  --set client.image.tag=$VERSION \
  --set server.config.server.allowed_client_trust_domain=staging.example.org
```

### Production Environment

Zero-trust deployment with specific SPIFFE ID authorization:

```bash
# Get latest version
VERSION=$(curl -s https://api.github.com/repos/sufield/e5s/releases/latest | grep '"tag_name"' | cut -d'"' -f4)

# Deploy client first to get its SPIFFE ID
helm install e5s-demo ./chart/e5s-demo \
  --namespace production \
  --create-namespace \
  --set server.enabled=false \
  --set server.image.tag=$VERSION \
  --set client.image.tag=$VERSION

# Get client SPIFFE ID (using e5s CLI)
CLIENT_SPIFFE_ID=$(e5s discover pod e5s-demo-client)

# Upgrade to enable server with specific SPIFFE ID
helm upgrade e5s-demo ./chart/e5s-demo \
  --namespace production \
  --reuse-values \
  --set server.enabled=true \
  --set server.config.server.allowed_client_spiffe_id="$CLIENT_SPIFFE_ID"
```

### Multi-Architecture Deployment

The Helm chart automatically uses multi-arch images that support both amd64 and arm64:

```bash
# Get latest version
VERSION=$(curl -s https://api.github.com/repos/sufield/e5s/releases/latest | grep '"tag_name"' | cut -d'"' -f4)

# Kubernetes will pull the correct image for node architecture
helm install e5s-demo ./chart/e5s-demo \
  --set server.image.tag=$VERSION
```

For explicit architecture selection:

```bash
# Use arm64-specific tag
helm install e5s-demo ./chart/e5s-demo \
  --set server.image.tag=${VERSION}-arm64 \
  --set client.image.tag=${VERSION}-arm64
```

## Helm Operations

### Upgrade Deployment

```bash
# Get new version
NEW_VERSION=$(curl -s https://api.github.com/repos/sufield/e5s/releases/latest | grep '"tag_name"' | cut -d'"' -f4)

# Upgrade to new version
helm upgrade e5s-demo ./chart/e5s-demo \
  --set server.image.tag=$NEW_VERSION \
  --set client.image.tag=$NEW_VERSION
```

### Rollback Deployment

```bash
# View release history
helm history e5s-demo

# Rollback to previous version
helm rollback e5s-demo

# Rollback to specific revision
helm rollback e5s-demo 2
```

### Uninstall Deployment

```bash
# Uninstall release
helm uninstall e5s-demo

# Uninstall from specific namespace
helm uninstall e5s-demo -n production
```

### View Configuration

```bash
# View current values
helm get values e5s-demo

# View all values (including defaults)
helm get values e5s-demo --all

# View rendered manifests
helm get manifest e5s-demo
```

## Troubleshooting

### SPIRE Socket Not Found

**Error:**
```
Error: workload: unable to connect to /spire/agent-socket/spire-agent.sock
```

**Solution:**
```bash
# Check SPIRE agent is running
kubectl get pods -n spire-system -l app=spire-agent

# Verify CSI driver is installed
kubectl get csidriver csi.spiffe.io

# Check pod has CSI volume mounted
kubectl describe pod -l app.kubernetes.io/component=server | grep -A5 Volumes
```

### Image Pull Errors

**Error:**
```
Failed to pull image "ghcr.io/sufield/e5s-demo-server:v0.1.0": rpc error: code = NotFound
```

**Solution:**
```bash
# Verify image exists (after GoReleaser release is published)
docker pull ghcr.io/sufield/e5s-demo-server:v0.1.0

# Check available tags at GitHub Packages:
# https://github.com/sufield?tab=packages&repo_name=e5s

# If no published images exist yet, build images locally:
# See examples/minikube-lowlevel/ for local development setup

# Or use latest tag if published releases are available
helm upgrade e5s-demo ./chart/e5s-demo --set server.image.tag=latest
```

### SVID Not Available

**Error:**
```
Error: unable to fetch X.509 SVID: no identity found
```

**Solution:**
```bash
# Verify SPIRE registration entries exist
kubectl exec -n spire-system spire-server-0 -- \
  /opt/spire/bin/spire-server entry show

# Create registration entry for server
kubectl exec -n spire-system spire-server-0 -- \
  /opt/spire/bin/spire-server entry create \
  -spiffeID spiffe://example.org/ns/default/sa/default \
  -parentID spiffe://example.org/spire/agent/k8s_psat/demo-cluster/node-uid \
  -selector k8s:ns:default \
  -selector k8s:sa:default \
  -selector k8s:pod-label:app.kubernetes.io/component:server
```

### Client Job Fails

**Error:**
```
Back-off pulling image or ErrImagePull
```

**Solution:**
```bash
# Check job status
kubectl describe job -l app.kubernetes.io/component=client

# View job logs
kubectl logs -l app.kubernetes.io/component=client

# Increase backoff limit
helm upgrade e5s-demo ./chart/e5s-demo \
  --reuse-values \
  --set client.job.backoffLimit=5

# Delete failed job to retry
kubectl delete job -l app.kubernetes.io/component=client
helm upgrade e5s-demo ./chart/e5s-demo --reuse-values
```

### Authorization Failures

**Error:**
```
client connection rejected: SPIFFE ID not authorized
```

**Solution:**
```bash
# Verify client SPIFFE ID
kubectl logs -l app.kubernetes.io/component=client | grep "SPIFFE ID"

# Update server configuration with correct SPIFFE ID
helm upgrade e5s-demo ./chart/e5s-demo \
  --reuse-values \
  --set server.config.server.allowed_client_spiffe_id="spiffe://example.org/ns/default/sa/default"

# Or use trust domain for permissive testing
helm upgrade e5s-demo ./chart/e5s-demo \
  --reuse-values \
  --set server.config.server.allowed_client_trust_domain="example.org" \
  --set server.config.server.allowed_client_spiffe_id=""
```

## Best Practices

### Version Pinning

Always pin specific versions in production:

```yaml
server:
  image:
    tag: v0.1.0  # Never use 'latest' in production (use actual release version)
    pullPolicy: IfNotPresent  # Cache images
```

### Zero-Trust Authorization

Use specific SPIFFE ID authorization in production:

```yaml
server:
  config:
    server:
      allowed_client_spiffe_id: "spiffe://example.org/ns/production/sa/api-client"
      # Do not set allowed_client_trust_domain
```

### High Availability

Deploy multiple replicas with appropriate resources:

```yaml
server:
  replicas: 3
  resources:
    requests:
      cpu: 100m
      memory: 128Mi
    limits:
      cpu: 200m
      memory: 256Mi
```

### Resource Management

Set appropriate resource limits:

```yaml
server:
  resources:
    requests:
      cpu: 100m
      memory: 128Mi
    limits:
      cpu: 500m
      memory: 512Mi

client:
  resources:
    requests:
      cpu: 50m
      memory: 64Mi
    limits:
      cpu: 200m
      memory: 256Mi
```

### Configuration Validation

Use the e5s CLI to validate configuration before deployment:

```bash
# Extract and validate server config
helm template e5s-demo ./chart/e5s-demo \
  | kubectl get configmap e5s-demo-server-config -o jsonpath='{.data.e5s\.yaml}' \
  | e5s validate -
```

## Integration with CI/CD

### GitOps with ArgoCD

Create an ArgoCD application:

```yaml
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: e5s-demo
  namespace: argocd
spec:
  project: default
  source:
    repoURL: https://github.com/sufield/e5s.git
    targetRevision: v0.1.0  # Use actual release tag
    path: chart/e5s-demo
    helm:
      values: |
        server:
          image:
            tag: v0.1.0  # Match targetRevision
          config:
            server:
              allowed_client_spiffe_id: "spiffe://example.org/ns/production/sa/api-client"
        client:
          image:
            tag: v0.1.0  # Match targetRevision
  destination:
    server: https://kubernetes.default.svc
    namespace: production
```

### GitHub Actions

Deploy from CI/CD pipeline:

```yaml
- name: Deploy e5s demo
  run: |
    helm upgrade --install e5s-demo ./chart/e5s-demo \
      --namespace production \
      --create-namespace \
      --set server.image.tag=${{ github.ref_name }} \
      --set client.image.tag=${{ github.ref_name }} \
      --wait --timeout 5m
```

### Terraform

Manage Helm releases with Terraform:

```hcl
resource "helm_release" "e5s_demo" {
  name       = "e5s-demo"
  chart      = "./chart/e5s-demo"
  namespace  = "production"
  create_namespace = true

  set {
    name  = "server.image.tag"
    value = "v0.1.0"  # Use actual release version
  }

  set {
    name  = "client.image.tag"
    value = "v0.1.0"  # Use actual release version
  }

  set {
    name  = "server.config.server.allowed_client_spiffe_id"
    value = "spiffe://example.org/ns/production/sa/api-client"
  }
}
```

## Migration from Raw Manifests

If you're currently using raw Kubernetes manifests, migrate to Helm:

```bash
# Export current configuration
kubectl get deployment e5s-server -o yaml > server-deployment.yaml
kubectl get configmap e5s-server-config -o yaml > server-config.yaml

# Create values file based on current config
cat > migration-values.yaml <<EOF
server:
  replicas: $(kubectl get deployment e5s-server -o jsonpath='{.spec.replicas}')
  config:
    server:
      listen_addr: ":8443"
      allowed_client_spiffe_id: "$(kubectl get configmap e5s-server-config -o jsonpath='{.data.e5s\.yaml}' | grep allowed_client_spiffe_id | awk '{print $2}')"
EOF

# Delete old resources
kubectl delete deployment e5s-server
kubectl delete service e5s-server
kubectl delete configmap e5s-server-config

# Install with Helm
helm install e5s-demo ./chart/e5s-demo -f migration-values.yaml
```

## Additional Resources

- [Helm Chart README](../chart/e5s-demo/README.md)
- [e5s CLI Tool Documentation](../cmd/e5s/README.md)
- [e5s Configuration Reference](../reference/api.md)
- [SPIRE Documentation](https://spiffe.io/docs/latest/spire/)
- [GitHub Releases](https://github.com/sufield/e5s/releases) - Download binaries and view Docker image tags

## Support

For issues and questions:

- Open an issue: https://github.com/sufield/e5s/issues
- Check FAQ: [doc../explanation/faq.md](../explanation/faq.md)
- View examples: [examples/](../examples/)
