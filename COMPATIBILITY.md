# Version Compatibility

This project is tested against the following tool and platform versions.

> **Note**
> These are *tested* versions, not strict minimums.
> Older or newer versions may work, but are not guaranteed.

## Latest Verified Matrix

- **e5s**: 0.1.0
- **Go**: go1.25.3
- **go-spiffe SDK**: v2.6.0

### Kubernetes Stack (for development/staging/production)

- **Helm**: v3.18.6
- **minikube** (for local dev): v1.37.0
- **kubectl**: Client Version: v1.33.4

### SPIFFE / SPIRE

- **SPIRE Helm Chart**: v0.27.0 (from spiffe/helm-charts-hardened)
- **SPIRE Server**: v1.13.0 (via Helm chart)
- **SPIRE Agent**: v1.13.0 (via Helm chart)

### Container Tools

- **Docker**: v28.5.2
- **kind**: v0.23.0

### Development/Security Tools

- **golangci-lint**: v1.64.8
- **govulncheck**: installed
- **gosec**: dev version
- **gitleaks**: dev version

---

## How This File is Updated

1. Before cutting a release, run:

   ```bash
   make env-versions
   ```

2. Review the generated file in `artifacts/env-versions-*.txt`
3. Update this file and the `CHANGELOG.md` section for the release

---

## Notes

### Go Version

The project requires Go 1.25.3 or higher. This is enforced in `go.mod` and verified in CI.

### Kubernetes Version

The project is tested with Kubernetes 1.28+. The SPIRE CSI driver requires specific Kubernetes versions - see [SPIRE CSI Driver compatibility](https://github.com/spiffe/spire/blob/main/support/k8s/k8s-workload-registrar/README.md).

### SPIRE Version

SPIRE 1.13 is required for:
- Improved CSI driver support
- Automatic workload registration
- Enhanced security features

Older versions may work but are not tested.

### minikube vs Production Kubernetes

- **minikube** is used for local development and testing
- **Production** deployments should use managed Kubernetes (GKE, EKS, AKS) or self-hosted clusters
- The library works identically on both, but SPIRE setup differs slightly

---

## Checking Your Environment

Use the e5s CLI to verify your environment:

```bash
# Check runtime versions
e5s version

# Check development requirements
e5s version --mode dev

# Check production requirements
e5s version --mode prod

# Detailed version information
e5s version --verbose
```

Or use the Makefile:

```bash
# Capture current environment versions
make env-versions

# Run all release checks (includes version verification)
make release-check
```
