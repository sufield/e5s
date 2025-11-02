# e5s-test Helm Chart

Helm chart for deploying e5s test infrastructure in Kubernetes.

## Architecture

This chart deploys the **infrastructure** components needed for testing:
- Server Deployment
- Server Service
- Configuration ConfigMap

**Test Jobs are managed separately** via kubectl (not Helm) to avoid job immutability issues. The job templates in this chart are disabled by default and provided as reference only.

The testing scripts use this pattern:
```bash
# Helm manages persistent infrastructure
helm upgrade --install e5s-test ./charts/e5s-test

# kubectl manages ephemeral test jobs
kubectl apply -f job.yaml
kubectl wait --for=condition=complete job/test
kubectl delete job test
```

This mirrors production CI/CD patterns where:
- Helm manages long-lived infrastructure
- CI pipelines create/delete test jobs dynamically

## Prerequisites

- Kubernetes cluster (Minikube for local testing)
- SPIRE installed with CSI driver (`csi.spiffe.io`)
- Workload registration entries configured

## Usage

### Deploy Infrastructure

```bash
helm upgrade --install e5s-test ./charts/e5s-test \
    --set client.enabled=false \
    --set unregisteredClient.enabled=false
```

### Run Tests

Use the provided scripts:
```bash
./scripts/test-prerelease.sh      # Initial setup
./scripts/rebuild-and-test.sh     # After code changes
./scripts/cleanup-prerelease.sh   # Cleanup
```

## Values

| Key | Description | Default |
|-----|-------------|---------|
| `spire.workloadSocket` | SPIRE agent socket path | `unix:///spire/agent-socket/spire-agent.sock` |
| `spire.trustDomain` | SPIFFE trust domain | `example.org` |
| `server.listenAddr` | Server listen address | `:8443` |
| `server.replicas` | Number of server replicas | `1` |
| `server.image.repository` | Server image | `e5s-server` |
| `server.image.tag` | Server image tag | `dev` |
| `client.enabled` | Enable client job (disabled by default) | `false` |
| `unregisteredClient.enabled` | Enable unregistered client job | `false` |

## Development

### Validate chart
```bash
helm lint ./charts/e5s-test
helm template e5s-test ./charts/e5s-test --debug
```

### Test deployment
```bash
helm install e5s-test ./charts/e5s-test --dry-run --debug
```

## Production Use

For production deployments:
1. Override trust domain: `--set spire.trustDomain=production.example.com`
2. Use proper image tags: `--set server.image.tag=v1.2.3`
3. Set replicas: `--set server.replicas=3`
4. Create a values file for your environment
