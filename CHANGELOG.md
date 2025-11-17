# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

### Changed

### Fixed

### Security

---

## [0.2.0] - 2025-11-17

### Added
- e5s CLI tool for SPIFFE ID management
  - `spiffe-id` command: Construct SPIFFE IDs from components
  - `discover` command: Discover SPIFFE IDs from Kubernetes resources
  - `validate` command: Validate e5s configuration files
  - `version` command: Show version information and environment details
  - `client` command: Make mTLS requests for data-plane debugging
  - `deploy` command: Deploy and manage e5s test environments
- Command registry pattern for CLI
- TableWriter helper for formatted output
- Comprehensive Makefile targets for release automation
- Version tracking system with `COMPATIBILITY.md` and `scripts/env-versions.sh`
- SUCCESS-PATH.md following Stu McLaren methodology for user journeys
- Comprehensive link checking with lychee
- Automated security scanning with gosec, govulncheck, and golangci-lint

### Changed
- Refactored CLI code structure with command registry
- Enhanced API with `Serve()` function for simplified server usage
- Enhanced `Get()` function with automatic logging
- Updated all documentation with CLI tool usage
- Cleaned up documentation navigation to single hub pattern
- Removed external SPIRE documentation cross-references
- Fixed all broken documentation links (27 fixes)

### Fixed
- Fixed test-demo directory gosec warning (unhandled w.Write error)
- Fixed TESTING_PRERELEASE.md expected output to match actual script behavior
- Fixed relative paths in documentation links

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

### v0.3.0 (Planned)
- Enhanced error messages
- Additional discovery methods
- Performance improvements

[Unreleased]: https://github.com/sufield/e5s/compare/v0.2.0...HEAD
[0.2.0]: https://github.com/sufield/e5s/compare/v0.1.0...v0.2.0
