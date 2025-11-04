# Version Information

## Current Release: v0.1.0

## Development Environment

Versions used for local development and testing:

| Component | Version | Notes |
|-----------|---------|-------|
| Go | 1.25.3 | Minimum required |
| Minikube | v1.30+ | For local Kubernetes testing |
| kubectl | v1.27+ | Kubernetes CLI |
| Helm | v3.12+ | Package manager for Kubernetes |
| Helmfile | v0.150.0+ | Optional, for orchestrating Helm releases |
| go-spiffe SDK | v2.6.0 | SPIFFE client library for Go |
| SPIRE | 1.13.3 | Identity framework |
| SPIRE Helm Chart | spiffe/spire 0.27.0 | Helm chart for SPIRE deployment |
| Falco | 4.19.4 | Optional runtime security monitoring |
| Falco Helm Chart | falcosecurity/falco 4.19.4 | Runtime security via Helm |
| Kubernetes | v1.31.0 | Minikube cluster version |
| golangci-lint | Latest | Static analysis tool |
| gosec | Latest | Security scanner |
| govulncheck | Latest | Vulnerability scanner |
| gitleaks | Latest | Secret scanner |

## Production Environment

Versions tested and recommended for production deployments:

| Component | Version | Notes |
|-----------|---------|-------|
| Go | 1.25.3 | Minimum required |
| go-spiffe SDK | v2.6.0 | SPIFFE client library for Go |
| SPIRE | 1.13.3+ | Minimum 1.9.0 |
| SPIRE Helm Chart | spiffe/spire 0.27.0+ | For Kubernetes deployments |
| Falco | 4.19.4+ | Optional runtime security monitoring |
| Falco Helm Chart | falcosecurity/falco 4.19.4+ | Enable with ENABLE_FALCO=true |
| Kubernetes | v1.31.0+ | Tested on v1.31.0 |
| kubectl | v1.27+ | Kubernetes CLI |
| Helm | v3.12+ | Package manager for Kubernetes |
| Helmfile | v0.150.0+ | Optional, for orchestrating Helm releases |

## Container Images

| Image | Registry | Tag | Digest |
|-------|----------|-----|--------|
| e5s-demo-server | ghcr.io/sufield/e5s-demo-server | v0.1.0 | - |
| e5s-demo-client | ghcr.io/sufield/e5s-demo-client | v0.1.0 | - |
| Alpine (base image) | docker.io/library/alpine | 3.22.2 | sha256:4b7ce07002c69e8f3d704a9c5d6fd3053be500b7f1c69fc0d80990c2ad8dd412 |

## Go Module

```bash
go get github.com/sufield/e5s@v0.1.0
```

## Release Artifacts

All release artifacts are available at: https://github.com/sufield/e5s/releases

Each release includes:
- Pre-built binaries (Linux/macOS, amd64/arm64)
- Docker images (multi-arch)
- Source code archives
- SHA256 checksums
- Cryptographic signatures (Cosign)

## Verification

### Verify Checksums
```bash
sha256sum -c e5s_0.1.0_SHA256SUMS
```

### Verify Signatures (Cosign)
```bash
cosign verify-blob \
  --certificate e5s_0.1.0_SHA256SUMS.pem \
  --signature e5s_0.1.0_SHA256SUMS.sig \
  --certificate-identity-regexp="https://github.com/sufield/e5s/" \
  --certificate-oidc-issuer="https://token.actions.githubusercontent.com" \
  e5s_0.1.0_SHA256SUMS
```

## Version History

| Version | Release Date | Notes |
|---------|--------------|-------|
| v0.1.0 | 2025-11-03 | Initial release |

## Support

- Bug Reports: https://github.com/sufield/e5s/issues
- Documentation: https://pkg.go.dev/github.com/sufield/e5s
- Release Notes: https://github.com/sufield/e5s/releases
