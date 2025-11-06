# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added
- e5s CLI tool for SPIFFE ID management
  - `spiffe-id` command: Construct SPIFFE IDs from components
  - `discover` command: Discover SPIFFE IDs from Kubernetes resources
  - `validate` command: Validate e5s configuration files
  - `version` command: Show version information and environment details
- Command registry pattern for CLI
- TableWriter helper for formatted output
- Comprehensive Makefile targets for release automation
- Version tracking system with `VERSIONS.md` and `scripts/env-versions.sh`

### Changed
- Refactored CLI code structure with command registry
- Enhanced API with `Serve()` function for simplified server usage
- Enhanced `Get()` function with automatic logging
- Updated all documentation with CLI tool usage

### Compatibility

Tested with:
- Go 1.25.3
- go-spiffe SDK v2.6.0
- Helm v3.18.6
- minikube v1.37.0
- Docker v28.5.2
- kind v0.23.0
- golangci-lint v1.64.8
- SPIRE Helm Chart v0.27.0
- SPIRE Server v1.13.0
- SPIRE Agent v1.13.0

---

## Release Template

When creating a new release, copy this template:

```markdown
## [X.Y.Z] - YYYY-MM-DD

### Added
- New features go here

### Changed
- Changes to existing functionality go here

### Deprecated
- Features that will be removed in future releases

### Removed
- Features removed in this release

### Fixed
- Bug fixes go here

### Security
- Security-related changes go here

### Compatibility

Tested with:
- Go X.Y.Z
- Kubernetes vX.Y.Z
- kubectl vX.Y.Z
- Helm vX.Y.Z
- minikube vX.Y.Z
- SPIRE Server vX.Y.Z
- SPIRE Agent vX.Y.Z
- go-spiffe vX.Y.Z

To capture environment versions for this release:
\`\`\`bash
make env-versions
cat artifacts/env-versions-dev.txt
\`\`\`
```

---

## Future Releases

### v0.2.0 (Planned)
- Enhanced error messages
- Additional discovery methods
- Performance improvements

[Unreleased]: https://github.com/sufield/e5s/compare/v0.1.0...HEAD
