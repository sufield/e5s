# Port Contracts

This document defines the contract for all ports in the system. These contracts must be honored by all implementations (in-memory, real SDK, mocks).

## Error Contract Philosophy

All ports return typed domain errors from `domain/errors.go`. This ensures:
- Callers can use `errors.Is()` for reliable error checking
- Real SDK adapters map SDK errors to domain errors
- Tests can assert exact error types
- Error handling is consistent across all implementations

### Sentinel Error Usage Patterns

**Direct Return (No Additional Context)**:
```go
if len(caCerts) == 0 {
    return nil, domain.ErrCANotInitialized
}
```

**Wrapped with Context** (preserves sentinel for `errors.Is()` while adding details):
```go
if identityNamespace == nil {
    return nil, fmt.Errorf("%w: identity namespace cannot be nil", domain.ErrIdentityDocumentInvalid)
}
```

**Wrapping SDK Errors**:
```go
doc, err := s.client.FetchX509SVID(ctx)
if err != nil {
    return nil, fmt.Errorf("%w: failed to fetch identity from SPIRE: %v", domain.ErrServerUnavailable, err)
}
```

**Key Rules**:
1. Always use `%w` verb to wrap sentinel errors (not `%v`)
2. Add context after the sentinel error, not before
3. Return sentinel directly when the error is self-explanatory
4. Wrap with context when additional information helps debugging

## Outbound Ports

### IdentityMapperRegistry

**Purpose**: Read-only registry of identity mappers (selectors → identity namespace mappings)

**Lifecycle**: Seeded at bootstrap, sealed before runtime, immutable after

**Methods**:

| Method | Signature | Description | Returns | Error Cases |
|--------|-----------|-------------|---------|-------------|
| `FindBySelectors` | `(ctx, selectors) → (*IdentityMapper, error)` | Finds mapper matching selectors (AND logic) | First matching mapper | `ErrNoMatchingMapper` if none match<br>`ErrInvalidSelectors` if nil/empty |
| `ListAll` | `(ctx) → ([]*IdentityMapper, error)` | Returns all seeded mappers | All mappers | `ErrRegistryEmpty` if no mappers |

**Selector Matching Logic** (AND):
```
Mapper selectors: {unix:uid:1000, unix:gid:1000}
Discovered selectors: {unix:uid:1000, unix:gid:1000, unix:path:/usr/bin/app}
Result: MATCH (all mapper selectors present)

Mapper selectors: {unix:uid:1000, unix:gid:1001}
Discovered selectors: {unix:uid:1000, unix:gid:1000}
Result: NO MATCH (unix:gid:1001 missing)
```

**Example Usage**:
```go
// Successful match
selectors := domain.NewSelectorSet()
selectors.Add(domain.ParseSelectorFromString("unix:uid:1000"))
mapper, err := registry.FindBySelectors(ctx, selectors)
// err == nil, mapper != nil

// No match
selectors := domain.NewSelectorSet()
selectors.Add(domain.ParseSelectorFromString("unix:uid:9999"))
mapper, err := registry.FindBySelectors(ctx, selectors)
// errors.Is(err, domain.ErrNoMatchingMapper) == true
```

---

### WorkloadAttestor

**Purpose**: Attests workload identity based on platform-specific attributes

**Methods**:

| Method | Signature | Description | Returns | Error Cases |
|--------|-----------|-------------|---------|-------------|
| `Attest` | `(ctx, workload ProcessIdentity) → ([]string, error)` | Generates selectors from process attributes | Selector strings | `ErrWorkloadAttestationFailed` if attestation fails<br>`ErrInvalidProcessIdentity` if workload invalid<br>`ErrNoAttestationData` if no selectors |

**Selector Format**: `"type:value"` (e.g., `"unix:uid:1000"`, `"k8s:namespace:prod"`)

**Example Usage**:
```go
// Unix attestation
workload := ports.ProcessIdentity{
    PID: 123,
    UID: 1000,
    GID: 1000,
    Path: "/usr/bin/app",
}
selectors, err := attestor.Attest(ctx, workload)
// selectors = ["unix:uid:1000", "unix:gid:1000"]

// Invalid workload
workload := ports.ProcessIdentity{UID: -1}
selectors, err := attestor.Attest(ctx, workload)
// errors.Is(err, domain.ErrInvalidProcessIdentity) == true
```

---

### Server

**Purpose**: SPIRE server operations (CA management, SVID issuance)

**Methods**:

| Method | Signature | Description | Returns | Error Cases |
|--------|-----------|-------------|---------|-------------|
| `IssueIdentity` | `(ctx, identityNamespace) → (*IdentityDocument, error)` | Issues X.509 SVID signed by CA | Identity document | `ErrIdentityDocumentInvalid` if namespace invalid<br>`ErrServerUnavailable` if server down<br>`ErrCANotInitialized` if CA missing |
| `GetTrustDomain` | `() → *TrustDomain` | Returns server's trust domain | Trust domain or nil | None (returns nil if not initialized) |
| `GetCA` | `() → *x509.Certificate` | Returns CA certificate | CA cert or nil | None (returns nil if not initialized) |

**Example Usage**:
```go
// Issue SVID
namespace, _ := parser.ParseFromString(ctx, "spiffe://example.org/workload")
doc, err := server.IssueIdentity(ctx, namespace)
// err == nil, doc contains X.509 cert + private key

// Check CA
ca := server.GetCA()
if ca == nil {
    return errors.New("CA not initialized")
}
```

---

### Agent

**Purpose**: SPIRE agent operations (workload attestation, SVID fetching)

**Methods**:

| Method | Signature | Description | Returns | Error Cases |
|--------|-----------|-------------|---------|-------------|
| `GetIdentity` | `(ctx) → (*Identity, error)` | Returns agent's own identity | Agent identity | `ErrAgentUnavailable` if not initialized |
| `FetchIdentityDocument` | `(ctx, workload) → (*Identity, error)` | Attest → Match → Issue → Return | Workload identity | `ErrWorkloadAttestationFailed` if attestation fails<br>`ErrNoMatchingMapper` if no registration<br>`ErrServerUnavailable` if server unreachable |

**Flow**: `Attest workload → Find mapper by selectors → Issue SVID from server → Return identity`

**Example Usage**:
```go
// Fetch workload SVID
workload := ports.ProcessIdentity{UID: 1000, PID: 123}
identity, err := agent.FetchIdentityDocument(ctx, workload)
// err == nil, identity contains SVID

// Unregistered workload
workload := ports.ProcessIdentity{UID: 9999}
identity, err := agent.FetchIdentityDocument(ctx, workload)
// errors.Is(err, domain.ErrNoMatchingMapper) == true
```

---

### TrustDomainParser

**Purpose**: Parses and validates trust domain strings

**SDK Equivalent**: `spiffeid.TrustDomainFromString()`

**Methods**:

| Method | Signature | Description | Returns | Error Cases |
|--------|-----------|-------------|---------|-------------|
| `FromString` | `(ctx, name) → (*TrustDomain, error)` | Parses trust domain string | Trust domain | `ErrInvalidTrustDomain` if empty, has scheme, or has path<br>`ErrInvalidTrustDomain` if invalid DNS format (SDK) |

**Validation Rules**:
- ✅ Valid: `"example.org"`, `"subdomain.example.org"`
- ❌ Invalid: `""`, `"spiffe://example.org"`, `"example.org/path"`

**Example Usage**:
```go
// Valid
td, err := parser.FromString(ctx, "example.org")
// err == nil, td.String() == "example.org"

// Invalid (has scheme)
td, err := parser.FromString(ctx, "spiffe://example.org")
// errors.Is(err, domain.ErrInvalidTrustDomain) == true
```

---

### IdentityNamespaceParser

**Purpose**: Parses and validates SPIFFE ID strings

**SDK Equivalent**: `spiffeid.FromString()`, `spiffeid.FromPath()`

**Methods**:

| Method | Signature | Description | Returns | Error Cases |
|--------|-----------|-------------|---------|-------------|
| `ParseFromString` | `(ctx, id) → (*IdentityNamespace, error)` | Parses SPIFFE ID URI | Identity namespace | `ErrInvalidIdentityNamespace` if empty/invalid<br>`ErrInvalidIdentityNamespace` if scheme != "spiffe"<br>`ErrInvalidIdentityNamespace` if trust domain empty |
| `ParseFromPath` | `(ctx, trustDomain, path) → (*IdentityNamespace, error)` | Constructs from components | Identity namespace | `ErrInvalidIdentityNamespace` if trust domain nil |

**Validation Rules**:
- ✅ Valid: `"spiffe://example.org/host"`, `"spiffe://example.org/workload/server"`
- ❌ Invalid: `""`, `"example.org/host"`, `"http://example.org/host"`

**Example Usage**:
```go
// Parse from string
ns, err := parser.ParseFromString(ctx, "spiffe://example.org/workload")
// err == nil, ns.TrustDomain().String() == "example.org", ns.Path() == "/workload"

// Parse from path
td, _ := tdParser.FromString(ctx, "example.org")
ns, err := parser.ParseFromPath(ctx, td, "/workload")
// err == nil, ns.String() == "spiffe://example.org/workload"
```

---

### TrustBundleProvider

**Purpose**: Provides trust bundles (root CAs) for certificate chain validation

**SDK Equivalent**: `bundle.Source`

**Methods**:

| Method | Signature | Description | Returns | Error Cases |
|--------|-----------|-------------|---------|-------------|
| `GetBundle` | `(ctx, trustDomain) → ([]byte, error)` | Gets trust bundle for trust domain | PEM-encoded CA cert(s) | `ErrTrustBundleNotFound` if no bundle<br>`ErrInvalidTrustDomain` if trust domain nil |
| `GetBundleForIdentity` | `(ctx, identityNamespace) → ([]byte, error)` | Gets bundle for identity's trust domain | PEM-encoded CA cert(s) | `ErrTrustBundleNotFound` if no bundle<br>`ErrInvalidIdentityNamespace` if namespace nil |

**Example Usage**:
```go
// Get bundle
td, _ := parser.FromString(ctx, "example.org")
bundle, err := provider.GetBundle(ctx, td)
// err == nil, bundle contains PEM-encoded CA cert(s)

// Use in validation (production with SDK)
svid, _ := x509svid.ParseX509SVID(certPEM, keyPEM)
err = x509svid.Verify(svid.Certificates, bundle)
```

---

### IdentityDocumentProvider

**Purpose**: Creates and validates identity documents (X.509 SVIDs)

**SDK Equivalent**: `x509svid` package

**Methods**:

| Method | Signature | Description | Returns | Error Cases |
|--------|-----------|-------------|---------|-------------|
| `CreateX509IdentityDocument` | `(ctx, identityNamespace, caCert, caKey) → (*IdentityDocument, error)` | Creates X.509 SVID | Identity document | `ErrIdentityDocumentInvalid` if inputs invalid |
| `ValidateIdentityDocument` | `(ctx, doc, expectedID) → error` | Validates SVID | nil or error | `ErrIdentityDocumentExpired` if expired<br>`ErrIdentityDocumentMismatch` if namespace mismatch<br>`ErrCertificateChainInvalid` if chain invalid (SDK) |

**Example Usage**:
```go
// Create SVID
namespace, _ := parser.ParseFromString(ctx, "spiffe://example.org/workload")
doc, err := provider.CreateX509IdentityDocument(ctx, namespace, caCert, caKey)
// err == nil, doc contains X.509 cert + private key

// Validate SVID
err = provider.ValidateIdentityDocument(ctx, doc, namespace)
// err == nil (valid)

// Expired SVID
expiredDoc := createExpiredDoc()
err = provider.ValidateIdentityDocument(ctx, expiredDoc, namespace)
// errors.Is(err, domain.ErrIdentityDocumentExpired) == true
```

---

## Inbound Ports

### IdentityClient

**Purpose**: Client interface for workloads to fetch their SVID

**SDK Equivalent**: `workloadapi.Client.FetchX509SVID()`

**Methods**:

| Method | Signature | Description | Returns | Error Cases |
|--------|-----------|-------------|---------|-------------|
| `FetchX509SVID` | `(ctx) → (*Identity, error)` | Fetches SVID for calling workload | Identity with SVID | `ErrWorkloadAttestationFailed`<br>`ErrNoMatchingMapper`<br>`ErrServerUnavailable` |

**Important**: No `callerIdentity` parameter - server extracts credentials from Unix socket connection

**Example Usage**:
```go
// Workload code
client := workloadapi.NewClient("/tmp/spire-agent/public/api.sock")
svid, err := client.FetchX509SVID(ctx)
// err == nil, svid contains identity + certificate

// Use for mTLS
tlsConfig := &tls.Config{
    Certificates: []tls.Certificate{svid.TLSCertificate()},
}
```

---

## Testing Guidelines

### Unit Tests

Mock interfaces with exact error returns:

```go
type MockRegistry struct {
    mapper *domain.IdentityMapper
    err    error
}

func (m *MockRegistry) FindBySelectors(ctx, selectors) (*domain.IdentityMapper, error) {
    return m.mapper, m.err
}

func TestFetchWithNoMatch(t *testing.T) {
    registry := &MockRegistry{err: domain.ErrNoMatchingMapper}
    service := NewService(registry)

    _, err := service.Fetch(ctx, selectors)
    require.Error(t, err)
    assert.True(t, errors.Is(err, domain.ErrNoMatchingMapper))
}
```

### Integration Tests

Use in-memory implementations to test full flows:

```go
func TestWorkloadAttestation(t *testing.T) {
    app, _ := app.Bootstrap(ctx, inmemory.NewInMemoryConfig(), compose.NewInMemoryDeps())

    workload := ports.ProcessIdentity{UID: 1000}
    identity, err := app.Agent.FetchIdentityDocument(ctx, workload)

    require.NoError(t, err)
    assert.Equal(t, "spiffe://example.org/test-workload", identity.IdentityNamespace.String())
}
```

---

## Contract Checklist

When implementing a new adapter:

- [ ] Returns exact domain errors from `domain/errors.go`
- [ ] Uses sentinel errors appropriately:
  - Direct return when no context needed: `return nil, domain.ErrCANotInitialized`
  - Wrapped with context: `return nil, fmt.Errorf("%w: additional context", domain.ErrSentinel)`
- [ ] Validates all inputs (nil checks, format validation)
- [ ] Handles all error cases documented in port contract
- [ ] Maps SDK errors to domain errors (if using real SDK)
- [ ] Unit tests assert exact error types with `errors.Is()`
- [ ] Integration tests cover happy path + all error cases
