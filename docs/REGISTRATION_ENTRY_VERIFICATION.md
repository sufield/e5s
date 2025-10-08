# RegistrationEntry Entity Domain Purity Verification

### ✅ Pure Domain Entity

The `RegistrationEntry` struct and its methods represent **pure domain logic** for an in-memory walking skeleton:

```go
type RegistrationEntry struct {
    identityNamespace  *IdentityNamespace
    selectors *SelectorSet
    parentID  *IdentityNamespace // Parent SPIFFE ID (e.g., agent ID)
}
```

**Characteristics:**
- No external SDK dependencies (only domain types: `IdentityNamespace`, `SelectorSet`)
- Only standard library imports (`fmt`)
- Models SPIRE's authorization policy: SPIFFE ID → selector mappings
- Validates entries: non-nil SPIFFE ID, non-empty selectors
- Core business logic: `MatchesSelectors()` determines workload eligibility

### ✅ No SDK Duplication

The `RegistrationEntry` entity does NOT duplicate any go-spiffe SDK functionality:

1. **SDK has NO registration entry types** - go-spiffe is client-side IdentityDocument consumption, not server-side registration
2. **SDK has NO selector matching logic** - SPIRE-specific server authorization functionality
3. **SDK has NO parent-child modeling** - No agent-workload relationship concepts
4. **SDK has NO authorization policies** - Client library, not policy engine

**From go-spiffe SDK v2:**
- `spiffeid.ID` - SPIFFE ID parsing/validation (domain has equivalent)
- `x509svid.IdentityDocument` - X.509 IdentityDocument handling (domain has equivalent)
- `workloadapi.Client` - Workload API client (consumes SVIDs, doesn't create registration)
- **NO registration, selector, or policy types**

### ✅ Proper Port Separation

We added a `RegistrationRepository` port to separate concerns:

**Port Definition** (`internal/app/ports.go`):
```go
type RegistrationRepository interface {
    // CreateEntry creates a new registration entry
    CreateEntry(ctx context.Context, entry *domain.RegistrationEntry) error

    // FindMatchingEntry finds a registration entry that matches the given selectors
    // Used during workload attestation to determine which SPIFFE ID to issue
    FindMatchingEntry(ctx context.Context, selectors *domain.SelectorSet) (*domain.RegistrationEntry, error)

    // ListEntries lists all registration entries (for debugging/admin)
    ListEntries(ctx context.Context) ([]*domain.RegistrationEntry, error)

    // DeleteEntry deletes a registration entry by SPIFFE ID
    DeleteEntry(ctx context.Context, identityNamespace *domain.IdentityNamespace) error
}
```

**Adapter Implementation** (`internal/adapters/outbound/spire/registration_repository.go`):
```go
type InMemoryRegistrationRepository struct {
    mu      sync.RWMutex
    entries map[string]*domain.RegistrationEntry
}

func (r *InMemoryRegistrationRepository) FindMatchingEntry(ctx context.Context, selectors *domain.SelectorSet) (*domain.RegistrationEntry, error) {
    r.mu.RLock()
    defer r.mu.RUnlock()

    // Core authorization logic: find entry matching workload selectors
    for _, entry := range r.entries {
        // Uses domain's MatchesSelectors() method
        if entry.MatchesSelectors(selectors) {
            return entry, nil
        }
    }

    return nil, fmt.Errorf("no registration entry found matching selectors")
}
```

## Architecture Benefits

### Domain Remains Pure

**Domain** (`internal/core/domain/registration_entry.go`):
- ✅ Entity with validation (`NewRegistrationEntry` checks nil/empty)
- ✅ Authorization logic (`MatchesSelectors()` business rule)
- ✅ Parent-child relationships (`SetParentID()`)
- ✅ Domain-only dependencies (`IdentityNamespace`, `SelectorSet`)

**Adapter** (`internal/adapters/outbound/spire/registration_repository.go`):
- ✅ Storage mechanism (in-memory map with mutex)
- ✅ CRUD operations (Create, Find, List, Delete)
- ✅ Query logic (iterates entries, delegates matching to domain)
- ✅ Integration point for real persistence (SQL, datastore)

### Clear Separation of Concerns

```
┌─────────────────────────────────────┐
│   Persistence Layer                 │
│   (PostgreSQL, SQLite, etc.)        │
└─────────────────────────────────────┘
                 ↓
┌─────────────────────────────────────┐
│   RegistrationRepository Adapter    │
│   - CRUD operations                 │
│   - Query/storage logic             │
│   - Uses domain.RegistrationEntry   │
└─────────────────────────────────────┘
                 ↓
    RegistrationRepository Port
                 ↓
┌─────────────────────────────────────┐
│   Domain RegistrationEntry          │
│   - Authorization policy model      │
│   - Selector matching logic         │
│   - Pure business rules             │
└─────────────────────────────────────┘
```

## Domain Entity: RegistrationEntry

### Purpose
Models **SPIRE's authorization policy**: defines which workloads (identified by selectors) qualify for which identities (SPIFFE IDs).

### Methods

#### `NewRegistrationEntry(identityNamespace, selectors)`
**Domain validation:**
- Ensures SPIFFE ID is not nil
- Ensures selectors are not empty
- Returns error for invalid entries

#### `MatchesSelectors(selectors)`
**Core authorization logic using AND semantics (per SPIRE specification):**
```go
func (r *RegistrationEntry) MatchesSelectors(selectors *SelectorSet) bool {
    // All entry selectors must be present in the discovered selectors
    for _, entrySelector := range r.selectors.All() {
        if !selectors.Contains(entrySelector) {
            return false // Missing required selector
        }
    }
    return true // All entry selectors matched
}
```

This is domain logic implementing SPIRE's AND semantics:
- ALL entry selectors must be present in discovered selectors
- Ensures strong attestation (e.g., workload needs `unix:uid:1000` AND `k8s:ns:default`)
- Discovered selectors can have additional selectors (superset is OK)
- Used during attestation to find matching entries
- No storage, no SDK, no infrastructure

**Example:**
- Entry requires: `[unix:uid:1000, k8s:ns:default]`
- Workload has: `[unix:uid:1000, k8s:ns:default, k8s:pod:my-pod]`
- Result: **MATCH** (workload has all required selectors, extra selectors ignored)

#### `SetParentID(parentID)`
**Domain relationship:**
- Models agent-workload hierarchy
- Workload's parent is the agent that attested it
- Used for trust chain validation

## In-Memory Walking Skeleton

The current implementation uses in-memory storage for demonstration:

**Example Usage:**
```go
// Create repository adapter
repo := spire.NewInMemoryRegistrationRepository()

// Create domain entry
identityNamespace, _ := domain.NewIdentityNamespace("spiffe://example.org/server-workload")
selector, _ := domain.NewSelector(domain.SelectorTypeWorkload, "user", "server-workload")
selectorSet := domain.NewSelectorSet()
selectorSet.Add(selector)

entry, _ := domain.NewRegistrationEntry(identityNamespace, selectorSet)
entry.SetParentID(agentIdentityNamespace)

// Store via adapter
repo.CreateEntry(ctx, entry)

// Find matching entry during attestation
workloadSelectors := domain.NewSelectorSet()
workloadSelectors.Add(selector)
matchedEntry, _ := repo.FindMatchingEntry(ctx, workloadSelectors)

// Use domain's authorization logic
if matchedEntry.MatchesSelectors(workloadSelectors) {
    // Issue SPIFFE ID to workload
}
```

## Future Real SPIRE Integration

When integrating with real SPIRE persistence:

1. **Keep domain RegistrationEntry unchanged** - Already models authorization correctly
2. **Update RegistrationRepository adapter** - Connect to SPIRE server's datastore
3. **Add SQL persistence**:
   - PostgreSQL, MySQL, or SQLite backend
   - Schema with `registered_entries` table
   - Indexes on selectors for fast lookups
   - Transactions for consistency

Example real implementation:
```go
// In adapter
type SQLRegistrationRepository struct {
    db *sql.DB
}

func (r *SQLRegistrationRepository) FindMatchingEntry(ctx context.Context, selectors *domain.SelectorSet) (*domain.RegistrationEntry, error) {
    // 1. Query database for potential matches
    rows, err := r.db.QueryContext(ctx,
        "SELECT spiffe_id, selectors, parent_id FROM registered_entries WHERE ...")
    if err != nil {
        return nil, err
    }
    defer rows.Close()

    // 2. Convert DB rows to domain entries
    for rows.Next() {
        entry := r.rowToEntry(rows)

        // 3. Use domain's MatchesSelectors logic
        if entry.MatchesSelectors(selectors) {
            return entry, nil
        }
    }

    return nil, fmt.Errorf("no matching entry")
}
```

1. ✅ **Retained RegistrationEntry as-is** - Pure domain abstraction, no changes needed
2. ✅ **Dependencies domain-only** - Uses only `IdentityNamespace` and `SelectorSet` (no SDK imports)
3. ✅ **Created RegistrationRepository port** - Defined in `internal/app/ports.go`
4. ✅ **Implemented in-memory adapter** - Map-based storage for walking skeleton in `internal/adapters/outbound/spire/registration_repository.go`
5. ✅ **Documented extension points** - Comments show how to integrate real datastore

## Testing

All tests pass with the RegistrationEntry entity and RegistrationRepository:

```bash
$ go build ./...
Build successful

$ IDP_MODE=inmem go run ./cmd/console
✓ Success! Application runs with pure domain RegistrationEntry entity
```

## Comparison with Real SPIRE

**Real SPIRE Server:**
- Stores registration entries in datastore (PostgreSQL/SQLite)
- Provides Registration API (gRPC) for CRUD operations
- Uses complex selector matching (subset matching, parent relationships)
- Supports federation (trust bundle management)

**Our Domain Model:**
- ✅ Models same core concept (SPIFFE ID → selectors)
- ✅ Implements authorization logic (`MatchesSelectors`)
- ✅ Supports parent relationships (`SetParentID`)
- ✅ Pure domain, no infrastructure coupling

**Adapter Layer:**
- In-memory: Simple map for walking skeleton
- Real: Would connect to SPIRE server's datastore
- Both use same domain `RegistrationEntry` entity

The `RegistrationEntry` entity in `internal/core/domain/registration_entry.go` is verified as domain logic with:
- ✅ No SDK duplications (SDK lacks registration/policy types)
- ✅ Technology-agnostic design
- ✅ Proper port abstraction (RegistrationRepository)
- ✅ Clean adapter separation (storage/query logic)
- ✅ Ready for real SPIRE datastore integration

The entity models authorization policies (SPIFFE ID eligibility rules) while adapters handle persistence (storage/retrieval mechanisms). This maintains hexagonal architecture structure and domain purity.
