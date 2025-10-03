# Domain Model

This directory contains the core domain model for the SPIRE identity system. The domain model follows **hexagonal architecture** principles and maintains **technology independence**.

## Domain Purity

The domain layer is **technology-agnostic** and **does NOT depend on external SDKs** (including go-spiffe). This ensures:

- **Portability**: Domain logic can work with any SPIFFE implementation
- **Testability**: Pure domain logic is easier to test without external dependencies
- **Maintainability**: Changes to SDKs don't affect core business logic
- **Clarity**: Domain concepts are expressed in our ubiquitous language

### What belongs in the domain:

✅ **Pure domain entities and value objects** (no SDK imports)
✅ **Domain-specific business logic** unique to our bounded context
✅ **Standard library types only** (strings, time, crypto/x509 for certificates)

### What does NOT belong in the domain:

❌ **go-spiffe SDK types** (`spiffeid.ID`, `x509svid.IdentityDocument`, etc.)
❌ **Framework-specific code** (HTTP, gRPC, databases)
❌ **Infrastructure concerns** (logging, metrics, configuration)

## Anti-Corruption Layer

The adapters act as an **anti-corruption layer** between the domain and external SDKs:

```
┌─────────────────────────────────────────┐
│         External SDK (go-spiffe)        │
│   spiffeid.ID, x509svid.IdentityDocument, etc.      │
└─────────────────────────────────────────┘
                     ↕
        Translation / Anti-Corruption
                     ↕
┌─────────────────────────────────────────┐
│   Adapters (outbound/spire/*.go)        │
│   - Translate SDK ↔ Domain types        │
│   - Use SDK internally only             │
└─────────────────────────────────────────┘
                     ↕
            Ports (app/ports.go)
         Use ONLY domain types
                     ↕
┌─────────────────────────────────────────┐
│     Core Domain (domain/*.go)           │
│   - TrustDomain                          │
│   - IdentityNamespace                             │
│   - IdentityDocument                                 │
│   - Selector                             │
│   - Pure business logic                  │
└─────────────────────────────────────────┘
```

## Domain Concepts

### Value Objects

Value objects are immutable and identified by their values, not by identity.

#### **TrustDomain** (`trust_domain.go`)
Represents the scope of SPIFFE identities.

```go
td, err := domain.NewTrustDomain("example.org")
td.Name() // "example.org"
```

#### **IdentityNamespace** (`identity_namespace.go`)
URI-based workload identifier in SPIFFE format. **Minimal value object** - parsing delegated to `IdentityNamespaceParser` port.

```go
// Parsing done via IdentityNamespaceParser adapter (not domain)
id, err := parser.ParseFromString(ctx, "spiffe://example.org/workload")

// Domain only provides getters
id.String() // "spiffe://example.org/workload"
id.TrustDomain() // *TrustDomain
id.Path() // "/workload"
id.Equals(other) // bool
id.IsInTrustDomain(td) // bool
```

**Refactoring Note**: Parsing logic moved to adapter to avoid duplicating go-spiffe SDK's `spiffeid.FromString`. Domain holds only parsed data. See [SPIFFE_ID_REFACTORING.md](SPIFFE_ID_REFACTORING.md) for details.

#### **Selector** (`selector.go`)
Key-value pair for workload attestation matching.

```go
selector, err := domain.NewSelector(domain.SelectorTypeWorkload, "uid", "1001")
selector.String() // "workload:uid:1001"
selector.Type() // SelectorTypeWorkload

// Handles multi-colon values
k8sSelector, err := domain.ParseSelectorFromString("k8s:pod:ns:default:podname")
k8sSelector.Value() // "ns:default:podname"
```

**Improvements:**
- Uses sentinel errors (`ErrSelectorInvalid`) for validation
- Robust multi-colon value parsing with `strings.Join()`
- Field-by-field equality checking
- See [SELECTOR_IMPROVEMENTS.md](SELECTOR_IMPROVEMENTS.md) for details

#### **SelectorSet** (`selector.go`)
Collection of selectors for matching with automatic deduplication.

```go
set := domain.NewSelectorSet()
set.Add(selector)
set.Add(selector) // Duplicate - not added
set.Contains(selector) // true
len(set.All()) // 1 (no duplicates)
```

**Set Semantics:**
- Enforces uniqueness automatically in `Add()`
- True mathematical set behavior
- Order-preserving (slice-based)

### Entities

Entities have identity and lifecycle.

#### **IdentityDocument** (`identity_document.go`)
SPIFFE Verifiable Identity Document - the issued credential.

```go
svid, err := domain.NewX509SVID(identityNamespace, cert, privateKey, chain)
svid.IsValid() // Checks time validity
svid.IsExpired() // Checks expiration
svid.ExpiresAt() // Returns expiration time
```

**Note**: Uses `crypto/x509.Certificate` from standard library (acceptable as it's not an external SDK).

#### **Workload** (`workload.go`)
Running software process to be identified.

```go
workload := domain.NewWorkload(pid, uid, gid, path)
workload.PID() // Process ID
workload.UID() // User ID
```

#### **Node** (`node.go`)
Host machine where the agent and workloads run. Represents the compute node's identity after attestation.

**Pure Domain Entity**: Models node lifecycle without SDK dependencies:
- Created unattested via `NewNode(identityNamespace)`
- Selectors populated during attestation via `SetSelectors()`
- Marked as attested via `MarkAttested()` after platform verification
- State checked via `IsAttested()`

```go
// Create unattested node
node := domain.NewNode(identityNamespace)

// During attestation (performed by NodeAttestor adapter)
node.SetSelectors(selectorSet)  // Platform selectors (e.g., aws:region, hostname)
node.MarkAttested()              // Mark as successfully attested

// Check attestation status
node.IsAttested() // true
```

**No SDK Duplication**: This is pure domain logic for in-memory walking skeleton. In real SPIRE:
- Node attestation uses platform-specific plugins (AWS IID, TPM, join tokens)
- Selectors extracted from platform metadata (instance tags, region, etc.)
- Our domain entity models the *result* of attestation, not the *process*

#### **RegistrationEntry** (`registration_entry.go`)
Maps SPIFFE ID to selectors, defining authorization policies for workload identity.

**Pure Domain Entity**: Models SPIRE's registration mechanism without SDK dependencies:
- Associates a SPIFFE ID with selector conditions
- Defines parent-child relationships (e.g., workload → agent)
- Validates that workloads meeting selector criteria qualify for the identity
- **Core authorization logic**: `MatchesSelectors()` uses **AND semantics** per SPIRE specification
  - ALL entry selectors must be present in discovered selectors
  - Ensures strong attestation (e.g., workload needs BOTH `unix:uid:1000` AND `k8s:ns:default`)

```go
// Create registration entry requiring multiple selectors
uidSelector, _ := domain.NewSelector(domain.SelectorTypeWorkload, "uid", "1000")
nsSelector, _ := domain.NewSelector(domain.SelectorTypeWorkload, "namespace", "default")
selectorSet := domain.NewSelectorSet()
selectorSet.Add(uidSelector)
selectorSet.Add(nsSelector)

entry, err := domain.NewRegistrationEntry(identityNamespace, selectorSet)
if err != nil {
    // Handles nil checks, empty selectors
}

// Set parent relationship (e.g., workload's parent is agent)
entry.SetParentID(agentIdentityNamespace)

// Authorization check during workload attestation (AND logic)
// Workload MUST have ALL entry selectors to qualify
discoveredSelectors := domain.NewSelectorSet()
discoveredSelectors.Add(uidSelector)
discoveredSelectors.Add(nsSelector)
discoveredSelectors.Add(otherSelector) // Extra selectors OK

if entry.MatchesSelectors(discoveredSelectors) {
    // TRUE: workload has ALL required selectors (uid:1000 AND ns:default)
    // Can issue this SPIFFE ID
}
```

**No SDK Duplication**: This is pure domain logic for in-memory walking skeleton. go-spiffe SDK:
- Has NO registration entry types (SDK is client-side IdentityDocument consumption, not server-side registration)
- Has NO selector matching logic (SPIRE-specific server functionality)
- Has NO parent-child relationship modeling
- Our domain entity models SPIRE server's registration mechanics, not present in SDK

### Domain Services

#### **AttestationService** (`attestation.go`)
Domain logic for attestation processes with sentinel error returns.

```go
service := domain.NewAttestationService()

// Match workload to registration entry (pure domain logic)
entry, err := service.MatchWorkloadToEntry(selectors, entries)
if err != nil {
    // Check error using errors.Is()
    if errors.Is(err, domain.ErrNoMatchingEntry) {
        // No entry found matching selectors
    }
    if errors.Is(err, domain.ErrInvalidSelectors) {
        // Invalid selectors provided
    }
}
```

**Sentinel Errors**: The domain uses sentinel errors (`errors.go`) for better error handling:
- `ErrNoMatchingEntry` - No registration entry matches selectors
- `ErrInvalidSelectors` - Selectors are nil or empty
- `ErrInvalidIdentityNamespace` - SPIFFE ID is nil or malformed
- `ErrNodeAttestationFailed` - Node attestation failed
- `ErrWorkloadAttestationFailed` - Workload attestation failed
- `ErrSVIDExpired`, `ErrSVIDInvalid`, `ErrSVIDMismatch` - IdentityDocument validation errors

Use with `errors.Is()` for checking and `fmt.Errorf("%w", ...)` for wrapping with context.

**Note**: IdentityDocument validation has been moved to an adapter port (`IdentityDocumentValidator`) to avoid duplicating go-spiffe SDK functionality. The SDK provides `x509svid.ParseAndVerify` and `Verify` for full chain-of-trust validation, which should be used in adapters rather than reimplemented in the domain.

## Usage in Ports

Ports (interfaces in `internal/app/ports.go`) use **ONLY domain types**:

```go
type SPIREServer interface {
    IssueIdentity(ctx context.Context, identityNamespace *domain.IdentityNamespace) (*domain.IdentityDocument, error)
    GetTrustDomain() *domain.TrustDomain
}

type IdentityStore interface {
    Register(ctx context.Context, identityNamespace *domain.IdentityNamespace, selector *domain.Selector) error
    GetIdentity(ctx context.Context, identityNamespace *domain.IdentityNamespace) (*Identity, error)
}

type RegistrationRepository interface {
    // Authorization policy storage - maps SPIFFE IDs to selector conditions
    CreateEntry(ctx context.Context, entry *domain.RegistrationEntry) error
    FindMatchingEntry(ctx context.Context, selectors *domain.SelectorSet) (*domain.RegistrationEntry, error)
    ListEntries(ctx context.Context) ([]*domain.RegistrationEntry, error)
    DeleteEntry(ctx context.Context, identityNamespace *domain.IdentityNamespace) error
}

type NodeAttestor interface {
    // AttestNode performs node attestation and returns attested domain.Node
    // In-memory: uses hardcoded selectors
    // Real SPIRE: uses platform attestation (AWS IID, TPM, etc.)
    AttestNode(ctx context.Context, identityNamespace *domain.IdentityNamespace) (*domain.Node, error)
}

type WorkloadAttestor interface {
    // Attest verifies workload and returns selectors
    Attest(ctx context.Context, workload WorkloadInfo) ([]string, error)
}

type IdentityDocumentValidator interface {
    // Validate uses SDK verification (e.g., x509svid.ParseAndVerify)
    Validate(ctx context.Context, svid *domain.IdentityDocument, expectedID *domain.IdentityNamespace) error
}

type IdentityNamespaceParser interface {
    // ParseFromString parses SPIFFE ID from URI string (abstracts SDK's spiffeid.FromString)
    ParseFromString(ctx context.Context, id string) (*domain.IdentityNamespace, error)

    // ParseFromPath creates SPIFFE ID from components (abstracts SDK's spiffeid.FromPath)
    ParseFromPath(ctx context.Context, trustDomain *domain.TrustDomain, path string) (*domain.IdentityNamespace, error)
}
```

## Translation in Adapters

Adapters translate between SDK types and domain types:

```go
// In internal/adapters/outbound/spire/server.go
func (s *InMemoryServer) IssueIdentity(ctx context.Context, identityNamespace *domain.IdentityNamespace) (*domain.IdentityDocument, error) {
    // Use identityNamespace.String() when calling x509 APIs
    spiffeURI, _ := url.Parse(identityNamespace.String())

    // Create x509.Certificate using standard library
    cert := &x509.Certificate{
        URIs: []*url.URL{spiffeURI},
        // ...
    }

    // Return domain IdentityDocument
    return domain.NewX509SVID(identityNamespace, cert, privateKey, chain)
}
```

```go
// Translation helper in internal/adapters/outbound/spire/translation.go
func domainToIdentity(identityNamespace *domain.IdentityNamespace, svid *domain.IdentityDocument) *app.Identity {
    return &app.Identity{
        IdentityNamespace: identityNamespace,
        Name:     extractNameFromIdentityNamespace(identityNamespace.String()),
        IdentityDocument:     svid,
    }
}
```

### IdentityDocument Validation Adapter

IdentityDocument validation is implemented as an adapter to avoid duplicating go-spiffe SDK functionality:

```go
// In internal/adapters/outbound/spire/validator.go
type IdentityDocumentValidator struct{}

func (v *IdentityDocumentValidator) Validate(ctx context.Context, svid *domain.IdentityDocument, expectedID *domain.IdentityNamespace) error {
    // Basic checks
    if svid == nil || !svid.IsValid() {
        return fmt.Errorf("IdentityDocument invalid")
    }

    // SPIFFE ID match
    if !svid.IdentityNamespace().Equals(expectedID) {
        return fmt.Errorf("SPIFFE ID mismatch")
    }

    // In real implementation with go-spiffe SDK:
    // bundle := ... // get trust bundle
    // _, err := x509svid.Verify(svid.Certificate(), svid.Chain(), bundle)
    // return err

    return nil
}
```

**Why this is an adapter, not domain logic:**
- The go-spiffe SDK provides `x509svid.ParseAndVerify` and `Verify` for chain-of-trust validation
- These SDK functions handle X.509 path validation, bundle verification, and trust domain checks
- Reimplementing this in the domain would duplicate SDK functionality
- The adapter can use the SDK's battle-tested verification logic
- Domain remains SDK-agnostic and focused on business rules

### Node Attestation Adapter

Node attestation is implemented as an adapter to handle platform-specific attestation logic:

```go
// In internal/adapters/outbound/attestor/node.go
type InMemoryNodeAttestor struct {
    trustDomain   string
    nodeSelectors map[string][]*domain.Selector
}

func (a *InMemoryNodeAttestor) AttestNode(ctx context.Context, identityNamespace *domain.IdentityNamespace) (*domain.Node, error) {
    // Create unattested node (pure domain logic)
    node := domain.NewNode(identityNamespace)

    // In-memory: use pre-registered selectors
    selectors := a.nodeSelectors[identityNamespace.String()]
    if len(selectors) == 0 {
        // Default for demonstration
        selector, _ := domain.NewSelector(domain.SelectorTypeNode, "hostname", "localhost")
        selectors = []*domain.Selector{selector}
    }

    // Populate selectors and mark attested
    selectorSet := domain.NewSelectorSet()
    for _, sel := range selectors {
        selectorSet.Add(sel)
    }
    node.SetSelectors(selectorSet)
    node.MarkAttested()

    return node, nil
}
```

**Why this is an adapter, not domain logic:**
- Real SPIRE uses platform-specific attestation plugins (AWS IID, GCP Instance Identity, TPM, join tokens)
- Platform validation happens outside the domain (e.g., verifying AWS signature with AWS APIs)
- Selector extraction is platform-specific (e.g., AWS instance tags, region, VPC)
- Domain `Node` entity models the *result* (attested state), adapter performs the *process* (platform attestation)
- In-memory walking skeleton simulates this with hardcoded selectors for demonstration

### Registration Repository Adapter

Registration entries are stored via an adapter to handle persistence and querying:

```go
// In internal/adapters/outbound/spire/registration_repository.go
type InMemoryRegistrationRepository struct {
    mu      sync.RWMutex
    entries map[string]*domain.RegistrationEntry
}

func (r *InMemoryRegistrationRepository) CreateEntry(ctx context.Context, entry *domain.RegistrationEntry) error {
    // Store entry with concurrent access protection
    r.mu.Lock()
    defer r.mu.Unlock()

    identityNamespaceStr := entry.IdentityNamespace().String()
    r.entries[identityNamespaceStr] = entry
    return nil
}

func (r *InMemoryRegistrationRepository) FindMatchingEntry(ctx context.Context, selectors *domain.SelectorSet) (*domain.RegistrationEntry, error) {
    r.mu.RLock()
    defer r.mu.RUnlock()

    // Core authorization: find entry matching workload selectors
    for _, entry := range r.entries {
        if entry.MatchesSelectors(selectors) {
            return entry, nil
        }
    }
    return nil, fmt.Errorf("no matching entry")
}
```

**Why this is an adapter, not domain logic:**
- Real SPIRE persists entries in a datastore (PostgreSQL, SQLite, etc.)
- Storage mechanism is infrastructure concern (CRUD operations, transactions, indexes)
- Domain `RegistrationEntry` models the authorization policy, adapter handles persistence
- `FindMatchingEntry` uses domain's `MatchesSelectors()` logic but adds storage query
- In-memory walking skeleton uses map for demonstration, real adapter would use SQL queries

## Benefits of This Approach

1. **Domain Independence**: Core business logic is independent of SPIFFE SDK implementation details
2. **Testing**: Can test domain logic without SDK dependencies
3. **Flexibility**: Can swap SPIFFE SDK or implementation without changing domain
4. **Clear Boundaries**: Anti-corruption layer provides clear translation points
5. **Maintainability**: SDK updates only affect adapters, not core domain

## References

- [Hexagonal Architecture](https://alistair.cockburn.us/hexagonal-architecture/)
- [Domain-Driven Design](https://martinfowler.com/bliki/DomainDrivenDesign.html)
- [Anti-Corruption Layer Pattern](https://docs.microsoft.com/en-us/azure/architecture/patterns/anti-corruption-layer)
