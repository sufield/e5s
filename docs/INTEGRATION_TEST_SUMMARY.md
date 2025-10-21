# SPIRE Integration Tests - Quick Reference

## Running Tests

```bash
# Simple - runs everything automatically
make test-integration-ci
```

This command:
1. Auto-creates SPIRE registration entries (if needed)
2. Waits for SPIRE agent socket to be ready
3. Runs integration tests in Kubernetes

## First Time Setup (Distroless Server)

If SPIRE server uses a distroless image, you need shell access to create registrations:

```bash
# Enable shell, create registrations, run tests
make spire-server-shell-enable
make register-test-workload
make test-integration-ci
```

## Troubleshooting

| Error | Cause | Fix |
|-------|-------|-----|
| `context deadline exceeded` | Missing registration entries | `make register-test-workload` |
| `executable file not found` | Distroless server | `make spire-server-shell-enable` |
| Socket wait timeout | SPIRE agent not running | `kubectl logs -l app=spire-agent -n spire-system` |

## How It Works

**Registration entries** map workload selectors to SPIFFE IDs:

```bash
# Entry created by setup-spire-registrations.sh
SPIFFE ID: spiffe://example.org/integration-test
Parent ID: spiffe://example.org/spire/agent/k8s_psat/... (auto-detected)
Selectors:
  - k8s:ns:spire-system
  - k8s:sa:default
  - k8s:pod-label:app:spire-integration-test
```

**Critical:** Parent ID must match the actual attested agent's SPIFFE ID. The setup script auto-detects this.

## Key Scripts

| Script | Purpose |
|--------|---------|
| `scripts/run-integration-tests-ci.sh` | Run tests (calls registration script automatically) |
| `scripts/setup-spire-registrations.sh` | Create/verify registration entries |
| `scripts/spire-server-enable-shell.sh` | Toggle distroless ↔ non-distroless image |

## Makefile Targets

```bash
make test-integration-ci           # Run integration tests (recommended)
make register-test-workload        # Create registration entries only
make spire-server-shell-enable     # Enable shell access (for distroless)
make spire-server-shell-disable    # Switch back to distroless
make spire-server-shell-status     # Check current image type
```

## Related Docs

- [Distroless Workaround](SPIRE_DISTROLESS_WORKAROUND.md) - Detailed distroless troubleshooting
