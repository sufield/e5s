# Control Plane: Registration as Seeded Data

## Overview

In this hexagonal, in-memory SPIRE implementation, **registration is NOT a runtime operation**. There is no "Register workload" API or mutation endpoint. Instead:

- **Registration = Seeded fixtures** loaded at startup
- **Runtime path = Attest → Match → Issue**
- **No mutable control plane** - fixtures are read-only after bootstrap

This aligns with hexagonal architecture: configuration is infrastructure, not behavior.

## Architecture Principles

### ❌ What We DON'T Have

- No registration API endpoints
- No CLI for workload registration
- No runtime mutations of the registry
- No public `Register()` method in application services

### ✅ What We DO Have

- **Immutable registry** seeded at startup from fixtures
- **Matching logic** that resolves selectors → identity mappings
- **Issuance flow** that attests → matches → mints certificates
- **Composition root seeding** in `Bootstrap()` function

## Current Implementation Analysis

### Seeding Flow (Composition Root)

**Location**: `internal/app/application.go` - `Bootstrap()` function

```go
// Step 1: Load fixtures from configuration
config, err := configLoader.Load(ctx)

// Step 2-7: Initialize ports and adapters...

// Step 8: SEED the registry (not "register" at runtime)
for _, workload := range config.Workloads {
    identityNamespace, _ := parser.ParseFromString(ctx, workload.SpiffeID)
    selector, _ := domain.ParseSelectorFromString(workload.Selector)

    // This is SEEDING, happens once at startup
    store.Register(ctx, identityNamespace, selector)
}
```

- ✅ Seeding happens in composition root (`Bootstrap()`)
- ✅ Data loaded from configuration fixtures (`config.Workloads`)
- ✅ No runtime mutation - this runs once during app initialization
- ⚠️ Method named `Register()` but acts as `Seed()` - consider renaming

### Registry Adapters

#### IdentityStore (Current Implementation)

**Port**: `internal/app/ports/outbound.go`

```go
type IdentityStore interface {
    Register(ctx context.Context, identityNamespace *domain.IdentityNamespace, selector *domain.Selector) error
    GetIdentity(ctx context.Context, identityNamespace *domain.IdentityNamespace) (*Identity, error)
    ListIdentities(ctx context.Context) ([]*Identity, error)
}
```

**Issues**:
- ⚠️ Named `Register()` suggests runtime mutation
- ⚠️ Should be `Seed()` or internal-only method
- ✅ Only called during bootstrap, not exposed to services

**In-Memory Adapter**: `internal/adapters/outbound/inmemory/store.go`

```go
type InMemoryStore struct {
    identities map[string]*Identity  // identityNamespace.String() → Identity
    selectors  map[string]string     // selector.String() → identityNamespace.String()
}
```

- ✅ Pure in-memory map
- ✅ No persistence
- ✅ Thread-safe with mutex
- ⚠️ Allows mutations (should be write-once at startup)

#### IdentityMapperRepository (Better Design)

**Port**: `internal/app/ports/outbound.go`

```go
type IdentityMapperRepository interface {
    CreateMapper(ctx context.Context, mapper *domain.IdentityMapper) error
    FindMatchingMapper(ctx context.Context, selectors *domain.SelectorSet) (*domain.IdentityMapper, error)
    ListMappers(ctx context.Context) ([]*domain.IdentityMapper, error)
    DeleteMapper(ctx context.Context, identityNamespace *domain.IdentityNamespace) error
}
```

**Analysis**:
- ⚠️ `CreateMapper()` and `DeleteMapper()` suggest runtime mutations
- ✅ `FindMatchingMapper()` is the core runtime operation
- ⚠️ Not currently used - `IdentityStore` is used instead

### Runtime Flow (Read-Only)

**Agent.FetchIdentityDocument()** - The only runtime path:

```
1. Workload calls agent.FetchIdentityDocument(processInfo)
2. Attestor computes selectors from process attributes
3. Store.GetIdentityBySelector(selector) → lookup in registry
4. Server.IssueIdentity(identityNamespace) → mint certificate
5. Return identity document to workload
```

**Key Points**:
- ✅ Pure read path - no mutations
- ✅ Selectors → IdentityNamespace mapping from seeded data
- ✅ Certificate minting is ephemeral (in-memory CA)
- ✅ No state changes to registry

## Recommended Improvements

### 1. Clarify Port Naming

**Current** (suggests mutation):
```go
type IdentityStore interface {
    Register(ctx context.Context, ...) error  // ❌ Misleading name
}
```

**Recommended** (emphasizes seeding):
```go
// RegistryPort - Read-only registry seeded at startup
type RegistryPort interface {
    // FindBySelectors matches selectors to an identity mapping
    FindBySelectors(ctx context.Context, selectors *domain.SelectorSet) (*domain.IdentityMapper, error)

    // ListAll returns all seeded mappings (for debugging)
    ListAll(ctx context.Context) ([]*domain.IdentityMapper, error)
}

// Internal seeding interface (not exposed as port)
type registrySeeder interface {
    seed(ctx context.Context, mapper *domain.IdentityMapper) error
}
```

### 2. Separate Seeding from Runtime

**Composition Root** (`application.go`):

```go
func Bootstrap(ctx context.Context, fixtures []WorkloadEntry, deps ApplicationDeps) (*Application, error) {
    // Initialize registry adapter
    registry := deps.CreateRegistry()

    // SEED registry from fixtures (private method)
    for _, fixture := range fixtures {
        identityNamespace, _ := parser.ParseFromString(ctx, fixture.SpiffeID)
        selectors, _ := parseSelectors(fixture.Selectors)
        mapper := domain.NewIdentityMapper(identityNamespace, selectors)

        // Internal seeding - NOT a port method
        registry.(*InMemoryRegistry).seed(ctx, mapper)
    }

    // Create agent with read-only registry
    agent := deps.CreateAgent(ctx, registry, ...)

    return &Application{Agent: agent, ...}, nil
}
```

- ✅ Fixtures passed directly to `Bootstrap()` (not via port)
- ✅ Seeding uses private method `.seed()` on concrete type
- ✅ Ports only expose read operations
- ✅ No mutation paths in domain or ports

### 3. Registry Adapter Design

**In-Memory Adapter**:

```go
type InMemoryRegistry struct {
    mappers map[string]*domain.IdentityMapper  // identityNamespace → mapper
    selectorIndex map[string]string            // selector → identityNamespace
    sealed bool                                // Prevent modifications after seeding
}

// seed is INTERNAL - not part of RegistryPort
func (r *InMemoryRegistry) seed(ctx context.Context, mapper *domain.IdentityMapper) error {
    if r.sealed {
        return fmt.Errorf("registry is sealed, cannot seed after bootstrap")
    }

    r.mappers[mapper.IdentityNamespace().String()] = mapper
    for _, sel := range mapper.Selectors().All() {
        r.selectorIndex[sel.String()] = mapper.IdentityNamespace().String()
    }
    return nil
}

// Seal marks registry as immutable after seeding
func (r *InMemoryRegistry) Seal() {
    r.sealed = true
}

// FindBySelectors implements RegistryPort (read-only)
func (r *InMemoryRegistry) FindBySelectors(ctx context.Context, selectors *domain.SelectorSet) (*domain.IdentityMapper, error) {
    // Match ALL selectors against mappers (AND logic)
    for _, mapper := range r.mappers {
        if mapper.MatchesSelectors(selectors) {
            return mapper, nil
        }
    }
    return nil, domain.ErrNoMatchingEntry
}
```

**Benefits**:
- ✅ Clear separation: seeding vs runtime
- ✅ Immutable after `Seal()` called
- ✅ Port only exposes read operations
- ✅ Type-safe seeding (private method on concrete type)

### 4. Fixture Loading Options

#### Option 1: Code Fixtures (Current)

```go
// internal/adapters/outbound/inmemory/config.go
func NewInMemoryConfig() *InMemoryConfig {
    return &InMemoryConfig{
        config: &ports.Config{
            TrustDomain: "example.org",
            Workloads: []ports.WorkloadEntry{
                {SpiffeID: "spiffe://example.org/server", Selector: "unix:uid:1001", UID: 1001},
                {SpiffeID: "spiffe://example.org/client", Selector: "unix:uid:1002", UID: 1002},
            },
        },
    }
}
```

**Pros**: ✅ Fastest, ✅ Type-safe, ✅ No parsing
**Cons**: ❌ Requires recompile to change

#### Option 2: YAML Fixtures (Immutable)

```yaml
# fixtures/workloads.yaml
trust_domain: example.org
workloads:
  - spiffe_id: spiffe://example.org/server
    selectors:
      - unix:uid:1001
      - env:ROLE=api
    uid: 1001
  - spiffe_id: spiffe://example.org/client
    selectors:
      - unix:uid:1002
    uid: 1002
```

```go
// Load once at boot, fail fast if invalid
func LoadFixtures(path string) ([]WorkloadEntry, error) {
    data, err := os.ReadFile(path)
    if err != nil {
        return nil, fmt.Errorf("failed to read fixtures: %w", err)
    }

    var config struct {
        TrustDomain string `yaml:"trust_domain"`
        Workloads []WorkloadEntry `yaml:"workloads"`
    }

    if err := yaml.Unmarshal(data, &config); err != nil {
        return nil, fmt.Errorf("invalid fixture format: %w", err)
    }

    return config.Workloads, nil
}
```

**Pros**: ✅ External config, ✅ Still immutable at runtime
**Cons**: ⚠️ Adds YAML dependency

#### Recommendation: Code Fixtures for Walking Skeleton

For in-memory demo mode:
- Use **code fixtures** (`NewInMemoryConfig()`)
- Keep it simple, no external files
- Easy to understand and modify
- Type-safe at compile time

For real deployment:
- YAML fixtures loaded once at boot
- Fail fast on parse errors
- Still immutable at runtime

## Attestation & Matching Policy

### Current Attestation (Unix UID)

**Adapter**: `internal/adapters/outbound/inmemory/attestor/unix.go`

```go
type UnixWorkloadAttestor struct {
    uidSelectors map[int]string  // UID → selector
}

func (a *UnixWorkloadAttestor) Attest(ctx context.Context, workload ports.ProcessIdentity) ([]string, error) {
    selector := a.uidSelectors[workload.UID]
    return []string{
        selector,                          // e.g., "unix:user:server-workload"
        fmt.Sprintf("unix:uid:%d", workload.UID),
        fmt.Sprintf("unix:gid:%d", workload.GID),
    }, nil
}
```

**Selectors Used**:
- ✅ `unix:user:<username>` (mapped from UID)
- ✅ `unix:uid:<uid>` (process UID)
- ✅ `unix:gid:<gid>` (process GID)

**Documentation**: Should explicitly document which selectors are used for demo attestation.

### Matching Policy (AND Logic)

**Current Implementation**: `domain.IdentityMapper.MatchesSelectors()`

```go
func (im *IdentityMapper) MatchesSelectors(selectors *domain.SelectorSet) bool {
    // ALL mapper selectors must be present in discovered selectors
    for _, mapperSelector := range im.selectors.All() {
        if !selectors.Contains(mapperSelector) {
            return false
        }
    }
    return true
}
```

**Policy**: **AND logic** - workload must have ALL required selectors

**Example**:
```
Mapper requires: [unix:uid:1001, k8s:ns:default]
Workload has:    [unix:uid:1001, k8s:ns:default, k8s:pod:api]
Result:          ✅ MATCH (workload has all required selectors + extras OK)

Mapper requires: [unix:uid:1001, env:prod]
Workload has:    [unix:uid:1001]
Result:          ❌ NO MATCH (missing env:prod selector)
```

**Future Extensions** (not needed for walking skeleton):
- ANY logic: workload matches if ANY selector matches
- Weighted priority: choose best match when multiple entries match
- Deny lists: explicit exclusions

## Identity Document Lifecycle

### Issuance (Ephemeral CA)

**Provider**: `internal/adapters/outbound/inmemory/identity_document_provider.go`

```go
func (p *InMemoryIdentityDocumentProvider) CreateX509IdentityDocument(
    ctx context.Context,
    identityNamespace *domain.IdentityNamespace,
    caCert, caKey interface{},
) (*domain.IdentityDocument, error) {
    // 1. Generate workload key pair
    // 2. Create certificate template with SPIFFE ID in URI SAN
    // 3. Sign with in-memory CA
    // 4. Return domain.IdentityDocument with expiration
}
```

**Certificate Properties**:
- ✅ RSA 2048-bit keys
- ✅ 24-hour validity (configurable TTL)
- ✅ SPIFFE ID in URI SAN field
- ✅ Signed by in-memory root CA

### Rotation & Caching

**Current**: No explicit rotation or caching

**Recommended** (future):
- Cache identity document in agent memory
- Re-issue when 80% of TTL reached
- Refresh on validation failure
- For demo: restart to get new certificates (sufficient)

### Revocation

**Current**: No revocation mechanism

**For Demo Mode**:
- ✅ Restart with new fixtures (sufficient for walking skeleton)
- ✅ Identity documents expire after 24h
- ❌ No CRL or OCSP (not needed for in-memory demo)

**For Real SPIRE**:
- Use SDK's bundle management for revocation
- CRL/OCSP checking via `x509svid.Verify()`

## Complete Flow Example

### Startup (Seeding)

```go
// main.go (composition root)
func main() {
    ctx := context.Background()

    // 1. Load fixtures (immutable data)
    fixtures := []WorkloadEntry{
        {SpiffeID: "spiffe://example.org/server", Selector: "unix:uid:1001", UID: 1001},
        {SpiffeID: "spiffe://example.org/client", Selector: "unix:uid:1002", UID: 1002},
    }

    // 2. Bootstrap with fixtures (seeding happens here)
    app, _ := Bootstrap(ctx, fixtures, deps)

    // 3. Seal registry (prevent further mutations)
    deps.SealRegistry()

    // 4. Run application
    cli.New(app).Run(ctx)
}
```

### Runtime (Read-Only)

```go
// Workload requests identity
workload := ports.ProcessIdentity{UID: 1001, PID: 12345, GID: 1001, Path: "/usr/bin/server"}

// 1. Attest workload → selectors
selectors, _ := attestor.Attest(ctx, workload)
// Returns: ["unix:user:server-workload", "unix:uid:1001", "unix:gid:1001"]

// 2. Match selectors → identity mapping (READ-ONLY)
selectorSet := domain.NewSelectorSet()
for _, sel := range selectors {
    selectorSet.Add(sel)
}
mapper, _ := registry.FindBySelectors(ctx, selectorSet)
// Returns: IdentityMapper{identityNamespace: "spiffe://example.org/server", ...}

// 3. Issue identity document
doc, _ := server.IssueIdentity(ctx, mapper.IdentityNamespace())
// Returns: IdentityDocument{identityNamespace, certificate, privateKey, expiresAt}

// 4. Return to workload
return &Identity{IdentityNamespace: mapper.IdentityNamespace(), IdentityDocument: doc}
```

- ✅ No mutations - only reads from seeded registry
- ✅ Selectors computed from process attributes
- ✅ Matching uses AND logic (all required selectors must match)
- ✅ Certificate minting is ephemeral (no state)

## Summary

### What This Implementation Does Right

✅ **Seeding in composition root** - `Bootstrap()` loads fixtures at startup
✅ **Read-only runtime path** - Attest → Match → Issue
✅ **No mutable APIs** - No public registration endpoints
✅ **Hexagonal architecture** - Fixtures are configuration, not behavior
✅ **Domain purity** - Business logic separate from infrastructure

### Recommended Refinements

1. **Rename `Register()` → `Seed()`** in ports (or make internal-only)
2. **Separate seeding interface** from runtime registry port
3. **Seal registry** after bootstrap to prevent mutations
4. **Document attestation selectors** used in demo mode
5. **Keep code fixtures** for walking skeleton (simplest)

> **Registration is NOT a behavior - it's configuration data.**
>
> Seed it once at startup. Make it immutable. Let the runtime resolve it.

This maintains hexagonal architecture purity while providing a practical, testable identity system.
