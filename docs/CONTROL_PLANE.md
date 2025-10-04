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
- No deprecated `IdentityStore` or `IdentityMapperRepository` interfaces

### ✅ What We DO Have

- **Immutable registry** seeded at startup from fixtures and sealed
- **Matching logic** that resolves selectors → identity namespace mappings
- **Issuance flow** that attests → matches → mints certificates
- **Composition root seeding** in `Bootstrap()` function
- **Clean port naming** - `IdentityMapperRegistry` (not "Port" suffix)

## Current Implementation

### Port Interface

**Location**: `internal/app/ports/outbound.go`

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

### In-Memory Adapter

**Location**: `internal/adapters/outbound/inmemory/registry.go`

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

// ListAll returns all seeded identity mappers (implements port)
func (r *InMemoryRegistry) ListAll(ctx context.Context) ([]*domain.IdentityMapper, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]*domain.IdentityMapper, 0, len(r.mappers))
	for _, mapper := range r.mappers {
		result = append(result, mapper)
	}
	return result, nil
}
```

**Key Features**:
- ✅ `Seed()` method is NOT part of port - internal only
- ✅ `Seal()` enforces immutability after bootstrap
- ✅ Port methods are read-only (safe for concurrent access)
- ✅ Selector matching uses domain logic (`mapper.MatchesSelectors()`)

### Seeding Flow (Composition Root)

**Location**: `internal/app/application.go` - `Bootstrap()` function

```go
func Bootstrap(ctx context.Context, configLoader ports.ConfigLoader, deps ApplicationDeps) (*Application, error) {
	// Step 1: Load configuration (fixtures)
	config, err := configLoader.Load(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}

	// Step 2: Initialize registry (will be seeded and sealed)
	registry := deps.CreateRegistry()

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
		if err := deps.SeedRegistry(registry, ctx, mapper); err != nil {
			return nil, fmt.Errorf("failed to seed registry for %s: %w", workload.SpiffeID, err)
		}
	}

	// Step 10: SEAL registry (prevent further mutations after seeding)
	deps.SealRegistry(registry)

	// Step 11: Initialize agent with sealed registry
	agent, err := deps.CreateAgent(ctx, config.AgentSpiffeID, server, registry, attestor, parser, docProvider)
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

**Seeding Characteristics**:
- ✅ Seeding happens in composition root (`Bootstrap()`)
- ✅ Data loaded from configuration fixtures (`config.Workloads`)
- ✅ No runtime mutation - this runs once during app initialization
- ✅ Registry sealed after seeding - immutable from that point forward
- ✅ Seeding methods accessed via `ApplicationDeps` interface (composition pattern)

### Dependency Injection Pattern

**Location**: `internal/adapters/outbound/compose/inmemory.go`

```go
type InMemoryDeps struct{}

func (d *InMemoryDeps) CreateRegistry() ports.IdentityMapperRegistry {
	return inmemory.NewInMemoryRegistry()
}

// SeedRegistry seeds the registry with an identity mapper (configuration, not runtime)
// This is called only during bootstrap - uses Seed() method on concrete type
func (d *InMemoryDeps) SeedRegistry(registry ports.IdentityMapperRegistry, ctx context.Context, mapper *domain.IdentityMapper) error {
	concreteRegistry, ok := registry.(*inmemory.InMemoryRegistry)
	if !ok {
		return fmt.Errorf("expected InMemoryRegistry for seeding")
	}
	return concreteRegistry.Seed(ctx, mapper)
}

// SealRegistry marks the registry as immutable after seeding
// This prevents any further mutations - registry becomes read-only
func (d *InMemoryDeps) SealRegistry(registry ports.IdentityMapperRegistry) {
	concreteRegistry, ok := registry.(*inmemory.InMemoryRegistry)
	if ok {
		concreteRegistry.Seal()
	}
}
```

**Key Points**:
- ✅ Type assertion to concrete type for seeding operations
- ✅ Seeding methods NOT part of port interface
- ✅ Clear documentation: "configuration, not runtime"
- ✅ Composition root controls when to seal

### Runtime Flow (Read-Only)

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

**Key Points**:
- ✅ Pure read path - no mutations
- ✅ Selectors → IdentityNamespace mapping from seeded data
- ✅ Certificate minting is ephemeral (in-memory CA)
- ✅ No state changes to registry
- ✅ Registry sealed - guaranteed immutable

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
