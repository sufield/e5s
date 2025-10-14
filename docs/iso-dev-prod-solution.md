# Solution: Isolating Development-Only Code Using Go Build Tags

## Overview

This document presents a complete solution to isolate development-only code from production builds using Go's native build constraint system (`//go:build` tags). This approach achieves complete separation while maintaining hexagonal architecture purity, type safety, performance, security, maintainability, and simplicity.

**Related Document**: See [iso-dev-prod.md](./iso-dev-prod.md) for detailed problem analysis.

## Solution Summary

Use Go build constraints to conditionally compile files based on environment:
- **Production builds**: `go build ./cmd/agent` (excludes dev code)
- **Development builds**: `go build -tags=dev ./cmd/agent` (includes dev code)

**Result**:
- ✅ Clean production binaries (no selector matching, identity mapping, or attestation code)
- ✅ Complete development environment (full in-memory SPIRE implementation)
- ✅ Single codebase (no duplication beyond minimal struct definitions)
- ✅ Zero runtime cost (conditional compilation, not runtime checks)
- ✅ Type-safe (compiler enforces correct usage per environment)

## No Compromises Achieved

| Requirement | How Solution Maintains It |
|-------------|--------------------------|
| **Hexagonal Architecture** | Domain remains pure; ports define contracts; adapters swappable. Prod gets minimal domain/ports, dev gets full. |
| **Type Safety** | Mutually-exclusive tags ensure single version per build. No runtime nil checks. Compile-time guarantees. |
| **Performance** | Zero runtime overhead. Dead code physically excluded, not eliminated post-compile. |
| **Security** | Reduces attack surface. Prod binary ~60KB smaller, no dev symbols/parsers. |
| **Maintainability** | Single codebase. Changes auto-apply per env. Minimal duplication (small struct definitions only). |
| **Simplicity** | Native Go feature. No external tools, code generation, or separate modules. Standard build commands. |

## Implementation Plan

### Phase 1: Tag Dev-Only Domain Files

**Objective**: Exclude selector, identity mapper, and attestation code from production.

#### Files to Tag

Add `//go:build dev` to the following domain files:

```
internal/domain/
├── selector.go              // Add //go:build dev
├── selector_set.go          // Add //go:build dev
├── selector_type.go         // Add //go:build dev
├── identity_mapper.go       // Add //go:build dev (or split, see below)
└── attestation.go           // Add //go:build dev
```

#### Example: Tagging selector.go

```go
//go:build dev

// Package domain models core SPIFFE concepts like selectors, identity credentials,
// and identity documents, abstracting from infrastructure dependencies.
package domain

// NOTE: This file is only included in development builds.
// Production deployments delegate selector matching to external SPIRE Server.

import (
    "fmt"
    "strings"
)

// Selector represents a key-value pair used to match workload or node attributes.
// ... rest of file unchanged ...
```

#### Handling Mixed Production/Dev Types

**Problem**: `IdentityMapper` might have fields needed by production type definitions but methods only used in dev.

**Solution Option A - Full Exclusion** (Recommended if prod doesn't reference IdentityMapper at all):

```go
//go:build dev

package domain

// IdentityMapper is only used in development builds
type IdentityMapper struct {
    identityCredential *IdentityCredential
    selectors          *SelectorSet
    parentID           *IdentityCredential
}

// ... all methods ...
```

**Solution Option B - Split File** (If prod needs type definition but not methods):

Create two files:

**`identity_mapper_base.go`** (no build tag - always compiled):
```go
package domain

// IdentityMapper represents an immutable mapping between workload selectors and an identity.
// In production, this type exists for interface compatibility but is not actively used.
// In development, this provides full selector matching functionality.
type IdentityMapper struct {
    identityCredential *IdentityCredential
    // Note: Production doesn't need selectors field
}

// Minimal production-safe methods
func (im *IdentityMapper) IdentityCredential() *IdentityCredential {
    return im.identityCredential
}
```

**`identity_mapper_dev.go`** (with `//go:build dev`):
```go
//go:build dev

package domain

// DevIdentityMapper extends IdentityMapper with development-specific fields and methods.
type DevIdentityMapper struct {
    IdentityMapper                    // Embed base
    selectors      *SelectorSet       // Dev-only field
    parentID       *IdentityCredential // Dev-only field
}

// NewIdentityMapper creates a new validated identity mapper (dev builds only).
func NewIdentityMapper(identityCredential *IdentityCredential, selectors *SelectorSet) (*DevIdentityMapper, error) {
    if identityCredential == nil {
        return nil, ErrInvalidIdentityCredential
    }
    if selectors == nil || len(selectors.All()) == 0 {
        return nil, ErrInvalidSelectors
    }

    return &DevIdentityMapper{
        IdentityMapper: IdentityMapper{identityCredential: identityCredential},
        selectors:      selectors,
    }, nil
}

// MatchesSelectors checks if this mapper matches the given workload selectors.
func (im *DevIdentityMapper) MatchesSelectors(selectors *SelectorSet) bool {
    // ... matching logic ...
}

// Dev-specific getters
func (im *DevIdentityMapper) Selectors() *SelectorSet {
    return im.selectors
}

func (im *DevIdentityMapper) ParentID() *IdentityCredential {
    return im.parentID
}
```

**Recommendation**: Use **Option A** (full exclusion) if production doesn't need `IdentityMapper` at all. Current analysis shows production SPIRE adapter doesn't reference it.

### Phase 2: Tag Dev-Only Port Interfaces

**Objective**: Move dev-only interfaces to separate file with build tag.

#### Current State: Single File

**`internal/ports/outbound.go`** (always compiled):
```go
package ports

import (
    "context"
    "crypto/x509"
    "github.com/pocket/hexagon/spire/internal/domain"
)

// Production interfaces
type Agent interface { ... }
type IdentityServer interface { ... }
type TrustDomainParser interface { ... }
type IdentityCredentialParser interface { ... }
type TrustBundleProvider interface { ... }
type IdentityDocumentProvider interface { ... }

// Dev-only interfaces (PROBLEM!)
type IdentityMapperRegistry interface {
    FindBySelectors(ctx context.Context, selectors *domain.SelectorSet) (*domain.IdentityMapper, error)
    ListAll(ctx context.Context) ([]*domain.IdentityMapper, error)
}

type WorkloadAttestor interface {
    Attest(ctx context.Context, workload ProcessIdentity) ([]string, error)
}

// Factories
type BaseAdapterFactory interface { ... }
type AgentFactory interface { ... }
type AdapterFactory interface { ... }
```

#### Refactored: Split into Two Files

**`internal/ports/outbound.go`** (no build tag - production):
```go
package ports

import (
    "context"
    "crypto/x509"
    "github.com/pocket/hexagon/spire/internal/domain"
)

// ConfigLoader loads application configuration
type ConfigLoader interface {
    Load(ctx context.Context) (*Config, error)
}

// Agent represents the identity agent functionality
type Agent interface {
    GetIdentity(ctx context.Context) (*Identity, error)
    FetchIdentityDocument(ctx context.Context, workload ProcessIdentity) (*Identity, error)
}

// IdentityServer represents the identity server functionality
type IdentityServer interface {
    IssueIdentity(ctx context.Context, identityCredential *domain.IdentityCredential) (*domain.IdentityDocument, error)
    GetTrustDomain() *domain.TrustDomain
    GetCA() *x509.Certificate
}

// TrustDomainParser parses and validates trust domain strings
type TrustDomainParser interface {
    FromString(ctx context.Context, name string) (*domain.TrustDomain, error)
}

// IdentityCredentialParser parses and validates identity credential strings
type IdentityCredentialParser interface {
    ParseFromString(ctx context.Context, id string) (*domain.IdentityCredential, error)
    ParseFromPath(ctx context.Context, trustDomain *domain.TrustDomain, path string) (*domain.IdentityCredential, error)
}

// TrustBundleProvider provides trust bundles for X.509 certificate chain validation
type TrustBundleProvider interface {
    GetBundle(ctx context.Context, trustDomain *domain.TrustDomain) ([]byte, error)
    GetBundleForIdentity(ctx context.Context, identityCredential *domain.IdentityCredential) ([]byte, error)
}

// IdentityDocumentCreator creates identity documents (X.509 SVIDs)
type IdentityDocumentCreator interface {
    CreateX509IdentityDocument(ctx context.Context, identityCredential *domain.IdentityCredential, caCert interface{}, caKey interface{}) (*domain.IdentityDocument, error)
}

// IdentityDocumentValidator validates identity documents
type IdentityDocumentValidator interface {
    ValidateIdentityDocument(ctx context.Context, doc *domain.IdentityDocument, expectedID *domain.IdentityCredential) error
}

// IdentityDocumentProvider combines creation and validation of identity documents
type IdentityDocumentProvider interface {
    IdentityDocumentCreator
    IdentityDocumentValidator
}

// BaseAdapterFactory provides minimal adapter creation methods shared by all implementations
type BaseAdapterFactory interface {
    CreateTrustDomainParser() TrustDomainParser
    CreateIdentityCredentialParser() IdentityCredentialParser
    CreateIdentityDocumentValidator() IdentityDocumentValidator
}

// AgentFactory creates SPIRE agents that delegate to external SPIRE infrastructure
type AgentFactory interface {
    BaseAdapterFactory
    CreateAgent(ctx context.Context, spiffeID string, parser IdentityCredentialParser) (Agent, error)
}

// AdapterFactory is the primary interface for SPIRE deployments
type AdapterFactory interface {
    BaseAdapterFactory
    AgentFactory
}
```

**`internal/ports/outbound_dev.go`** (new file with `//go:build dev`):
```go
//go:build dev

package ports

import (
    "context"
    "github.com/pocket/hexagon/spire/internal/domain"
)

// IdentityMapperRegistry provides read-only access to the identity mapper registry seeded at startup.
// This interface is only available in development builds for in-memory implementations.
//
// In production deployments, SPIRE Server manages registration entries. Workloads only fetch
// their identity via Workload API - no local registry or selector matching is needed.
//
// Error Contract:
// - FindBySelectors returns domain.ErrNoMatchingMapper if no mapper matches
// - FindBySelectors returns domain.ErrInvalidSelectors if selectors are nil/empty
// - ListAll returns domain.ErrRegistryEmpty if no mappers seeded
type IdentityMapperRegistry interface {
    // FindBySelectors finds an identity mapper matching the given selectors (AND logic)
    // This is the core runtime operation: selectors → identity credential mapping
    // All mapper selectors must be present in discovered selectors for a match
    FindBySelectors(ctx context.Context, selectors *domain.SelectorSet) (*domain.IdentityMapper, error)

    // ListAll returns all seeded identity mappers (for debugging/admin)
    ListAll(ctx context.Context) ([]*domain.IdentityMapper, error)
}

// WorkloadAttestor verifies workload identity based on platform-specific attributes.
// This interface is only available in development builds for in-memory attestation.
//
// In production deployments, SPIRE Agent performs attestation. Workloads connect
// to the agent's Unix socket, and the agent extracts credentials and attests automatically.
//
// Error Contract:
// - Returns domain.ErrWorkloadAttestationFailed if attestation fails
// - Returns domain.ErrInvalidProcessIdentity if workload info is invalid
// - Returns domain.ErrNoAttestationData if no selectors can be generated
type WorkloadAttestor interface {
    // Attest verifies a workload and returns its selectors
    // Selectors format: "type:value" (e.g., "unix:uid:1000", "k8s:namespace:prod")
    Attest(ctx context.Context, workload ProcessIdentity) ([]string, error)
}
```

**Benefits**:
- Production builds don't see `IdentityMapperRegistry` or `WorkloadAttestor` interfaces
- Domain types they reference (`SelectorSet`, `IdentityMapper`) can be dev-only
- Clear documentation about dev-only nature

### Phase 3: Split Application Layer Structs

**Objective**: Remove dev-only fields from production application structs.

#### Current State: Single File (Polluted)

**`internal/app/service.go`**:
```go
package app

import (
    "github.com/pocket/hexagon/spire/internal/ports"
)

type IdentityService struct {
    agent    ports.Agent
    registry ports.IdentityMapperRegistry  // ← Dev-only field in production!
}

func NewIdentityService(agent ports.Agent, registry ports.IdentityMapperRegistry) *IdentityService {
    return &IdentityService{
        agent:    agent,
        registry: registry,
    }
}

// Methods using both agent and registry...
```

#### Refactored: Environment-Specific Struct Definitions

**`internal/app/service.go`** (common methods, no build tag):
```go
package app

import (
    "context"
    "github.com/pocket/hexagon/spire/internal/ports"
)

// Common methods that work across both prod and dev
// Note: These methods should only use fields that exist in both versions

// Example: A method that only uses agent (available in both prod and dev)
func (s *IdentityService) GetAgentIdentity(ctx context.Context) (*ports.Identity, error) {
    return s.agent.GetIdentity(ctx)
}

// Other common methods...
```

**`internal/app/service_prod.go`** (new file, `//go:build !dev`):
```go
//go:build !dev

package app

import (
    "context"
    "github.com/pocket/hexagon/spire/internal/ports"
)

// IdentityService in production only needs the agent.
// Registry and attestation are handled by external SPIRE infrastructure.
type IdentityService struct {
    agent ports.Agent
}

// NewIdentityService creates a new identity service for production.
func NewIdentityService(agent ports.Agent) *IdentityService {
    return &IdentityService{
        agent: agent,
    }
}

// Production-specific methods (if any)
// Most methods will be in service.go (common file)
```

**`internal/app/service_dev.go`** (new file, `//go:build dev`):
```go
//go:build dev

package app

import (
    "context"
    "github.com/pocket/hexagon/spire/internal/ports"
)

// IdentityService in development includes registry for local identity mapping.
type IdentityService struct {
    agent    ports.Agent
    registry ports.IdentityMapperRegistry
}

// NewIdentityService creates a new identity service for development.
func NewIdentityService(agent ports.Agent, registry ports.IdentityMapperRegistry) *IdentityService {
    return &IdentityService{
        agent:    agent,
        registry: registry,
    }
}

// Development-specific methods that use registry
func (s *IdentityService) ListAllMappings(ctx context.Context) (int, error) {
    mappers, err := s.registry.ListAll(ctx)
    if err != nil {
        return 0, err
    }
    return len(mappers), nil
}

// Other dev methods...
```

#### Similarly for Application Struct

**`internal/app/application_prod.go`** (`//go:build !dev`):
```go
//go:build !dev

package app

import (
    "github.com/pocket/hexagon/spire/internal/ports"
)

// Application holds production application components.
type Application struct {
    Config                *ports.Config
    IdentityClientService *IdentityClientService
    Agent                 ports.Agent
    // No Registry in production
}
```

**`internal/app/application_dev.go`** (`//go:build dev`):
```go
//go:build dev

package app

import (
    "github.com/pocket/hexagon/spire/internal/ports"
)

// Application holds development application components.
type Application struct {
    Config                *ports.Config
    IdentityClientService *IdentityClientService
    Agent                 ports.Agent
    Registry              ports.IdentityMapperRegistry
}
```

### Phase 4: Split Bootstrap Logic

**Objective**: Separate production and development bootstrap to avoid importing dev code in production.

#### Current State: Conditional Logic (Problematic)

**`internal/app/bootstrap.go`**:
```go
package app

func Bootstrap(ctx context.Context, configLoader ports.ConfigLoader, factory ports.AdapterFactory) (*Application, error) {
    // Load config
    config, err := configLoader.Load(ctx)

    // Create components
    agent, _ := factory.CreateAgent(...)

    // Conditionally create registry
    var registry ports.IdentityMapperRegistry
    if devMode {
        registry = factory.CreateRegistry(...)  // ← Imports dev code!
    }

    return &Application{
        Agent:    agent,
        Registry: registry,  // nil in production
    }, nil
}
```

**Problem**: Production imports dev interfaces even though registry is nil.

#### Refactored: Separate Bootstrap Files

**`internal/app/bootstrap_common.go`** (shared logic, no build tag):
```go
package app

import (
    "context"
    "github.com/pocket/hexagon/spire/internal/ports"
)

// loadConfig is a helper used by both prod and dev bootstrap.
func loadConfig(ctx context.Context, loader ports.ConfigLoader) (*ports.Config, error) {
    config, err := loader.Load(ctx)
    if err != nil {
        return nil, err
    }
    return config, nil
}

// Other common helpers...
```

**`internal/app/bootstrap_prod.go`** (`//go:build !dev`):
```go
//go:build !dev

package app

import (
    "context"
    "fmt"
    "github.com/pocket/hexagon/spire/internal/ports"
)

// Bootstrap initializes the application for production deployment.
// Production uses external SPIRE infrastructure - no local registry or attestation.
func Bootstrap(
    ctx context.Context,
    configLoader ports.ConfigLoader,
    factory ports.AdapterFactory,
) (*Application, error) {
    // Load configuration
    config, err := loadConfig(ctx, configLoader)
    if err != nil {
        return nil, fmt.Errorf("failed to load config: %w", err)
    }

    // Create parsers
    parser := factory.CreateTrustDomainParser()
    credParser := factory.CreateIdentityCredentialParser()

    // Create agent (connects to SPIRE Workload API)
    agent, err := factory.CreateAgent(ctx, config.AgentSpiffeID, credParser)
    if err != nil {
        return nil, fmt.Errorf("failed to create agent: %w", err)
    }

    // Create identity client service
    clientService := NewIdentityClientService(agent)

    // Create identity service (production version - no registry parameter)
    identityService := NewIdentityService(agent)

    return &Application{
        Config:                config,
        IdentityClientService: clientService,
        Agent:                 agent,
        // No Registry field in production
    }, nil
}
```

**`internal/app/bootstrap_dev.go`** (`//go:build dev`):
```go
//go:build dev

package app

import (
    "context"
    "fmt"
    "github.com/pocket/hexagon/spire/internal/ports"
)

// Bootstrap initializes the application for development with in-memory components.
// Development uses local registry, attestor, and server - no external SPIRE needed.
func Bootstrap(
    ctx context.Context,
    configLoader ports.ConfigLoader,
    factory DevAdapterFactory,  // Dev-specific factory interface
) (*Application, error) {
    // Load configuration
    config, err := loadConfig(ctx, configLoader)
    if err != nil {
        return nil, fmt.Errorf("failed to load config: %w", err)
    }

    // Create parsers
    parser := factory.CreateTrustDomainParser()
    credParser := factory.CreateIdentityCredentialParser()
    docProvider := factory.CreateIdentityDocumentProvider()

    // Create and seed registry
    registry, err := factory.CreateRegistry(ctx, config.Workloads, credParser)
    if err != nil {
        return nil, fmt.Errorf("failed to create registry: %w", err)
    }

    // Create attestor
    attestor := factory.CreateAttestor(config.Workloads)

    // Create server
    server, err := factory.CreateServer(ctx, config.TrustDomain, parser, docProvider)
    if err != nil {
        return nil, fmt.Errorf("failed to create server: %w", err)
    }

    // Create agent (in-memory implementation)
    agent, err := factory.CreateAgent(ctx, config.AgentSpiffeID, server, registry, attestor, credParser, docProvider)
    if err != nil {
        return nil, fmt.Errorf("failed to create agent: %w", err)
    }

    // Create identity client service
    clientService := NewIdentityClientService(agent)

    // Create identity service (dev version - includes registry parameter)
    identityService := NewIdentityService(agent, registry)

    return &Application{
        Config:                config,
        IdentityClientService: clientService,
        Agent:                 agent,
        Registry:              registry,
    }, nil
}
```

**Key Differences**:
- Production: No registry creation, simpler factory interface
- Development: Full in-memory stack with registry, attestor, server
- No shared code path that imports dev types

### Phase 5: Update Tests

**Objective**: Ensure tests respect build tags and don't pollute production test coverage.

#### Tag Dev-Only Tests

Add `//go:build dev` to tests that test dev-only functionality:

**`internal/domain/selector_test.go`**:
```go
//go:build dev

package domain

import (
    "testing"
)

func TestParseSelector(t *testing.T) {
    // Tests for selector parsing
    // Only runs with -tags=dev
}
```

**`internal/domain/identity_mapper_test.go`**:
```go
//go:build dev

package domain

import (
    "testing"
)

func TestIdentityMapper_MatchesSelectors(t *testing.T) {
    // Tests for selector matching
    // Only runs with -tags=dev
}
```

#### Keep Production Tests Untagged

**`internal/domain/identity_credential_test.go`** (no build tag):
```go
package domain

import (
    "testing"
)

func TestIdentityCredential_String(t *testing.T) {
    // Tests for production functionality
    // Always runs
}
```

#### Test Commands

```bash
# Run production tests only (no dev code)
go test ./internal/... -short

# Run development tests (includes dev code)
go test -tags=dev ./internal/... -v

# Run all tests with coverage
go test -tags=dev -coverprofile=dev.out ./internal/...
go test -coverprofile=prod.out ./internal/...

# Compare coverage
go tool cover -func=prod.out | grep selector  # Should show 0% or not found
go tool cover -func=dev.out | grep selector   # Should show coverage
```

### Phase 6: Update Build and Deployment

**Objective**: Provide clear build commands for each environment.

#### Update Makefile

**`Makefile`** (add targets):
```makefile
# Build production binaries (excludes dev code)
.PHONY: build-prod
build-prod:
	@echo "Building production binaries (no dev code)..."
	@mkdir -p bin
	go build -ldflags="-s -w" -o bin/agent-prod ./cmd/agent
	go build -ldflags="-s -w" -o bin/workload-prod ./cmd/workload
	@echo "Production binaries built:"
	@ls -lh bin/*-prod
	@echo ""
	@echo "Verifying no dev symbols..."
	@go tool nm bin/agent-prod | grep -i selector && echo "WARNING: Found selector symbols!" || echo "✓ Clean (no selector symbols)"
	@go tool nm bin/agent-prod | grep -i identitymapper && echo "WARNING: Found mapper symbols!" || echo "✓ Clean (no mapper symbols)"

# Build development binaries (includes dev code)
.PHONY: build-dev
build-dev:
	@echo "Building development binaries (with dev code)..."
	@mkdir -p bin
	go build -tags=dev -o bin/agent-dev ./cmd/agent
	go build -tags=dev -o bin/workload-dev ./cmd/workload
	go build -tags=dev -o bin/demo ./cmd
	@echo "Development binaries built:"
	@ls -lh bin/*-dev bin/demo

# Test production code (no dev tests)
.PHONY: test-prod
test-prod:
	@echo "Running production tests..."
	go test -short ./internal/...

# Test development code (includes dev tests)
.PHONY: test-dev
test-dev:
	@echo "Running development tests..."
	go test -tags=dev -v ./internal/...

# Run all tests with coverage
.PHONY: test-all
test-all:
	@echo "Running all tests with coverage..."
	@mkdir -p coverage
	go test -tags=dev -coverprofile=coverage/dev.out ./internal/...
	go test -coverprofile=coverage/prod.out ./internal/...
	@echo ""
	@echo "Coverage reports generated:"
	@echo "  - coverage/dev.out (with dev code)"
	@echo "  - coverage/prod.out (production only)"
	@echo ""
	@echo "View coverage:"
	@echo "  go tool cover -html=coverage/dev.out"
	@echo "  go tool cover -html=coverage/prod.out"

# Compare binary sizes
.PHONY: compare-sizes
compare-sizes: build-prod build-dev
	@echo "Binary size comparison:"
	@echo "Production:"
	@ls -lh bin/agent-prod
	@echo ""
	@echo "Development:"
	@ls -lh bin/agent-dev
	@echo ""
	@echo "Difference:"
	@stat --printf="Prod: %s bytes\n" bin/agent-prod
	@stat --printf="Dev:  %s bytes\n" bin/agent-dev
	@echo ""
	@echo "Expected savings: ~60KB in production"
```

#### Update CI/CD Pipeline

**`.github/workflows/test.yml`** (example):
```yaml
name: Test

on: [push, pull_request]

jobs:
  test-production:
    name: Test Production Build
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3
      - uses: actions/setup-go@v4
        with:
          go-version: '1.25.1'

      - name: Build production binary
        run: go build -o bin/agent-prod ./cmd/agent

      - name: Verify no dev symbols
        run: |
          echo "Checking for dev-only symbols..."
          ! go tool nm bin/agent-prod | grep -i selector
          ! go tool nm bin/agent-prod | grep -i identitymapper
          echo "✓ Production binary is clean"

      - name: Run production tests
        run: go test -short ./internal/...

  test-development:
    name: Test Development Build
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3
      - uses: actions/setup-go@v4
        with:
          go-version: '1.25.1'

      - name: Build development binary
        run: go build -tags=dev -o bin/agent-dev ./cmd/agent

      - name: Run development tests
        run: go test -tags=dev -v ./internal/...

      - name: Coverage report
        run: |
          go test -tags=dev -coverprofile=coverage.out ./internal/...
          go tool cover -func=coverage.out
```

#### Docker Builds

**`Dockerfile.prod`**:
```dockerfile
FROM golang:1.25.1-alpine AS builder

WORKDIR /app
COPY . .

# Build production binary (no dev code)
RUN go build -ldflags="-s -w" -o /bin/agent ./cmd/agent

FROM alpine:latest
COPY --from=builder /bin/agent /bin/agent

ENTRYPOINT ["/bin/agent"]
```

**`Dockerfile.dev`**:
```dockerfile
FROM golang:1.25.1-alpine AS builder

WORKDIR /app
COPY . .

# Build development binary (with dev code)
RUN go build -tags=dev -o /bin/agent ./cmd/agent

FROM alpine:latest
COPY --from=builder /bin/agent /bin/agent

ENTRYPOINT ["/bin/agent"]
```

## Verification Steps

After implementation, verify the solution works correctly:

### 1. Symbol Verification

```bash
# Build both versions
make build-prod
make build-dev

# Check production binary (should have NO dev symbols)
go tool nm bin/agent-prod | grep -i selector
# Expected: No output (or error "no matches")

go tool nm bin/agent-prod | grep -i identitymapper
# Expected: No output (or error "no matches")

# Check development binary (should have dev symbols)
go tool nm bin/agent-dev | grep -i selector
# Expected: Multiple selector-related symbols

go tool nm bin/agent-dev | grep -i identitymapper
# Expected: Multiple identitymapper-related symbols
```

### 2. Binary Size Comparison

```bash
# Compare sizes
make compare-sizes

# Expected output:
# Production: ~12.9 MB (stripped)
# Development: ~13.0 MB (stripped)
# Savings: ~60-100 KB
```

### 3. Test Coverage Verification

```bash
# Run tests and generate coverage
make test-all

# Check that selector code has 0% coverage in prod
go tool cover -func=coverage/prod.out | grep selector
# Expected: selector.go: 0.0% (or not listed)

# Check that selector code has coverage in dev
go tool cover -func=coverage/dev.out | grep selector
# Expected: selector.go: 85.5% (or similar)
```

### 4. Compilation Verification

```bash
# Production build should succeed without dev tags
go build -o bin/test-prod ./cmd/agent
echo "✓ Production build successful"

# Development build should require -tags=dev
go build -o bin/test-dev ./cmd/agent
# Expected: Build fails OR builds without dev functionality

go build -tags=dev -o bin/test-dev ./cmd/agent
echo "✓ Development build successful"
```

### 5. Runtime Verification

**Production Runtime**:
```bash
# Run production agent
./bin/agent-prod

# Expected: Should NOT have access to:
# - IdentityMapperRegistry
# - WorkloadAttestor
# - Selector matching logic

# Should successfully:
# - Connect to SPIRE Workload API (if SPIRE is running)
# - Fetch identities via external SPIRE
```

**Development Runtime**:
```bash
# Run development agent
IDP_MODE=inmem ./bin/agent-dev

# Expected: Should have access to:
# - In-memory registry
# - Unix attestor
# - Full selector matching

# Should successfully:
# - Seed local registry
# - Attest workloads locally
# - Issue SVIDs from in-memory CA
```

## Benefits Summary

### 1. Production Benefits

**Before**:
```
Production binary: 13.0 MB (stripped)
Contains: selector.go, selector_set.go, identity_mapper.go, attestation.go
Symbols: ~75 dev-related symbols
Risk: Unused code, potential bugs in dev-only parsers
```

**After**:
```
Production binary: 12.9 MB (stripped)
Contains: Only identity_credential.go, identity_document.go, trust_domain.go
Symbols: 0 dev-related symbols
Risk: Minimal attack surface, only production code
```

**Gains**:
- ✅ ~60-100 KB smaller binary
- ✅ No dev-only parsing/matching code
- ✅ Reduced attack surface
- ✅ Clearer production capabilities

### 2. Development Benefits

**Before**:
```
Development binary: 13.0 MB
Contains: All code (prod + dev mixed)
Clarity: Dev code mixed with prod, confusing
```

**After**:
```
Development binary: 13.0 MB (same size)
Contains: All code (prod + dev explicitly separated)
Clarity: Clear separation via build tags
```

**Gains**:
- ✅ Explicit dev-only types and interfaces
- ✅ No production pollution concerns
- ✅ Can refactor dev code without affecting prod
- ✅ Better documentation (build tags show intent)

### 3. Maintenance Benefits

**Before**:
```
Files: Mixed prod/dev in same files
Changes: Dev refactoring risks breaking prod builds
Tests: Dev tests run in prod coverage reports
```

**After**:
```
Files: Clear _prod.go and _dev.go suffixes
Changes: Dev refactoring is isolated, can't break prod
Tests: Separate test runs for prod vs dev
```

**Gains**:
- ✅ Safer refactoring (dev changes don't affect prod)
- ✅ Clearer intent (file names show environment)
- ✅ Easier code review (obvious which environment)
- ✅ Better test isolation (prod tests = prod code only)

### 4. Architecture Benefits

**Before**:
```
Hexagonal architecture: Present but polluted
Domain: Contains dev-only logic
Ports: Contains dev-only interfaces
```

**After**:
```
Hexagonal architecture: Clean separation per environment
Domain: Prod domain separate from dev domain
Ports: Prod ports separate from dev ports
```

**Gains**:
- ✅ Hexagonal purity maintained
- ✅ Clear port boundaries per environment
- ✅ Domain remains infrastructure-agnostic
- ✅ Adapters still swappable (within environment)

## Migration Checklist

Use this checklist to track implementation progress:

### Phase 1: Domain Files
- [ ] Add `//go:build dev` to `selector.go`
- [ ] Add `//go:build dev` to `selector_set.go`
- [ ] Add `//go:build dev` to `selector_type.go`
- [ ] Add `//go:build dev` to `identity_mapper.go` (or split)
- [ ] Add `//go:build dev` to `attestation.go`
- [ ] Verify production builds without errors

### Phase 2: Port Interfaces
- [ ] Create `outbound_dev.go` with `//go:build dev`
- [ ] Move `IdentityMapperRegistry` to `outbound_dev.go`
- [ ] Move `WorkloadAttestor` to `outbound_dev.go`
- [ ] Update `outbound.go` to only have prod interfaces
- [ ] Verify production imports don't reference dev interfaces

### Phase 3: Application Layer
- [ ] Create `service_prod.go` with `//go:build !dev`
- [ ] Create `service_dev.go` with `//go:build dev`
- [ ] Move common methods to `service.go` (no tag)
- [ ] Create `application_prod.go` with `//go:build !dev`
- [ ] Create `application_dev.go` with `//go:build dev`

### Phase 4: Bootstrap
- [ ] Create `bootstrap_common.go` with shared helpers
- [ ] Create `bootstrap_prod.go` with `//go:build !dev`
- [ ] Create `bootstrap_dev.go` with `//go:build dev`
- [ ] Delete or rename old `bootstrap.go`
- [ ] Verify both bootstrap paths work independently

### Phase 5: Tests
- [ ] Add `//go:build dev` to selector tests
- [ ] Add `//go:build dev` to identity_mapper tests
- [ ] Add `//go:build dev` to attestation tests
- [ ] Keep prod tests (identity_credential, etc.) untagged
- [ ] Run `make test-prod` and `make test-dev` successfully

### Phase 6: Build System
- [ ] Update Makefile with new targets
- [ ] Add CI/CD jobs for prod and dev builds
- [ ] Create Dockerfile.prod and Dockerfile.dev
- [ ] Update documentation (README, PRODUCTION_VS_DEVELOPMENT.md)
- [ ] Add verification scripts to CI

### Phase 7: Verification
- [ ] Run symbol verification (`make compare-sizes`)
- [ ] Run coverage verification (`make test-all`)
- [ ] Test production runtime (no dev capabilities)
- [ ] Test development runtime (full dev capabilities)
- [ ] Review code with team

## Troubleshooting

### Issue: Production Build Fails with "undefined: SelectorSet"

**Cause**: Production code is still referencing dev-only types.

**Fix**:
1. Find the reference: `git grep -n SelectorSet internal/`
2. Check if file needs build tag or needs refactoring
3. Ensure all dev-referencing code is in `_dev.go` files

### Issue: Development Build Fails with "NewIdentityService expects 1 argument, got 2"

**Cause**: Calling wrong version of constructor (prod vs dev mismatch).

**Fix**:
1. Check bootstrap file has correct build tag
2. Ensure `bootstrap_dev.go` has `//go:build dev`
3. Verify factory types match (dev factory for dev bootstrap)

### Issue: Tests Fail with "IdentityMapperRegistry not found"

**Cause**: Test file doesn't have `//go:build dev` but references dev types.

**Fix**:
1. Add `//go:build dev` to test file
2. Run with: `go test -tags=dev ./...`

### Issue: Binary Size Didn't Decrease

**Cause**: Dev code still being compiled into production binary.

**Fix**:
1. Verify build command: `go build ./cmd/agent` (no `-tags=dev`)
2. Check symbol table: `go tool nm bin/agent | grep selector`
3. Ensure all dev files have correct build tags
4. Rebuild from scratch: `rm -rf bin && make build-prod`

## Conclusion

This solution uses Go's native build constraint system to achieve complete isolation of development-only code from production builds, with zero compromises:

| Requirement | Status | Evidence |
|-------------|--------|----------|
| **Hexagonal Architecture** | ✅ Maintained | Clean domain/ports/adapters separation per environment |
| **Type Safety** | ✅ Maintained | Compile-time enforcement via mutual-exclusive tags |
| **Performance** | ✅ Improved | Dead code physically excluded, ~60KB savings |
| **Security** | ✅ Improved | Reduced attack surface, no dev parsers in prod |
| **Maintainability** | ✅ Improved | Explicit separation, safer refactoring |
| **Simplicity** | ✅ Maintained | Native Go feature, no external tools |

**Implementation Effort**: ~2-3 days
**Maintenance Overhead**: Minimal (standard Go build tags)
**Risk Level**: Low (incremental changes, verifiable at each step)

**Recommendation**: Implement this solution if:
- Binary size is important (embedded systems, edge deployment)
- Security audits require minimal attack surface
- Team is growing and needs clear prod/dev boundaries
- Long-term maintenance where dev code might diverge significantly

This solution represents the **cleanest possible separation** without fragmenting the codebase into separate modules or introducing runtime overhead.
