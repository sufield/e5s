# Versioning

e5s follows [Semantic Versioning 2.0.0](https://semver.org/).

## Version Format

Versions are tagged in git using the format: `v<MAJOR>.<MINOR>.<PATCH>`

Examples:
- `v0.1.0` - Initial pre-1.0 release
- `v0.2.0` - New features added (pre-1.0)
- `v0.2.1` - Bug fix release
- `v1.0.0` - First stable release
- `v1.1.0` - New features (backward compatible)
- `v2.0.0` - Breaking changes

## Semantic Versioning Rules

Given a version number MAJOR.MINOR.PATCH:

- **MAJOR** version increments for incompatible API changes
- **MINOR** version increments for backward-compatible functionality additions
- **PATCH** version increments for backward-compatible bug fixes

## Pre-1.0 Development (Current Phase)

During the 0.x.x phase:
- The API is not yet stable
- Minor version bumps (0.x.0) may include breaking changes
- We'll clearly document any breaking changes in release notes
- Aim for 1.0.0 when the API is stable and battle-tested

## Version Information in Binaries

All released binaries include version information that can be viewed with the `--version` flag:

```bash
$ e5s-example-server --version
e5s-example-server v0.1.0
  commit: abc1234
  built:  2024-01-15T10:30:00Z
```

This version information is automatically injected during the build process by GoReleaser using git tags and commit information.

## Go Module Versioning

For Go modules, versions follow the Go module versioning conventions:

### Installing a Specific Version

```bash
# Latest version
go get github.com/sufield/e5s@latest

# Specific version
go get github.com/sufield/e5s@v0.1.0

# Specific commit (for development/testing)
go get github.com/sufield/e5s@commit-hash
```

### Version Constraints in go.mod

```go
require (
    github.com/sufield/e5s v0.1.0
)
```

## Release Process

1. **Tag the release:**
   ```bash
   git tag -a v0.1.0 -m "Release v0.1.0"
   git push origin v0.1.0
   ```

2. **Create GitHub Release:**
   - Go to https://github.com/sufield/e5s/releases/new
   - Select the tag
   - Add release notes describing changes
   - Publish the release

3. **Automated Build:**
   - GoReleaser automatically builds binaries for multiple platforms
   - Docker images are built and pushed to GHCR
   - Release artifacts are uploaded to GitHub Releases
   - pkg.go.dev is automatically updated

## Version History

### v0.1.0 (Planned First Release)

Initial release of the e5s library featuring:
- High-level API (e5s.Start, e5s.Client, e5s.Run)
- Low-level API (pkg/spiffehttp, pkg/spire)
- Automatic certificate rotation
- SPIFFE ID verification
- TLS 1.3 enforcement
- Convention-over-configuration design
- Comprehensive documentation
- Example applications
- Docker images for demos
- Kubernetes/Helm support

## Deprecation Policy

### During 0.x.x (Pre-1.0)

- Breaking changes will be clearly documented in release notes
- Deprecated features will be removed in the next minor version
- Migration guides will be provided for significant changes

### After 1.0.0

- Deprecated features will be marked and documented
- Deprecated features will remain for at least one major version
- Clear migration paths will be provided
- Breaking changes only in major version bumps

## Checking Your Version

### Library Version (Go Module)

Check your `go.mod` file:
```go
require github.com/sufield/e5s v0.1.0
```

Or run:
```bash
go list -m github.com/sufield/e5s
```

### Binary Version

Run the binary with `--version`:
```bash
e5s-example-server --version
```

### Docker Image Version

Check the image tag:
```bash
docker images | grep e5s-demo-server
```

Or run the container:
```bash
docker run ghcr.io/sufield/e5s-demo-server:v0.1.0 --version
```

## Support Policy

### Current Versions

- Latest stable release (v0.x.x currently)
- Previous minor version (for 30 days after new minor release)

### Long-Term Support (After 1.0.0)

Once v1.0.0 is released:
- Latest major version: Full support
- Previous major version: Security fixes for 6 months
- Older versions: Community support only

## Version Compatibility

### Go Version Compatibility

- Minimum Go version: 1.25.3
- We test against: Go 1.25.3 and latest stable
- We support: Current and previous Go minor versions

### SPIRE Version Compatibility

- Minimum SPIRE version: 1.8.0
- Recommended: Latest stable SPIRE release
- We test against: SPIRE 1.8.x, 1.9.x, 1.10.x

## Questions?

For questions about versioning or compatibility:
- Open an issue: https://github.com/sufield/e5s/issues
- Check the changelog: https://github.com/sufield/e5s/releases
- Read the migration guide: docs/MIGRATION.md
