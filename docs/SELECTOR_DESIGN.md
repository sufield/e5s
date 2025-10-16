# Selector Domain Entities - Design Rationale

**Type**: Architecture Decision Record (ADR)
**Status**: ✅ COMPLETED - This document describes the current implementation

---

## Overview

This document explains **WHY** we created custom selector domain entities instead of using SPIRE SDK types. This is a fundamental architectural decision for the hexagonal architecture implementation.

## Production vs Development

**Production Mode**: When using external SPIRE infrastructure (`SPIREAdapterFactory`), selector domain logic is **NOT required**. SPIRE Server manages all selector matching against registration entries. Your application simply fetches SVIDs via Workload API.

**Development Mode**: When using in-memory implementations (`InMemoryAdapterFactory`), selector domain logic **IS required** for local attestation and matching.

See `docs/PRODUCTION_VS_DEVELOPMENT.md` for detailed architectural comparison.

## The Problem

**SPIRE's Architecture has a Gap**: SPIRE is split into two main components:

1. **SPIRE Server** - Manages registration entries and issues SVIDs
2. **SPIRE Agent** - Attests workloads and fetches SVIDs

The go-spiffe SDK (`github.com/spiffe/go-spiffe/v2`) is designed for workload consumption - it provides APIs to:
- Fetch X.509/JWT SVIDs
- Validate SVIDs
- Manage trust bundles

However, the SDK does NOT provide the logic for:
- Workload attestation (discovering selectors)
- Selector matching (finding which SPIFFE ID to assign)
- Registration entry management (mapping selectors → SPIFFE IDs)

## Why go-spiffe SDK Lacks This

The go-spiffe SDK is intentionally minimal and focused on SVID consumption:

```go
// What go-spiffe SDK provides (for workloads):
client := workloadapi.New(...)
svid, err := client.FetchX509SVID(ctx)  // ✅ Get my identity
```

The SDK assumes you're running next to a SPIRE Agent that has already:
1. Attested your workload
2. Matched selectors against registration entries
3. Determined which SPIFFE ID you should get

## What SPIRE Server API Has (But We Don't Use)

The SPIRE Server API (`github.com/spiffe/spire-api-sdk/proto/spire/api/types`) has a basic `Selector` protobuf type:

```go
type Selector struct {
    Type  string  // e.g., "unix"
    Value string  // e.g., "uid:1000"
}
```

**Why we don't use it**:
1. **Server-focused**: Designed for SPIRE Server API calls, not in-process matching logic
2. **No matching logic**: Doesn't implement `MatchesSelectors()` or AND logic
3. **No validation**: No domain invariants or business rules
4. **No collections**: No `SelectorSet` with uniqueness guarantees
5. **Protobuf overhead**: We don't need protobuf serialization in our domain layer

## Our Requirements (Hexagonal Architecture)

We're building an implementation that demonstrates the complete SPIRE flow in-process:

```
Workload Request
    ↓
1. Attest (discover selectors: unix:uid:1000, unix:gid:1000)
    ↓
2. Match (find IdentityMapper with matching selectors)
    ↓
3. Issue (generate SVID for the matched SPIFFE ID)
    ↓
4. Return SVID to workload
```

**Step 2 (Match)** requires domain logic that doesn't exist in either SDK:

```go
// Our domain logic (not in any SDK):
mapper, err := registry.FindBySelectors(ctx, selectorSet)

// IdentityMapper implements business rules:
func (im *IdentityMapper) MatchesSelectors(selectors *SelectorSet) bool {
    // AND logic: ALL mapper selectors must be present
    for _, required := range im.selectors.All() {
        if !selectors.Contains(required) {
            return false
        }
    }
    return true
}
```

## What Our Selector Domain Entities Provide

### 1. Domain Model (`internal/domain/selector.go`)

```go
type Selector struct {
    selectorType SelectorType  // "node" | "workload"
    key          string         // "uid"
    value        string         // "1000"
    formatted    string         // "workload:uid:1000" (cached)
}
```

**Benefits**:
- ✅ **Validation**: Empty keys/values rejected (`ErrSelectorInvalid`)
- ✅ **Parsing**: Handles `"unix:uid:1000"` format with multi-colon values
- ✅ **Equality**: Field-by-field comparison for robustness
- ✅ **Immutability**: DDD aggregate root pattern

### 2. Selector Set with Guarantees

```go
type SelectorSet struct {
    // Map key is the formatted selector string for O(1) lookup
    selectors map[string]*Selector
}

func (ss *SelectorSet) Add(selector *Selector) {
    if selector != nil {
        // Map automatically deduplicates - uniqueness guarantee
        ss.selectors[selector.formatted] = selector
    }
}

// All returns defensive copy (immutability)
func (ss *SelectorSet) All() []*Selector {
    result := make([]*Selector, 0, len(ss.selectors))
    for _, s := range ss.selectors {
        result = append(result, s)
    }
    return result
}
```

**Benefits**:
- ✅ **Uniqueness**: Map-based storage automatically deduplicates (O(1))
- ✅ **Performance**: O(1) add/contains instead of O(n) slice iteration
- ✅ **Immutability**: Defensive copies prevent external mutation
- ✅ **Type-safe**: Compile-time guarantees

### 3. Identity Mapper with Business Logic

```go
type IdentityMapper struct {
    identityCredential *IdentityCredential
    selectors         *SelectorSet  // Required selectors
}

func (im *IdentityMapper) MatchesSelectors(discovered *SelectorSet) bool {
    // AND logic: Mapper requires [uid:1000, gid:1000]
    //            Workload has [uid:1000, gid:1000, path:/app]
    //            → MATCH (all required present)
    for _, required := range im.selectors.All() {
        if !discovered.Contains(required) {
            return false
        }
    }
    return true
}
```

**Example**:
```go
// Registration entry (like SPIRE registration):
mapper := NewIdentityMapper(
    "spiffe://example.org/webapp",
    selectors: [unix:uid:1000, unix:gid:1000]
)

// Workload attestation discovers:
discovered := [unix:uid:1000, unix:gid:1000, unix:path:/usr/bin/webapp]

// Match?
mapper.MatchesSelectors(discovered) // true - all required selectors present
```

### 4. Registry with Matching Algorithm

```go
func (r *InMemoryRegistry) FindBySelectors(ctx, selectors) (*IdentityMapper, error) {
    for _, mapper := range r.mappers {
        if mapper.MatchesSelectors(selectors) {
            return mapper, nil  // First match wins
        }
    }
    return nil, domain.ErrNoMatchingMapper
}
```

## Architecture Comparison

### Production SPIRE

```
┌─────────────┐   gRPC API    ┌──────────────┐   Workload API   ┌─────────────┐
│ SPIRE Server│◄─────────────►│  SPIRE Agent │◄────────────────►│ Your App    │
│ (Postgres)  │               │  (eBPF/Unix) │  (Unix Socket)   │             │
└─────────────┘               └──────────────┘                  └─────────────┘
      │                              │
      │ Registration Entries         │ Attestation
      │ (selectors → SPIFFE IDs)     │ (discover selectors)
      │                              │
      └──────────────┬───────────────┘
                     │
            Selector Matching
    (inside SPIRE Server, NOT your app)
```

**Key Point**: Your application does NOT implement selector matching in production. You just call:
```go
client.FetchX509SVID(ctx)  // SPIRE handles everything
```

### Our Hexagonal Learning Implementation

```
┌────────────────────────────────────────┐
│  In-Process (Single Binary)            │
│                                        │
│  ┌──────────┐    ┌─────────────┐      │
│  │ Registry │    │   Agent     │      │
│  │(mappers) │◄───┤ (attestor)  │      │
│  └──────────┘    └─────────────┘      │
│       │                                │
│   Selector Matching (Our Domain Logic) │
│   - IdentityMapper.MatchesSelectors()  │
│   - Registry.FindBySelectors()         │
│   - SelectorSet uniqueness             │
└────────────────────────────────────────┘
```

## Why This Design?

### 1. Learning
Shows the complete SPIRE flow in one codebase:
- Attestation → Selector discovery
- Matching → Find SPIFFE ID
- Issuance → Generate SVID

### 2. Hexagonal Architecture
Clean separation between:
- **Domain** (selectors, matching logic) - pure business rules
- **Infrastructure** (SPIRE SDK, in-memory storage) - adapters

### 3. Testable
```go
// Unit test - no SPIRE needed
mapper := NewIdentityMapper(spiffeID, selectors)
assert.True(t, mapper.MatchesSelectors(discovered))

// Integration test - with real SPIRE
agent := spire.NewAgent(client, ...)
identity, err := agent.FetchIdentityDocument(ctx, workload)
```

### 4. Domain-Driven Design
Selector matching is **business logic**, not infrastructure:
- ✅ AND logic (all required selectors must be present)
- ✅ Uniqueness guarantees
- ✅ Validation rules
- ✅ Invariants enforced

### 5. Portable (Development Only)
Can swap implementations for development/testing:
```go
// In-memory (development/testing)
registry := inmemory.NewInMemoryRegistry()
mapper, err := registry.FindBySelectors(ctx, selectors)

// Production SPIRE - NO registry needed
// SPIRE Server manages registration entries externally
agent, err := spire.NewAgent(ctx, client, agentSpiffeID, parser)
identity, err := agent.FetchIdentityDocument(ctx, workload)
```

**Note**: Production deployments use `SPIREAdapterFactory` which returns `nil` for registry. All selector matching happens in external SPIRE Server.

## SDK Responsibility Matrix

| Component | Provided By | Purpose |
|-----------|-------------|---------|
| **SVID Fetching** | go-spiffe SDK | Workload gets its identity |
| **SVID Validation** | go-spiffe SDK | Verify JWT/X.509 SVIDs |
| **Trust Bundles** | go-spiffe SDK | Manage CA certificates |
| **Selector Protobuf** | spire-api-sdk | SPIRE Server API communication |
| **Selector Domain Model** | **Our Code** | **In-process selector matching & validation** |
| **IdentityMapper Matching** | **Our Code** | **Business logic: which SPIFFE ID for these selectors?** |
| **Registration Storage** | SPIRE Server | Persistent storage of mappings |

## Concrete Example: Complete Flow

```go
// 1. Bootstrap - seed registry (like SPIRE registration entries)
registry := inmemory.NewInMemoryRegistry()
registry.Seed(ctx, NewIdentityMapper(
    spiffeID:  "spiffe://example.org/webapp",
    selectors: [unix:uid:1000, unix:gid:1000],
))

// 2. Workload request arrives
workload := ProcessIdentity{UID: 1000, GID: 1000, PID: 12345}

// 3. Attest - discover selectors (like SPIRE workload attestor)
selectorStrings := attestor.Attest(ctx, workload)
// Returns: ["workload:unix:uid:1000", "workload:unix:gid:1000"]

// 4. Parse to domain objects
selectorSet := NewSelectorSet()
for _, s := range selectorStrings {
    selector := ParseSelectorFromString(s)  // Our parsing logic
    selectorSet.Add(selector)
}

// 5. Match - find identity mapper (like SPIRE agent matching)
mapper, err := registry.FindBySelectors(ctx, selectorSet)
// Uses: mapper.MatchesSelectors(selectorSet) - Our matching logic

// 6. Issue SVID for matched SPIFFE ID
doc, err := server.IssueIdentity(ctx, mapper.IdentityCredential())

// 7. Return to workload
return &Identity{
    IdentityCredential: mapper.IdentityCredential(),
    IdentityDocument:  doc,
}
```

## Comparison with Real SPIRE

**What SPIRE Does**:
```bash
# Register workload
spire-server entry create \
  -spiffeID spiffe://example.org/webapp \
  -selector unix:uid:1000 \
  -selector unix:gid:1000

# Agent matches selectors internally and issues SVID
# Workload calls: client.FetchX509SVID()
```

**What We Do**:
```go
// Equivalent registration (in-memory)
registry.Seed(ctx, NewIdentityMapper(
    "spiffe://example.org/webapp",
    selectors: [unix:uid:1000, unix:gid:1000],
))

// Equivalent matching (in-process)
mapper, _ := registry.FindBySelectors(ctx, discovered)
doc, _ := server.IssueIdentity(ctx, mapper.IdentityCredential())
```

## When to Use Each Approach

### Use SPIRE Production Deployment When:
- ✅ Running distributed workloads across multiple nodes
- ✅ Need production-grade attestation (eBPF, TPM, cloud platforms)
- ✅ Require persistent registration entries
- ✅ Need SPIRE's full feature set (federation, CRL, etc.)

### Use Our Implementation When:
- ✅ Learning SPIRE concepts and architecture
- ✅ Testing selector matching logic
- ✅ Prototyping identity-based systems
- ✅ Unit testing without external dependencies
- ✅ Understanding hexagonal architecture patterns

## Summary

**We added selector domain entities because**:
- ✅ go-spiffe SDK doesn't provide selector matching logic (by design)
- ✅ SPIRE API SDK's `Selector` is for server API calls, not in-process matching
- ✅ Selector matching is core business logic in SPIRE's identity issuance flow
- ✅ Hexagonal architecture requires domain models separate from infrastructure
- ✅ Educational value: demonstrates complete SPIRE flow in understandable code

**The selector domain layer bridges the gap between**:
- Workload attestation (discovering selectors)
- Identity issuance (determining which SPIFFE ID to assign)

This gap exists because SPIRE distributes this logic across Server and Agent components, while we implement it in a single, testable, educational codebase.

## References

- [SPIRE Concepts - Selectors](https://spiffe.io/docs/latest/spire-about/spire-concepts/)
- [SPIRE Registration Entries](https://spiffe.io/docs/latest/deploying/registering/)
- [go-spiffe SDK Documentation](https://pkg.go.dev/github.com/spiffe/go-spiffe/v2)
- [SPIRE API SDK Types](https://pkg.go.dev/github.com/spiffe/spire-api-sdk/proto/spire/api/types)
