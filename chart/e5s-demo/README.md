# e5s-demo Helm Chart

Helm chart for deploying the e5s SPIFFE/SPIRE mTLS demo server and client.

## Prerequisites

- Kubernetes cluster with SPIRE installed
- SPIRE CSI driver installed (`csi.spiffe.io`)
- Helm 3.x

## Installation

Docker images are automatically published to GitHub Container Registry when a release is created. Check [GitHub Releases](https://github.com/sufield/e5s/releases) for available versions.

### Quick Start with GoReleaser Images

Install using images from GoReleaser (replace v0.1.0 with desired version)

```bash
helm install e5s-demo ./chart/e5s-demo \
  --set server.image.tag=v0.1.0 \
  --set client.image.tag=v0.1.0
```

### Using Latest Images

```bash
helm install e5s-demo ./chart/e5s-demo
```

## Configuration

The following table lists the configurable parameters:

| Parameter | Description | Default |
|-----------|-------------|---------|
| `server.enabled` | Enable server deployment | `true` |
| `server.image.repository` | Server image repository | `ghcr.io/sufield/e5s-demo-server` |
| `server.image.tag` | Server image tag | `latest` |
| `server.replicas` | Number of server replicas | `1` |
| `client.enabled` | Enable client job | `true` |
| `client.image.repository` | Client image repository | `ghcr.io/sufield/e5s-demo-client` |
| `client.image.tag` | Client image tag | `latest` |
| `client.job.enabled` | Run client as Kubernetes Job | `true` |

See `values.yaml` for all configuration options.

## Customizing Configuration

Create a custom `values.yaml`:

```yaml
server:
  image:
    tag: v0.1.0
  config:
    server:
      allowed_client_spiffe_id: "spiffe://example.org/specific/client"

client:
  image:
    tag: v0.1.0
```

Install with custom values:

```bash
helm install e5s-demo ./chart/e5s-demo -f custom-values.yaml
```

## Uninstallation

```bash
helm uninstall e5s-demo
```

## Examples

### Deploy for specific Git tag

```bash
helm install e5s-demo ./chart/e5s-demo \
  --set server.image.tag=v0.2.0 \
  --set client.image.tag=v0.2.0
```

### Disable client job

```bash
helm install e5s-demo ./chart/e5s-demo \
  --set client.job.enabled=false
```

## Troubleshooting

### SPIRE Socket Not Found

Ensure SPIRE agent is running and the CSI driver is installed:

```bash
kubectl get pods -n spire-system
```

```bash
kubectl get csidriver
```

### Image Pull Errors

If using a specific tag, ensure the image exists:

```bash
docker pull ghcr.io/sufield/e5s-demo-server:v0.1.0
```
