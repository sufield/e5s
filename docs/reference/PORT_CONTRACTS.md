# Port Contracts

This document defines the contract for all ports in the system. These contracts must be honored by all implementations (SPIRE SDK, mocks).

## Table of Contents

1. [Error Contract Philosophy](#error-contract-philosophy)
2. [Production Ports](#production-ports)
   - [MTLSServer](#mtlsserver)
   - [MTLSClient](#mtlsclient)
   - [Agent](#agent)
   - [TrustDomainParser](#trustdomainparser)
   - [IdentityCredentialParser](#identitycredentialparser)
   - [IdentityDocumentValidator](#identitydocumentvalidator)
   - [IdentityProvider](#identityprovider)
3. [Testing Guidelines](#testing-guidelines)
4. [Contract Checklist](#contract-checklist)

---

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
if identityCredential == nil {
    return nil, fmt.Errorf("%w: identity credential cannot be nil", domain.ErrIdentityDocumentInvalid)
}
```

**Wrapping SDK Errors**:
```go
doc, err := s.client.FetchX509SVID(ctx)
if err != nil {
    return nil, fmt.Errorf("%w: failed to fetch identity from SPIRE: %v", domain.ErrServerUnavailable, err)
}
```

**Rules**:
1. Always use `%w` verb to wrap sentinel errors (not `%v`)
2. Add context after the sentinel error, not before
3. Return sentinel directly when the error is self-explanatory
4. Wrap with context when additional information helps debugging

---

## Production Ports

These ports are used in production deployments with real SPIRE infrastructure.

**Location**: `internal/ports/identityserver.go`, `internal/ports/outbound.go`, `internal/ports/inbound.go`

### MTLSServer

**Purpose**: Serves HTTPS with SPIFFE/mTLS authentication

**SDK Equivalent**: `workloadapi.X509Source` + `http.Server` with mTLS config

**Location**: `internal/ports/identityserver.go:50`

**Methods**:

| Method | Signature | Description | Returns | Error Cases |
|--------|-----------|-------------|---------|-------------|
| `Handle` | `(pattern, handler) → error` | Registers HTTP handler (before Start) | nil or error | `ErrCannotRegisterAfterStart` if called after Start() |
| `Start` | `(ctx) → error` | Starts HTTPS server (blocks) | nil or error | `ErrBindFailed` if port unavailable<br>`ErrServerFailed` if serve fails |
| `Shutdown` | `(ctx) → error` | Graceful shutdown with timeout | nil or error | `ErrShutdownTimeout` if timeout exceeded |
| `Close` | `() → error` | Releases resources (X509Source) | nil or error | `ErrCloseFailed` if cleanup fails |

**Configuration**:
```go
type MTLSConfig struct {
    WorkloadAPI WorkloadAPIConfig  // Socket path to SPIRE agent
    SPIFFE      SPIFFEConfig       // Authorization policy (ONE required)
    HTTP        HTTPConfig         // Server settings (address, timeouts)
}
```

**Authorization Policies** (exactly ONE required):
- `AllowedPeerID`: Specific client SPIFFE ID (exact match)
- `AllowedTrustDomain`: Any client from trust domain

**Identity Extraction**:

Handlers access authenticated identity using port-level abstractions:

```go
func handler(w http.ResponseWriter, r *http.Request) {
    // Use port-level identity accessor (adapter-agnostic)
    id, ok := ports.PeerIdentity(r.Context())
    if !ok {
        http.Error(w, "No identity", http.StatusInternalServerError)
        return
    }

    // id.SPIFFEID = "spiffe://example.org/client"
    // id.TrustDomain = "example.org"
    // id.Path = "/client"

    fmt.Fprintf(w, "Authenticated as: %s\n", id.SPIFFEID)
}
```

The adapter automatically injects `ports.Identity` into the request context during mTLS authentication. Handlers depend on ports, not on adapter-specific code.

**Example Usage**:
```go
server, err := identityserver.New(ctx, ports.MTLSConfig{
    WorkloadAPI: ports.WorkloadAPIConfig{
        SocketPath: "unix:///tmp/spire-agent/public/api.sock",
    },
    SPIFFE: ports.SPIFFEConfig{
        AllowedTrustDomain: "example.org",
    },
    HTTP: ports.HTTPConfig{
        Address: ":8443",
    },
})
if err != nil {
    return err
}
defer server.Close()

err = server.Handle("/api/hello", http.HandlerFunc(handler))
if err != nil {
    return err
}

server.Start(ctx)  // Blocks until shutdown
```

**See Also**: `examples/zeroconfig-example/` for complete usage example

---

### MTLSClient

**Purpose**: Performs HTTP over SPIFFE/mTLS

**SDK Equivalent**: `http.Client` with `workloadapi.X509Source` transport

**Location**: `internal/ports/identityserver.go:69`

**Methods**:

| Method | Signature | Description | Returns | Error Cases |
|--------|-----------|-------------|---------|-------------|
| `Do` | `(ctx, req) → (*http.Response, error)` | Executes HTTP request with mTLS | Response or error | `ErrConnectionFailed` if server unreachable<br>`ErrTLSHandshakeFailed` if auth fails |
| `Close` | `() → error` | Releases resources | nil or error | `ErrCloseFailed` if cleanup fails |

**Configuration**:
```go
type MTLSConfig struct {
    WorkloadAPI WorkloadAPIConfig  // Socket path to SPIRE agent
    SPIFFE      SPIFFEConfig       // Server verification (optional)
    HTTP        HTTPConfig         // Client settings (timeouts, connection pool)
}
```

**Example Implementation**:
```go
client, err := httpclient.New(ctx, httpclient.Config{
    WorkloadAPI: httpclient.WorkloadAPIConfig{
        SocketPath: "unix:///tmp/spire-agent/public/api.sock",
    },
    SPIFFE: httpclient.SPIFFEConfig{
        ExpectedServerID: "spiffe://example.org/server",
    },
    HTTP: httpclient.HTTPClientConfig{
        Timeout: 30 * time.Second,
    },
})
if err != nil {
    return err
}
defer client.Close()

resp, err := client.Get(ctx, "https://server:8443/api/hello")
```

**See Also**: `examples/zeroconfig-example/` for usage example

---

### Agent

**Purpose**: SPIRE agent operations (identity fetching, SVID refresh)

**SDK Equivalent**: `workloadapi.Client`

**Location**: `internal/ports/outbound.go:9`

**Methods**:

| Method | Signature | Description | Returns | Error Cases |
|--------|-----------|-------------|---------|-------------|
| `GetIdentity` | `(ctx) → (*IdentityDocument, error)` | Returns agent's own identity, refreshing if expired | Agent identity document | `ErrAgentUnavailable` if SPIRE unavailable<br>`ErrIdentityDocumentExpired` if cannot refresh |
| `FetchIdentityDocument` | `(ctx, workload) → (*IdentityDocument, error)` | Fetches workload SVID via SPIRE Workload API | Workload identity document | `ErrWorkloadNotFound` if not registered<br>`ErrServerUnavailable` if SPIRE unavailable |
| `Close` | `() → error` | Releases resources | nil or error | `ErrCloseFailed` if cleanup fails |

**Flow**: `Application → SPIRE Agent (via Workload API) → SPIRE Server → Return SVID`

**Example Usage**:
```go
// Fetch agent's own identity
doc, err := agent.GetIdentity(ctx)
if err != nil {
    return err
}

// Fetch workload identity via SPIRE
workload := domain.NewWorkload(123, 1000, 1000, "/usr/bin/app")
doc, err := agent.FetchIdentityDocument(ctx, workload)
if err != nil {
    if errors.Is(err, domain.ErrWorkloadNotFound) {
        // Workload not registered in SPIRE
    }
    return err
}
```

---

### TrustDomainParser

**Purpose**: Parses and validates trust domain strings

**SDK Equivalent**: `spiffeid.TrustDomainFromString()`

**Location**: `internal/ports/outbound.go:15`

**Methods**:

| Method | Signature | Description | Returns | Error Cases |
|--------|-----------|-------------|---------|-------------|
| `FromString` | `(ctx, name) → (*TrustDomain, error)` | Parses trust domain string | Trust domain | `ErrInvalidTrustDomain` if empty/has scheme/has path<br>`ErrInvalidTrustDomain` if invalid DNS format |

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

### IdentityCredentialParser

**Purpose**: Parses and validates SPIFFE ID strings

**SDK Equivalent**: `spiffeid.FromString()`, `spiffeid.FromPath()`

**Location**: `internal/ports/outbound.go:19`

**Methods**:

| Method | Signature | Description | Returns | Error Cases |
|--------|-----------|-------------|---------|-------------|
| `ParseFromString` | `(ctx, id) → (*IdentityCredential, error)` | Parses SPIFFE ID URI | Identity credential | `ErrInvalidIdentityCredential` if empty/invalid<br>`ErrInvalidIdentityCredential` if scheme != "spiffe"<br>`ErrInvalidIdentityCredential` if trust domain empty |
| `ParseFromPath` | `(ctx, trustDomain, path) → (*IdentityCredential, error)` | Constructs from components | Identity credential | `ErrInvalidIdentityCredential` if trust domain nil |

**Validation Rules**:
- ✅ Valid: `"spiffe://example.org/host"`, `"spiffe://example.org/workload/server"`
- ❌ Invalid: `""`, `"example.org/host"`, `"http://example.org/host"`

**Example Usage**:
```go
// Parse from string
cred, err := parser.ParseFromString(ctx, "spiffe://example.org/workload")
// err == nil, cred.TrustDomain().String() == "example.org", cred.Path() == "/workload"

// Parse from path
td, _ := tdParser.FromString(ctx, "example.org")
cred, err := parser.ParseFromPath(ctx, td, "/workload")
// err == nil, cred.String() == "spiffe://example.org/workload"
```

---

### IdentityDocumentValidator

**Purpose**: Validates identity documents (X.509 SVIDs)

**SDK Equivalent**: `x509svid.Verify()`

**Location**: `internal/ports/outbound.go:24`

**Methods**:

| Method | Signature | Description | Returns | Error Cases |
|--------|-----------|-------------|---------|-------------|
| `ValidateIdentityDocument` | `(ctx, doc, expectedID) → error` | Validates SVID | nil or error | `ErrIdentityDocumentExpired` if expired<br>`ErrIdentityDocumentMismatch` if ID mismatch<br>`ErrCertificateChainInvalid` if chain invalid |

**Example Usage**:
```go
// Validate SVID
err = validator.ValidateIdentityDocument(ctx, doc, expectedCred)
// err == nil (valid)

// Expired SVID
expiredDoc := createExpiredDoc()
err = validator.ValidateIdentityDocument(ctx, expiredDoc, expectedCred)
// errors.Is(err, domain.ErrIdentityDocumentExpired) == true
```

---

### IdentityProvider

**Purpose**: Client interface for workloads to fetch their SVID

**SDK Equivalent**: `workloadapi.Client.FetchX509SVID()`

**Location**: `internal/ports/inbound.go:10`

**Methods**:

| Method | Signature | Description | Returns | Error Cases |
|--------|-----------|-------------|---------|-------------|
| `FetchIdentity` | `(ctx) → (*dto.Identity, error)` | Fetches identity for calling workload | Identity DTO with SVID | `ErrWorkloadAttestationFailed`<br>`ErrNoMatchingMapper`<br>`ErrServerUnavailable` |
| `Close` | `() → error` | Releases resources | nil or error | `ErrCloseFailed` if cleanup fails |

**Important**: No `callerIdentity` parameter - server extracts credentials from Unix socket connection

**Example Usage**:
```go
// Workload code
provider, err := CreateIdentityProvider(ctx, config)
if err != nil {
    return err
}
defer provider.Close()

identity, err := provider.FetchIdentity(ctx)
// err == nil, identity contains SVID

// Use for mTLS
tlsConfig := &tls.Config{
    Certificates: []tls.Certificate{identity.TLSCertificate()},
}
```

---

## Testing Guidelines

### Unit Tests

Mock interfaces with exact error returns:

```go
type MockX509Fetcher struct {
    doc *domain.IdentityDocument
    err error
}

func (m *MockX509Fetcher) FetchX509SVID(ctx context.Context) (*domain.IdentityDocument, error) {
    return m.doc, m.err
}

func TestAgentGetIdentityRefresh(t *testing.T) {
    mockFetcher := &MockX509Fetcher{doc: freshDoc}
    agent := spire.NewAgent(ctx, mockFetcher, agentSpiffeID, parser)

    doc, err := agent.GetIdentity(ctx)
    require.NoError(t, err)
    assert.False(t, doc.IsExpired())
}
```

### Integration Tests

Test with real SPIRE infrastructure in containerized environment:

```go
func TestWorkloadIdentityFetch(t *testing.T) {
    // Requires SPIRE Agent + Server (e.g., via Docker Compose)
    config := config.Load()
    agent := spire.NewAgent(ctx, client, config.AgentSpiffeID, parser)

    workload := domain.NewWorkload(123, 1000, 1000, "/usr/bin/app")
    doc, err := agent.FetchIdentityDocument(ctx, workload)

    require.NoError(t, err)
    assert.Equal(t, "spiffe://example.org/test-workload", doc.IdentityCredential().String())
}
```

### Production Integration Tests

Use real SPIRE infrastructure:

```go
func TestMTLSAuthentication(t *testing.T) {
    // Requires SPIRE server and agent running
    server, err := identityserver.New(ctx, ports.MTLSConfig{
        WorkloadAPI: ports.WorkloadAPIConfig{
            SocketPath: "unix:///tmp/spire-agent/public/api.sock",
        },
        SPIFFE: ports.SPIFFEConfig{
            AllowedTrustDomain: "example.org",
        },
        HTTP: ports.HTTPConfig{
            Address: ":8443",
        },
    })
    require.NoError(t, err)
    defer server.Close()

    // Test actual mTLS handshake
    // ...
}
```

---

## Contract Checklist

When implementing a new adapter:

### Error Handling
- [ ] Returns exact domain errors from `domain/errors.go`
- [ ] Uses sentinel errors appropriately:
  - Direct return when no context needed: `return nil, domain.ErrCANotInitialized`
  - Wrapped with context: `return nil, fmt.Errorf("%w: additional context", domain.ErrSentinel)`
- [ ] Maps SDK errors to domain errors (if using real SDK)

### Input Validation
- [ ] Validates all inputs (nil checks, format validation)
- [ ] Returns appropriate error for each validation failure
- [ ] Handles edge cases (empty strings, zero values, nil pointers)

### Testing
- [ ] Unit tests assert exact error types with `errors.Is()`
- [ ] Integration tests cover happy path + all error cases
- [ ] Production tests use real SPIRE infrastructure

### Documentation
- [ ] Port interface documented with error contract
- [ ] Example usage provided
- [ ] Cross-references to related docs

### Resource Management
- [ ] `Close()` method is idempotent
- [ ] Resources released in reverse order of acquisition
- [ ] Errors from cleanup operations are reported
- [ ] Context cancellation is respected where applicable

---

## See Also

- **[ARCHITECTURE.md](../architecture/ARCHITECTURE.md)** - Hexagonal architecture patterns and principles
- **[DOMAIN.md](../architecture/DOMAIN.md)** - Domain model and types
- **`internal/ports/*.go`** - Actual port interface definitions

---

## Summary

### Production Ports (SPIRE SDK)
- **MTLSServer**: HTTPS server with SPIFFE/mTLS authentication
- **MTLSClient**: HTTP client with SPIFFE/mTLS authentication
- **Agent**: Workload identity fetching and SVID refresh
- **TrustDomainParser**: Parse trust domain strings
- **IdentityCredentialParser**: Parse SPIFFE ID strings
- **IdentityDocumentValidator**: Validate X.509 SVIDs
- **IdentityProvider**: Client interface for fetching SVID

All implementations must honor the error contracts and validation rules specified in this document.
