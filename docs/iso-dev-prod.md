# Problem: Isolating Development-Only Code from Production Builds

## Overview

The codebase currently includes development-only code (selector matching, in-memory attestation, identity mapping) in production builds. While this is architecturally sound and has minimal binary size impact, it represents a conceptual pollution where unused code paths exist in production deployments. This document analyzes the problem in detail without proposing solutions.

## The Core Problem

**Production deployments of SPIRE workloads should only contain code for fetching identities from external SPIRE infrastructure, not code for performing selector matching and attestation locally.**

However, due to Go's type system constraints and the current hexagonal architecture design, development-only domain logic must be compiled into production binaries even though it's never executed.

## What Code is Development-Only?

### In-Memory Adapters (Already Excluded via Build Tags)

These are properly isolated with `//go:build dev` tags:

```
internal/adapters/outbound/inmemory/
├── agent.go                    // In-memory SPIRE agent
├── server.go                   // In-memory SPIRE server
├── registry.go                 // In-memory identity mapper registry
├── identity_document_provider.go
├── attestor/
│   └── unix.go                 // Unix UID-based attestation
└── ...
```

**Status**: ✅ Excluded from production builds via `//go:build dev`

### Development Entry Points (Already Excluded)

```
cmd/main.go                     // CLI demo tool
internal/adapters/outbound/compose/inmemory.go  // In-memory factory
```

**Status**: ✅ Excluded from production builds via `//go:build dev`

### Domain Logic (The Problem)

These domain files are **included in production** but contain development-only logic:

```
internal/domain/
├── selector.go                 // ~240 lines - Selector parsing and matching
├── selector_set.go             // ~140 lines - Selector collection
├── selector_type.go            // ~40 lines  - Selector type enum
├── identity_mapper.go          // ~180 lines - Selector → Identity mapping
└── attestation.go              // ~150 lines - Attestation result types
                                 ────────────
                                 ~750 lines total
```

**Status**: ❌ **Included in production builds** (the problem)

### Port Interfaces (The Problem)

These interfaces are **always compiled** but only implemented by dev adapters:

```go
// internal/ports/outbound.go

// Only implemented by: internal/adapters/outbound/inmemory/registry.go
type IdentityMapperRegistry interface {
    FindBySelectors(ctx context.Context, selectors *domain.SelectorSet) (*domain.IdentityMapper, error)
    ListAll(ctx context.Context) ([]*domain.IdentityMapper, error)
}

// Only implemented by: internal/adapters/outbound/inmemory/attestor/unix.go
type WorkloadAttestor interface {
    Attest(ctx context.Context, workload ProcessIdentity) ([]string, error)
}
```

**Status**: ❌ **Included in production builds** (the problem)

## Why This is a Problem

### 1. Conceptual Pollution

Production binaries contain code that should never execute:

```go
// This method exists in production but is NEVER called
func (im *IdentityMapper) MatchesSelectors(selectors *SelectorSet) bool {
    // 15 lines of matching logic
    // Dead code in production
}
```

**Mental Model Violation**: Developers reading production code see unused types and methods.

### 2. Binary Size Impact (Minor but Measurable)

```
Domain selector/mapper code:     ~750 lines
Estimated compiled size:         ~50KB (0.4% of 13MB stripped binary)
Port interface metadata:         ~10KB
Total overhead:                  ~60KB
```

While small, this represents code that provides **zero value** in production.

### 3. Increased Attack Surface

More code = more potential vulnerabilities:

```go
// ParseSelector is in production binary
func ParseSelector(selectorType SelectorType, s string) (*Selector, error) {
    // String parsing logic
    // Could have edge cases, parsing bugs
    // But is never called in production
}
```

Even if dead code elimination removes unused functions, the type definitions and any constructors referenced by interfaces remain.

### 4. Maintenance Burden

Changes to dev-only code can break production builds:

```go
// Refactoring SelectorSet breaks production compilation
type IdentityMapper struct {
    selectors *SelectorSet  // Production must compile this
}
```

Developers must maintain code paths that production never uses.

### 5. Testing Overhead

Tests must cover code that production doesn't use:

```bash
# Running selector tests that production never executes
go test ./internal/domain -run Selector
# Tests pass, but production doesn't need this functionality
```

### 6. Deployment Confusion

Production deployments include types that suggest capabilities they don't have:

```go
// This interface exists in production binary
// Suggests you can register workloads at runtime
// But production has no implementation!
type IdentityMapperRegistry interface {
    FindBySelectors(...)
    ListAll(...)
}
```

**False Capabilities**: The presence of these interfaces implies production supports local identity mapping.

## The Dependency Chain (Why Extraction is Hard)

### Problem 1: Domain Types Reference Each Other

```go
// identity_mapper.go (needs to be in production for type definitions)
type IdentityMapper struct {
    identityCredential *IdentityCredential  // ✅ Used in production
    selectors          *SelectorSet         // ❌ Dev-only, but struct field!
    parentID           *IdentityCredential  // ✅ Used in production
}
```

**Issue**: Can't split `IdentityMapper` because struct fields can't have build tags.

### Problem 2: Port Interfaces Reference Domain Types

```go
// ports/outbound.go (always compiled)
type IdentityMapperRegistry interface {
    FindBySelectors(ctx context.Context, selectors *domain.SelectorSet) (*domain.IdentityMapper, error)
    //                                              ^^^^^^^^^^^^^^^^^^^  ^^^^^^^^^^^^^^^^^^^^
    //                                              Dev-only type        Dev-only type
}
```

**Issue**: Interface signature forces `SelectorSet` and `IdentityMapper` types to exist in production.

### Problem 3: Factory Interfaces Reference Domain Types

```go
// Application bootstrap code (always compiled)
type Application struct {
    Agent                 ports.Agent
    Registry              ports.IdentityMapperRegistry  // ← Dev-only interface
    IdentityClientService *IdentityClientService
}
```

**Issue**: Even though production sets `Registry: nil`, the field type must compile.

### Problem 4: Attestation Results Use SelectorSet

```go
// domain/attestation.go (always compiled)
type WorkloadAttestationResult struct {
    workload  *Workload
    selectors *SelectorSet  // ← Dev-only type
    attested  bool
}
```

**Issue**: Attestation domain logic references `SelectorSet`, forcing it into builds.

## Concrete Examples of the Problem

### Example 1: Production Binary Contains Unused Selector Matching

```bash
# Build production agent
go build -o bin/agent-prod ./cmd/agent

# Check if selector matching code is present
go tool nm bin/agent-prod | grep -i selector
# Output shows selector-related symbols despite production never using them

# Disassemble to see actual code
go tool objdump -s IdentityMapper.MatchesSelectors bin/agent-prod
# Shows compiled machine code for a method production never calls
```

### Example 2: False Capability Signals

```go
// Developer looking at production code sees:
app, err := app.Bootstrap(ctx, configLoader, factory)
// app.Registry is type ports.IdentityMapperRegistry

// Developer might think:
// "I can call app.Registry.FindBySelectors() in production!"

// Reality:
app.Registry == nil  // In production, this is always nil
```

**Developer Confusion**: Type signatures imply capabilities that don't exist.

### Example 3: Refactoring Risk

```go
// Developer refactors SelectorSet
type SelectorSet struct {
    selectors map[string]*Selector
    metadata  *SelectorMetadata  // New field, dev-only
}

// Build production:
go build ./cmd/agent
# Compiles successfully, adds metadata code to production binary
# Even though production never uses SelectorSet at runtime
```

**Risk**: Dev refactoring unnecessarily changes production binary.

## Where the Pollution Exists

### Production SPIRE Adapter (Clean - No Pollution)

```go
// internal/adapters/outbound/spire/agent.go
type Agent struct {
    x509Source *workloadapi.X509Source  // ✅ Production type
    parser     ports.IdentityCredentialParser  // ✅ Production type
    // No references to SelectorSet, IdentityMapper, etc.
}
```

**Status**: ✅ **Clean** - Only uses production types

### Domain Layer (Polluted - The Problem)

```go
// internal/domain/selector.go
func ParseSelectorFromString(s string) (*Selector, error) { ... }  // ❌ In production
func MustParseSelectorFromString(s string) *Selector { ... }       // ❌ In production

// internal/domain/identity_mapper.go
func (im *IdentityMapper) MatchesSelectors(*SelectorSet) bool { ... }  // ❌ In production

// internal/domain/attestation.go
type WorkloadAttestationResult struct {
    selectors *SelectorSet  // ❌ In production
}
```

**Status**: ❌ **Polluted** - Contains dev-only code

### Port Layer (Polluted - The Problem)

```go
// internal/ports/outbound.go
type IdentityMapperRegistry interface {
    FindBySelectors(...) (...)  // ❌ Dev-only interface in production
}

type WorkloadAttestor interface {
    Attest(...) (...)  // ❌ Dev-only interface in production
}
```

**Status**: ❌ **Polluted** - Contains dev-only interfaces

### Application Layer (Partially Polluted)

```go
// internal/app/service.go
type IdentityService struct {
    agent    ports.Agent                    // ✅ Production type
    registry ports.IdentityMapperRegistry   // ❌ Dev-only interface
}
```

**Status**: ⚠️ **Partially Polluted** - References dev-only interfaces

## Quantifying the Problem

### Code Statistics

```bash
# Count lines in dev-only domain files
wc -l internal/domain/selector*.go internal/domain/identity_mapper.go internal/domain/attestation.go
#   242 internal/domain/selector.go
#   138 internal/domain/selector_set.go
#    39 internal/domain/selector_type.go
#   184 internal/domain/identity_mapper.go
#   157 internal/domain/attestation.go
# ─────
#   760 total lines

# Count methods that production never calls
grep -c "^func.*Selector" internal/domain/selector*.go
# 8 functions related to selectors

grep -c "^func.*IdentityMapper" internal/domain/identity_mapper.go
# 7 functions related to identity mapping
```

### Binary Size Impact

```bash
# Build with and without domain files (hypothetical)
go build -o bin/agent-full ./cmd/agent
ls -lh bin/agent-full
# 18M (with debug symbols)
# 13M (stripped with -ldflags="-s -w")

# Estimated removal if isolatable:
# Domain code:        ~50KB
# Interface metadata: ~10KB
# Total savings:      ~60KB (0.46% of stripped binary)
```

### Symbol Count

```bash
# Count selector-related symbols in production binary
go tool nm bin/agent-prod | grep -i selector | wc -l
# ~45 symbols related to selectors

go tool nm bin/agent-prod | grep -i IdentityMapper | wc -l
# ~30 symbols related to identity mapping
```

**~75 symbols** in production binary for code that's never executed.

## Use Case Analysis

### Production Use Case (What Should Be There)

**Requirement**: Workload fetches its identity from external SPIRE agent.

**Needed Code**:
```
✅ domain.IdentityCredential       - Parse SPIFFE IDs
✅ domain.IdentityDocument         - Hold X.509 SVIDs
✅ domain.TrustDomain              - Validate trust domains
✅ ports.Agent                     - Interface to SPIRE
✅ spire.Agent                     - Real SPIRE agent adapter
✅ Translation layer               - Domain ↔ SDK conversion
```

**Flow**:
```
Workload → Workload API → SPIRE Agent → Fetch SVID → Return
```

**NOT Needed**:
```
❌ domain.Selector              - SPIRE Server does selector matching
❌ domain.SelectorSet           - Not used in fetch flow
❌ domain.IdentityMapper        - SPIRE Server manages registration
❌ domain.AttestationService    - SPIRE Agent attests, not workload
❌ ports.IdentityMapperRegistry - No local registry in production
❌ ports.WorkloadAttestor       - SPIRE Agent handles attestation
```

### Development Use Case (What's Actually There)

**Requirement**: Demonstrate SPIRE concepts without external infrastructure.

**Needed Code**:
```
✅ All production code (above)
✅ domain.Selector               - Parse selectors for matching
✅ domain.SelectorSet            - Store selector collections
✅ domain.IdentityMapper         - Map selectors → identities
✅ domain.AttestationService     - Perform local attestation
✅ ports.IdentityMapperRegistry  - Local registry interface
✅ ports.WorkloadAttestor        - Local attestation interface
✅ inmemory.InMemoryAgent        - In-memory SPIRE agent
✅ inmemory.InMemoryServer       - In-memory SPIRE server
✅ inmemory.InMemoryRegistry     - In-memory registration
✅ attestor.UnixWorkloadAttestor - Unix UID attestation
```

**Flow**:
```
Workload → InMemoryAgent → Attest → Match Selectors → Issue SVID → Return
                            ↓         ↓                ↓
                         Attestor  Registry         Server
                         (dev)     (dev)            (dev)
```

## Current State Summary

| Component | Production Need | Actual Status | Problem |
|-----------|----------------|---------------|---------|
| `domain.IdentityCredential` | ✅ Required | ✅ Included | None |
| `domain.IdentityDocument` | ✅ Required | ✅ Included | None |
| `domain.TrustDomain` | ✅ Required | ✅ Included | None |
| `domain.Selector` | ❌ Not needed | ❌ Included | **Pollution** |
| `domain.SelectorSet` | ❌ Not needed | ❌ Included | **Pollution** |
| `domain.IdentityMapper` | ❌ Not needed | ❌ Included | **Pollution** |
| `domain.AttestationService` | ❌ Not needed | ❌ Included | **Pollution** |
| `ports.IdentityMapperRegistry` | ❌ Not needed | ❌ Included | **Pollution** |
| `ports.WorkloadAttestor` | ❌ Not needed | ❌ Included | **Pollution** |
| `inmemory.*` adapters | ❌ Not needed | ✅ Excluded | None (build tags work) |

**Summary**: 5 domain files + 2 port interfaces are polluting production builds.

## The Architecture Tension

### Hexagonal Architecture Purity vs Build Cleanliness

**Hexagonal Principle**: Domain should be complete, self-contained, infrastructure-agnostic.

**Current Design**:
```
Domain (Complete)
  ↓
Ports (All interfaces)
  ↓
Adapters (Swappable: inmemory vs spire)
```

**Benefit**: Clean separation, testable domain, swappable adapters.

**Cost**: Domain includes dev-only logic, ports include dev-only interfaces.

### Type System Constraints

**Go's Limitation**: Struct fields can't have build tags.

```go
// This is INVALID Go:
type IdentityMapper struct {
    identityCredential *IdentityCredential

    //go:build dev
    selectors *SelectorSet  // ❌ Syntax error
}
```

**Implication**: If `IdentityMapper` needs to exist in production (for type definitions), all its fields must exist too.

### Interface Signature Constraints

**Go's Limitation**: Interface methods define parameter types.

```go
// If this interface exists in production...
type IdentityMapperRegistry interface {
    FindBySelectors(ctx context.Context, selectors *domain.SelectorSet) (...)
    //                                              ^^^^^^^^^^^^^^^^^^^
    //                                              Type must exist
}
```

**Implication**: Interface signatures force all parameter types to compile.

## Real-World Impact Assessment

### Severity: LOW to MODERATE

**Why LOW**:
- Binary size impact: ~60KB (0.46% of total)
- No performance impact (dead code elimination)
- No runtime cost (code never executes)
- Go's linker removes unused functions

**Why MODERATE**:
- Conceptual pollution (unused code in production)
- Maintenance burden (must maintain dev-only code)
- Developer confusion (false capability signals)
- Testing overhead (covering unused code paths)
- Potential security surface (more code = more bugs)

### When This Becomes a Bigger Problem

1. **Embedded Systems**: If binary size is critical (<1MB targets)
2. **Security Audits**: Auditors question why selector parsing exists in production
3. **Team Growth**: New developers confused by unused code
4. **Domain Expansion**: Dev-only code grows to 10,000+ LOC
5. **Compliance**: Regulations require provable code exclusion

### When This is Acceptable

1. **Rapid Development**: Simplicity > purity
2. **Small Teams**: Everyone understands the architecture
3. **Short-Lived**: Temporary dev setup, will migrate to real SPIRE
4. **Low Stakes**: Internal tool, not production-critical

## Documentation of Current Workarounds

### File-Level Comments (Current Approach)

Every dev-primarily file has a header comment:

```go
// NOTE: This file (selector.go) is primarily used by the in-memory implementation.
// In production deployments using real SPIRE, selector matching is delegated to SPIRE Server.
// However, these types must remain in production builds because:
// 1. Domain types (Node, IdentityMapper) reference SelectorSet
// 2. Factory interfaces (AdapterFactory.SeedRegistry) use domain.IdentityMapper
// ...
```

**Effectiveness**: ✅ Documents the problem, doesn't solve it.

### Build Tags on Adapters (Current Approach)

In-memory adapters use `//go:build dev`:

```go
//go:build dev

package inmemory
```

**Effectiveness**: ✅ Excludes adapters, ❌ Can't exclude domain types.

### Dead Code Elimination (Compiler's Job)

Go's linker removes unused functions:

```bash
# Unused methods are eliminated at link time
go build -ldflags="-s -w" ./cmd/agent
```

**Effectiveness**: ⚠️ Reduces binary size, but type metadata remains.

## Measurement Methodology

To quantify the pollution in your own build:

### 1. Symbol Analysis

```bash
# Build production binary
go build -o bin/agent ./cmd/agent

# List all symbols
go tool nm bin/agent > symbols.txt

# Count dev-only symbols
grep -i selector symbols.txt | wc -l
grep -i identitymapper symbols.txt | wc -l

# Analyze symbol sizes
go tool nm -size bin/agent | grep -i selector
```

### 2. Binary Comparison (Hypothetical)

```bash
# Current binary
go build -ldflags="-s -w" -o bin/agent-current ./cmd/agent
ls -lh bin/agent-current

# If domain could be excluded (requires code changes):
# go build -tags=no_dev_domain -ldflags="-s -w" -o bin/agent-minimal ./cmd/agent
# ls -lh bin/agent-minimal
# diff <(go tool nm bin/agent-current) <(go tool nm bin/agent-minimal)
```

### 3. Code Coverage Analysis

```bash
# Run production code paths
go test -coverprofile=prod.out ./internal/adapters/outbound/spire/...

# Check if selector code is covered
go tool cover -func=prod.out | grep selector
# Should show 0% coverage for selector matching

# This proves selector code is never executed in production
```

## Conclusion

The current architecture includes **~760 lines of dev-only domain code** in production builds due to:

1. **Type System Constraints**: Struct fields can't have build tags
2. **Interface Signatures**: Port interfaces reference dev-only types
3. **Dependency Chains**: Production types reference dev-only types
4. **Hexagonal Purity**: Complete domain model includes dev functionality

**Impact**: ~60KB binary overhead (0.46%), conceptual pollution, maintenance burden.

**Severity**: LOW to MODERATE - acceptable for most use cases, but problematic for:
- Embedded systems with strict size limits
- Security-critical deployments requiring minimal attack surface
- Large teams needing clear production/dev boundaries
- Long-term maintenance where dev code might diverge significantly

This document establishes the problem space. Solutions require trade-offs between:
- Architectural purity vs build cleanliness
- Simplicity vs separation
- Type safety vs conditional compilation
- Current ease vs future flexibility
