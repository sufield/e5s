# Documentation Structure

Documentation is organized by audience and purpose to help you find what you need quickly.

## Categories

### [guide/](guide/) - User Documentation
**Audience**: Application developers using this library

How to use the library in your applications:
- [QUICKSTART.md](guide/QUICKSTART.md) - Get started with manual testing
- [BUILD_MODES.md](guide/BUILD_MODES.md) - Development vs production builds
- [PRODUCTION_VS_DEVELOPMENT.md](guide/PRODUCTION_VS_DEVELOPMENT.md) - Mode differences
- [PRODUCTION_WORKLOAD_API.md](guide/PRODUCTION_WORKLOAD_API.md) - Production API usage
- [TROUBLESHOOTING.md](guide/TROUBLESHOOTING.md) - Common issues and solutions
- [EDITOR_SETUP.md](guide/EDITOR_SETUP.md) - IDE configuration for contributors

### [architecture/](architecture/) - Design Reference
**Audience**: Advanced users, contributors, auditors

How the library works under the hood:
- [ARCHITECTURE.md](architecture/ARCHITECTURE.md) - Overall system design
- [CORE.md](architecture/CORE.md) - Core concepts and models
- [DOMAIN.md](architecture/DOMAIN.md) - Domain language and aggregates
- [PORT_CONTRACTS.md](architecture/PORT_CONTRACTS.md) - Hexagonal architecture ports
- [SPIFFE_ID_REFACTORING.md](architecture/SPIFFE_ID_REFACTORING.md) - Identity model
- [INVARIANTS.md](architecture/INVARIANTS.md) - System guarantees and constraints

### [engineering/](engineering/) - Testing & Verification
**Audience**: Contributors and maintainers

How we build, test, and verify correctness:
- [TESTING.md](engineering/TESTING.md) - Testing strategy overview
- [TESTING_GUIDE.md](engineering/TESTING_GUIDE.md) - Detailed testing procedures
- [TEST_ARCHITECTURE.md](engineering/TEST_ARCHITECTURE.md) - Test structure
- [END_TO_END_TESTS.md](engineering/END_TO_END_TESTS.md) - E2E test design
- [INTEGRATION_TEST_SUMMARY.md](engineering/INTEGRATION_TEST_SUMMARY.md) - Integration tests
- [INTEGRATION_TEST_OPTIMIZATION.md](engineering/INTEGRATION_TEST_OPTIMIZATION.md) - Performance
- [VERIFICATION.md](engineering/VERIFICATION.md) - Verification strategies
- [REGISTRATION_ENTRY_VERIFICATION.md](engineering/REGISTRATION_ENTRY_VERIFICATION.md) - SPIRE registration
- [DESIGN_BY_CONTRACT.md](engineering/DESIGN_BY_CONTRACT.md) - Contract-based design
- [DEBUG_MODE.md](engineering/DEBUG_MODE.md) - Debug utilities
- [ARCHITECTURE_REVIEW.md](engineering/ARCHITECTURE_REVIEW.md) - Design critiques

### [roadmap/](roadmap/) - Future Direction
**Audience**: Core team and long-term contributors

Where the project is heading:
- [REFACTORING_PATTERNS.md](roadmap/REFACTORING_PATTERNS.md) - Ongoing refactors
- [ITERATIONS_SUMMARY.md](roadmap/ITERATIONS_SUMMARY.md) - Development history
- [PROJECT_SETUP_STATUS.md](roadmap/PROJECT_SETUP_STATUS.md) - Setup progress

### [infra-notes/](infra-notes/) - Infrastructure & Operations
**Audience**: Platform engineers (will move to separate repo)

Operations and infrastructure concerns (to be migrated to `spire-infra` repo):
- [SPIRE_DISTROLESS_WORKAROUND.md](infra-notes/SPIRE_DISTROLESS_WORKAROUND.md) - Image hardening
- [security-tools.md](infra-notes/security-tools.md) - Security scanning
- [codeql-local-setup.md](infra-notes/codeql-local-setup.md) - Local security analysis
- [UNIFIED_CONFIG_IMPROVEMENTS.md](infra-notes/UNIFIED_CONFIG_IMPROVEMENTS.md) - Config proposals

## Documentation Principles

1. **Code-facing docs live with code** - Architecture, ports, invariants, and test strategies are versioned with the library because they describe product guarantees.

2. **User docs are product** - Guide documents help users adopt the library and should be ruthlessly up-to-date and tested.

3. **Engineering docs prevent rot** - Test architecture and verification docs must evolve with code or they become misleading.

4. **Roadmap docs are non-contractual** - Future plans and refactoring patterns are guidance, not guarantees.

5. **Infra docs will split** - Operations and deployment concerns will move to a separate repository when we ship Helm charts and infrastructure code.

## Future Repository Structure

When the project grows, documentation will be distributed across repos:

```
spire/                  # Core library (this repo)
├── docs/
│   ├── guide/         # User documentation
│   ├── architecture/  # Design reference
│   ├── engineering/   # Test & verification
│   └── roadmap/       # Future direction

spire-infra/           # Infrastructure repo (future)
├── helm/
├── spire-config/
└── docs/              # Operations documentation
    ├── SPIRE_SETUP.md
    ├── DEPLOYMENT.md
    └── ...

spire-site/            # Public website (future)
└── docs/              # Simplified narrative
    ├── overview/
    ├── quickstart/
    └── architecture/  # High-level only
```

## Finding What You Need

**I want to...**

- **Use this library** → Start with [guide/QUICKSTART.md](guide/QUICKSTART.md)
- **Understand how it works** → Read [architecture/ARCHITECTURE.md](architecture/ARCHITECTURE.md)
- **Contribute code** → See [engineering/TESTING.md](engineering/TESTING.md) and [EDITOR_SETUP.md](guide/EDITOR_SETUP.md)
- **Debug issues** → Check [guide/TROUBLESHOOTING.md](guide/TROUBLESHOOTING.md)
- **Review design decisions** → Explore [architecture/](architecture/)
- **Set up infrastructure** → See [infra-notes/](infra-notes/) (temporary location)
- **Understand roadmap** → Browse [roadmap/](roadmap/)

## Contributing to Documentation

Documentation is part of the product. When contributing:

1. **Update docs with code changes** - If you change behavior, update the relevant doc
2. **Keep examples working** - All code examples should be tested
3. **Write for your audience** - User docs should be beginner-friendly, engineering docs can be technical
4. **Link between docs** - Help readers navigate related topics
5. **Date major proposals** - Roadmap docs should have "Last updated" dates
