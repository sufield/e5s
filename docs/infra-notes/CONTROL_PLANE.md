# Control Plane: Registration as Seeded Data

**Build Tag**: `//go:build dev` - This document describes **development-only** implementation.

In **production mode** with external SPIRE, the control plane (registry, attestation, matching) is managed by SPIRE Server infrastructure.

## Overview

Registration is NOT a runtime operation. There is no "Register workload" API or mutation endpoint. Instead:

- **Registration = Seeded data** loaded at startup from configuration
- **Runtime path = Attest → Match → Issue** (read-only operations)
- **No mutable control plane** - registry is sealed after bootstrap

This aligns with hexagonal architecture: configuration is infrastructure, not behavior.

---

## Table of Contents

1. [Control Plane Components](#control-plane-components)
2. [Port Interfaces](#port-interfaces)
3. [Bootstrap Flow](#bootstrap-flow)
4. [Runtime Flow](#runtime-flow-read-only)
5. [Architecture Diagram](#control-plane-architecture-diagram)
6. [Design Rationale](#design-rationale)

---

## Control Plane Components

All control plane code is under `//go:build dev` and excluded from production builds.

### 1. Server (Identity Issuance)

**Location**: `internal/adapters/outbound/inmemory/server.go`
**Build Tag**: `//go:build dev`

**Responsibilities**:
- CA certificate generation and management (deterministic for testing)
- Identity document (X.509 SVID) issuance via `IssueIdentity()`
- Trust domain enforcement (credential must match server trust domain)
- Root of trust (CA certificate) provider

**Port Interface**: `ports.IdentityServer` (`internal/ports/outbound_dev.go:62-73`)

**Methods**:
```go
// IssueIdentity issues an X.509 identity document for an identity credential
func (s *InMemoryServer) IssueIdentity(ctx context.Context, identityCredential *domain.IdentityCredential) (*domain.IdentityDocument, error)

// GetTrustDomain returns the trust domain this server manages
func (s *InMemoryServer) GetTrustDomain() *domain.TrustDomain

// GetCACertPEM returns the CA certificate as PEM bytes (root of trust)
func (s *InMemoryServer) GetCACertPEM() []byte
```

**Implementation Details** (`server.go:25-31`):
```go
type InMemoryServer struct {
    trustDomain         *domain.TrustDomain
    caCert              *x509.Certificate
    ca               *rsa.Private
    certificateProvider ports.IdentityDocumentProvider
}
```

**Behavior**:
- Trust domain enforcement: `server.go:66-68` validates credential trust domain matches server
- CA initialization check: `server.go:71-73` ensures CA materials exist before issuance
- Deterministic CA generation: Uses fixed time (2099-01-01) and seeded RNG for testing

---

### 2. Registry (Workload Registration Storage)

**Location**: `internal/adapters/outbound/inmemory/registry.go`
**Build Tag**: `//go:build dev`

**Responsibilities**:
- Stores identity mapper registrations (selector → SPIFFE ID mappings)
- Immutable after seeding - sealed during bootstrap
- Read-only runtime queries via `FindBySelectors()`
- **No concurrency control** - sequential access only in dev mode

**Port Interface**: `ports.IdentityMapperRegistry` (`internal/ports/outbound_dev.go:26-34`)

** Methods**:
```go
// Seed adds an identity mapper (INTERNAL - used only during bootstrap)
// Not part of port interface - called only by factory during composition
func (r *InMemoryRegistry) Seed(ctx context.Context, mapper *domain.IdentityMapper) error

// Seal marks the registry as immutable after bootstrap
func (r *InMemoryRegistry) Seal()

// FindBySelectors finds an identity mapper matching selectors (READ-ONLY port method)
// Uses AND logic: ALL mapper selectors must be present in discovered selectors
func (r *InMemoryRegistry) FindBySelectors(ctx context.Context, selectors *domain.SelectorSet) (*domain.IdentityMapper, error)

// ListAll returns all seeded identity mappers (READ-ONLY port method, for debugging/admin)
func (r *InMemoryRegistry) ListAll(ctx context.Context) ([]*domain.IdentityMapper, error)
```

**Implementation Details** (`registry.go:19-22`):
```go
type InMemoryRegistry struct {
    mappers map[string]*domain.IdentityMapper // identityCredential.String() → IdentityMapper
    sealed  bool                              // Prevents modifications after bootstrap
}
// NOTE: No sync.RWMutex - "No concurrency support needed - all access is sequential in test/dev mode"
```

** Behavior**:
- Seeding validation: `registry.go:37-38` prevents seeding after seal
- Deterministic ordering: `registry.go:82-87` sorts s for stable iteration
- Sealed enforcement: Once `Seal()` called, `Seed()` returns `domain.ErrRegistrySealed`

**Design Note**: No `Register()` or mutation methods exposed via port interface - seeding happens internally during bootstrap only.

---

### 3. Bootstrap (Seeding Orchestration)

**Location**: `internal/app/bootstrap_dev.go`
**Build Tag**: `//go:build dev`

**Responsibilities**:
- Loads workload registrations from configuration fixtures (`config.Workloads`)
- Creates pre-seeded registry via factory (seeding happens in factory, not bootstrap)
- Wires all control plane components (server, agent, registry, services)
- Guards against infinite waits with default 10-second timeout

** Function** (`bootstrap_dev.go:23-83`):
```go
func Bootstrap(ctx context.Context, configLoader ports.ConfigLoader, factory *compose.InMemoryAdapterFactory) (*Application, error) {
    // Step 1: Load configuration (fixtures)
    cfg, err := configLoader.Load(ctx)
    if err != nil {
        return nil, fmt.Errorf("load config: %w", err)
    }

    // Step 2: Initialize parsers and providers
    tdParser := factory.CreateTrustDomainParser()
    idParser := factory.CreateIdentityCredentialParser()
    docProvider := factory.CreateIdentityDocumentProvider()

    // Step 3: Create server (with CA generation)
    server, err := factory.CreateServer(ctx, cfg.TrustDomain, tdParser, docProvider)
    if err != nil {
        return nil, fmt.Errorf("create server: %w", err)
    }

    // Step 4: Create registry - ALREADY SEEDED by factory
    // Factory handles: parse workloads → create mappers → seed registry → return sealed
    registry, err := factory.CreateRegistry(ctx, cfg.Workloads, idParser)
    if err != nil {
        return nil, fmt.Errorf("create registry: %w", err)
    }

    // Step 5: Create attestor (configured with workload UIDs)
    attestor := factory.CreateAttestor(cfg.Workloads)

    // Step 6: Create agent (uses registry + server)
    agent, err := factory.CreateAgent(ctx, cfg.AgentSpiffeID, server, registry, attestor, idParser, docProvider)
    if err != nil {
        return nil, fmt.Errorf("create agent: %w", err)
    }

    // Step 7: Initialize services
    identitySvc, err := NewIdentityClientService(agent)
    if err != nil {
        return nil, fmt.Errorf("create identity client service: %w", err)
    }
    service := NewIdentityService(agent, registry)

    // Step 8: Wire application with constructor validation
    return New(cfg, service, identitySvc, agent, registry)
}
```

** Differences from Old Documentation**:
- **NO manual seeding loop** in bootstrap - seeding happens inside `factory.CreateRegistry()`
- **NO separate `SealRegistry()` call** - registry returned already sealed from factory
- Registry creation is ONE call: `factory.CreateRegistry(ctx, cfg.Workloads, idParser)`
- Timeout guard: `bootstrap_dev.go:33-36` adds default 10s timeout if none provided

---

### 4. Configuration Loader (Registration Data Source)

**Location**: `internal/adapters/outbound/inmemory/config.go`
**Build Tag**: `//go:build dev`

**Responsibilities**:
- Loads workload registration fixtures (hardcoded, not from database)
- Provides defensive copies to prevent mutation
- Validates trust domain consistency across all workloads
- Returns `dto.Config` with `Workloads` slice containing UID → SPIFFE ID mappings

**Port Interface**: `ports.ConfigLoader` (`internal/ports/outbound_dev.go:12-15`)

**Example Data** (`config.go:24-44`):
```go
cfg := &dto.Config{
    TrustDomain:   "example.org",
    AgentSpiffeID: "spiffe://example.org/host",
    Workloads: []dto.WorkloadEntry{
        {
            SpiffeID: "spiffe://example.org/server-workload",
            Selector: "unix:uid:1001",
            UID:      1001,
        },
        {
            SpiffeID: "spiffe://example.org/client-workload",
            Selector: "unix:uid:1002",
            UID:      1002,
        },
        {
            SpiffeID: "spiffe://example.org/test-workload",
            Selector: "unix:uid:1000",
            UID:      1000,
        },
    },
}
```

**Validation** (`config.go:72-102`):
- Trust domain format validation
- Agent SPIFFE ID must be in trust domain
- All workload SPIFFE IDs must be in trust domain
- Selector format must match `unix:uid:<UID>` pattern

---

### 5. Adapter Factory (Component Creation)

**Location**: `internal/adapters/outbound/compose/inmemory.go`
**Build Tag**: `//go:build dev`

**Responsibilities**:
- Creates all control plane components (registry, server, agent)
- **Handles seeding internally** - registry returned already seeded and sealed
- Type-safe construction with concrete types (no interface{}))
- Provides dev-only implementations of all ports

** Method - CreateRegistry** (`inmemory.go:28-56`):
```go
func (f *InMemoryAdapterFactory) CreateRegistry(
    ctx context.Context,
    workloads []dto.WorkloadEntry,
    parser ports.IdentityCredentialParser,
) (*inmemory.InMemoryRegistry, error) {
    registry := inmemory.NewInMemoryRegistry()

    // Seed registry with workload configurations
    for _, workload := range workloads {
        identityCredential, err := parser.ParseFromString(ctx, workload.SpiffeID)
        if err != nil {
            return nil, err
        }

        selector, err := domain.ParseSelectorFromString(workload.Selector)
        if err != nil {
            return nil, err
        }

        selectorSet := domain.NewSelectorSet()
        selectorSet.Add(selector)

        mapper, err := domain.NewIdentityMapper(identityCredential, selectorSet)
        if err != nil {
            return nil, err
        }

        // SEED happens here, inside factory
        if err := registry.Seed(ctx, mapper); err != nil {
            return nil, err
        }
    }

    // Registry returned already seeded (NOT sealed yet in current impl)
    return registry, nil
}
```

**Other Factory Methods**:
- `CreateServer()` - Creates InMemoryServer with CA generation
- `CreateAgent()` - Creates InMemoryAgent with concrete types
- `CreateAttestor()` - Creates UnixWorkloadAttestor with UID registrations
- `CreateTrustDomainParser()` - Creates parser for trust domain validation
- `CreateIdentityCredentialParser()` - Creates parser for SPIFFE ID validation
- `CreateIdentityDocumentProvider()` - Creates X.509 certificate generator

**Design Note**: Factory uses **concrete types** (`*inmemory.InMemoryRegistry`) not interfaces, enabling access to internal `Seed()` method during composition.

---

### 6. Application (Composition Root)

**Location**: `internal/app/application_dev.go`
**Build Tag**: `//go:build dev`

**Responsibilities**:
- Holds all wired components (agent, registry, services, config)
- Provides accessor methods for components
- Implements `Close()` for resource cleanup

**Structure** (`application_dev.go:14-20`):
```go
type Application struct {
    cfg   *dto.Config
    svc   ports.Service             // optional demo service
    ics   ports.IdentityIssuer      // server-side issuance facade
    agent ports.Agent
    reg   ports.IdentityMapperRegistry
}
```

---

## Port Interfaces

All dev-only port interfaces are in `internal/ports/outbound_dev.go` with `//go:build dev` tag.

### IdentityMapperRegistry Port

**Location**: `internal/ports/outbound_dev.go:26-34`

```go
// IdentityMapperRegistry provides read-only access to the identity mapper registry seeded at startup.
// Dev-only: in production, SPIRE Server manages registration entries.
//
// Error Contract:
// - FindBySelectors: domain.ErrNoMatchingMapper if no mapper matches
// - FindBySelectors: domain.ErrInvalidSelectors if selectors are nil/empty
// - ListAll:         domain.ErrRegistryEmpty if no mappers seeded
//
// Implementations should respect ctx cancellation where applicable.
type IdentityMapperRegistry interface {
    // FindBySelectors finds an identity mapper matching the given selectors.
    // AND semantics: all mapper selectors must be present in the discovered selectors.
    // Returns the first matching mapper (deterministic order depends on registry seeding).
    FindBySelectors(ctx context.Context, selectors *domain.SelectorSet) (*domain.IdentityMapper, error)

    // ListAll returns all seeded identity mappers (for debugging/admin).
    ListAll(ctx context.Context) ([]*domain.IdentityMapper, error)
}
```

**Design Rationale**:
- No mutation methods in port interface (read-only after bootstrap)
- Name emphasizes domain concept (mapper registry) over architectural pattern
- Error contract documented inline with port definition
- Seeding happens via internal methods, not exposed through port

---

### IdentityServer Port

**Location**: `internal/ports/outbound_dev.go:62-73`

```go
// IdentityServer represents identity server functionality for in-memory/dev mode only.
// Dev-only: in production, SPIRE Server runs as external infrastructure.
//
// Error Contract:
// - IssueIdentity: domain.ErrIdentityDocumentInvalid if identity credential invalid
// - IssueIdentity: domain.ErrServerUnavailable if server unavailable
// - IssueIdentity: domain.ErrCANotInitialized if CA not initialized
// - GetTrustDomain: never returns error (returns nil if not initialized)
// - GetCACertPEM:  returns empty slice if CA not initialized
//
// Implementations should respect ctx cancellation where applicable.
type IdentityServer interface {
    // IssueIdentity issues an identity document for an identity credential.
    // Generates X.509 certificate signed by CA with identity credential in URI SAN.
    IssueIdentity(ctx context.Context, identityCredential *domain.IdentityCredential) (*domain.IdentityDocument, error)

    // GetTrustDomain returns the trust domain this server manages.
    GetTrustDomain() *domain.TrustDomain

    // GetCACertPEM returns the CA certificate as PEM bytes (root of trust).
    // Returns empty slice if CA not initialized - caller must check.
    GetCACertPEM() []byte
}
```

---

### ConfigLoader Port

**Location**: `internal/ports/outbound_dev.go:12-15`

```go
// ConfigLoader loads runtime configuration (dev-only).
type ConfigLoader interface {
    Load(ctx context.Context) (*dto.Config, error)
}
```

---

### WorkloadAttestor Port

**Location**: `internal/ports/outbound_dev.go:45-49`

```go
// WorkloadAttestor verifies workload identity based on platform-specific attributes.
// Dev-only: in production, SPIRE Agent performs attestation automatically.
//
// Error Contract:
// - domain.ErrWorkloadAttestationFailed if attestation fails
// - domain.ErrInvalidProcessIdentity   if workload info is invalid
// - domain.ErrNoAttestationData        if no selectors can be generated
//
// Implementations should respect ctx cancellation where applicable.
type WorkloadAttestor interface {
    // Attest verifies a workload and returns its selectors.
    // Selectors must be formatted as "type:key:value" (e.g., "unix:uid:1000", "k8s:namespace:prod").
    Attest(ctx context.Context, workload *domain.Workload) ([]string, error)
}
```

---

## Bootstrap Flow

**Seeding happens entirely inside the factory, not in bootstrap code.**

```
1. Load Config
   ↓
2. Create Parsers & Providers
   ↓
3. Create Server (generates CA)
   ↓
4. Create Registry ← SEEDING HAPPENS HERE (inside factory.CreateRegistry)
   │ Factory logic:
   │ - Create empty registry
   │ - Loop over config.Workloads
   │ - Parse each workload's SPIFFE ID and selectors
   │ - Create IdentityMapper domain entity
   │ - Call registry.Seed(mapper)
   │ - Return seeded registry
   ↓
5. Create Attestor (registers UIDs)
   ↓
6. Create Agent (uses registry + server)
   ↓
7. Create Services (uses agent + registry)
   ↓
8. Wire Application
```

** Characteristics**:
- Seeding is **encapsulated in factory** - bootstrap doesn't touch registry internals
- Registry returned from `CreateRegistry()` is **already seeded** (but not sealed in current impl)
- **No separate seal step** visible in bootstrap (could be added in factory if needed)
- **Single responsibility**: Bootstrap wires components, factory handles component internals

---

## Runtime Flow (Read-Only)

**Agent.FetchIdentityDocument()** - The only runtime path for workload SVID issuance

**Location**: `internal/adapters/outbound/inmemory/agent.go:107-145`

```go
func (a *InMemoryAgent) FetchIdentityDocument(ctx context.Context, workload *domain.Workload) (*domain.IdentityDocument, error) {
    // Step 1: Attest the workload to get selectors
    selectorStrings, err := a.attestor.Attest(ctx, workload)
    if err != nil {
        return nil, fmt.Errorf("inmemory: workload attestation failed: %w", err)
    }
    if len(selectorStrings) == 0 {
        return nil, domain.ErrNoAttestationData
    }

    // Step 2: Convert selector strings to SelectorSet
    selectorSet := domain.NewSelectorSet()
    for _, selStr := range selectorStrings {
        selector, err := domain.ParseSelectorFromString(selStr)
        if err != nil {
            return nil, fmt.Errorf("inmemory: invalid selector %q: %w", selStr, err)
        }
        selectorSet.Add(selector)
    }

    // Step 3: Match selectors against registry (READ-ONLY operation)
    mapper, err := a.registry.FindBySelectors(ctx, selectorSet)
    if err != nil {
        return nil, fmt.Errorf("inmemory: no identity mapper found for selectors: %w", err)
    }

    // Step 4: Issue identity document from server
    doc, err := a.server.IssueIdentity(ctx, mapper.IdentityCredential())
    if err != nil {
        return nil, fmt.Errorf("inmemory: failed to issue identity document: %w", err)
    }

    // Step 5: Return identity document
    return doc, nil
}
```

**Flow Summary**:
```
Workload (PID) → Agent.FetchIdentityDocument(workload)
                   ↓
                 Attestor.Attest(workload) → ["unix:uid:1001"]
                   ↓
                 Registry.FindBySelectors(selectors) → IdentityMapper
                   ↓
                 Server.IssueIdentity(identityCredential) → X.509 SVID
                   ↓
                 Return IdentityDocument
```

**Characteristics**:
- Pure read path - no mutations to registry
- Selectors → IdentityCredential mapping from seeded data
- Certificate minting is ephemeral (new cert each time)
- No state changes to registry (sealed and immutable)

---

## Control Plane Architecture Diagram

```
┌────────────────────────────────────────────────────────┐
│         Bootstrap (Composition Root)                    │
│         internal/app/bootstrap_dev.go                   │
│  ┌────────────────────────────────────────────────┐   │
│  │ 1. Load Config (fixtures)                       │   │
│  │ 2. Create Server (CA generation)                │   │
│  │ 3. Create Registry ← SEEDING INSIDE FACTORY     │   │
│  │    Factory.CreateRegistry():                    │   │
│  │    - Parse workloads                            │   │
│  │    - Create mappers                             │   │
│  │    - Seed registry                              │   │
│  │    - Return seeded registry                     │   │
│  │ 4. Create Agent (wires registry + server)       │   │
│  │ 5. Create Services                              │   │
│  │ 6. Wire Application                             │   │
│  └────────────────────────────────────────────────┘   │
└────────────────────────────────────────────────────────┘
                       │
          ┌────────────┴────────────┐
          │                         │
          ▼                         ▼
┌──────────────────┐      ┌──────────────────┐
│   Server         │      │   Registry       │
│   (CA + Issue)   │      │   (Registrations)│
│                  │      │                  │
│ • IssueIdentity()│      │ • FindBySelectors│
│ • GetCACertPEM() │      │ • ListAll()      │
│ • GetTrustDomain │      │   [SEEDED]       │
└──────────────────┘      └──────────────────┘
         │                         │
         └────────┬────────────────┘
                  │
                  ▼
         Runtime (Data Plane)
         Agent.FetchIdentityDocument()
         ↓
         Attest → Match → Issue → Return
```

---

## Design Rationale

### 1. Seeding is Configuration, Not Behavior

Loading identity mappers from fixtures is an **infrastructure concern**, not domain behavior. This is why:
- Seeding happens during composition (factory), not application logic
- No seeding methods exposed through port interfaces
- Registry consumed as read-only collection at runtime

### 2. Immutability After Bootstrap

Registry is immutable after seeding (can be sealed) which:
- Prevents accidental mutations during runtime
- Makes concurrent access safe without locks (sequential in dev anyway)
- Aligns with "configuration as code" - loaded once, never changed

### 3. Clear Separation of Concerns

- **Bootstrap**: Wires components together
- **Factory**: Handles component creation and internal setup (seeding)
- **Registry**: Provides read-only access via port interface
- **Seeding**: Internal operation, not exposed to application layer

### 4. Domain-Focused Naming

- `IdentityMapperRegistry` signals intent (collection of mappers)
- Avoids architectural jargon ("Port" suffix)
- Self-descriptive - developers understand purpose immediately

### 5. No Dead Code

- Single registry implementation (`InMemoryRegistry`)
- No unused interfaces or backward-compatible mutation paths
- Deprecated `IdentityStore` and `IdentityMapperRepository` removed

---

## Directory Structure

```
internal/
├── adapters/outbound/inmemory/
│   ├── server.go              ← Server (CA + SVID issuance)
│   ├── registry.go            ← Registry (workload registrations)
│   ├── config.go              ← Config loader (fixture data)
│   ├── agent.go               ← Agent (uses registry + server)
│   ├── identity_document_provider.go  ← Certificate generation
│   └── attestor/
│       └── unix.go            ← Unix UID-based attestor
│
├── adapters/outbound/compose/
│   └── inmemory.go            ← Factory (component creation + seeding)
│
├── app/
│   ├── bootstrap_dev.go       ← Bootstrap (composition root)
│   ├── application_dev.go     ← Application struct
│   └── service_dev.go         ← Identity service
│
└── ports/
    ├── outbound.go            ← Production port interfaces
    └── outbound_dev.go        ← Dev-only port interfaces

All files above have //go:build dev tag
```

---

## What is NOT Control Plane

These are **data plane** (runtime) components:

- `internal/adapters/outbound/inmemory/agent.go` - Workload attestation + SVID fetching (runtime)
- `internal/adapters/inbound/cli/cli.go` - Presentation layer (demo CLI)
- `internal/adapters/inbound/identityserver/` - mTLS HTTP server (uses production SPIRE)

**Note**: There are NO `workloadapi` inbound/outbound adapters - only `identityserver` for mTLS.

---

## What We DON'T Have

- No registration API endpoints (no REST/gRPC for registering workloads)
- No CLI for workload registration (fixtures only)
- No runtime mutations of the registry (sealed after bootstrap)
- No public `Register()` method in application services
- No deprecated `IdentityStore` or `IdentityMapperRepository` interfaces

---

## What We DO Have

- Immutable registry seeded at startup from fixtures
- Matching logic that resolves selectors → identity credential mappings
- Issuance flow that attests → matches → mints certificates
- Encapsulated seeding in factory (composition root delegates to factory)
- Good port naming - `IdentityMapperRegistry` (not "Port" suffix)
- Clear dev-only marking with `//go:build dev` tags

---

## Summary Table

| Component | Location | Build Tag | Role | Mutable? |
|-----------|----------|-----------|------|----------|
| **Server** | `internal/adapters/outbound/inmemory/server.go` | `//go:build dev` | CA + SVID issuance | No (stateless) |
| **Registry** | `internal/adapters/outbound/inmemory/registry.go` | `//go:build dev` | Workload registrations | **No (sealed)** |
| **Bootstrap** | `internal/app/bootstrap_dev.go` | `//go:build dev` | Composition root | N/A (runs once) |
| **Config Loader** | `internal/adapters/outbound/inmemory/config.go` | `//go:build dev` | Fixture data source | No (read-only) |
| **Factory** | `internal/adapters/outbound/compose/inmemory.go` | `//go:build dev` | Component creation + seeding | N/A (bootstrap) |
| **Application** | `internal/app/application_dev.go` | `//go:build dev` | Wired components holder | No (immutable after construction) |

---

## Cross-References

- **Port Contracts**: See `docs/PORT_CONTRACTS.md` for all port interface definitions
- **Domain Model**: See `docs/DOMAIN.md` for IdentityMapper and other domain entities
- **Production mTLS**: See `examples/zeroconfig-example/` for production SPIRE Workload API usage
- **Invariants**: See `docs/INVARIANTS.md` for domain invariants and error handling

---

1. **Seeding happens in factory** - not visible in bootstrap code
2. **Registry is read-only** after construction - no mutation methods in port
3. **All code is dev-only** - excluded from production via `//go:build dev`
4. **Bootstrap orchestrates** - factory encapsulates component internals
5. **Runtime is pure reads** - Attest → Match (FindBySelectors) → Issue → Return

[Needs to be revised]