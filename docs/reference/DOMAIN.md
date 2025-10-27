---
type: reference
audience: intermediate
---

# Domain Model

The domain directory contains the domain model for the SPIRE identity system.

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

### Entities

Entities have identity and lifecycle.

#### **IdentityDocument** (`identity_document.go`)

SPIFFE Verifiable Identity Document - the issued credential.

```go
doc := domain.NewIdentityDocumentFromComponents(
    identityCredential,
    cert,       // *x509.Certificate
    chain,      // []*x509.Certificate
)

doc.IsValid() // Checks time validity
doc.IsExpired() // Checks expiration
doc.ExpiresAt() // Returns expiration time
doc.IdentityCredential() // Returns identity
doc.Certificate() // Returns the certificate

// Note: Private keys are NOT part of the domain model
// - In production: Managed by SDK's X509SVID type
// - In dev/testing: Stored in dto.Identity at the DTO layer
```

Uses `crypto/x509.Certificate` from standard library (acceptable as it's not an external SDK).

**Invariants**:

- IdentityCredential is never nil for valid document
- For X.509 documents, cert and chain are non-nil (private keys managed by adapters/DTO)
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

## Domain Errors

**Sentinel Errors**: The domain uses sentinel errors (`errors.go`) for better error handling:

- `ErrInvalidIdentityCredential` - SPIFFE ID is nil or malformed
- `ErrInvalidTrustDomain` - Trust domain is invalid
- `ErrWorkloadNotFound` - Workload not found
- `ErrInvalidWorkload` - Workload information is invalid
- `ErrIdentityDocumentExpired` - Identity document is expired or not yet valid
- `ErrIdentityDocumentInvalid` - Identity document is invalid
- `ErrCertificateChainInvalid` - Certificate chain validation failed

Use with `errors.Is()` for checking and `fmt.Errorf("%w", ...)` for wrapping with context.

Full chain-of-trust validation is handled by adapters using go-spiffe SDK's verification capabilities, not reimplemented in domain.

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

**See Also**: [PORT_CONTRACTS.md](PORT_CONTRACTS.md) for complete port contracts and error handling

## Translation in Adapters

Adapters translate between SDK types and domain types:

### SPIRE SDK Adapter

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

    // Validate private key is usable (adapter validates SDK's key, but doesn't store in domain)
    // Note: Private keys remain in the SDK's X509SVID, NOT in domain.IdentityDocument
    signer := svid.PrivateKey
    if signer == nil {
        return nil, fmt.Errorf("%w: missing/invalid private key", domain.ErrIdentityDocumentInvalid)
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

    // Create domain identity document (no private key - managed by adapter)
    return domain.NewIdentityDocumentFromComponents(
        identityCredential,
        leaf,
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

## Domain Files

Current domain implementation:

| File | Purpose |
|------|---------|
| `trust_domain.go` | Trust domain value object |
| `identity_credential.go` | SPIFFE ID value object |
| `identity_document.go` | SVID entity |
| `workload.go` | Workload entity |
| `errors.go` | Sentinel errors |
| `doc.go` | Package documentation |

## References

- **[PORT_CONTRACTS.md](PORT_CONTRACTS.md)** - Complete port contracts and error handling
- **[INVARIANTS.md](INVARIANTS.md)** - Domain invariants and validation rules
- [Hexagonal Architecture](https://alistair.cockburn.us/hexagonal-architecture/)
- [Domain-Driven Design](https://martinfowler.com/bliki/DomainDrivenDesign.html)
- [Anti-Corruption Layer Pattern](https://docs.microsoft.com/en-us/azure/architecture/patterns/anti-corruption-layer)
