# Minikube Dev Infrastructure

This directory contains the complete Helm/Minikube infrastructure setup for local SPIRE development.

## Prerequisites

See [VERSION.md](../../../VERSION.md) for all required component versions.

- Minikube
- kubectl
- Helm
- Helmfile (optional, will fallback to direct helm commands)
- Docker (for Minikube driver)

## Quick Start

### Using Make (Recommended)

```bash
# Start Minikube and deploy SPIRE
make minikube-up

# Check infrastructure status
make minikube-status

# Stop and cleanup
make minikube-down

# Delete cluster completely
make minikube-delete
```

### Using Shell Scripts

```bash
# Start infrastructure
cd infra/dev/minikube/scripts
./cluster-up.sh start

# Check status
./cluster-down.sh status

# Stop infrastructure
./cluster-down.sh stop

# Delete cluster
./cluster-down.sh delete
```

### Using Go CLI (Dev Build)

```bash
# Build the dev binary
make dev-build

# Start infrastructure
./bin/cp-minikube up

# Check status
./bin/cp-minikube status

# Stop infrastructure
./bin/cp-minikube down
```

## Optional: Falco Runtime Security

Falco provides runtime security monitoring for SPIRE and mTLS applications. It detects threats by monitoring syscalls and container behavior.

### Installing Falco

```bash
# Deploy SPIRE + Falco
ENABLE_FALCO=true helmfile -e dev apply

# Or using make (if configured)
ENABLE_FALCO=true make minikube-up
```

### What Falco Monitors

1. **SPIRE Socket Access**
   - Unauthorized access to `/tmp/spire-agent/public/api.sock`
   - Permission modifications on SPIRE directories

2. **mTLS Server Behavior**
   - Unexpected port bindings (should be 8443)
   - Outbound connections from server containers
   - Shell spawning in security-sensitive containers

3. **Certificate Security**
   - Unauthorized .crt/.pem/.key file modifications
   - TLS handshake anomalies

4. **Container Escape Detection**
   - /proc manipulation attempts
   - Privileged container warnings
   - CAP_SYS_ADMIN usage

### Viewing Falco Alerts

```bash
# Real-time alerts
kubectl logs -n falco -l app.kubernetes.io/name=falco -f

# Filter by priority
kubectl logs -n falco -l app.kubernetes.io/name=falco | grep CRITICAL

# Export alerts to file
kubectl logs -n falco -l app.kubernetes.io/name=falco > falco-alerts.log
```

### Custom Rules

Falco is pre-configured with SPIRE-specific rules in `values-falco.yaml`:

- `Unauthorized SPIRE Socket Access` (CRITICAL)
- `mTLS Server Wrong Port Binding` (WARNING)
- `Suspicious Certificate File Modification` (CRITICAL)
- `Shell Spawned in mTLS Container` (WARNING)
- `SPIRE Directory Permission Modification` (CRITICAL)

### Disabling Falco

```bash
# Deploy without Falco (default)
helmfile -e dev apply

# Or explicitly disable
ENABLE_FALCO=false helmfile -e dev apply
```

### Resource Usage

Falco adds minimal overhead:
- CPU: 100m request, 500m limit
- Memory: 256Mi request, 512Mi limit
- Driver: modern_ebpf (no kernel module required)

For more details, see [security/README.md](../../../security/) and [security/FALCO_GUIDE.md](../../../security/FALCO_GUIDE.md).

## Directory Structure

```
examples/minikube-lowlevel/infra/
├── helmfile.yaml                  # Helmfile orchestration
├── values-minikube.yaml           # Dev-hardened values
├── values-minikube-secrets.yaml.template  # Secrets template
├── values-minikube-secrets.yaml   # Generated secrets (gitignored)
├── values-falco.yaml              # Falco security monitoring config (optional)
├── charts/                        # Downloaded Helm charts (cache)
└── ../scripts/
    ├── cluster-up.sh              # Start cluster and deploy SPIRE
    ├── cluster-down.sh            # Stop/cleanup cluster
    └── wait-ready.sh              # Wait for deployments
```

## Configuration

### Trust Domain
- **Dev**: `example.org`
- **Prod**: See `deploy/values/values-prod.yaml`

### Socket Path
- **Agent Socket**: `/tmp/agent.sock`
- Mounted as hostPath in Minikube

### Service Exposure
- **Dev**: NodePort (30081 for server, 30082 for agent)
- **Prod**: ClusterIP only

### Security Settings
All pods run with:
- `runAsNonRoot: true`
- `runAsUser: 1000`
- `seccompProfile: RuntimeDefault`
- `capabilities: drop ALL`
- `readOnlyRootFilesystem: true`

### Resource Limits
- **CPU**: 100m request, 1000m limit
- **Memory**: 128Mi request, 1Gi limit

## Accessing SPIRE

### SPIRE Server

```bash
# Port-forward to SPIRE server
kubectl port-forward -n spire-system svc/spire-server 8081:8081

# Or use NodePort
minikube service -n spire-system spire-server --url
```

### SPIRE Agent

```bash
# Check agent status
kubectl get ds -n spire-system spire-agent

# View agent logs
kubectl logs -n spire-system -l app=spire-agent
```

### Agent Socket

The agent socket is available at `/tmp/agent.sock` on each node:

```bash
# SSH into Minikube node
minikube ssh -p hexagon-spire

# Check socket
ls -la /tmp/agent.sock
```

## Workload Registration

```bash
# Create a workload entry
kubectl exec -n spire-system deployment/spire-server -- \
  /opt/spire/bin/spire-server entry create \
  -parentID spiffe://example.org/spire/agent/k8s_psat/hexagon-spire/default \
  -spiffeID spiffe://example.org/workload \
  -selector k8s:ns:default \
  -selector k8s:pod-label:app:my-app
```

## Troubleshooting

### Check Infrastructure Status

```bash
# Cluster status
minikube status -p hexagon-spire

# SPIRE components
kubectl get all -n spire-system

# Pod logs
kubectl logs -n spire-system deployment/spire-server
kubectl logs -n spire-system daemonset/spire-agent
```

### Reset Everything

```bash
# Complete cleanup
make minikube-delete

# Restart from scratch
make minikube-up
```

### Common Issues

**Minikube not starting**:
```bash
# Check Docker is running
docker ps

# Try deleting and recreating
minikube delete -p hexagon-spire
make minikube-up
```

**Helm charts not found**:
```bash
# Update Helm repos
helm repo add spiffe https://spiffe.github.io/helm-charts-hardened/
helm repo update
```

**Pods not starting**:
```bash
# Check events
kubectl get events -n spire-system --sort-by='.lastTimestamp'

# Check pod status
kubectl describe pod -n spire-system <pod-name>
```

## Development Workflow

1. **Start Infrastructure**: `make minikube-up`
2. **Deploy Workload**: Build and deploy your application
3. **Register Workload**: Create SPIRE entries
4. **Test**: Verify identity attestation
5. **Iterate**: Make changes and redeploy
6. **Cleanup**: `make minikube-down`

## Production Differences

See `deploy/values/values-prod.yaml` for production configuration differences:

| Setting | Dev | Prod |
|---------|-----|------|
| Service Type | NodePort | ClusterIP |
| Replicas | 1 | 3 (HA) |
| Log Level | DEBUG | INFO |
| CPU Limit | 1000m | 2000m |
| Memory Limit | 1Gi | 2Gi |
| Affinity | None | Pod Anti-Affinity |
| Node Selector | None | Dedicated nodes |

## CI/CD Integration

### Build Guardrails

The Makefile includes targets to prevent dev code from leaking into production:

```bash
# Verify production build excludes dev code
make test-prod-build
```

### Build Tags

All dev infrastructure uses `//go:build dev` tags:
- `internal/controlplane/adapters/helm/install_dev.go`
- `wiring/cp_helm_minikube_dev.go`
- `cmd/cp-minikube/main_dev.go`

Production builds will **NOT** include these files.

### Docker/Helm Exclusions

- `.dockerignore` - Excludes `infra/`, `cmd/cp-minikube/`, `internal/controlplane/`
- `.helmignore` - Excludes dev infrastructure, scripts, and Go source

## Customization

### Changing Trust Domain

Edit `values-minikube.yaml`:

```yaml
global:
  spiffe:
    trustDomain: "your-domain.com"
```

### Adding Plugins

Edit `values-minikube.yaml` under `nodeAttestor` or `keyManager`:

```yaml
nodeAttestor:
  - plugin: "k8s_psat"
    config:
      cluster: "hexagon-spire"
```

### Resource Adjustments

Edit `values-minikube.yaml`:

```yaml
resources:
  requests:
    cpu: 200m
    memory: 256Mi
  limits:
    cpu: 2000m
    memory: 2Gi
```

## References

- [SPIRE Documentation](https://spiffe.io/docs/latest/spire/)
- [Helm Charts](https://github.com/spiffe/helm-charts-hardened)
- [Minikube Documentation](https://minikube.sigs.k8s.io/docs/)
- [SPIFFE Workload API](https://github.com/spiffe/spiffe/blob/main/standards/SPIFFE_Workload_API.md)
