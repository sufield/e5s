# Domain Model: Core Business Concepts

This document describes the domain model for the SPIRE hexagonal architecture implementation. The domain model contains pure business logic with no infrastructure dependencies.

**Location**: `internal/domain/`

---

## Table of Contents

1. [Overview](#overview)
2. [Value Objects](#value-objects)
3. [Entities](#entities)
4. [Domain Services](#domain-services)
5. [Build Tags](#build-tags)
6. [Design Principles](#design-principles)
7. [Usage Examples](#usage-examples)
8. [Cross-References](#cross-references)

---

## Overview

The domain model uses **ubiquitous language** from SPIRE/SPIFFE specifications. All concepts use official SPIRE terminology to maintain consistency with the broader ecosystem.

**Key Characteristics**:
- Pure Go domain model (no external dependencies except `crypto/x509` from stdlib)
- Immutable value objects (compared by value)
- Entities with identity and lifecycle
- Domain invariants enforced at construction
- Rich domain model (behavior + data together)

---

## Value Objects

Value objects are **immutable** and compared by **value**, not identity.

### TrustDomain

**File**: `trust_domain.go`
**Build Tag**: None (used in both dev and production)

**Purpose**: The scope of identities issued by a SPIRE Server, defining the namespace for SPIFFE IDs.

**Format**: Domain name without scheme or path (e.g., `example.org`)

**Responsibilities**:
- Validates trust domain format (no `://`, no `/`, no whitespace)
- Provides equality comparison
- Immutable value object

**Example**:
```go
td, err := domain.NewTrustDomain("example.org")
// td.String() → "example.org"
```

**Validation Rules** (`trust_domain.go:49-119`):
- Cannot be empty
- Cannot contain `://` (no scheme)
- Cannot contain `/` (no path)
- Cannot contain whitespace
- Must be valid DNS name format

---

### IdentityCredential

**File**: `identity_credential.go`
**Build Tag**: None (used in both dev and production)

**Purpose**: A unique, URI-formatted identifier (SPIFFE ID) for a workload or agent, serving as the core identity in the system.

**Format**: `spiffe://<trust-domain>/<path>`

**Examples**:
- `spiffe://example.org/host` (agent)
- `spiffe://example.org/server-workload` (workload)
- `spiffe://example.org/db/postgres` (workload with nested path)

**Responsibilities**:
- Parses and validates SPIFFE ID URIs
- Extracts trust domain and path components
- Checks trust domain membership
- Immutable value object

**Key Methods** (`identity_credential.go:33-287`):
```go
// Accessor methods
func (ic *IdentityCredential) TrustDomain() *TrustDomain
func (ic *IdentityCredential) Path() string
func (ic *IdentityCredential) String() string  // Full SPIFFE ID

// Comparison
func (ic *IdentityCredential) Equal(other *IdentityCredential) bool

// Validation
func (ic *IdentityCredential) IsInTrustDomain(td *TrustDomain) bool
```

**Validation Rules**:
- Must start with `spiffe://`
- Trust domain must be valid
- Path must start with `/` (except root path which can be empty)

---

### Selector

**File**: `selector.go`
**Build Tag**: `//go:build dev` (dev-only)

**Purpose**: A key-value pair used to match workload or node attributes during attestation.

**Format**: `<type>:<key>:<value>` (e.g., `unix:uid:1001`)

**Types** (from `selector_type.go`):
- **Node selectors** (`SelectorTypeNode`): Match node/host attributes
- **Workload selectors** (`SelectorTypeWorkload`): Match workload process attributes

**Examples**:
- `unix:uid:1001` - Unix user ID selector
- `unix:user:server-workload` - Unix username selector
- `k8s:namespace:default` - Kubernetes namespace selector
- `k8s:pod:name:frontend-7d5f4c8b9-xk2lp` - Kubernetes pod name selector

**Responsibilities**:
- Parses selector strings with format validation
- Categorizes by type (node vs workload)
- Provides formatted string representation
- Immutable value object

**Key Methods** (`selector.go:31-229`):
```go
// Parsing
func ParseSelectorFromString(formatted string) (*Selector, error)

// Accessors
func (s *Selector) Type() SelectorType
func (s *Selector) Key() string
func (s *Selector) Value() string
func (s *Selector) Formatted() string  // Returns "type:key:value"
```

**Validation Rules**:
- Must have exactly 3 colon-separated parts
- Type must be valid (`node` or `workload`)
- Key cannot be empty
- Value cannot be empty

---

### SelectorSet

**File**: `selector_set.go`
**Build Tag**: `//go:build dev` (dev-only)

**Purpose**: A collection of unique selectors with O(1) operations.

**Responsibilities**:
- Stores selectors with automatic deduplication
- Provides O(1) add/contains operations (map-based)
- NOT thread-safe (callers must synchronize)

**Key Methods** (`selector_set.go:21-139`):
```go
// Construction
func NewSelectorSet(selectors ...*Selector) *SelectorSet

// Mutation
func (ss *SelectorSet) Add(selector *Selector)

// Query
func (ss *SelectorSet) Contains(selector *Selector) bool
func (ss *SelectorSet) Len() int
func (ss *SelectorSet) IsEmpty() bool
func (ss *SelectorSet) All() []*Selector          // Returns slice (new allocation)
func (ss *SelectorSet) Strings() []string         // Returns formatted strings
```

**Example**:
```go
set := domain.NewSelectorSet()
set.Add(selector1)
set.Add(selector2)
if set.Contains(selector1) {
    // Found
}
```

---

### Workload

**File**: `workload.go`
**Build Tag**: None (used in both dev and production)

**Purpose**: A software process or service requesting an identity; the primary entity that undergoes attestation.

**Attributes** (`workload.go:12-18`):
- PID (Process ID)
- UID (User ID)
- GID (Group ID)
- Path (executable path)

**Responsibilities**:
- Encapsulates workload process information
- Provides attribute access
- Simple value object (no complex validation)

**Key Methods**:
```go
func NewWorkload(pid, uid, gid int, path string) *Workload
func (w *Workload) PID() int
func (w *Workload) UID() int
func (w *Workload) GID() int
func (w *Workload) Path() string
```

**Example**:
```go
workload := domain.NewWorkload(12345, 1001, 1001, "/usr/bin/server")
```

---

## Entities

Entities have **identity** and **lifecycle**. They are mutable (to varying degrees).

### IdentityDocument

**File**: `identity_document.go`
**Build Tag**: None (used in both dev and production)

**Purpose**: SPIFFE Verifiable Identity Document (SVID) - the issued identity artifact provided to workloads for authentication and authorization.

**Format**: **X.509-only** (certificate-based)

**Why X.509-only?**
- **Simplicity**: X.509 is the primary SPIFFE format for mTLS and service mesh use cases
- **Focus**: Reduces complexity by supporting one well-tested format
- **Extensibility**: JWT can be added via adapters if needed in future (without domain changes)
- **Production-ready**: go-spiffe SDK's X.509 support is mature and battle-tested

**Components** (`identity_document.go:35-104`):
```go
type IdentityDocument struct {
    identityCredential *IdentityCredential
    cert               []byte  // X.509 certificate (DER or PEM)
    privateKey         interface{}  // Crypto private key (RSA, ECDSA, etc.)
    chain              [][]byte  // Certificate chain (DER or PEM)
    expiration         time.Time
}
```

**Responsibilities**:
- Encapsulates identity credentials + cryptographic materials
- Validates expiration
- Provides certificate and key access
- Entity with lifecycle (has expiration)

**Key Methods**:
```go
// Construction (from components)
func NewIdentityDocumentFromComponents(
    identityCredential *IdentityCredential,
    cert []byte,
    privateKey interface{},
    chain [][]byte,
) (*IdentityDocument, error)

// Accessors
func (id *IdentityDocument) IdentityCredential() *IdentityCredential
func (id *IdentityDocument) Cert() []byte
func (id *IdentityDocument) PrivateKey() interface{}
func (id *IdentityDocument) Chain() [][]byte
func (id *IdentityDocument) Expiration() time.Time

// Validation
func (id *IdentityDocument) IsValid() bool  // Checks expiration
func (id *IdentityDocument) IsExpired() bool
```

**Validation**:
- Identity credential must be non-nil
- Certificate must be non-empty
- Private key must be non-nil
- Expiration must be in the future for `IsValid()` to return true

---

### IdentityMapper

**File**: `identity_mapper.go`
**Build Tag**: `//go:build dev` (dev-only entity)

**Purpose**: An immutable mapping between workload selectors and an identity credential, defining the conditions under which a workload qualifies for a specific identity.

**This replaces the old `RegistrationEntry` concept in the dev-only implementation.**

**Components** (`identity_mapper.go:28-32`):
```go
type IdentityMapper struct {
    identityCredential *IdentityCredential
    selectors          *SelectorSet
    parentID           *IdentityCredential  // Optional parent (e.g., agent ID)
}
```

**Responsibilities**:
- Maps selectors to identities
- Matches workload selectors using AND logic
- Immutable except for `parentID` (can be set once during initialization)
- Entity with identity

**Matching Semantics** (`identity_mapper.go:149-163`):
- **AND logic**: ALL mapper selectors must be present in workload selectors
- Extra workload selectors are ignored (don't prevent match)

**Example**:
```go
// Mapper selectors: [unix:uid:1000, k8s:namespace:prod]
// Workload selectors: [unix:uid:1000, k8s:namespace:prod, k8s:pod:web]
// Result: MATCH (all mapper selectors present, extra k8s:pod:web ignored)

// Workload selectors: [unix:uid:1000]
// Result: NO MATCH (missing k8s:namespace:prod)
```

**Key Methods**:
```go
// Construction
func NewIdentityMapper(
    identityCredential *IdentityCredential,
    selectors *SelectorSet,
) (*IdentityMapper, error)

// Accessors
func (im *IdentityMapper) IdentityCredential() *IdentityCredential
func (im *IdentityMapper) Selectors() *SelectorSet
func (im *IdentityMapper) ParentID() *IdentityCredential

// Mutation (only method that modifies state)
func (im *IdentityMapper) SetParentID(parentID *IdentityCredential)

// Matching
func (im *IdentityMapper) MatchesSelectors(selectors *SelectorSet) bool

// Validation
func (im *IdentityMapper) IsZero() bool
```

**Validation** (`identity_mapper.go:58-64`):
- Identity credential must be non-nil
- Selectors must be non-nil and non-empty

---

## Domain Services

Domain services encapsulate business logic that doesn't naturally belong to a single entity.

### AttestationService

**File**: `attestation.go`
**Build Tag**: `//go:build dev` (dev-only service)

**Purpose**: Encapsulates domain logic for attestation processes, specifically matching workload selectors to identity mappers.

**Responsibilities**:
- Finds the most specific identity mapper matching workload selectors
- Stateless and safe for concurrent use

**Key Method** (`attestation.go:113-168`):
```go
func (s *AttestationService) MatchWorkloadToMapper(
    selectors *SelectorSet,
    mappers []*IdentityMapper,
) (*IdentityMapper, error)
```

**Matching Policy** (deterministic and order-independent):
1. **Specificity**: Select mapper with highest number of required selectors (most restrictive)
2. **Tie-breaking**: If multiple mappers have same specificity, pick lexicographically smallest IdentityCredential string (stable, repeatable)
3. **Nil-safe**: Skips nil mappers in input list (defensive)

**Example**:
```
Mapper A: requires [type:app]                  (1 selector)
Mapper B: requires [type:app, env:prod]        (2 selectors)
Mapper C: requires [type:app, env:prod]        (2 selectors, ID > B)

Workload has: [type:app, env:prod, region:us]

All match, but B wins (specificity=2, smaller ID than C)
```

**Error Handling**:
- Returns `domain.ErrInvalidSelectors` if selectors nil/empty
- Returns `domain.ErrNoMatchingMapper` if no mapper matches

**Value Object**: `WorkloadAttestationResult` (`attestation.go:21-25`)
```go
type WorkloadAttestationResult struct {
    workload  *Workload
    selectors *SelectorSet
    attested  bool
}
```

---

## Build Tags

Several domain files use `//go:build dev` to exclude them from production builds:

| File | Build Tag | Reason |
|------|-----------|--------|
| `trust_domain.go` | None | Used in both dev and production |
| `identity_credential.go` | None | Used in both dev and production |
| `identity_document.go` | None | Used in both dev and production |
| `workload.go` | None | Used in both dev and production |
| `selector.go` | `//go:build dev` | Dev-only (prod uses SPIRE Server selectors) |
| `selector_set.go` | `//go:build dev` | Dev-only (prod uses SPIRE Server) |
| `selector_type.go` | `//go:build dev` | Dev-only (prod uses SPIRE Server) |
| `identity_mapper.go` | `//go:build dev` | Dev-only (prod uses SPIRE Server registration entries) |
| `attestation.go` | `//go:build dev` | Dev-only (prod uses SPIRE Agent/Server attestation) |

**Why Build Tags?**

In production deployments with real SPIRE infrastructure:
- SPIRE Server manages registration entries (no need for `IdentityMapper`)
- SPIRE Agent/Server handle attestation (no need for `AttestationService`)
- Selectors are managed by SPIRE (no need for domain selector types)

Workloads only interact with SPIRE Workload API to fetch identities - all matching and attestation happens in SPIRE infrastructure.

---

## Design Principles

### 1. Ubiquitous Language

All domain concepts use SPIRE/SPIFFE terminology:
- `TrustDomain`, `IdentityCredential` (SPIFFE ID), `IdentityDocument` (SVID), `Selector`
- Names match official SPIRE specification

### 2. Value Objects vs Entities

**Value Objects** (immutable, compared by value):
- `TrustDomain` - DNS name namespace
- `IdentityCredential` - SPIFFE ID
- `Selector` - Attestation attribute
- `SelectorSet` - Collection of selectors
- `Workload` - Process information

**Entities** (identity, lifecycle):
- `IdentityDocument` - Has expiration (lifecycle)
- `IdentityMapper` - Has identity credential (identity), mostly immutable except parentID

### 3. Domain Invariants

Invariants enforced at construction time (fail fast):
- `IdentityCredential` must follow `spiffe://` URI format
- `TrustDomain` cannot contain schemes (`://`) or paths (`/`)
- `Selector` must have type, key, and value (all non-empty)
- `IdentityDocument` validates expiration
- `IdentityMapper` requires non-nil credential and non-empty selectors

See `docs/INVARIANTS.md` for complete invariant documentation.

### 4. No Infrastructure Dependencies

- Pure Go domain model
- No database, HTTP, or external SDK dependencies
- Only `crypto/x509` for certificate types (standard library)
- Only `time` and `errors` from stdlib

### 5. Rich Domain Model

- Behavior and data together (not anemic domain model)
- Domain logic lives in domain objects
- Validation happens at construction time
- Methods encapsulate business rules

### 6. Error Handling

Domain defines **sentinel errors** for consistent error handling across adapters:

**Registry Errors** (`errors.go:30-43`):
- `ErrNoMatchingMapper` - No mapper matches selectors
- `ErrRegistrySealed` - Registry sealed, cannot add entries
- `ErrRegistryEmpty` - Registry has no entries

**Validation Errors** (`errors.go:45-68`):
- `ErrInvalidIdentityCredential` - Nil or malformed credential
- `ErrInvalidTrustDomain` - Nil, empty, or malformed trust domain
- `ErrInvalidSelectors` - Nil or empty selectors
- `ErrSelectorInvalid` - Invalid selector format

**Identity Document Errors** (`errors.go:70-91`):
- `ErrIdentityDocumentExpired` - Expired or not yet valid
- `ErrIdentityDocumentInvalid` - Nil, malformed, or invalid
- `ErrIdentityDocumentMismatch` - Doesn't match expected credential
- `ErrCertificateChainInvalid` - Chain verification failed
- `ErrTrustBundleNotFound` - Trust bundle not found

**Attestation Errors** (`errors.go:93-106`):
- `ErrWorkloadAttestationFailed` - Attestation failed
- `ErrNoAttestationData` - No attestation data available
- `ErrInvalidProcessIdentity` - Process identity invalid

**Server/Agent Errors** (`errors.go:108-121`):
- `ErrServerUnavailable` - SPIRE server unavailable
- `ErrAgentUnavailable` - SPIRE agent unavailable
- `ErrCANotInitialized` - CA certificate not initialized

All adapters must return these exact errors for consistent error handling. See `docs/INVARIANTS.md` for error contracts.

---

## Usage Examples

### Creating Core Value Objects

```go
// Create trust domain
td, err := domain.NewTrustDomain("example.org")
if err != nil {
    return err
}

// Create identity credential (SPIFFE ID)
identityCredential, err := domain.NewIdentityCredentialFromComponents(td, "/server-workload")
if err != nil {
    return err
}

// Alternative: parse from string
identityCredential, err := domain.NewIdentityCredentialFromString("spiffe://example.org/server-workload")
if err != nil {
    return err
}
```

### Working with Selectors (Dev-Only)

```go
// Create selectors
selector1, err := domain.ParseSelectorFromString("unix:uid:1001")
if err != nil {
    return err
}

selector2, err := domain.ParseSelectorFromString("k8s:namespace:prod")
if err != nil {
    return err
}

// Create selector set
selectors := domain.NewSelectorSet(selector1, selector2)
selectors.Add(selector3)  // Add more

// Check containment
if selectors.Contains(selector1) {
    fmt.Println("Found selector")
}

// Iterate
for _, sel := range selectors.All() {
    fmt.Println(sel.Formatted())  // "unix:uid:1001", "k8s:namespace:prod"
}
```

### Creating Identity Mapper (Dev-Only)

```go
// Create identity mapper (registration entry)
mapper, err := domain.NewIdentityMapper(identityCredential, selectors)
if err != nil {
    return err
}

// Optionally set parent ID (agent)
agentID, _ := domain.NewIdentityCredentialFromComponents(td, "/spire/agent/node-1")
mapper.SetParentID(agentID)

// Check if workload matches
workloadSelectors := domain.NewSelectorSet(/* ... */)
if mapper.MatchesSelectors(workloadSelectors) {
    fmt.Println("Workload qualifies for identity:", mapper.IdentityCredential())
}
```

### Attestation Service (Dev-Only)

```go
// Create attestation service
attestationSvc := domain.NewAttestationService()

// Find best matching mapper for workload
workloadSelectors := domain.NewSelectorSet(
    domain.MustParseSelectorFromString("unix:uid:1001"),
    domain.MustParseSelectorFromString("k8s:namespace:prod"),
)

mappers := []*domain.IdentityMapper{mapper1, mapper2, mapper3}

matchedMapper, err := attestationSvc.MatchWorkloadToMapper(workloadSelectors, mappers)
if err != nil {
    if errors.Is(err, domain.ErrNoMatchingMapper) {
        return fmt.Errorf("no mapper found for workload")
    }
    return err
}

fmt.Println("Matched identity:", matchedMapper.IdentityCredential())
```

### Working with Identity Documents

```go
// Create identity document (typically done by adapter)
doc, err := domain.NewIdentityDocumentFromComponents(
    identityCredential,
    certBytes,      // X.509 certificate (DER or PEM)
    privateKey,     // crypto.PrivateKey
    chainBytes,     // Certificate chain
)
if err != nil {
    return err
}

// Check validity
if !doc.IsValid() {
    return fmt.Errorf("%w: identity document expired", domain.ErrIdentityDocumentExpired)
}

// Access components
cert := doc.Cert()
key := doc.PrivateKey()
expiration := doc.Expiration()
```

---

## Relationship to Ports

The domain model is used by **port interfaces** (in `internal/ports/`):
- Ports define operations using domain types as parameters and return values
- Adapters translate between domain types and infrastructure (SDK, database, etc.)
- Core business logic operates exclusively on domain entities and value objects

**Example Port** (from `internal/ports/outbound_dev.go:26-34`):
```go
type IdentityMapperRegistry interface {
    // Uses domain types: SelectorSet, IdentityMapper
    FindBySelectors(ctx context.Context, selectors *domain.SelectorSet) (*domain.IdentityMapper, error)
    ListAll(ctx context.Context) ([]*domain.IdentityMapper, error)
}
```

See `docs/PORT_CONTRACTS.md` for complete port interface documentation.

---

## Domain Files Summary

| File | Purpose | Build Tag | Entity/Value |
|------|---------|-----------|--------------|
| `trust_domain.go` | Trust domain namespace | None | Value Object |
| `identity_credential.go` | SPIFFE ID | None | Value Object |
| `identity_document.go` | X.509 SVID | None | Entity |
| `workload.go` | Process information | None | Value Object |
| `selector.go` | Attestation attribute | `//go:build dev` | Value Object |
| `selector_set.go` | Selector collection | `//go:build dev` | Value Object |
| `selector_type.go` | Selector type enum | `//go:build dev` | Enum |
| `identity_mapper.go` | Selector-to-identity mapping | `//go:build dev` | Entity |
| `attestation.go` | Attestation service + result | `//go:build dev` | Service + Value |
| `errors.go` | Sentinel errors | None | Error definitions |
| `doc.go` | Package documentation | None | Documentation |

---

## Benefits

1. **Clear Semantics**: SPIRE concepts are explicit types, not strings or maps
2. **Type Safety**: Compile-time validation of domain operations
3. **Testability**: Pure domain logic easily unit tested (no mocks needed)
4. **Documentation**: Domain model serves as living documentation of business rules
5. **Refactoring Safety**: Changes to domain propagate through type system
6. **IDE Support**: Autocomplete and navigation work seamlessly with typed domain
7. **Build Tag Optimization**: Production builds exclude dev-only code automatically

---

## Cross-References

- **Port Contracts**: See `docs/PORT_CONTRACTS.md` for port interface definitions using domain types
- **Domain Invariants**: See `docs/INVARIANTS.md` for complete invariant and validation documentation
- **Control Plane**: See `docs/CONTROL_PLANE.md` for how IdentityMapper is used in dev mode
- **mTLS Architecture**: See `docs/MTLS.md` for how IdentityDocument is used in production

---

## Key Takeaways

1. **X.509-only**: No JWT support (by design, for simplicity)
2. **Build tags separate dev/prod**: Selector and mapper code excluded from production
3. **IdentityMapper replaces RegistrationEntry**: Dev-only entity for workload-to-identity mapping
4. **Immutability by default**: Most types are immutable value objects
5. **Rich domain model**: Behavior lives in domain objects, not in services
6. **Sentinel errors**: Consistent error handling across all adapters
7. **No Node entity**: Production uses SPIRE's node attestation; dev mode doesn't model nodes explicitly
