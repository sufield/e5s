# SPIRE Distroless Server - Workaround Guide

## Problem

The SPIRE server is using a distroless image, which doesn't include a shell or standard utilities. This prevents using the `spire-server` CLI for creating registration entries.

**Error message**:
```
OCI runtime exec failed: exec failed: unable to start container process:
exec: "test": executable file not found in $PATH: unknown
```

## Quick Solution (Development)

### Option 1: Use Helper Script (Recommended)

```bash
# Enable shell access (switches to non-distroless image)
make spire-server-shell-enable

# Create registration entries
make register-test-workload

# Optional: Switch back to distroless
make spire-server-shell-disable
```

### Option 2: Manual Image Change

```bash
# For StatefulSet
kubectl set image statefulset/spire-server -n spire-system \
  spire-server=ghcr.io/spiffe/spire-server:1.9.0

# Wait for rollout
kubectl rollout status statefulset/spire-server -n spire-system

# Create registration entries
./scripts/setup-spire-registrations.sh
```

## Understanding the Images

### Non-Distroless (Development)
- **Image**: `ghcr.io/spiffe/spire-server:1.9.0`
- **Contains**: Shell (`/bin/sh`), standard utilities, debugging tools
- **Pros**: Easy CLI access, debugging, registration management
- **Cons**: Larger attack surface, bigger image size
- **Use for**: Development, testing, troubleshooting

### Distroless (Production)
- **Image**: `ghcr.io/spiffe/spire-server:1.9.0-distroless`
- **Contains**: Only the SPIRE server binary and minimal runtime
- **Pros**: Minimal attack surface, smaller image, production-ready
- **Cons**: No shell access, no CLI utilities
- **Use for**: Production deployments

## Check Current Image

```bash
# Via Make
make spire-server-shell-status

# Or manually
kubectl get statefulset/spire-server -n spire-system \
  -o jsonpath='{.spec.template.spec.containers[?(@.name=="spire-server")].image}'
```

## Production Solutions

For production environments where distroless is required:

### Option 1: SPIRE Server API

Enable the SPIRE Server API and use it for registration instead of CLI.

**Server configuration** (`server.conf`):
```hcl
server {
    # ... other config ...

    # Enable experimental API
    experimental {
        # Registration API
        named_pipe_path = "/tmp/spire-server/private/api.sock"
    }
}
```

**Create entries via API**:
```bash
# Use grpcurl or custom Go client
grpcurl -unix -plaintext \
  /tmp/spire-server/private/api.sock \
  spire.api.server.entry.v1.Entry/BatchCreateEntry
```

### Option 2: SPIRE Controller Manager (Kubernetes CRDs)

Use the SPIRE Controller Manager to manage registrations via Kubernetes Custom Resources.

**Install**:
```bash
helm repo add spiffe https://spiffe.github.io/helm-charts-hardened/
helm install spire-controller-manager spiffe/spire-controller-manager \
  -n spire-system
```

**Create entry via CRD**:
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

### Option 3: Pre-baked Registrations

Create registration entries during SPIRE server deployment via init container.

## Complete Workflow

### Development Setup

1. **Check current image**:
   ```bash
   make spire-server-shell-status
   ```

2. **If distroless, enable shell**:
   ```bash
   make spire-server-shell-enable
   ```

3. **Create registrations**:
   ```bash
   make register-test-workload
   ```

4. **Run integration tests**:
   ```bash
   make test-integration-ci
   ```

5. **Optional: Switch back to distroless**:
   ```bash
   make spire-server-shell-disable
   ```

### CI/CD Pipeline

The CI script (`scripts/run-integration-tests-ci.sh`) handles this automatically:
1. Detects if server is distroless
2. Shows helpful error message with workaround
3. Continues with tests (will fail if registrations missing)

**To fix in CI**:
- Pre-create registrations in cluster setup
- Or use non-distroless server for integration test environments
- Or use SPIRE Controller Manager with CRDs

## Troubleshooting

### Registration script fails with "executable file not found"

**Cause**: Server is distroless

**Fix**: Run `make spire-server-shell-enable` first

### Integration tests timeout with "context deadline exceeded"

**Cause**: No registration entries exist for test workloads

**Fix**:
1. Enable shell: `make spire-server-shell-enable`
2. Create entries: `make register-test-workload`
3. Run tests: `make test-integration-ci`

### Want to keep distroless in production but test locally

**Solution**: Use different images per environment

In development values:
```yaml
# values-dev.yaml
server:
  image:
    repository: ghcr.io/spiffe/spire-server
    tag: 1.9.0  # Non-distroless
```

In production values:
```yaml
# values-prod.yaml
server:
  image:
    repository: ghcr.io/spiffe/spire-server
    tag: 1.9.0-distroless  # Distroless
```

## Scripts Reference

| Script | Purpose |
|--------|---------|
| `scripts/spire-server-enable-shell.sh` | Switch between distroless/non-distroless |
| `scripts/setup-spire-registrations.sh` | Create SPIRE registration entries |
| `scripts/run-integration-tests-ci.sh` | Run integration tests (auto-registration) |

## Makefile Targets

| Target | Description |
|--------|-------------|
| `make spire-server-shell-enable` | Enable shell access (non-distroless) |
| `make spire-server-shell-disable` | Disable shell (distroless) |
| `make spire-server-shell-status` | Check current image type |
| `make register-test-workload` | Create registration entries |
| `make test-integration-ci` | Run integration tests |

## Related Documentation

- [Integration Test Improvements](INTEGRATION_TEST_IMPROVEMENTS.md)
- [SPIRE Integration Test Fix Guide](SPIRE_INTEGRATION_TEST_FIX.md)
- [SPIRE Server Registration API](https://spiffe.io/docs/latest/deploying/spire_server/#registration-api)
- [SPIRE Controller Manager](https://github.com/spiffe/spire-controller-manager)
