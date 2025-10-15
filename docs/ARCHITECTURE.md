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

This project implements SPIRE's Workload API using hexagonal (ports and adapters) architecture. The design enables:

1. **Separation of concerns**: Domain logic isolated from infrastructure
2. **Testability**: In-memory implementations for testing without real SPIRE
3. **Flexibility**: Easy swap between in-memory and real SPIRE implementations
4. **SDK readiness**: Interfaces designed to match go-spiffe SDK signatures

**Concepts**:
- **SPIFFE ID**: Unique identity credential (e.g., `spiffe://example.org/workload`)
- **SVID (Identity Document)**: X.509 certificate proving workload identity
- **Workload Attestation**: Process of verifying workload attributes (UID, path, etc.)
- **Selector**: Key-value attribute used for attestation (e.g., `unix:uid:1000`)
- **Trust Bundle**: Root CA certificates for verifying SVIDs

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
│  │  - Selector, SelectorSet                             │       │
│  │  - IdentityMapper (selector → identity mapping)      │       │
│  │                                                       │       │
│  │  Domain Errors (Typed Sentinels)                     │       │
│  │  - ErrNoMatchingMapper                               │       │
│  │  - ErrWorkloadAttestationFailed                      │       │
│  │  - ErrIdentityDocumentExpired                        │       │
│  │  - ... (20+ typed errors)                            │       │
│  └──────────────────────────────────────────────────────┘       │
├──────────────────────────────────────────────────────────────────┤
│                    Outbound Ports                                │
│  ┌──────────────────────────────────────────────────────┐       │
│  │  Server                                               │       │
│  │  - IssueIdentity(namespace) → IdentityDocument       │       │
│  │  - GetTrustDomain() → TrustDomain                    │       │
│  │  - GetCA() → *x509.Certificate                       │       │
│  │                                                       │       │
│  │  Agent                                                │       │
│  │  - GetIdentity() → Identity                          │       │
│  │  - FetchIdentityDocument(workload) → Identity        │       │
│  │                                                       │       │
│  │  IdentityMapperRegistry                              │       │
│  │  - FindBySelectors(selectors) → IdentityMapper      │       │
│  │  - ListAll() → []*IdentityMapper                    │       │
│  │                                                       │       │
│  │  WorkloadAttestor                                    │       │
│  │  - Attest(workload) → []string (selectors)          │       │
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
│  ┌─────────────────┐           ┌──────────────────────┐         │
│  │ In-Memory       │           │ Real SPIRE SDK       │         │
│  │ (Walking        │           │ (Production)         │         │
│  │  Skeleton)      │           │ (Future)             │         │
│  │                 │           │                      │         │
│  │ - Registry      │           │ - SDK Parsers        │         │
│  │ - Server        │           │ - Workload API       │         │
│  │ - Agent         │           │ - Bundle Source      │         │
│  │ - Attestors     │           │ - X.509 SVID         │         │
│  │ - Parsers       │           │ - Chain Verify       │         │
│  └─────────────────┘           └──────────────────────┘         │
│                                                                  │
│  ┌──────────────────────────────────────────────────┐           │
│  │ Workload API Client (used by workloads)          │           │
│  │ - HTTP over Unix socket                          │           │
│  │ - Credential extraction (SO_PEERCRED)            │           │
│  └──────────────────────────────────────────────────┘           │
└──────────────────────────────────────────────────────────────────┘
```

**Principles**:
1. **Dependencies point inward**: Outer layers depend on inner, never reverse
2. **Domain is pure**: No infrastructure dependencies (crypto/x509 used only for types)
3. **Ports define contracts**: Interfaces owned by application layer
4. **Adapters are swappable**: In-memory ↔ Real SPIRE with zero domain changes

---

## Layers

### 1. Domain Layer (`internal/domain/`)

**Purpose**: Pure business logic and entities

**Contents**:
- **Entities**: `TrustDomain`, `IdentityCredential`, `IdentityDocument`, `Selector`, `IdentityMapper`
- **Value Objects**: `SelectorSet` (collection with uniqueness guarantees)
- **Errors**: Typed sentinel errors (`ErrNoMatchingMapper`, `ErrIdentityDocumentExpired`, etc.)

**Rules**:
- ✅ No external dependencies (except stdlib types like `crypto/x509.Certificate`)
- ✅ Immutable entities (no setters, return new instances)
- ✅ Rich domain models (behavior + data)
- ❌ No I/O, no database, no HTTP
- ❌ No SDK dependencies

**Example**:
```go
// Domain entity with behavior
type IdentityMapper struct {
    identityCredential *IdentityCredential
    selectors         *SelectorSet
}

// Business logic method
func (m *IdentityMapper) MatchesSelectors(discovered *SelectorSet) bool {
    // AND logic: all mapper selectors must be present in discovered
    for _, required := range m.selectors.All() {
        if !discovered.Contains(required) {
            return false
        }
    }
    return true
}
```

---

### 2. Port Layer (`internal/ports/`)

**Purpose**: Define contracts between layers (interfaces only)

**Contents**:
- **Inbound Ports**: `IdentityProvider`, `CLI`, `Service` (dev-only)
- **Outbound Ports**: `Agent`, `IdentityMapperRegistry`, `WorkloadAttestor`, `TrustDomainParser`, etc.
- **Configuration**: `MTLSConfig`, `WorkloadAPIConfig`, `SPIFFEConfig`, `HTTPConfig` (mTLS server/client config)

**Data Transfer Objects**: Moved to `internal/dto/`
- `dto.Identity` - Identity transport DTO
- `dto.Config` - Runtime configuration (dev/prod variants)
- `dto.WorkloadEntry` - Workload registration (dev-only)
- `dto.Message` - Demo message (dev-only)

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
// Error Contract:
// - FindBySelectors returns domain.ErrNoMatchingMapper if no mapper matches
// - FindBySelectors returns domain.ErrInvalidSelectors if selectors are nil/empty
type IdentityMapperRegistry interface {
    FindBySelectors(ctx context.Context, selectors *domain.SelectorSet) (*domain.IdentityMapper, error)
    ListAll(ctx context.Context) ([]*domain.IdentityMapper, error)
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

**Structure**:
```
adapters/
├── inbound/          # Handle external requests
│   ├── workloadapi/  # HTTP server over Unix socket
│   └── cli/          # Command-line presentation
└── outbound/         # Implement infrastructure ports
    ├── inmemory/     # In-memory implementations
    ├── spire/        # Real SPIRE SDK implementations (future)
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
// Outbound adapter implementing port
type InMemoryRegistry struct {
    mappers map[string]*domain.IdentityMapper
    sealed  bool
}

func (r *InMemoryRegistry) FindBySelectors(
    ctx context.Context,
    selectors *domain.SelectorSet,
) (*domain.IdentityMapper, error) {
    // Input validation
    if selectors == nil || len(selectors.All()) == 0 {
        return nil, fmt.Errorf("%w: selectors are nil or empty", domain.ErrInvalidSelectors)
    }

    // Infrastructure logic
    for _, mapper := range r.mappers {
        if mapper.MatchesSelectors(selectors) {
            return mapper, nil
        }
    }

    // Map to domain error
    return nil, fmt.Errorf("%w: no mapper matches selectors", domain.ErrNoMatchingMapper)
}
```

---

## Data Flows

### 1. Workload SVID Fetch

#### Development Mode (In-Memory)

**Flow**: Workload → API Server → Service → Agent → Attestor → Registry → Server → Response

```
┌─────────────┐
│  Workload   │  (Process with UID 1000)
└──────┬──────┘
       │ 1. GET /svid/x509
       │    (Unix socket connection)
       ▼
┌────────────────────────────┐
│  Workload API Server       │  (Inbound Adapter)
│  internal/adapters/        │
│  inbound/workloadapi/      │
└──────┬─────────────────────┘
       │ 2. Extract caller credentials
       │    (SO_PEERCRED: UID=1000, PID=12345)
       ▼
┌────────────────────────────┐
│  IdentityClientService     │  (Application Layer)
│  internal/app/             │
└──────┬─────────────────────┘
       │ 3. FetchIdentityDocument(*domain.Workload{UID:1000, PID:12345})
       ▼
┌────────────────────────────┐
│  InMemoryAgent             │  (In-Memory Adapter)
│  internal/adapters/        │
│  outbound/inmemory/        │
└──────┬─────────────────────┘
       │ 4. Attest(workload)
       ▼
┌────────────────────────────┐
│  WorkloadAttestor          │  (Attestor Adapter)
│  Returns: [unix:uid:1000,  │
│            unix:gid:1000]   │
└──────┬─────────────────────┘
       │ 5. FindBySelectors(selectors)
       ▼
┌────────────────────────────┐
│  IdentityMapperRegistry    │  (Registry Adapter)
│  Matches: unix:uid:1000 →  │
│  spiffe://example.org/     │
│  test-workload             │
└──────┬─────────────────────┘
       │ 6. IssueIdentity(namespace)
       ▼
┌────────────────────────────┐
│  InMemoryServer            │  (Server Adapter)
│  Generates X.509 cert with │
│  SPIFFE ID in URI SAN      │
└──────┬─────────────────────┘
       │ 7. Return Identity{SVID, IdentityCredential}
       ▼
┌────────────────────────────┐
│  Workload API Server       │
│  Serializes to JSON        │
└──────┬─────────────────────┘
       │ 8. HTTP 200 OK
       │    {spiffe_id, x509_svid, expires_at}
       ▼
┌─────────────┐
│  Workload   │  Uses SVID for mTLS
└─────────────┘
```

**Notes**:
- Credential extraction happens at adapter boundary (Unix socket)
- Attestation uses platform-specific mechanisms (Unix UID/GID)
- Registry lookup uses AND logic (all selectors must match)
- Server generates cryptographic material
- Response is JSON over HTTP/Unix

#### Production Mode (SPIRE)

**Flow**: Workload → SPIRE Agent → SPIRE Server (external infrastructure)

```
┌─────────────┐
│  Workload   │  (Process with UID 1000)
└──────┬──────┘
       │ 1. Connect to SPIRE Workload API
       │    (Unix socket: /var/run/spire/sockets/agent.sock)
       ▼
┌────────────────────────────┐
│  SPIRE Agent               │  (External Process)
│  - Extracts credentials    │
│  - Attests workload        │
└──────┬─────────────────────┘
       │ 2. Request SVID from SPIRE Server
       │    (with workload selectors)
       ▼
┌────────────────────────────┐
│  SPIRE Server              │  (External Process)
│  - Matches selectors       │
│  - Issues SVID             │
│  - Returns certificate     │
└──────┬─────────────────────┘
       │ 3. Return SVID to workload
       ▼
┌─────────────┐
│  Workload   │  Uses SVID for mTLS
└─────────────┘
```

**Notes**:
- **ALL operations delegated to external SPIRE**
- No local registry, attestation, or selector matching
- SPIRE Agent handles credential extraction automatically
- SPIRE Server manages registration entries and selector matching
- Your application simply calls `client.FetchX509SVID(ctx)` via Workload API

---

### 2. CLI Demo Flow (Development Only)

**Purpose**: Demonstrate hexagonal architecture without HTTP infrastructure

**Important**: The CLI demo uses `InMemoryAgent` which is **NOT** used for HTTP mTLS examples. HTTP services use production `identityserver` adapter connecting to real SPIRE Workload API.

#### CLI Demo

```
┌────────────────┐
│  cmd/main.go   │  (CLI Entry Point - dev only, build tag: dev)
└───────┬────────┘
        │ 1. Bootstrap application with in-memory adapters
        ▼
┌────────────────────────────┐
│  CLI Adapter               │  (Inbound Adapter)
│  internal/adapters/        │  - Presentation layer only
│  inbound/cli/              │  - Formats output
│                            │  - Orchestrates demo flow
└───────┬────────────────────┘
        │ 2. c.application.Agent.FetchIdentityDocument(workload)
        ▼
┌────────────────────────────┐
│  Application Layer         │  (Domain orchestration)
│  internal/app/             │
│  - Application struct      │
│  - Holds Agent port        │
└───────┬────────────────────┘
        │ 3. Calls Agent port (outbound)
        ▼
┌────────────────────────────┐
│  InMemoryAgent             │  (Outbound Adapter)
│  internal/adapters/        │  - Implements ports.Agent interface
│  outbound/inmemory/        │  - Infrastructure layer
│  agent.go                  │  - Talks to InMemoryServer
└───────┬────────────────────┘
        │ 4. Attestation → Registry → Server
        ▼
┌────────────────────────────┐
│  Infrastructure            │
│  - WorkloadAttestor        │
│  - Registry                │
│  - Server (issues SVIDs)   │
└────────────────────────────┘
```

#### Why Agent is in Outbound Adapters

The agent is placed as an outbound adapter because:

1. **Direction of dependency**: CLI (inbound) → Application → Agent (outbound) → Infrastructure
2. **Infrastructure-facing**: Agent talks to SPIRE Server (infrastructure), not to domain logic
3. **Port implementation**: Agent implements `ports.Agent` interface (outbound port)
4. **Hexagonal principle**: Inbound adapters drive the application, outbound adapters are driven by it

**Flow**:
```
CLI (drives application)
  ↓
Application (orchestrates use cases)
  ↓
Agent Port (outbound interface)
  ↓
InMemoryAgent (outbound adapter implementation)
  ↓
InMemoryServer (simulated SPIRE infrastructure)
```

Just because CLI calls it doesn't make it inbound. The agent is called BY the application to interact with infrastructure, making it outbound.

#### CLI Demo vs HTTP mTLS

| Aspect | CLI Demo | HTTP mTLS (Production) |
|--------|----------|----------------------|
| Entry Point | `cmd/main.go` | `examples/identityserver-example/main.go` |
| Inbound Adapter | CLI (console) | identityserver (HTTP) |
| Agent Used | InMemoryAgent | Production SPIRE Workload API |
| Build Tag | `dev` | None (production code) |
| Purpose | Architecture demo | Real mTLS communication |
| Scope | Out of scope for "two services using mTLS" | **In scope** |

**For HTTP mTLS**: Use `examples/identityserver-example/` and `examples/httpclient/` which connect to real SPIRE via Workload API. These do NOT use InMemoryAgent.

---

### 3. Bootstrap Flow

**Flow**: main.go → Bootstrap → Create Dependencies → Wire Ports → Seed Registry → Seal

```
┌────────────────┐
│  main.go       │  IDP_MODE=inmem
└───────┬────────┘
        │ 1. Select dependency factory
        ▼
┌────────────────────────────┐
│  compose.NewInMemoryDeps() │  (Composition Root)
└───────┬────────────────────┘
        │ 2. Create all adapters
        ▼
┌────────────────────────────┐
│  app.Bootstrap()           │
│                            │
│  Step 1: Load Config       │
│  ├─ ConfigLoader port      │
│  └─ Returns: trust domain, │
│     agent ID, workload     │
│     entries                │
└───────┬────────────────────┘
        │
        ▼
┌────────────────────────────┐
│  Step 2: Create Core Ports │
│  ├─ TrustDomainParser      │
│  ├─ IdentityCredentialParser│
│  ├─ IdentityDocumentProvider│
│  ├─ TrustBundleProvider    │
│  └─ WorkloadAttestor       │
└───────┬────────────────────┘
        │
        ▼
┌────────────────────────────┐
│  Step 3: Create Server     │
│  ├─ Generate CA cert       │
│  └─ Initialize trust domain│
└───────┬────────────────────┘
        │
        ▼
┌────────────────────────────┐
│  Step 4: Create Registry   │
│  (Unsealed, mutable)       │
└───────┬────────────────────┘
        │
        ▼
┌────────────────────────────┐
│  Step 5: Seed Registry     │
│  For each workload entry:  │
│  ├─ Parse selectors        │
│  ├─ Parse identity credential│
│  ├─ Create IdentityMapper  │
│  └─ Registry.Seed(mapper)  │
└───────┬────────────────────┘
        │
        ▼
┌────────────────────────────┐
│  Step 6: Seal Registry     │
│  (Now immutable, read-only)│
└───────┬────────────────────┘
        │
        ▼
┌────────────────────────────┐
│  Step 7: Create Agent      │
│  ├─ Initialize agent SVID  │
│  └─ Wire with registry,    │
│     server, attestor       │
└───────┬────────────────────┘
        │
        ▼
┌────────────────────────────┐
│  Step 8: Create Services   │
│  ├─ IdentityClientService  │
│  └─ DemoService (optional) │
└───────┬────────────────────┘
        │
        ▼
┌────────────────────────────┐
│  Return Application        │
│  ├─ Agent                  │
│  ├─ Server                 │
│  ├─ Registry               │
│  ├─ IdentityClientService  │
│  └─ Config                 │
└────────────────────────────┘
```

**Notes**:
- Seeding happens **before** runtime (configuration phase)
- Registry is **sealed** after seeding (immutable)
- Agent gets own SVID during bootstrap
- All dependencies wired at composition root
- No runtime registration API

---

## Port Contracts

All ports have complete error contracts documented in `docs/PORT_CONTRACTS.md` and `internal/ports/outbound.go`.

**Example Contract**:

```go
// IdentityMapperRegistry provides read-only access to identity mappings
//
// Error Contract:
// - FindBySelectors returns domain.ErrNoMatchingMapper if no mapper matches
// - FindBySelectors returns domain.ErrInvalidSelectors if selectors are nil/empty
// - ListAll returns domain.ErrRegistryEmpty if no mappers seeded
type IdentityMapperRegistry interface {
    FindBySelectors(ctx context.Context, selectors *domain.SelectorSet) (*domain.IdentityMapper, error)
    ListAll(ctx context.Context) ([]*domain.IdentityMapper, error)
}
```

**Contract Guarantees**:
1. **Exact error types**: Adapters **must** return documented sentinel errors
2. **Validation**: Ports validate inputs and return typed errors
3. **Idempotency**: Read operations are side-effect free
4. **Immutability**: No mutations after sealing (registry)

---

## Adapter Implementations

### In-Memory Adapters (`internal/adapters/outbound/inmemory/`)

**Purpose**: Walking skeleton for development and testing

**Characteristics**:
- ✅ Pure Go implementation (no external dependencies)
- ✅ Fast, deterministic
- ✅ Complete port coverage
- ✅ Typed domain errors
- ❌ No real cryptographic verification (basic time checks only)
- ❌ No persistence (ephemeral)

**Implementations**:
- `InMemoryRegistry`: Map-based selector matching
- `InMemoryServer`: Self-signed certificate generation
- `InMemoryAgent`: Orchestrates attestation → matching → issuance
- `UnixWorkloadAttestor`: UID-based selector generation
- `InMemoryTrustDomainParser`: Simple string validation
- `InMemoryIdentityCredentialParser`: URI parsing
- `InMemoryTrustBundleProvider`: PEM-encoded multi-CA bundles
- `InMemoryIdentityDocumentProvider`: Certificate generation and basic validation

---

### Real SPIRE Adapters (`internal/adapters/outbound/spire/`)

**Status**: ✅ **Complete** - Production SPIRE adapters fully implemented and tested

**Purpose**: Production integration with go-spiffe SDK that **fully delegates** to external SPIRE infrastructure

**Architecture**: Production mode delegates ALL operations to external SPIRE:
- ❌ **No local registry** - SPIRE Server manages registration entries
- ❌ **No local attestation** - SPIRE Agent performs workload attestation
- ❌ **No local selector matching** - SPIRE Server matches selectors
- ✅ **Full delegation** - Agent fetches SVIDs from SPIRE Workload API

**Characteristics**:
- ✅ Full cryptographic verification using go-spiffe SDK
- ✅ Real SPIRE server/agent communication via Workload API
- ✅ Bundle management and trust domain handling
- ✅ X.509 SVID support
- ✅ Automatic workload attestation through external SPIRE Agent

**Implemented Components**:
- `SPIREClient`: go-spiffe Workload API client wrapper with connection management
- `Agent`: Production agent that **fully delegates** to external SPIRE (no local registry/attestor)
  - Fetches SVIDs directly from SPIRE Workload API
  - SPIRE Agent handles credential extraction and attestation
  - SPIRE Server performs selector matching and SVID issuance
- `Server`: Production server using SPIRE CA certificates and trust bundles
- `Translation`: Domain model conversions using `spiffeid` package
  - `TranslateSPIFFEIDToIdentityCredential`: Converts `spiffeid.ID` to domain types
  - `TranslateTrustDomainToSPIFFEID`: Converts domain TrustDomain to `spiffeid.TrustDomain`
  - `TranslateX509SVIDToIdentityDocument`: Converts `x509svid.SVID` to domain IdentityDocument
- Identity operations using `workloadapi.Client`:
  - `FetchX509SVID`: X.509 SVID fetching (delegates to SPIRE)

**Usage**:
```go
// Create SPIRE client
client, err := spire.NewSPIREClient(ctx, config)

// Create production agent (no registry/attestor needed - fully delegated to SPIRE)
agent, err := spire.NewAgent(ctx, client, agentSpiffeID, parser)

// Create production server
server, err := spire.NewServer(ctx, client, trustDomainStr, trustDomainParser)
```

In production, selector domain logic is not required. SPIRE Server handles all selector matching against its own registration entries. See `docs/PRODUCTION_VS_DEVELOPMENT.md` for architecture comparison.

---

### Workload API Client (`internal/adapters/outbound/workloadapi/`)

**Purpose**: Client library for workloads to fetch SVIDs

**Characteristics**:
- HTTP client over Unix domain socket
- Sends caller credentials (UID/PID/GID) in headers (demo)
- Production: Relies on SO_PEERCRED for automatic credential extraction
- JSON response parsing

**Usage**:
```go
client := workloadapi.NewClient("/tmp/spire-agent/public/api.sock")
svid, err := client.FetchX509SVID(ctx)
```

---

## Dependency Injection

**Composition Root**: `internal/adapters/outbound/compose/`

**Pattern**: Factory interfaces for creating dependencies

```go
type InMemoryDeps struct{}

func (d *InMemoryDeps) CreateRegistry() ports.IdentityMapperRegistry {
    return inmemory.NewInMemoryRegistry()
}

func (d *InMemoryDeps) CreateServer(...) (ports.IdentityServer, error) {
    return inmemory.NewInMemoryServer(...)
}

// ... other factory methods
```

**Switching Implementations**:

```go
// In main.go
var deps compose.Dependencies

switch os.Getenv("IDP_MODE") {
case "inmem":
    deps = compose.NewInMemoryDeps()
case "spire":
    deps = compose.NewSPIREDeps(socketPath)  // Future
}

app, _ := app.Bootstrap(ctx, configLoader, deps)
```

**Benefits**:
- Single place to change implementations (Open/Closed Principle)
- Test with in-memory, deploy with real SPIRE
- No domain changes when swapping adapters

---

## Error Handling

**Strategy**: Typed sentinel errors in domain layer

**Error Types** (`internal/domain/errors.go`):
```go
// Registry errors
var ErrNoMatchingMapper = errors.New("no identity mapper found matching selectors")
var ErrRegistryEmpty = errors.New("registry is empty")
var ErrRegistrySealed = errors.New("registry is sealed, cannot seed after bootstrap")

// Identity document errors
var ErrIdentityDocumentExpired = errors.New("identity document is expired or not yet valid")
var ErrIdentityDocumentInvalid = errors.New("identity document is invalid")
var ErrCertificateChainInvalid = errors.New("certificate chain validation failed")

// Attestation errors
var ErrWorkloadAttestationFailed = errors.New("workload attestation failed")
var ErrNoAttestationData = errors.New("no attestation data available")

// ... 20+ total errors
```

**Usage Pattern**:
```go
// Adapter wraps with context
mapper, err := registry.FindBySelectors(ctx, selectors)
if err != nil {
    return nil, fmt.Errorf("%w: no mapper matches selectors %v", domain.ErrNoMatchingMapper, selectors)
}

// Caller checks with errors.Is()
if errors.Is(err, domain.ErrNoMatchingMapper) {
    // Handle specific case
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

**Selector Generation**:
```go
selectors := []string{
    "unix:uid:1000",
    "unix:gid:1000",
    "unix:path:/usr/bin/workload",
}
```

**Security Properties**:
- Kernel-verified credentials (cannot be spoofed)
- Per-connection isolation
- Defense-in-depth: multiple selectors (AND logic)

---

### 2. SVID Issuance

**Flow**:
1. Attest workload → get selectors
2. Match selectors in registry (AND logic: **all** must match)
3. Generate X.509 certificate with SPIFFE ID in URI SAN
4. Sign with CA private key
5. Return SVID to workload

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

**Pattern**: Mock ports, test domain logic

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
    agent := inmemory.NewInMemoryAgent(..., registry, ...)

    _, err := agent.FetchIdentityDocument(ctx, workload)

    assert.True(t, errors.Is(err, domain.ErrNoMatchingMapper))
}
```

---

### Integration Tests

**Pattern**: Use in-memory implementations for full flow

```go
func TestWorkloadAttestation(t *testing.T) {
    // Bootstrap with in-memory implementations
    app, _ := app.Bootstrap(ctx, inmemory.NewInMemoryConfig(), compose.NewInMemoryDeps())

    // Test full flow
    workload := domain.NewWorkload(123, 1000, 1000, "/usr/bin/app")
    doc, err := app.Agent.FetchIdentityDocument(ctx, workload)

    require.NoError(t, err)
    assert.Equal(t, "spiffe://example.org/test-workload", doc.IdentityCredential().String())
}
```

---

### Contract Tests (Future)

**Purpose**: Verify port implementations obey contracts

```go
func TestRegistryContract_FindBySelectors(t *testing.T) {
    registry := inmemory.NewInMemoryRegistry()

    // Test error contract: nil selectors → ErrInvalidSelectors
    _, err := registry.FindBySelectors(ctx, nil)
    assert.True(t, errors.Is(err, domain.ErrInvalidSelectors))

    // Test error contract: no match → ErrNoMatchingMapper
    selectors := domain.NewSelectorSet(/* ... */)
    _, err = registry.FindBySelectors(ctx, selectors)
    assert.True(t, errors.Is(err, domain.ErrNoMatchingMapper))
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

### 2. AND Logic for Selector Matching

**Decision**: ALL mapper selectors must be present in discovered selectors

**Rationale**:
- Defense-in-depth: multiple attributes required
- Precision: `unix:uid:1000 AND unix:gid:1000` is more specific than `unix:uid:1000 OR unix:gid:1000`
- Matches SPIRE's registration entry semantics

**Example**:
```
Mapper selectors: {unix:uid:1000, unix:gid:1000}
Discovered selectors: {unix:uid:1000, unix:gid:1000, unix:path:/app}
Result: MATCH (all mapper selectors present)

Mapper selectors: {unix:uid:1000, unix:gid:1001}
Discovered selectors: {unix:uid:1000, unix:gid:1000}
Result: NO MATCH (unix:gid:1001 missing)
```

---

### 3. SDK-Agnostic Ports

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

### 4. PEM-Encoded Trust Bundles

**Decision**: `TrustBundleProvider.GetBundle` returns `[]byte` (PEM), not parsed `*x509bundle.Bundle`

**Rationale**:
- Port remains SDK-agnostic
- Adapter can return SDK bundle or raw PEM
- Consumer (validator) handles parsing

**Trade-off**: Slight parsing overhead, but cleaner abstraction

---

## References

- [Hexagonal Architecture](https://alistair.cockburn.us/hexagonal-architecture/)
- [Domain-Driven Design](https://martinfowler.com/bliki/DomainDrivenDesign.html)
- [SPIFFE Specification](https://github.com/spiffe/spiffe)
- [SPIRE Documentation](https://spiffe.io/docs/latest/spire/)
- [go-spiffe SDK](https://github.com/spiffe/go-spiffe)
- [Port Contracts](docs/PORT_CONTRACTS.md)
- [SDK Migration Guide](docs/SDK_MIGRATION.md)
