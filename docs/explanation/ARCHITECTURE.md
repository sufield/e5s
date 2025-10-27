---
type: explanation
audience: advanced
---

# Architecture

This document describes the hexagonal architecture implementation of the SPIRE Workload API, including layer boundaries, data flows, and design decisions.

## Table of Contents

- [Overview](#overview)
- [Hexagonal Architecture](#hexagonal-architecture)
- [Layers](#layers)
- [Data Flows](#data-flows)
- [Port Contracts](#port-contracts)
- [Adapter Implementations](#adapter-implementations)
- [Dependency Injection](#dependency-injection)
- [Error Handling](#error-handling)
- [Security Model](#security-model)

---

## Overview

This project implements a SPIRE Workload API wrapper using hexagonal architecture. The design enables:

1. **Separation of concerns**: Domain logic isolated from infrastructure
2. **Testability**: Unit tests without requiring SPIRE infrastructure
3. **Flexibility**: Clean adapter layer for SPIRE integration
4. **SDK integration**: Uses go-spiffe SDK through adapter layer

**Concepts**:
- **SPIFFE ID**: Unique identity credential (e.g., `spiffe://example.org/workload`)
- **SVID (Identity Document)**: X.509 certificate proving workload identity
- **Workload Attestation**: SPIRE process of verifying workload identity
- **Trust Bundle**: Root CA certificates for verifying SVIDs
- **Trust Domain**: Namespace for identities (e.g., `example.org`)

---

## Hexagonal Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                      Inbound Adapters                            │
│  ┌─────────────────┐           ┌──────────────────────┐         │
│  │ Workload API    │           │ CLI Demo             │         │
│  │ Server          │           │ Adapter              │         │
│  │ (HTTP/Unix)     │           │ (Presentation)       │         │
│  └────────┬────────┘           └──────────┬───────────┘         │
│           │                               │                     │
│           └───────────────┬───────────────┘                     │
│                           │                                     │
├───────────────────────────┼─────────────────────────────────────┤
│                      Inbound Ports                               │
│  ┌────────────────────────▼─────────────────────────────┐       │
│  │  IdentityClient                                      │       │
│  │  - FetchX509SVID(ctx) → (*Identity, error)          │       │
│  │  - FetchX509SVIDWithConfig(ctx, tls) → Identity     │       │
│  │                                                       │       │
│  │  Service (demonstration logic)                       │       │
│  │  - ExchangeMessage(from, to, content) → Message     │       │
│  └──────────────────────────────────────────────────────┘       │
├───────────────────────────────────────────────────────────────  ─┤
│                    Application Layer                             │
│  ┌──────────────────────────────────────────────────────┐       │
│  │  IdentityClientService                               │       │
│  │  - Handles SVID fetch requests from workloads        │       │
│  │  - Delegates to Agent port                           │       │
│  │                                                       │       │
│  │  Application (Bootstrap & Composition)               │       │
│  │  - Wires all dependencies                            │       │
│  │  - Initializes server, agent, registry               │       │
│  └──────────────────────────────────────────────────────┘       │
├──────────────────────────────────────────────────────────────────┤
│                    Domain Layer                                  │
│  ┌──────────────────────────────────────────────────────┐       │
│  │  Core Entities (Pure Business Logic)                 │       │
│  │  - TrustDomain                                        │       │
│  │  - IdentityCredential (SPIFFE ID)                     │       │
│  │  - IdentityDocument (SVID)                           │       │
│  │  - Workload (process information)                    │       │
│  │                                                       │       │
│  │  Domain Errors (Typed Sentinels)                     │       │
│  │  - ErrInvalidIdentityCredential                      │       │
│  │  - ErrInvalidTrustDomain                             │       │
│  │  - ErrIdentityDocumentExpired                        │       │
│  │  - ... (10+ typed errors)                            │       │
│  └──────────────────────────────────────────────────────┘       │
├──────────────────────────────────────────────────────────────────┤
│                    Outbound Ports                                │
│  ┌──────────────────────────────────────────────────────┐       │
│  │  Agent (SPIRE Integration)                           │       │
│  │  - GetIdentity() → Identity                          │       │
│  │  - FetchIdentityDocument(workload) → Identity        │       │
│  │  - WatchIdentityUpdates(ctx) → Stream               │       │
│  │                                                       │       │
│  │  TrustDomainParser                                   │       │
│  │  - FromString(name) → TrustDomain                   │       │
│  │                                                       │       │
│  │  IdentityCredentialParser                             │       │
│  │  - ParseFromString(id) → IdentityCredential          │       │
│  │  - ParseFromPath(td, path) → IdentityCredential      │       │
│  │                                                       │       │
│  │  TrustBundleProvider                                 │       │
│  │  - GetBundle(trustDomain) → []byte (PEM)            │       │
│  │  - GetBundleForIdentity(namespace) → []byte         │       │
│  │                                                       │       │
│  │  IdentityDocumentProvider                            │       │
│  │  - CreateX509IdentityDocument(...) → Document       │       │
│  │  - ValidateIdentityDocument(...) → error            │       │
│  └──────────────────────────────────────────────────────┘       │
├──────────────────────────────────────────────────────────────────┤
│                    Outbound Adapters                             │
│  ┌──────────────────────────────────────────────────────────┐   │
│  │ SPIRE SDK Adapters (Production)                        │   │
│  │                                                           │   │
│  │ - SDK Parsers (TrustDomain, IdentityCredential)        │   │
│  │ - Workload API Client (X.509 SVID fetch)               │   │
│  │ - Bundle Source (Trust bundle management)              │   │
│  │ - X.509 SVID Provider (Certificate operations)         │   │
│  │ - Chain Verifier (Certificate validation)              │   │
│  └──────────────────────────────────────────────────────────┘   │
└──────────────────────────────────────────────────────────────────┘
```

**Principles**:
1. **Dependencies point inward**: Outer layers depend on inner, never reverse
2. **Domain is pure**: No infrastructure dependencies (crypto/x509 used only for types)
3. **Ports define contracts**: Interfaces owned by application layer
4. **Adapters use SPIRE SDK**: go-spiffe SDK integration through adapter layer

---

## Layers

### 1. Domain Layer (`internal/domain/`)

**Purpose**: Pure business logic and value objects

**Contents**:
- **Value Objects**: `TrustDomain`, `IdentityCredential`, `IdentityDocument`, `Workload`
- **Errors**: Typed sentinel errors (`ErrInvalidIdentityCredential`, `ErrIdentityDocumentExpired`, etc.)

**Rules**:
- ✅ No external dependencies (except stdlib types like `crypto/x509.Certificate`)
- ✅ Immutable value objects (no setters, return new instances)
- ✅ Validation at construction
- ❌ No I/O, no database, no HTTP
- ❌ No SDK dependencies

**Example**:
```go
// Domain value object with validation
type IdentityCredential struct {
    trustDomain *TrustDomain
    path        string
    uri         string  // cached
}

// Constructor with validation and normalization
func NewIdentityCredentialFromComponents(
    trustDomain *TrustDomain,
    path string,
) *IdentityCredential {
    normalized := normalizePath(path)
    uri := fmt.Sprintf("spiffe://%s%s", trustDomain.String(), normalized)
    return &IdentityCredential{
        trustDomain: trustDomain,
        path:        normalized,
        uri:         uri,
    }
}
```

---

### 2. Port Layer (`internal/ports/`)

**Purpose**: Define contracts between layers (interfaces only)

**Contents**:
- **Inbound Ports**: `IdentityProvider` - Workload identity fetching
- **Outbound Ports**: `Agent`, `TrustDomainParser`, `IdentityCredentialParser`, `TrustBundleProvider`
- **Configuration**: `MTLSConfig`, `WorkloadAPIConfig`, `SPIFFEConfig`, `HTTPConfig`

**Data Transfer Objects**: Located in `internal/dto/`
- `dto.Identity` - Identity transport DTO with credentials and certificate
- `dto.Config` - Runtime configuration
- `dto.Message` - Demo message exchange

**Rules**:
- ✅ Interfaces owned by application layer
- ✅ SDK-agnostic (use domain types, not SDK types)
- ✅ Complete error contracts (documented return errors)
- ✅ DTOs separated into `internal/dto/` package
- ❌ No implementation details
- ❌ No business logic in DTOs

**Example**:
```go
// Port interface with error contract
type Agent interface {
    // GetIdentity returns the agent's own identity
    GetIdentity(ctx context.Context) (*dto.Identity, error)

    // FetchIdentityDocument fetches identity for a workload from SPIRE
    FetchIdentityDocument(ctx context.Context, workload *domain.Workload) (*dto.Identity, error)

    // WatchIdentityUpdates streams identity updates
    WatchIdentityUpdates(ctx context.Context) (<-chan *dto.Identity, error)
}
```

---

### 3. Application Layer (`internal/app/`)

**Purpose**: Orchestrate domain logic and use cases

**Contents**:
- **Services**: `IdentityClientService` (handles SVID fetch requests)
- **Bootstrap**: `Application` struct and `Bootstrap()` function
- **Use Cases**: Coordinate between ports to fulfill business requirements

**Rules**:
- ✅ Orchestrates port calls
- ✅ Enforces business rules
- ✅ Transaction boundaries (if needed)
- ❌ No infrastructure details (delegates to ports)

**Example**:
```go
type IdentityClientService struct {
    agent ports.Agent
}

func (s *IdentityClientService) IssueIdentity(
    ctx context.Context,
    workload *domain.Workload,
) (*dto.Identity, error) {
    // Orchestration: delegate to agent port
    doc, err := s.agent.FetchIdentityDocument(ctx, workload)
    if err != nil {
        return nil, fmt.Errorf("failed to fetch identity document: %w", err)
    }
    // Build DTO from domain object
    return &dto.Identity{
        IdentityCredential: doc.IdentityCredential(),
        IdentityDocument:   doc,
    }, nil
}
```

---

### 4. Adapter Layer (`internal/adapters/`)

**Purpose**: Implement ports with infrastructure

TODO

**Structure**:
```
adapters/
├── inbound/          # Handle external requests
│   ├── workloadapi/  # HTTP server over Unix socket
│   └── cli/          # Command-line presentation
└── outbound/         # Implement infrastructure ports
    ├── spire/        # SPIRE SDK integrations
    ├── workloadapi/  # Workload API client
    └── compose/      # Dependency injection factories
```

**Rules**:
- ✅ Implement port interfaces
- ✅ Handle infrastructure concerns (I/O, SDK calls, etc.)
- ✅ Map infrastructure errors to domain errors
- ❌ No business logic (delegate to application layer)

**Example**:
```go
// Outbound adapter implementing ports.Agent interface
type Agent struct {
    client X509Fetcher  // go-spiffe SDK client
    opts   Options
    mu     sync.RWMutex
    agentIdentity *dto.Identity
}

func (a *Agent) GetIdentity(ctx context.Context) (*domain.IdentityDocument, error) {
    // Check if refresh needed
    a.mu.RLock()
    current := a.agentIdentity
    need := needsRefresh(current.IdentityDocument, a.opts)
    a.mu.RUnlock()

    if need {
        // Fetch fresh SVID from SPIRE via SDK
        doc, err := a.client.FetchX509SVID(ctx)
        if err != nil {
            return nil, fmt.Errorf("refresh agent identity: %w", err)
        }
        // Update cached identity
        a.mu.Lock()
        a.agentIdentity.IdentityDocument = doc
        a.mu.Unlock()
    }

    return current.IdentityDocument, nil
}
```

---

## Data Flows

### 1. Workload Identity Fetch via SPIRE

**Flow**: Application → SPIRE Agent (via go-spiffe SDK) → SPIRE Server

```
┌─────────────────────┐
│  Application        │  (Your workload process)
│  internal/app/      │
└──────┬──────────────┘
       │ 1. Initialize SPIRE client
       │    client, _ := workloadapi.New(ctx, workloadapi.WithAddr("unix:///run/spire/sockets/agent.sock"))
       ▼
┌────────────────────────────┐
│  SPIRE Agent Adapter       │  (Adapter Layer)
│  internal/adapters/        │
│  outbound/spire/agent.go   │
└──────┬─────────────────────┘
       │ 2. FetchX509Context(ctx)
       │    - Uses go-spiffe SDK
       │    - Connects to SPIRE Agent socket
       ▼
┌────────────────────────────┐
│  SPIRE Agent               │  (External - deployed in cluster)
│  - Attests workload        │
│  - Validates registration  │
└──────┬─────────────────────┘
       │ 3. Request SVID from Server
       │    - Sends workload attestation
       │    - Includes node attestation
       ▼
┌────────────────────────────┐
│  SPIRE Server              │  (External - deployed in cluster)
│  - Matches registration    │
│  - Issues X.509 SVID       │
│  - Signs with CA           │
└──────┬─────────────────────┘
       │ 4. Return SVID + Bundle
       ▼
┌────────────────────────────┐
│  SPIRE Agent               │
│  - Caches SVID             │
│  - Watches for updates     │
└──────┬─────────────────────┘
       │ 5. X509Context{SVID, Bundle}
       ▼
┌────────────────────────────┐
│  SPIRE Agent Adapter       │
│  - Converts to dto.Identity│
│  - Extracts credentials    │
└──────┬─────────────────────┘
       │ 6. dto.Identity
       ▼
┌─────────────────────┐
│  Application        │  Uses identity for mTLS
│  - Certificate      │
│  - Private key      │
│  - Trust bundle     │
└─────────────────────┘
```

**Key Points**:
- **Delegated attestation**: SPIRE Agent handles all workload verification
- **SPIRE-managed registration**: SPIRE Server matches workload to registration entries
- **Automatic rotation**: SPIRE Agent automatically renews SVIDs before expiration
- **Watch for updates**: Application can watch for identity updates via streaming API
- **go-spiffe SDK**: All SPIRE communication uses official go-spiffe SDK

---

### 2. Application Bootstrap

**Flow**: main.go → Create SPIRE Client → Initialize Services → Start Server

```
┌────────────────┐
│  main.go       │  (Application entry point)
└───────┬────────┘
        │ 1. Load configuration
        │    - SPIRE socket path
        │    - Server listen address
        │    - mTLS settings
        ▼
┌────────────────────────────┐
│  app.Bootstrap()           │  (Application initialization)
│                            │
│  Step 1: Create SPIRE      │
│  Client                    │
│  ├─ workloadapi.New()      │
│  └─ Connect to SPIRE Agent │
└───────┬────────────────────┘
        │
        ▼
┌────────────────────────────┐
│  Step 2: Create Agent      │
│  Adapter                   │
│  ├─ spire.NewAgent()       │
│  └─ Wraps go-spiffe client │
└───────┬────────────────────┘
        │
        ▼
┌────────────────────────────┐
│  Step 3: Create Parsers    │
│  ├─ TrustDomainParser      │
│  ├─ IdentityCredentialParser│
│  └─ TrustBundleProvider    │
└───────┬────────────────────┘
        │
        ▼
┌────────────────────────────┐
│  Step 4: Initialize        │
│  Application               │
│  └─ Wire all dependencies  │
└───────┬────────────────────┘
        │
        ▼
┌────────────────────────────┐
│  Step 5: Start Services    │
│  ├─ HTTP server (optional) │
│  ├─ Identity client        │
│  └─ Ready for requests     │
└────────────────────────────┘
```

**Key Points**:
- **Single SPIRE connection**: One client connects to SPIRE Agent
- **go-spiffe SDK**: All SPIRE operations use official SDK
- **Automatic updates**: SPIRE client watches for SVID rotation
- **Configuration-driven**: Socket path and settings from config
- **Clean shutdown**: Graceful close of SPIRE connections

---

## Port Contracts

All ports have complete error contracts documented in `../reference/PORT_CONTRACTS.md` and `internal/ports/`.

**Example Contract**:

```go
// Agent provides access to SPIRE workload identities
//
// Error Contract:
// - GetIdentity returns error if SPIRE client not initialized
// - FetchIdentityDocument returns error if workload attestation fails
// - WatchIdentityUpdates returns error if stream cannot be established
type Agent interface {
    // GetIdentity returns the agent's own identity
    GetIdentity(ctx context.Context) (*dto.Identity, error)

    // FetchIdentityDocument fetches identity for a workload from SPIRE
    FetchIdentityDocument(ctx context.Context, workload *domain.Workload) (*dto.Identity, error)

    // WatchIdentityUpdates streams identity updates
    WatchIdentityUpdates(ctx context.Context) (<-chan *dto.Identity, error)
}
```

**Contract Guarantees**:
1. **Exact error types**: Adapters **must** return documented errors
2. **Validation**: Ports validate inputs and return typed errors
3. **Idempotency**: Read operations are side-effect free
4. **SDK encapsulation**: go-spiffe types converted to domain types at adapter boundary

---

## Adapter Implementations

### SPIRE SDK Adapters (`internal/adapters/outbound/spire/`)

**Status**: ✅ **Production Ready** - Fully implemented and tested

**Purpose**: Integration with SPIRE infrastructure using go-spiffe SDK

**Architecture**: All identity operations delegated to external SPIRE:
- ✅ **SPIRE Agent**: Handles workload attestation automatically
- ✅ **SPIRE Server**: Manages registration entries and issues SVIDs
- ✅ **go-spiffe SDK**: Official SDK for all SPIRE communication
- ✅ **Workload API**: Standard SPIRE protocol over Unix socket

**Characteristics**:
- ✅ Full cryptographic verification using go-spiffe SDK
- ✅ Real SPIRE server/agent communication via Workload API
- ✅ Bundle management and trust domain handling
- ✅ X.509 SVID support with automatic rotation
- ✅ Streaming identity updates

**Implemented Components**:

1. **Agent** (`agent.go`):
   - Wraps `workloadapi.X509Source` from go-spiffe SDK
   - Fetches SVIDs directly from SPIRE Workload API
   - Watches for identity updates and automatic rotation
   - Converts SDK types to domain types at adapter boundary

2. **Parsers** (`parsers.go`):
   - `TrustDomainParser`: Converts strings to domain.TrustDomain
   - `IdentityCredentialParser`: Parses SPIFFE IDs using spiffeid package
   - Validates format according to SPIFFE specification

3. **Translation** (`translation.go`):
   - `TranslateSPIFFEIDToIdentityCredential`: spiffeid.ID → domain.IdentityCredential
   - `TranslateTrustDomainToSPIFFEID`: domain.TrustDomain → spiffeid.TrustDomain
   - `TranslateX509ContextToIdentity`: X509Context → dto.Identity

**Usage Example**:
```go
// Create SPIRE client (uses go-spiffe SDK)
source, err := workloadapi.NewX509Source(
    ctx,
    workloadapi.WithClientOptions(workloadapi.WithAddr("unix:///run/spire/sockets/agent.sock")),
)

// Create agent adapter
agent := spire.NewAgent(source, parsers)

// Fetch identity (delegates to SPIRE)
identity, err := agent.GetIdentity(ctx)
```

**Integration Flow**:
```
Application → Agent Adapter → go-spiffe SDK → SPIRE Agent → SPIRE Server
```

---

## Dependency Injection

**Composition Root**: `internal/adapters/outbound/compose/`

**Pattern**: Factory interfaces for creating dependencies

```go
type SPIREDeps struct {
    socketPath string
}

func (d *SPIREDeps) CreateAgent(ctx context.Context, config *config.Config) (ports.Agent, error) {
    client, err := workloadapi.New(ctx, workloadapi.WithAddr(d.socketPath))
    if err != nil {
        return nil, fmt.Errorf("failed to create SPIRE client: %w", err)
    }
    parser := spire.NewIdentityCredentialParser()
    return spire.NewAgent(ctx, client, config.AgentSpiffeID, parser)
}

// ... other factory methods
```

**Usage**:

```go
// In main.go
config := config.Load()
deps := compose.NewSPIREDeps(config.SocketPath)

app, err := app.Bootstrap(ctx, config, deps)
if err != nil {
    log.Fatalf("Bootstrap failed: %v", err)
}
```

**Benefits**:
- Single place to configure SPIRE dependencies (Open/Closed Principle)
- Centralized dependency creation and lifecycle management
- No domain changes when updating adapter implementations

---

## Error Handling

**Strategy**: Typed sentinel errors in domain layer

**Error Types** (`internal/domain/errors.go`):
```go
// Identity credential errors
var ErrInvalidIdentityCredential = errors.New("identity credential is invalid")
var ErrInvalidTrustDomain = errors.New("trust domain is invalid")

// Identity document errors
var ErrIdentityDocumentExpired = errors.New("identity document is expired or not yet valid")
var ErrIdentityDocumentInvalid = errors.New("identity document is invalid")
var ErrCertificateChainInvalid = errors.New("certificate chain validation failed")

// Workload errors
var ErrWorkloadNotFound = errors.New("workload not found")
var ErrInvalidWorkload = errors.New("workload information is invalid")

// ... additional domain errors
```

**Usage**:
```go
// Adapter wraps with context
doc, err := agent.GetIdentity(ctx)
if err != nil {
    return nil, fmt.Errorf("failed to get agent identity: %w", err)
}

// Caller checks with errors.Is()
if errors.Is(err, domain.ErrIdentityDocumentExpired) {
    // Handle expired document
}
```

**Benefits**:
- Type-safe error checking (`errors.Is`)
- Consistent error vocabulary across layers
- Adapter implementation flexibility (can add context)
- Contract enforcement (adapters must return exact types)

---

## Security Model

### 1. Workload Attestation

**Mechanism**: Extract process credentials from Unix socket connection

```go
// Server side (SO_PEERCRED)
ucred, _ := syscall.GetsockoptUcred(fd, syscall.SOL_SOCKET, syscall.SO_PEERCRED)
workload := domain.NewWorkload(
    int(ucred.Pid),
    int(ucred.Uid),
    int(ucred.Gid),
    "", // path extracted separately if needed
)
```

**Security Properties**:
- Kernel-verified credentials (cannot be spoofed by workload process)
- Per-connection isolation via Unix socket file descriptor
- SPIRE Agent performs attestation based on process credentials
- Workload identity determined by SPIRE registration entries

---

### 2. SVID Issuance

**Flow** (handled by SPIRE infrastructure):
1. SPIRE Agent attests workload based on process credentials
2. SPIRE Server matches workload to registration entries
3. SPIRE Server generates X.509 certificate with SPIFFE ID in URI SAN
4. SPIRE Server signs certificate with CA private key
5. SPIRE Agent delivers SVID to workload via Workload API

**Properties**:
- Only registered workloads get SVIDs
- SPIFFE ID uniquely identifies workload
- Short-lived certificates (TTL: 1 hour default)
- Automatic rotation (SDK feature)

---

### 3. Trust Model

**Trust Domain**: Root of trust (e.g., `example.org`)

**Trust Bundle**: Root CA certificates for verification

**Verification**:
```go
// With go-spiffe SDK (production)
fullChain := append([]*x509.Certificate{cert}, intermediates...)
bundleSource, _ := x509bundle.Parse(trustDomain, bundlePEM)
spiffeID, chains, err := x509svid.Verify(fullChain, bundleSource)
```

**Federation**: Multiple trust domains can establish trust via bundle exchange

---

### 4. mTLS (Future)

**Client Authentication**:
```go
tlsConfig := &tls.Config{
    Certificates: []tls.Certificate{svid.TLSCertificate()},
    RootCAs:      bundlePool,
    VerifyPeerCertificate: verifySPIFFEID,
}
```

**Server Authentication**: Verify client SPIFFE ID matches expected namespace

---

## Testing Strategy

### Unit Tests

Mock ports, test domain logic

```go
type MockX509Fetcher struct {
    doc *domain.IdentityDocument
    err error
}

func (m *MockX509Fetcher) FetchX509SVID(ctx context.Context) (*domain.IdentityDocument, error) {
    return m.doc, m.err
}

func TestAgentRefreshExpiredDocument(t *testing.T) {
    mockFetcher := &MockX509Fetcher{doc: freshDoc}
    agent := spire.NewAgent(ctx, mockFetcher, agentSpiffeID, parser)

    doc, err := agent.GetIdentity(ctx)

    assert.NoError(t, err)
    assert.False(t, doc.IsExpired())
}
```

---

### Integration Tests

Test with real SPIRE infrastructure in containerized environment

```go
func TestWorkloadIdentityFetch(t *testing.T) {
    // Requires SPIRE Agent + Server running (e.g., via Docker Compose)
    config := config.Load()
    agent := spire.NewAgent(ctx, client, config.AgentSpiffeID, parser)

    // Fetch identity via SPIRE Workload API
    doc, err := agent.GetIdentity(ctx)

    require.NoError(t, err)
    assert.False(t, doc.IsExpired())
    assert.Equal(t, config.TrustDomain, doc.IdentityCredential().TrustDomain())
}
```

---

### Contract Tests (Future)

**Purpose**: Verify port implementations obey contracts

```go
func TestAgentContract_GetIdentity(t *testing.T) {
    agent := spire.NewAgent(ctx, client, agentSpiffeID, parser)

    // Test contract: returns non-expired document
    doc, err := agent.GetIdentity(ctx)
    require.NoError(t, err)
    assert.False(t, doc.IsExpired())

    // Test contract: refreshes when expired
    // (implementation-specific test)
}
```

---

## Design Decisions

### 1. Immutable Registry

**Decision**: Registry is seeded at bootstrap, sealed before runtime

**Rationale**:
- Registration is **configuration**, not runtime behavior
- Prevents runtime mutation bugs
- Clear separation: seeding (bootstrap) vs. lookup (runtime)
- Matches SPIRE's registration entry model

**Alternative Rejected**: Runtime registration API
- ❌ Introduces state management complexity
- ❌ Requires synchronization (locks, transactions)
- ❌ Security: who can register?

---

### 2. SDK-Agnostic Ports

**Decision**: Ports use domain types (`*domain.TrustDomain`), not SDK types (`spiffeid.TrustDomain`)

**Rationale**:
- Hexagonal architecture: domain owns types
- Flexibility: can swap SDK versions or implementations
- Testability: no SDK mocking required

**Adapter Responsibility**: Convert between domain and SDK types

```go
func (p *SDKTrustDomainParser) FromString(ctx context.Context, name string) (*domain.TrustDomain, error) {
    sdkTD, err := spiffeid.TrustDomainFromString(name)  // SDK type
    if err != nil {
        return nil, fmt.Errorf("%w: %v", domain.ErrInvalidTrustDomain, err)
    }
    return domain.NewTrustDomain(sdkTD.String())  // Convert to domain type
}
```

---

### 3. PEM-Encoded Trust Bundles

**Decision**: `TrustBundleProvider.GetBundle` returns `[]byte` (PEM), not parsed `*x509bundle.Bundle`

**Rationale**:
- Port remains SDK-agnostic
- Adapter can return SDK bundle or raw PEM
- Consumer (validator) handles parsing

**Trade-off**: Slight parsing overhead, but cleaner abstraction

---

## References

- [Hexagonal Architecture](https://alistair.cockburn.us/hexagonal-architecture/)
- [SPIFFE Specification](https://github.com/spiffe/spiffe)
- [SPIRE Documentation](https://spiffe.io/docs/latest/spire/)
- [go-spiffe SDK](https://github.com/spiffe/go-spiffe)
- [Port Contracts](docs/PORT_CONTRACTS.md)
