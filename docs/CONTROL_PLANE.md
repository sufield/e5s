# Control Plane: Registration as Seeded Data

In this hexagonal, in-memory SPIRE implementation, registration is NOT a runtime operation. There is no "Register workload" API or mutation endpoint. Instead:

- **Registration = Seeded fixtures** loaded at startup
- **Runtime path = Attest → Match → Issue**
- **No mutable control plane** - fixtures are read-only after bootstrap

This aligns with hexagonal architecture: configuration is infrastructure, not behavior.

---

## Control Plane Components and Directories

This implementation does NOT have a traditional mutable control plane. Instead, it uses **"registration as seeded data"** - an immutable approach where workload registrations are loaded once at startup and sealed.

---

### ore Control Plane Components

#### 1. **Server (Identity Issuance)**
**Location**: `internal/adapters/outbound/inmemory/server.go`

**Responsibilities**:
- CA certificate generation and management
- Identity document (X.509 SVID) issuance via `IssueIdentity()`
- Trust domain management
- Root of trust (CA certificate) provider

**Methods**:
- `IssueIdentity(ctx, identityNamespace)` - Issues SVIDs for attested workloads
- `GetTrustDomain()` - Returns trust domain
- `GetCA()` - Returns CA certificate

**Port Interface**: `ports.IdentityServer` (defined in `internal/ports/outbound.go:54-73`)

**Implementation**:
```go
// InMemoryServer is an in-memory implementation of SPIRE server
type InMemoryServer struct {
    trustDomain          *domain.TrustDomain
    caCert               *x509.Certificate
    caKey                *rsa.PrivateKey
    certificateProvider  ports.IdentityDocumentProvider
}

// IssueIdentity issues an X.509 identity document for an identity namespace
// No verification of registration - that's done by the agent during attestation/matching
func (s *InMemoryServer) IssueIdentity(ctx context.Context, identityNamespace *domain.IdentityNamespace) (*domain.IdentityDocument, error)
```

---

#### 2. **Registry (Workload Registration Storage)**
**Location**: `internal/adapters/outbound/inmemory/registry.go`

**Responsibilities**:
- Stores identity mapper registrations (selector → SPIFFE ID mappings)
- Immutable after seeding - sealed at startup
- Read-only runtime queries via `FindBySelectors()`

**Methods**:
- `Seed(ctx, mapper)` - Internal only, called during bootstrap
- `Seal()` - Makes registry immutable
- `FindBySelectors(ctx, selectors)` - Runtime lookup (read-only)
- `ListAll(ctx)` - Returns all registrations (admin/debug)

**Port Interface**: `ports.IdentityMapperRegistry` (defined in `internal/ports/outbound.go:15-31`)

**Implementation**:
```go
type InMemoryRegistry struct {
    mu      sync.RWMutex
    mappers map[string]*domain.IdentityMapper
    sealed  bool
}

// Seed adds an identity mapper (INTERNAL - used only during bootstrap)
// Do not call from application services - it's infrastructure/configuration only
func (r *InMemoryRegistry) Seed(ctx context.Context, mapper *domain.IdentityMapper) error {
    r.mu.Lock()
    defer r.mu.Unlock()

    if r.sealed {
        return fmt.Errorf("registry is sealed, cannot seed after bootstrap")
    }

    key := mapper.IdentityNamespace().String()
    r.mappers[key] = mapper
    return nil
}

// Seal marks the registry as immutable after seeding
func (r *InMemoryRegistry) Seal() {
    r.mu.Lock()
    defer r.mu.Unlock()
    r.sealed = true
}

// FindBySelectors finds an identity mapper matching the given selectors (implements port)
func (r *InMemoryRegistry) FindBySelectors(ctx context.Context, selectors *domain.SelectorSet) (*domain.IdentityMapper, error) {
    r.mu.RLock()
    defer r.mu.RUnlock()

    for _, mapper := range r.mappers {
        if mapper.MatchesSelectors(selectors) {
            return mapper, nil
        }
    }
    return nil, domain.ErrNoMatchingMapper
}
```

**Design Note**: No `Register()` or mutation methods exposed via port - seeding happens internally during bootstrap.

---

#### 3. **Bootstrap/Composition Root (Seeding Logic)**
**Location**: `internal/app/application.go` - `Bootstrap()` function

**Responsibilities**:
- Loads workload registrations from configuration fixtures (`config.Workloads`)
- Seeds the registry with identity mappers (selector → SPIFFE ID)
- Seals the registry to prevent mutations
- Wires all control plane components (server, agent, registry)

**Steps**:
```go
func Bootstrap(ctx context.Context, configLoader ports.ConfigLoader, factory ports.AdapterFactory) (*Application, error) {
    // Step 1: Load configuration (fixtures)
    config, err := configLoader.Load(ctx)
    if err != nil {
        return nil, fmt.Errorf("failed to load config: %w", err)
    }

    // Step 2: Initialize registry (will be seeded and sealed)
    registry := factory.CreateRegistry()

    // Steps 3-8: Initialize other adapters (parser, server, attestor, etc.)...

    // Step 9: SEED registry with identity mappers (configuration, not runtime)
    for _, workload := range config.Workloads {
        // Parse identity namespace from fixture
        identityNamespace, err := parser.ParseFromString(ctx, workload.SpiffeID)
        if err != nil {
            return nil, fmt.Errorf("invalid identity namespace %s: %w", workload.SpiffeID, err)
        }

        // Parse selectors from fixture
        selector, err := domain.ParseSelectorFromString(workload.Selector)
        if err != nil {
            return nil, fmt.Errorf("invalid selector %s: %w", workload.Selector, err)
        }

        // Create selector set for mapper
        selectorSet := domain.NewSelectorSet()
        selectorSet.Add(selector)

        // Create identity mapper (domain entity)
        mapper, err := domain.NewIdentityMapper(identityNamespace, selectorSet)
        if err != nil {
            return nil, fmt.Errorf("failed to create identity mapper for %s: %w", workload.SpiffeID, err)
        }

        // SEED registry (internal method, not exposed via port)
        if err := factory.SeedRegistry(registry, ctx, mapper); err != nil {
            return nil, fmt.Errorf("failed to seed registry for %s: %w", workload.SpiffeID, err)
        }
    }

    // Step 10: SEAL registry (prevent further mutations after seeding)
    factory.SealRegistry(registry)

    // Step 11: Initialize agent with sealed registry
    agent, err := factory.CreateAgent(ctx, config.AgentSpiffeID, server, registry, attestor, parser, docProvider)
    if err != nil {
        return nil, fmt.Errorf("failed to create SPIRE agent: %w", err)
    }

    // Step 12: Initialize core service
    service := NewIdentityService(agent, registry)

    return &Application{
        Config:   config,
        Service:  service,
        Agent:    agent,
        Registry: registry,
    }, nil
}
```

This is the only place where workload registrations are loaded/seeded.

**Seeding Characteristics**:
- ✅ Seeding happens in composition root (`Bootstrap()`)
- ✅ Data loaded from configuration fixtures (`config.Workloads`)
- ✅ No runtime mutation - this runs once during app initialization
- ✅ Registry sealed after seeding - immutable from that point forward
- ✅ Seeding methods accessed via `AdapterFactory` interface (composition pattern)

---

#### 4. **Configuration Loader (Registration Data Source)**
**Location**: `internal/adapters/outbound/inmemory/config.go`

**Responsibilities**:
- Loads workload registration **fixtures** (not from a database)
- Provides `Config.Workloads` slice with UID → SPIFFE ID mappings
- Read-only data source

**Port Interface**: `ports.ConfigLoader` (defined in `internal/ports/outbound.go:10-13`)

**Example Data**:
```go
Workloads: []WorkloadConfig{
    {SpiffeID: "spiffe://example.org/server-workload", UID: 1001, Selector: "unix:uid:1001"},
    {SpiffeID: "spiffe://example.org/client-workload", UID: 1002, Selector: "unix:uid:1002"},
    {SpiffeID: "spiffe://example.org/test-workload", UID: 1000, Selector: "unix:uid:1000"},
}
```

---

#### 5. **Adapter Factory (Seeding Operations)**
**Location**: `internal/adapters/outbound/compose/inmemory.go`

**Responsibilities**:
- **Creates** control plane components (registry, server)
- **Provides seeding methods** `SeedRegistry()` and `SealRegistry()`
- Type-asserts to concrete types to call internal methods

**Methods**:
```go
type InMemoryAdapterFactory struct{}

func (f *InMemoryAdapterFactory) CreateRegistry() ports.IdentityMapperRegistry {
    return inmemory.NewInMemoryRegistry()
}

func (f *InMemoryAdapterFactory) CreateServer(ctx context.Context, trustDomain string, trustDomainParser ports.TrustDomainParser, docProvider ports.IdentityDocumentProvider) (ports.IdentityServer, error) {
    return inmemory.NewInMemoryServer(ctx, trustDomain, trustDomainParser, docProvider)
}

// SeedRegistry seeds the registry with an identity mapper (configuration, not runtime)
// This is called only during bootstrap - uses Seed() method on concrete type
func (f *InMemoryAdapterFactory) SeedRegistry(registry ports.IdentityMapperRegistry, ctx context.Context, mapper *domain.IdentityMapper) error {
    concreteRegistry, ok := registry.(*inmemory.InMemoryRegistry)
    if !ok {
        return fmt.Errorf("expected InMemoryRegistry for seeding")
    }
    return concreteRegistry.Seed(ctx, mapper)
}

// SealRegistry marks the registry as immutable after seeding
// This prevents any further mutations - registry becomes read-only
func (f *InMemoryAdapterFactory) SealRegistry(registry ports.IdentityMapperRegistry) {
    concreteRegistry, ok := registry.(*inmemory.InMemoryRegistry)
    if ok {
        concreteRegistry.Seal()
    }
}
```

**Port Interface**: `ports.AdapterFactory` (defined in `internal/ports/outbound.go:197-212`)

- ✅ Type assertion to concrete type for seeding operations
- ✅ Seeding methods NOT part of port interface
- ✅ Clear documentation: "configuration, not runtime"
- ✅ Composition root controls when to seal

---

### Directory Structure

```
internal/
├── adapters/outbound/inmemory/
│   ├── server.go              ← Server (CA + SVID issuance)
│   ├── registry.go            ← Registry (workload registrations)
│   ├── config.go              ← Config loader (fixture data)
│   ├── agent.go               ← Agent (uses registry + server)
│   └── identity_document_provider.go  ← Certificate generation
│
├── adapters/outbound/compose/
│   └── inmemory.go            ← Factory (seeding orchestration)
│
├── app/
│   └── application.go         ← Bootstrap (seeding happens here)
│
└── ports/
    └── outbound.go            ← Port interfaces (Server, Registry, Factory)
```

---

### 🔍 What is NOT Control Plane

These are data plane (runtime) components:

- ❌ `internal/adapters/outbound/inmemory/agent.go` - **Data plane** (workload attestation + SVID fetching)
- ❌ `internal/adapters/inbound/workloadapi/server.go` - **Data plane** (Workload API server)
- ❌ `internal/adapters/outbound/workloadapi/client.go` - **Data plane** (Workload API client)
- ❌ `internal/adapters/outbound/inmemory/attestor/` - **Data plane** (workload attestation)
- ❌ `internal/adapters/inbound/cli/cli.go` - **Presentation layer** (demo CLI)

---

## Architecture Principles

### ❌ What We DON'T Have

- No registration API endpoints
- No CLI for workload registration
- No runtime mutations of the registry
- No public `Register()` method in application services
- No deprecated `IdentityStore` or `IdentityMapperRepository` interfaces

### ✅ What We DO Have

- Immutable registry seeded at startup from fixtures and sealed
- Matching logic that resolves selectors → identity namespace mappings
- Issuance flow that attests → matches → mints certificates
- Composition root seeding** in `Bootstrap()` function
- Good port naming - `IdentityMapperRegistry` (not "Port" suffix)

---

## Port Interfaces

### IdentityMapperRegistry Port

**Location**: `internal/ports/outbound.go`

```go
// IdentityMapperRegistry provides read-only access to the identity mapper registry seeded at startup
// This is the runtime interface - seeding happens via internal methods during bootstrap
// No mutations allowed after seeding - registry is immutable
type IdentityMapperRegistry interface {
    // FindBySelectors finds an identity mapper matching the given selectors (AND logic)
    // This is the core runtime operation: selectors → identity namespace mapping
    FindBySelectors(ctx context.Context, selectors *domain.SelectorSet) (*domain.IdentityMapper, error)

    // ListAll returns all seeded identity mappers (for debugging/admin)
    ListAll(ctx context.Context) ([]*domain.IdentityMapper, error)
}
```

**Design Rationale**:
- ✅ No mutation methods in port interface
- ✅ Name emphasizes domain concept (mapper registry) over architectural pattern
- ✅ Self-descriptive - signals seeded/immutable collection
- ✅ Core operation `FindBySelectors()` reads naturally

---

### Server Port

**Location**: `internal/ports/outbound.go`

```go
// Server represents the identity server functionality
type IdentityServer interface {
    // IssueIdentity issues an identity document for an identity namespace
    // Generates X.509 certificate signed by CA with identity namespace in URI SAN
    IssueIdentity(ctx context.Context, identityNamespace *domain.IdentityNamespace) (*domain.IdentityDocument, error)

    // GetTrustDomain returns the trust domain this server manages
    GetTrustDomain() *domain.TrustDomain

    // GetCA returns the CA certificate (root of trust)
    // Returns nil if CA not initialized - caller must check
    GetCA() *x509.Certificate
}
```

---

## Runtime Flow (Read-Only)

**Agent.FetchIdentityDocument()** - The only runtime path:

**Location**: `internal/adapters/outbound/inmemory/agent.go`

```go
func (a *InMemoryAgent) FetchIdentityDocument(ctx context.Context, workload ports.ProcessIdentity) (*ports.Identity, error) {
    // Step 1: Attest the workload to get selectors
    selectorStrings, err := a.attestor.Attest(ctx, workload)
    if err != nil {
        return nil, fmt.Errorf("workload attestation failed: %w", err)
    }

    // Step 2: Convert selector strings to SelectorSet
    selectorSet := domain.NewSelectorSet()
    for _, selStr := range selectorStrings {
        selector, err := domain.ParseSelectorFromString(selStr)
        if err != nil {
            return nil, fmt.Errorf("invalid selector %s: %w", selStr, err)
        }
        selectorSet.Add(selector)
    }

    // Step 3: Match selectors against registry (READ-ONLY operation)
    mapper, err := a.registry.FindBySelectors(ctx, selectorSet)
    if err != nil {
        return nil, fmt.Errorf("no identity mapper found for selectors: %w", err)
    }

    // Step 4: Issue identity document from server
    doc, err := a.server.IssueIdentity(ctx, mapper.IdentityNamespace())
    if err != nil {
        return nil, fmt.Errorf("failed to issue identity document: %w", err)
    }

    // Step 5: Return identity with document
    return &ports.Identity{
        IdentityNamespace:   mapper.IdentityNamespace(),
        Name:             extractNameFromIdentityNamespace(mapper.IdentityNamespace()),
        IdentityDocument: doc,
    }, nil
}
```

**Flow Summary**:
```
1. Workload calls agent.FetchIdentityDocument(processInfo)
2. Attestor computes selectors from process attributes
3. Registry.FindBySelectors(selectors) → lookup in immutable registry
4. Server.IssueIdentity(identityNamespace) → mint certificate
5. Return identity document to workload
```

- ✅ Pure read path - no mutations
- ✅ Selectors → IdentityNamespace mapping from seeded data
- ✅ Certificate minting is ephemeral (in-memory CA)
- ✅ No state changes to registry
- ✅ Registry sealed - guaranteed immutable

---

## Control Plane Architecture Diagram

```
┌──────────────────────────────────────────────────────┐
│         Bootstrap (Composition Root)                  │
│         internal/app/application.go                   │
│  ┌──────────────────────────────────────────────┐   │
│  │ 1. Load Config (fixtures)                     │   │
│  │ 2. Create Registry                            │   │
│  │ 3. SEED Registry (loop over Workloads)       │   │
│  │ 4. SEAL Registry (immutable)                  │   │
│  │ 5. Initialize Server (CA generation)          │   │
│  └──────────────────────────────────────────────┘   │
└──────────────────────────────────────────────────────┘
                       │
          ┌────────────┴────────────┐
          │                         │
          ▼                         ▼
┌──────────────────┐      ┌──────────────────┐
│   Server         │      │   Registry       │
│   (CA + Issue)   │      │   (Registrations)│
│                  │      │                  │
│ • IssueIdentity()│      │ • FindBySelectors│
│ • GetCA()        │      │ • ListAll()      │
│ • GetTrustDomain │      │   [SEALED]       │
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

## Design Summary

### Why This Design Works

1. **Seeding is Configuration**: Loading identity mappers from fixtures is infrastructure concern, not domain behavior
2. **Immutability**: Registry sealed after bootstrap prevents accidental mutations
3. **Clear Separation**: Seeding methods are internal, port methods are read-only
4. **Domain-Focused Naming**: `IdentityMapperRegistry` signals intent over pattern
5. **No Dead Code**: Only one registry implementation, no unused interfaces

### What Was Removed

All deprecated code has been deleted:

- ❌ `IdentityStore` interface (old mutable store)
- ❌ `IdentityMapperRepository` interface (unused alternative design)
- ❌ `InMemoryStore` implementation (replaced by `InMemoryRegistry`)
- ❌ Backward-compatible mutation paths

### Current State

✅ **Single Source of Truth**: `IdentityMapperRegistry` port with `InMemoryRegistry` adapter
✅ **Immutable After Bootstrap**: `Seal()` enforces read-only guarantee
✅ **Clean Runtime Path**: Attest → Match (FindBySelectors) → Issue
✅ **No Architectural Jargon**: "Registry" not "Port" in naming
✅ **No Dead Code**: All unused interfaces deleted

---

## Summary

| Component | Location | Role | Mutable? |
|-----------|----------|------|----------|
| **Server** | `internal/adapters/outbound/inmemory/server.go` | CA + SVID issuance | No (stateless CA) |
| **Registry** | `internal/adapters/outbound/inmemory/registry.go` | Workload registrations | **No (sealed)** |
| **Bootstrap** | `internal/app/application.go` | Seeding orchestration | N/A (runs once) |
| **Config Loader** | `internal/adapters/outbound/inmemory/config.go` | Fixture data source | No (read-only) |
| **Factory** | `internal/adapters/outbound/compose/inmemory.go` | Component creation + seeding | N/A (bootstrap) |

---

## Characteristics

1. **No Mutation API**: Registry is sealed after bootstrap - no runtime registration
2. **Configuration-Based**: Workload registrations loaded from fixtures (not database)
3. **Single Seeding Point**: `Bootstrap()` is the only place registrations are loaded
4. **Immutable Runtime**: After seal, registry is read-only (concurrent-safe)
5. **Clean Separation**: Seeding methods are internal, port methods are read-only

All control plane code is in **`internal/adapters/outbound/inmemory/`**, **`internal/adapters/outbound/compose/`**, and **`internal/app/`**.
