# SPIRE Distroless Server - Workaround

## Problem

Distroless SPIRE server images don't include a shell, preventing CLI access for creating registration entries.

**Error**:
```
OCI runtime exec failed: exec: "test": executable file not found in $PATH
```

## Quick Fix (Development)

```bash
# Switch to non-distroless image (enables CLI)
make spire-server-shell-enable

# Create registration entries
make register-test-workload

# Run tests
make test-integration-ci

# Optional: switch back to distroless
make spire-server-shell-disable
```

## Check Current Image

```bash
make spire-server-shell-status
```

## Image Comparison

| Image | Shell | CLI Access | Security | Use Case |
|-------|-------|------------|----------|----------|
| `spire-server:1.9.0` | ✅ Yes | ✅ Yes | Lower | Development, testing |
| `spire-server:1.9.0-distroless` | ❌ No | ❌ No | Higher | Production |

## Production Solutions

For production where distroless is required:

### Option 1: SPIRE Server API

Enable the SPIRE Server API in `server.conf`:

```hcl
server {
    experimental {
        named_pipe_path = "/tmp/spire-server/private/api.sock"
    }
}
```

Use `grpcurl` or custom client to create entries via API.

### Option 2: SPIRE Controller Manager (Kubernetes CRDs)

```bash
helm install spire-controller-manager spiffe/spire-controller-manager -n spire-system
```

Create entries via Kubernetes Custom Resources:

```yaml
apiVersion: spire.spiffe.io/v1alpha1
kind: ClusterSPIFFEID
metadata:
  name: integration-test
spec:
  spiffeIDTemplate: "spiffe://example.org/integration-test"
  podSelector:
    matchLabels:
      app: spire-integration-test
  workloadSelectorTemplates:
    - "k8s:ns:{{ .PodMeta.Namespace }}"
    - "k8s:sa:{{ .PodSpec.ServiceAccountName }}"
```

### Option 3: Init Container

Create registration entries during deployment via init container that runs before server starts.

## Helper Scripts

| Script | Purpose |
|--------|---------|
| `scripts/spire-server-enable-shell.sh enable` | Switch to non-distroless |
| `scripts/spire-server-enable-shell.sh disable` | Switch to distroless |
| `scripts/spire-server-enable-shell.sh status` | Check current image |
| `scripts/setup-spire-registrations.sh` | Create registration entries |

## Troubleshooting

**Registration fails with "executable file not found"**
→ Run `make spire-server-shell-enable` first

**Tests timeout with "context deadline exceeded"**
→ No registrations exist. Run `make register-test-workload`

**Want distroless in production but non-distroless in dev?**
→ Use different Helm values per environment:

```yaml
# values-dev.yaml
server:
  image:
    tag: 1.9.0  # Non-distroless

# values-prod.yaml
server:
  image:
    tag: 1.9.0-distroless  # Distroless
```

## References

- [SPIRE Server Registration API](https://spiffe.io/docs/latest/deploying/spire_server/#registration-api)
- [SPIRE Controller Manager](https://github.com/spiffe/spire-controller-manager)
