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

❌ **go-spiffe SDK types** (`spiffeid.ID`, `x509svid.SVID`, etc.)
❌ **Framework-specific code** (HTTP, gRPC, databases)
❌ **Infrastructure concerns** (logging, metrics, configuration)

## Anti-Corruption Layer

The adapters act as an **anti-corruption layer** between the domain and external SDKs:

```
┌─────────────────────────────────────────┐
│         External SDK (go-spiffe)        │
│   spiffeid.ID, x509svid.SVID, etc.      │
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
         Ports (internal/ports/*.go)
         Use ONLY domain types
                     ↕
┌─────────────────────────────────────────┐
│     Core Domain (domain/*.go)           │
│   - TrustDomain                         │
│   - IdentityCredential                  │
│   - IdentityDocument                    │
│   - Selector / SelectorSet              │
│   - IdentityMapper (dev-only)           │
│   - Workload                            │
│   - Pure business logic                 │
└─────────────────────────────────────────┘
```

## Domain Concepts

### Value Objects

Value objects are immutable and identified by their values, not by identity.

#### **TrustDomain** (`trust_domain.go`)
Represents the scope of SPIFFE identities.

```go
td := domain.NewTrustDomainFromName("example.org")
td.String() // "example.org"
td.Equals(other) // bool
```

**Invariants**:
- Name is never empty (guaranteed by constructor)
- Immutable after construction
- Equality is based on name string comparison
- See [INVARIANTS.md](INVARIANTS.md) for complete list

#### **IdentityCredential** (`identity_credential.go`)
URI-based workload identifier in SPIFFE format. **Minimal value object** - parsing delegated to `IdentityCredentialParser` port.

```go
// Parsing done via IdentityCredentialParser adapter (not domain)
// For dev mode:
id := domain.NewIdentityCredentialFromComponents(trustDomain, "/workload")

// For production (via parser adapter):
parser := spire.NewIdentityCredentialParser()
id, err := parser.ParseFromString(ctx, "spiffe://example.org/workload")

// Domain provides getters only
id.String() // "spiffe://example.org/workload"
id.TrustDomain() // *TrustDomain
id.Path() // "/workload"
id.Equals(other) // bool
id.IsInTrustDomain(td) // bool
```

**Refactoring Note**: Parsing logic moved to adapter to avoid duplicating go-spiffe SDK's `spiffeid.FromString`. Domain holds only parsed data.

**Invariants**:
- TrustDomain is never nil
- Path defaults to "/" if empty (never stored as empty string)
- URI is always formatted as "spiffe://<trustDomain><path>"
- Immutable after construction
- See [INVARIANTS.md](INVARIANTS.md) for complete list

#### **Selector** (`selector.go`)
Key-value pair for workload attestation matching.

```go
selector, err := domain.NewSelector(domain.SelectorTypeWorkload, "uid", "1001")
selector.Formatted() // "workload:uid:1001"
selector.Type() // SelectorTypeWorkload
selector.Key() // "uid"
selector.Value() // "1001"

// Handles multi-colon values
k8sSelector, err := domain.ParseSelectorFromString("k8s:pod:ns:default:podname")
k8sSelector.Value() // "ns:default:podname"
```

**Improvements**:
- Uses sentinel errors (`ErrSelectorInvalid`) for validation
- Robust multi-colon value parsing with `strings.Join()`
- Field-by-field equality checking
- Formatted string caching for performance

**Invariants**:
- Key and value are never empty after construction
- Formatted matches "type:key:value" pattern
- Type, key, and value are immutable
- See [INVARIANTS.md](INVARIANTS.md) for complete list

#### **SelectorSet** (`selector_set.go`)
Collection of selectors for matching with automatic deduplication.

```go
set := domain.NewSelectorSet()
set.Add(selector)
set.Add(selector) // Duplicate - not added
set.Contains(selector) // true
len(set.All()) // 1 (no duplicates)
```

**Set Semantics**:
- Enforces uniqueness automatically in `Add()`
- True mathematical set behavior
- Order-preserving (slice-based)
- Defensive copy returned by `All()`

**Invariants**:
- Set contains no duplicate selectors (uniqueness)
- `All()` returns defensive copy to prevent external mutation
- See [INVARIANTS.md](INVARIANTS.md) for complete list

### Entities

Entities have identity and lifecycle.

#### **IdentityDocument** (`identity_document.go`)
SPIFFE Verifiable Identity Document - the issued credential.

```go
doc := domain.NewIdentityDocumentFromComponents(
    identityCredential,
    cert,       // *x509.Certificate
    privateKey, // crypto.Signer
    chain,      // []*x509.Certificate
)

doc.IsValid() // Checks time validity
doc.IsExpired() // Checks expiration
doc.ExpiresAt() // Returns expiration time
doc.IdentityCredential() // Returns identity
```

**Note**: Uses `crypto/x509.Certificate` from standard library (acceptable as it's not an external SDK).

**Invariants**:
- IdentityCredential is never nil for valid document
- For X.509 documents, cert/privateKey/chain are non-nil
- IsExpired() iff time.Now().After(expiresAt)
- Immutable after creation
- See [INVARIANTS.md](INVARIANTS.md) for complete list

#### **Workload** (`workload.go`)
Running software process to be identified.

```go
workload := domain.NewWorkload(pid, uid, gid, path)
workload.PID() // Process ID
workload.UID() // User ID
workload.GID() // Group ID
workload.Path() // Executable path

err := workload.Validate()
// Returns ErrWorkloadInvalid if PID <= 0 or path is empty
```

**Invariants**:
- PID must be > 0
- Path must be non-empty
- See [INVARIANTS.md](INVARIANTS.md) for complete list

#### **IdentityMapper** (`identity_mapper.go`) - Dev-Only

**Build Tag**: `//go:build dev`

Maps workload selectors to identity credentials. Represents SPIRE's registration entry concept for in-memory development mode.

**Design Philosophy**:
- **Dev-Only**: Only included in development builds
- **Production**: SPIRE Server manages registration entries via CLI: `spire-server entry create`
- **Educational**: Helps understand SPIRE's authorization model without deploying infrastructure

```go
// Create mapper requiring multiple selectors
uidSelector := domain.MustParseSelectorFromString("unix:uid:1000")
nsSelector := domain.MustParseSelectorFromString("k8s:namespace:prod")

selectorSet := domain.NewSelectorSet()
selectorSet.Add(uidSelector)
selectorSet.Add(nsSelector)

mapper, err := domain.NewIdentityMapper(identityCredential, selectorSet)
if err != nil {
    // Handles nil checks, empty selectors
}

// Set parent relationship (e.g., workload's parent is agent)
mapper.SetParentID(agentIdentityCredential)

// Authorization check during workload attestation (AND logic)
// Workload MUST have ALL mapper selectors to qualify
discoveredSelectors := domain.NewSelectorSet()
discoveredSelectors.Add(uidSelector)
discoveredSelectors.Add(nsSelector)
discoveredSelectors.Add(otherSelector) // Extra selectors OK

if mapper.MatchesSelectors(discoveredSelectors) {
    // TRUE: workload has ALL required selectors (uid:1000 AND ns:prod)
    // Can issue this SPIFFE ID
}
```

**AND Semantics**:
- ALL mapper selectors must be present in discovered selectors
- Extra discovered selectors are ignored
- Per SPIRE specification for strong attestation

**Why Dev-Only**:
- Real SPIRE Server manages registration via persistent database
- Production workloads only fetch identities via Workload API
- In-memory version enables local development and testing

**Invariants**:
- IdentityCredential is never nil after construction
- Selectors is never nil or empty after construction
- MatchesSelectors() uses AND logic (ALL selectors required)
- See [INVARIANTS.md](INVARIANTS.md) for complete list

### Domain Services

#### **AttestationService** (`attestation.go`)
Domain logic for attestation processes with sentinel error returns.

```go
service := domain.NewAttestationService()

// Match workload to identity mapper (pure domain logic)
// This demonstrates the authorization flow without SPIRE infrastructure
result, err := service.AttestWorkload(selectors, mappers)
if err != nil {
    // Check error using errors.Is()
    if errors.Is(err, domain.ErrNoMatchingMapper) {
        // No mapper found matching selectors
    }
    if errors.Is(err, domain.ErrInvalidSelectors) {
        // Invalid selectors provided
    }
}
```

**Sentinel Errors**: The domain uses sentinel errors (`errors.go`) for better error handling:
- `ErrNoMatchingMapper` - No identity mapper matches selectors
- `ErrInvalidSelectors` - Selectors are nil or empty
- `ErrInvalidIdentityCredential` - SPIFFE ID is nil or malformed
- `ErrWorkloadAttestationFailed` - Workload attestation failed
- `ErrIdentityDocumentExpired`, `ErrIdentityDocumentInvalid`, `ErrIdentityDocumentMismatch` - Document validation errors
- `ErrCANotInitialized` - CA not available
- `ErrServerUnavailable` - Server unreachable

Use with `errors.Is()` for checking and `fmt.Errorf("%w", ...)` for wrapping with context.

**Note**: Full chain-of-trust validation is handled by adapters using go-spiffe SDK's `x509svid.ParseAndVerify` and `Verify`, not reimplemented in domain.

## Usage in Ports

Ports (interfaces in `internal/ports/*.go`) use **ONLY domain types**:

### Production Ports

Used with real SPIRE infrastructure:

```go
// internal/ports/identityserver.go
type MTLSServer interface {
    Handle(pattern string, handler http.Handler) error
    Start(ctx context.Context) error
    Shutdown(ctx context.Context) error
    Close() error
}

type MTLSClient interface {
    Do(ctx context.Context, req *http.Request) (*http.Response, error)
    Close() error
}

// internal/ports/outbound.go
type Agent interface {
    GetIdentity(ctx context.Context) (*domain.IdentityDocument, error)
    FetchIdentityDocument(ctx context.Context, workload *domain.Workload) (*domain.IdentityDocument, error)
    Close() error
}

type TrustDomainParser interface {
    FromString(ctx context.Context, name string) (*domain.TrustDomain, error)
}

type IdentityCredentialParser interface {
    ParseFromString(ctx context.Context, id string) (*domain.IdentityCredential, error)
    ParseFromPath(ctx context.Context, trustDomain *domain.TrustDomain, path string) (*domain.IdentityCredential, error)
}

type IdentityDocumentValidator interface {
    ValidateIdentityDocument(ctx context.Context, doc *domain.IdentityDocument, expectedID *domain.IdentityCredential) error
}

// internal/ports/inbound.go
type IdentityProvider interface {
    FetchIdentity(ctx context.Context) (*dto.Identity, error)
    Close() error
}
```

### Dev-Only Ports

Used in development mode with in-memory implementations:

```go
// internal/ports/outbound_dev.go (//go:build dev)

type IdentityMapperRegistry interface {
    FindBySelectors(ctx context.Context, selectors *domain.SelectorSet) (*domain.IdentityMapper, error)
    ListAll(ctx context.Context) ([]*domain.IdentityMapper, error)
}

type WorkloadAttestor interface {
    Attest(ctx context.Context, workload *domain.Workload) ([]string, error)
}

type IdentityServer interface {
    IssueIdentity(ctx context.Context, identityCredential *domain.IdentityCredential) (*domain.IdentityDocument, error)
    GetTrustDomain() *domain.TrustDomain
    GetCACertPEM() []byte
}

type IdentityDocumentCreator interface {
    CreateX509IdentityDocument(ctx context.Context, identityCredential *domain.IdentityCredential, caCert interface{}, caKey interface{}) (*domain.IdentityDocument, error)
}

type IdentityDocumentProvider interface {
    IdentityDocumentCreator
    IdentityDocumentValidator
}

type TrustBundleProvider interface {
    GetBundle(ctx context.Context, trustDomain *domain.TrustDomain) ([]byte, error)
    GetBundleForIdentity(ctx context.Context, identityCredential *domain.IdentityCredential) ([]byte, error)
}
```

**See Also**: [PORT_CONTRACTS.md](PORT_CONTRACTS.md) for complete port contracts and error handling

## Translation in Adapters

Adapters translate between SDK types and domain types:

### Dev-Mode Adapter (In-Memory)

```go
// In internal/adapters/outbound/inmemory/server.go
type InMemoryServer struct {
    trustDomain         *domain.TrustDomain
    caCert              *x509.Certificate
    caKey               *rsa.PrivateKey
    certificateProvider ports.IdentityDocumentProvider
}

func (s *InMemoryServer) IssueIdentity(ctx context.Context, identityCredential *domain.IdentityCredential) (*domain.IdentityDocument, error) {
    if identityCredential == nil {
        return nil, fmt.Errorf("%w: identity credential cannot be nil", domain.ErrIdentityDocumentInvalid)
    }

    // Enforce trust-domain parity
    if identityCredential.TrustDomain().String() != s.trustDomain.String() {
        return nil, fmt.Errorf("%w: trust domain mismatch", domain.ErrIdentityDocumentInvalid)
    }

    if s.caCert == nil || s.caKey == nil {
        return nil, fmt.Errorf("%w: CA not initialized", domain.ErrCANotInitialized)
    }

    // Delegate to certificate provider port
    doc, err := s.certificateProvider.CreateX509IdentityDocument(ctx, identityCredential, s.caCert, s.caKey)
    if err != nil {
        return nil, fmt.Errorf("%w: %w", domain.ErrServerUnavailable, err)
    }

    return doc, nil
}

func (s *InMemoryServer) GetTrustDomain() *domain.TrustDomain {
    return s.trustDomain
}

func (s *InMemoryServer) GetCACertPEM() []byte {
    if s.caCert == nil {
        return nil
    }
    return pem.EncodeToMemory(&pem.Block{
        Type:  "CERTIFICATE",
        Bytes: s.caCert.Raw,
    })
}
```

### Production Adapter (SPIRE SDK)

```go
// In internal/adapters/outbound/spire/translation.go

// TranslateX509SVIDToIdentityDocument converts go-spiffe SVID to domain IdentityDocument
func TranslateX509SVIDToIdentityDocument(svid *x509svid.SVID) (*domain.IdentityDocument, error) {
    if svid == nil {
        return nil, fmt.Errorf("%w: nil SVID", domain.ErrIdentityDocumentInvalid)
    }

    // Validate SVID ID is non-zero
    if svid.ID.IsZero() {
        return nil, fmt.Errorf("%w: zero SPIFFE ID", domain.ErrIdentityDocumentInvalid)
    }

    // Validate certificates exist
    if len(svid.Certificates) == 0 || svid.Certificates[0] == nil {
        return nil, fmt.Errorf("%w: missing leaf certificate", domain.ErrIdentityDocumentInvalid)
    }
    leaf := svid.Certificates[0]

    // Validate private key is usable
    signer, ok := svid.PrivateKey.(crypto.Signer)
    if !ok || signer == nil {
        return nil, fmt.Errorf("%w: invalid private key", domain.ErrIdentityDocumentInvalid)
    }

    // Verify private key matches certificate
    if !publicKeysEqual(leaf.PublicKey, signer.Public()) {
        return nil, fmt.Errorf("%w: key mismatch", domain.ErrIdentityDocumentInvalid)
    }

    // Convert SPIFFE ID to domain IdentityCredential
    identityCredential, err := TranslateSPIFFEIDToIdentityCredential(svid.ID)
    if err != nil {
        return nil, fmt.Errorf("%w: %v", domain.ErrInvalidIdentityCredential, err)
    }

    // Defensive copy of certificate chain
    chain := make([]*x509.Certificate, len(svid.Certificates))
    copy(chain, svid.Certificates)

    // Create domain identity document
    return domain.NewIdentityDocumentFromComponents(
        identityCredential,
        leaf,
        signer,
        chain,
    )
}

// TranslateSPIFFEIDToIdentityCredential converts go-spiffe ID to domain IdentityCredential
func TranslateSPIFFEIDToIdentityCredential(id spiffeid.ID) (*domain.IdentityCredential, error) {
    if id.IsZero() {
        return nil, fmt.Errorf("%w: zero SPIFFE ID", domain.ErrInvalidIdentityCredential)
    }

    // Extract trust domain
    trustDomain := domain.NewTrustDomainFromName(id.TrustDomain().String())

    // Extract path (root IDs have empty path per SPIFFE spec)
    path := id.Path()
    if path == "" {
        path = "/" // Domain uses "/" for root identity
    }

    return domain.NewIdentityCredentialFromComponents(trustDomain, path), nil
}

// TranslateIdentityCredentialToSPIFFEID converts domain IdentityCredential to go-spiffe ID
func TranslateIdentityCredentialToSPIFFEID(identityCredential *domain.IdentityCredential) (spiffeid.ID, error) {
    if identityCredential == nil {
        return spiffeid.ID{}, fmt.Errorf("%w: nil identity credential", domain.ErrInvalidIdentityCredential)
    }

    // Parse trust domain
    sdkTD, err := spiffeid.TrustDomainFromString(identityCredential.TrustDomain().String())
    if err != nil {
        return spiffeid.ID{}, fmt.Errorf("%w: %w", domain.ErrInvalidIdentityCredential, err)
    }

    // Handle root path explicitly
    if identityCredential.Path() == "/" {
        return spiffeid.FromSegments(sdkTD) // Root ID
    }

    // Split path into segments
    segments := segmentsFromPath(identityCredential.Path())
    return spiffeid.FromSegments(sdkTD, segments...)
}
```

### Why Translation is in Adapters, Not Domain

**The adapter provides anti-corruption**:
- go-spiffe SDK provides `spiffeid.FromString()`, `x509svid.ParseAndVerify()`, etc.
- Reimplementing in domain would duplicate SDK functionality
- Domain remains SDK-agnostic and focused on business rules
- Adapters can use SDK's battle-tested validation logic
- Easy to swap SDK versions or implementations

## Benefits of This Approach

1. **Domain Independence**: Core business logic is independent of SPIFFE SDK implementation details
2. **Testing**: Can test domain logic without SDK dependencies
3. **Flexibility**: Can swap SPIFFE SDK or implementation without changing domain
4. **Clear Boundaries**: Anti-corruption layer provides clear translation points
5. **Maintainability**: SDK updates only affect adapters, not core domain
6. **Dev/Prod Separation**: Build tags enable dev-only entities for learning without affecting production

## Domain Files

Current domain implementation:

| File | Purpose | Build Tag |
|------|---------|-----------|
| `trust_domain.go` | Trust domain value object | (always) |
| `identity_credential.go` | SPIFFE ID value object | (always) |
| `identity_document.go` | SVID entity | (always) |
| `selector.go` | Selector value object | (always) |
| `selector_set.go` | Selector collection | (always) |
| `selector_type.go` | Selector type enum | (always) |
| `workload.go` | Workload entity | (always) |
| `identity_mapper.go` | Identity mapper entity | `dev` |
| `attestation.go` | Attestation service | (always) |
| `errors.go` | Sentinel errors | (always) |
| `doc.go` | Package documentation | (always) |

## References

- **[PORT_CONTRACTS.md](PORT_CONTRACTS.md)** - Complete port contracts and error handling
- **[INVARIANTS.md](INVARIANTS.md)** - Domain invariants and validation rules
- **[MTLS.md](MTLS.md)** - Production mTLS usage guide
- [Hexagonal Architecture](https://alistair.cockburn.us/hexagonal-architecture/)
- [Domain-Driven Design](https://martinfowler.com/bliki/DomainDrivenDesign.html)
- [Anti-Corruption Layer Pattern](https://docs.microsoft.com/en-us/azure/architecture/patterns/anti-corruption-layer)
