# Changelog

All notable changes to the e5s project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [0.1.0] - 2025-11-03

### Added

#### Core Library Features
- **High-Level API** for building mTLS services with SPIRE
  - `e5s.Run()` - Convention-over-configuration server that blocks until Ctrl+C
  - `e5s.Start()` - Config-file-driven server with explicit lifecycle management
  - `e5s.StartServer()` - Environment-variable-based server with intelligent defaults
  - `e5s.Client()` - Config-file-driven HTTP client with mTLS
  - `e5s.NewClient()` - Environment-variable-based HTTP client
  - `e5s.Get()` and `e5s.Post()` - Convenience functions for single requests
  - `e5s.PeerID()` - Extract authenticated peer's SPIFFE ID from requests
  - `e5s.PeerInfo()` - Extract full peer information (ID + certificates)

#### Low-Level API
- **pkg/spiffehttp** - HTTP server and client with SPIFFE mTLS support
  - `NewServerTLSConfig()` - Create server TLS config with client verification
  - `NewClientTLSConfig()` - Create client TLS config with server verification
  - Authorization policies: specific SPIFFE ID or trust domain matching
  - Automatic certificate rotation with zero downtime
  - TLS 1.3 enforcement with secure cipher suites

- **pkg/spire** - SPIRE Workload API integration
  - `NewSource()` - Connect to SPIRE and fetch X.509 identities
  - Automatic certificate rotation via SPIRE Workload API
  - Configurable initial fetch timeout
  - Thread-safe X509Source wrapper

#### Configuration
- YAML-based configuration with validation
- Environment variable support for container deployments
- Intelligent defaults (convention over configuration)
- Clear error messages for misconfigurations

#### Security
- Mutual TLS (mTLS) with SPIFFE identity verification
- Automatic certificate rotation (zero downtime)
- TLS 1.3 minimum (TLS 1.2 with secure ciphers allowed)
- ReadHeaderTimeout protection against Slowloris attacks
- Path traversal prevention in configuration loading
- Comprehensive security scanning:
  - gosec (Go security checker)
  - govulncheck (Go vulnerability scanner)
  - CodeQL (semantic code analysis)
  - gitleaks (secret scanning)
- Fuzzing tests for security-critical code paths
- All GitHub Actions dependencies pinned to commit SHAs

#### Documentation
- Comprehensive API documentation at pkg.go.dev
- Detailed API guide (docs/API.md)
- Quickstart guide (docs/QUICKSTART_LIBRARY.md)
- Architecture documentation (docs/e5s.md)
- Integration testing guide
- Security policy (SECURITY.md)
- Contributing guidelines (CONTRIBUTING.md)
- Issue and pull request templates

#### Distribution
- Multi-architecture binaries (Linux/macOS, amd64/arm64)
- Docker images for examples:
  - `ghcr.io/sufield/e5s-demo-server`
  - `ghcr.io/sufield/e5s-demo-client`
- GoReleaser automation for releases
- Kubernetes/Helm deployment support
- Pre-built binaries with version information embedded

#### Development & Testing
- Comprehensive unit tests (41 tests)
- Integration tests with real SPIRE deployment
- Fuzzing tests (5 fuzz functions, 59k+ executions)
- GitHub Actions CI/CD workflows:
  - Linting (golangci-lint)
  - Security scanning (gosec, govulncheck, CodeQL, gitleaks)
  - Automated testing on push and PR
  - Weekly fuzzing runs
  - Release automation
- Dependabot configuration for automated dependency updates

### Changed
N/A - Initial release

### Deprecated
N/A - Initial release

### Removed
N/A - Initial release

### Fixed
N/A - Initial release

### Security
- All security best practices implemented from day one
- No known vulnerabilities in this release
- OpenSSF Scorecard compliance achieved

## Upgrade Guide

### From Nothing to v0.1.0

This is the first release. To get started:

1. **Install the library:**
   ```bash
   go get github.com/sufield/e5s@v0.1.0
   ```

2. **Quick start (server):**
   ```go
   package main

   import (
       "fmt"
       "net/http"
       "github.com/sufield/e5s"
   )

   func main() {
       http.HandleFunc("/api", func(w http.ResponseWriter, r *http.Request) {
           id, _ := e5s.PeerID(r)
           fmt.Fprintf(w, "Hello %s\n", id)
       })
       e5s.Run(http.DefaultServeMux)
   }
   ```

3. **Quick start (client):**
   ```go
   resp, err := e5s.Get("https://server:8443/api")
   ```

4. **Read the documentation:**
   - Quickstart: [docs/QUICKSTART_LIBRARY.md](docs/QUICKSTART_LIBRARY.md)
   - API Guide: [docs/API.md](docs/API.md)
   - Full API: https://pkg.go.dev/github.com/sufield/e5s

### Requirements

- **Go**: 1.23 or later
- **SPIRE**: 1.8.0 or later (1.10.0 recommended)
- **OS**: Linux, macOS, or Windows (with WSL for best experience)

### Breaking Changes

N/A - Initial release

### Migration Notes

N/A - Initial release

## Release Information

### v0.1.0 Release Notes

This is the initial production-ready release of e5s, a lightweight Go library for building mutual TLS services with SPIFFE identity verification.

**Should you upgrade?**
- If you're building new mTLS services with SPIRE, start here.
- If you're using raw TLS or custom mTLS implementations, consider migrating to benefit from automatic certificate rotation and SPIFFE identity verification.

**What's the upgrade impact?**
- **New projects**: Zero impact - just add the dependency and start coding.
- **Existing projects**: Requires code changes to integrate, but the high-level API minimizes migration effort. See the migration guide in docs/API.md.

**Known limitations:**
- This is a 0.x.x release, meaning the API may evolve based on user feedback.
- Currently supports HTTP/HTTPS only (no gRPC support yet, though the low-level API can be used to build gRPC support).
- Requires a SPIRE deployment (not suitable for projects without SPIRE infrastructure).

**Compatibility:**
- Go 1.23+ required
- SPIRE 1.8.0+ required (tested with 1.8.x, 1.9.x, 1.10.x)
- Linux, macOS, Windows (WSL)

**Release Assets:**
- Source code archives
- Pre-built binaries for Linux and macOS (amd64/arm64)
- Docker images for demo applications
- SHA256 checksums for verification

For detailed information, see the [full documentation](https://github.com/sufield/e5s/tree/v0.1.0/docs).

---

[0.1.0]: https://github.com/sufield/e5s/releases/tag/v0.1.0
[Unreleased]: https://github.com/sufield/e5s/compare/v0.1.0...HEAD
