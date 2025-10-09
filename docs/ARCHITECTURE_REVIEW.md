# Architecture Review: Port Placement and Adapter Complexity

This document analyzes two architectural concerns:

1. **Port Definitions Placement**: Ports currently live in `internal/app/ports/` - should they be at `internal/ports/`?
2. **Adapter Complexity**: The `inmemory/` adapter contains many files - are these truly adapter concerns or misplaced business logic?

## Current Structure

```
internal/
├── domain/                    # Pure domain entities (no dependencies)
├── app/                       # Application layer (use cases/services)
│   ├── ports/                 # Port interfaces (current location)
│   │   ├── inbound.go         # Service interface
│   │   ├── outbound.go        # Outbound port interfaces
│   │   └── types.go           # Port-specific types
│   ├── application.go         # Composition root (Bootstrap)
│   └── service.go             # IdentityService (core use case)
└── adapters/                  # Infrastructure implementations
    ├── inbound/cli/           # CLI adapter
    └── outbound/
        ├── inmemory/          # In-memory implementations (11 files)
        ├── spire/             # Real SPIRE SDK implementations (future)
        └── compose/           # Dependency factories
```

## Issue 1: Port Placement

### Current Design: `internal/app/ports/`

**Rationale**:
- Ports define the **application's needs** (what it requires from infrastructure)
- Ports are "owned" by the application layer conceptually
- Keeps related concepts together (app + its interfaces)

**Pros**:
- ✅ Clear ownership: application defines what it needs
- ✅ Port changes are clearly application-driven
- ✅ Follows "inside-out" hexagonal thinking (core defines interfaces)

**Cons**:
- ⚠️ Creates coupling perception: adapters depend on `app/ports`
- ⚠️ Less clear boundaries when viewed as directories
- ⚠️ "Port" jargon buried inside app layer

### Alternative: `internal/ports/`

**Rationale**:
- Ports as explicit architectural boundary layer
- Symmetry: `domain/`, `ports/`, `app/`, `adapters/`
- Adapters import `ports` not `app/ports`

**Pros**:
- ✅ Clearer visual separation of concerns
- ✅ Adapters depend on `ports` (not `app`)
- ✅ Explicit boundary layer in directory structure
- ✅ Easier to explain hexagonal architecture

**Cons**:
- ⚠️ Ports feel "orphaned" - who owns them?
- ⚠️ Less clear that application defines interfaces
- ⚠️ Potential for ports to become dumping ground

### Recommendation: **Keep at `internal/app/ports/`**

**Why**:
1. **Dependency Inversion Principle**: The application layer owns the interfaces, adapters implement them
2. **Walking Skeleton Context**: This is a learning/reference implementation - clarity about who owns ports is valuable
3. **Go Import Paths**: `app/ports` reads naturally - "application's ports"
4. **Current State Works**: No actual architectural problems with current placement

**What If You Move It?**

If `internal/ports/` feels cleaner for your team:
- Move `internal/app/ports/` → `internal/ports/`
- Update all imports (`app/ports` → `ports`)
- Add package documentation explaining ports are defined by application needs
- This is purely organizational - no architectural change

**Verdict**: Either works. Current placement is theoretically sound. Moving to `internal/ports/` is also valid if visual clarity matters more than ownership signaling.

---

## Issue 2: Adapter File Complexity

### Current InMemory Adapter Files

```
internal/adapters/outbound/inmemory/
├── agent.go                           # SPIRE agent adapter (✅ adapter)
├── server.go                          # SPIRE server adapter (✅ adapter)
├── registry.go                        # Identity mapper registry (✅ adapter)
├── config.go                          # Configuration loader (✅ adapter)
├── translation.go                     # Anti-corruption layer (✅ adapter)
├── validator.go                       # Identity document validator (⚠️ thin wrapper)
├── identity_credential_parser.go       # Parses SPIFFE IDs (⚠️ parsing logic)
├── trust_domain_parser.go             # Parses trust domains (⚠️ parsing logic)
├── identity_document_provider.go      # Creates X.509 certificates (⚠️ complex logic)
└── attestor/
    ├── unix.go                        # Unix workload attestor (✅ adapter)
    └── node.go                        # Node attestor (✅ adapter)
```

### Analysis by File

#### ✅ True Adapters (Infrastructure Concerns)

**`agent.go`, `server.go`**:
- Orchestrate SPIRE flows (Attest → Match → Issue)
- Manage CA certificates and issuance
- Infrastructure coordination - **correctly placed**

**`registry.go`**:
- In-memory map storage
- Seeding and sealing mechanism
- Infrastructure storage - **correctly placed**

**`config.go`**:
- Hardcoded configuration loading
- Pure infrastructure - **correctly placed**

**`attestor/unix.go`, `attestor/node.go`**:
- Platform-specific attestation (UID lookup, process info)
- Infrastructure/OS interaction - **correctly placed**

**`translation.go`**:
- Anti-corruption layer between domain and adapter internals
- Maps domain types to internal representations
- Classic adapter pattern - **correctly placed**

#### ⚠️ Questionable Placement (Potential Domain Logic)

**`identity_credential_parser.go`** (77 lines):
- **What it does**: Parses SPIFFE ID strings (`spiffe://example.org/path`) into domain types
- **Why it's questionable**: Parsing logic with validation rules (scheme check, path validation)
- **Alternative**: This could be domain logic OR SDK wrapper
- **Current port**: `IdentityCredentialParser` interface

**Analysis**:
```go
// Current: Adapter contains parsing + validation logic
func (p *InMemoryIdentityCredentialParser) ParseFromString(ctx, id string) (*domain.IdentityCredential, error) {
    // Validation: scheme must be "spiffe"
    // Validation: URI format
    // Extraction: trust domain from host
    // Extraction: path component
    // Construction: domain.IdentityCredential
}
```

**Questions**:
1. Is this "parsing" or "validation"?
2. Is URI parsing infrastructure (adapter concern) or business rule (domain concern)?
3. In real SPIRE, this would delegate to `go-spiffe` SDK - so is this SDK replacement?

**Verdict**: **Correctly placed as adapter** ✅
- **Why**: This is SDK abstraction logic
- In real implementation, this would call `spiffeid.FromString()` from `go-spiffe` SDK
- The parsing rules belong to SPIFFE spec, not our domain
- Port abstraction allows swapping in real SDK later

**`trust_domain_parser.go`** (47 lines):
- Similar to above - parses trust domain strings
- Validates format (no scheme, no path)
- Would use `spiffeid.TrustDomainFromString()` in real implementation
- **Verdict**: **Correctly placed as adapter** ✅ (same rationale)

**`identity_document_provider.go`** (144 lines):
- **What it does**: Creates X.509 certificates with SPIFFE extensions
- **Why it's questionable**: Complex certificate generation logic
- **Current port**: `IdentityDocumentProvider` interface

**Analysis**:
```go
// Contains:
// - RSA key generation
// - X.509 certificate template creation
// - SPIFFE URI embedding
// - Certificate signing with CA
// - Validation logic (expiration checks)
```

**Questions**:
1. Is certificate generation infrastructure or domain logic?
2. Should domain entities know about X.509 details?
3. Is this SDK replacement code?

**Verdict**: **Correctly placed as adapter** ✅
- **Why**: Certificate operations are infrastructure concerns
- X.509 is a serialization/transport format, not domain concept
- Domain only cares about `IdentityDocument` abstraction
- In real implementation, would use `x509svid` package from SDK
- Properly abstracts crypto details from domain

**`validator.go`** (61 lines):
- **What it does**: Validates identity documents
- Currently contains basic expiration + identity credential match checks
- Very thin wrapper around domain methods

**Analysis**:
```go
func (v *IdentityDocumentValidator) Validate(ctx, cert, expectedID) error {
    if !cert.IsValid() { ... }                    // Calls domain method
    if !cert.IdentityCredential().Equals(expectedID) { ... }  // Calls domain method
}
```

**Questions**:
1. Is this adding any value beyond domain methods?
2. Should validation be in domain?
3. Is this port necessary?

**Verdict**: **Questionable - potentially redundant** ⚠️
- Current implementation just calls domain methods
- Port exists but adds minimal value
- In real implementation with SDK, would do chain-of-trust verification
- **Consider**: Remove port if not using SDK features, OR keep for future SDK integration

### Adapter Complexity Summary

**Total Files**: 11 files (including attestor subdirectory)

**Breakdown**:
- ✅ **Core adapters** (agent, server, registry, config): 4 files - essential
- ✅ **Platform adapters** (attestor/*): 2 files - essential
- ✅ **Translation layer**: 1 file - essential
- ✅ **SDK abstraction** (parsers, provider): 3 files - correct for SDK replacement
- ⚠️ **Thin wrapper** (validator): 1 file - questionable value

**Conclusion**: **Adapter complexity is justified** ✅

Most files are legitimate SDK abstractions (parsers, certificate provider) that will delegate to `go-spiffe` SDK in real implementation. This is the "walking skeleton" pattern - implementing SDK behavior in-memory for learning/testing.

---

## Recommendations

### 1. Port Placement: No Action Required

**Current state is architecturally sound**. Ports in `app/ports/` correctly signal application ownership of interfaces.

**Optional improvement** (cosmetic only):
- If visual clarity is priority: Move to `internal/ports/`
- If ownership clarity is priority: Keep at `internal/app/ports/`

### 2. Adapter Complexity: Minor Cleanup

#### Action Item 1: Consider Removing `validator.go`

**Current state**:
```go
// Port interface
type IdentityDocumentValidator interface {
    Validate(ctx context.Context, doc *domain.IdentityDocument, expectedID *domain.IdentityCredential) error
}

// Adapter (just wraps domain methods)
func (v *IdentityDocumentValidator) Validate(ctx, cert, expectedID) error {
    if !cert.IsValid() { return err }                         // Domain method
    if !cert.IdentityCredential().Equals(expectedID) { ... }   // Domain method
    return nil
}
```

**Recommendation**:
- **Option A**: Remove port and adapter, call domain methods directly
- **Option B**: Keep port for future SDK integration (chain-of-trust validation)

**Verdict**: **Keep for now** - port is designed for SDK integration. When using real `go-spiffe`, this will call `x509svid.Verify(cert, chain, bundle)`.

#### Action Item 2: Add Clarifying Comments

Add package-level documentation to `inmemory/` explaining its role:

```go
// Package inmemory provides in-memory implementations of SPIRE ports.
//
// This package serves as a "walking skeleton" - a minimal implementation that
// demonstrates the architecture without external dependencies. In a production
// system, these adapters would be replaced with go-spiffe SDK wrappers.
//
// SDK Abstraction Files:
// - identity_credential_parser.go: Would use spiffeid.FromString()
// - trust_domain_parser.go: Would use spiffeid.TrustDomainFromString()
// - identity_document_provider.go: Would use x509svid package
// - validator.go: Would use x509svid.Verify()
//
// These files contain "temporary business logic" that replicates SDK behavior
// for learning purposes. They are NOT domain logic - they are infrastructure
// implementations that will be replaced with SDK delegation.
package inmemory
```

#### Action Item 3: Document Adapter Purpose

Add comments to each parser/provider file:

```go
// InMemoryIdentityCredentialParser provides simple string-based parsing without SDK.
// In production, this would delegate to go-spiffe SDK's spiffeid.FromString().
// This implementation exists to demonstrate the architecture without external deps.
type InMemoryIdentityCredentialParser struct{}
```

### 3. File Organization: No Changes Needed

The current structure is clean:
- Core adapters (agent, server, registry) at top level
- Attestation subdirectory for platform-specific logic
- SDK abstraction files clearly named

---

## Comparison with Alternatives

### What if Parsing Was in Domain?

**Option**: Move `IdentityCredentialParser` logic to domain constructors

```go
// domain/identity_credential.go
func NewIdentityCredentialFromString(id string) (*IdentityCredential, error) {
    // Parse URI, validate scheme, extract components...
}
```

**Problems**:
- ❌ Domain now depends on SPIFFE spec format (coupling)
- ❌ Can't swap parsing implementations (SDK vs custom)
- ❌ Domain constructors become complex with validation
- ❌ Testing becomes harder (need valid SPIFFE URIs everywhere)

**Current port approach**:
- ✅ Domain remains format-agnostic (just trust domain + path)
- ✅ Parsing is infrastructure concern (ports allow swapping)
- ✅ Easy to test domain (use simple constructors)
- ✅ Adapter handles SDK integration details

### What if We Merged Parser Files?

**Option**: Combine `identity_credential_parser.go` + `trust_domain_parser.go` into `parsers.go`

**Analysis**:
- Each file implements a specific port interface
- Separate files = clear single responsibility
- Current organization follows Go conventions (one type per file for complex types)

**Verdict**: Current separation is appropriate.

---

## Final Recommendations

### Keep Current Architecture ✅

1. **Port placement**: `internal/app/ports/` is theoretically correct
2. **Adapter files**: All files are justified (SDK abstractions)
3. **Complexity**: Appropriate for hexagonal + SDK abstraction pattern

### Optional Improvements

1. **Add package documentation** to `inmemory/` explaining SDK abstraction role
2. **Add file comments** to parser/provider files referencing future SDK replacement
3. **Consider `internal/ports/`** if team prefers visual clarity (cosmetic change only)

### Do NOT Change

1. ❌ Do not move parsing logic to domain (breaks abstraction)
2. ❌ Do not remove parser ports (needed for SDK integration)
3. ❌ Do not merge adapter files (current separation is clean)

---

## Appendix: Port Interface Summary

**All current ports are necessary**:

| Port | Purpose | In-Memory Adapter | Real SDK Adapter |
|------|---------|-------------------|------------------|
| `IdentityMapperRegistry` | Selector → Identity mapping | In-memory map | SPIRE Server API |
| `IdentityCredentialParser` | Parse SPIFFE IDs | String parsing | `spiffeid.FromString()` |
| `TrustDomainParser` | Parse trust domains | String validation | `spiffeid.TrustDomainFromString()` |
| `IdentityDocumentProvider` | Create X.509 SVIDs | Manual cert gen | `x509svid` package |
| `IdentityDocumentValidator` | Validate SVIDs | Basic checks | `x509svid.Verify()` |
| `WorkloadAttestor` | Attest workloads | UID lookup | SPIRE attestor plugins |
| `NodeAttestor` | Attest nodes | Mock | Platform-specific (AWS, TPM) |
| `Agent` | SPIRE agent operations | In-memory | SPIRE Agent API |
| `Server` | SPIRE server operations | In-memory CA | SPIRE Server API |
| `ConfigLoader` | Load configuration | Hardcoded | File/env/etcd |

**Conclusion**: Every port serves a clear purpose. No redundancy.
