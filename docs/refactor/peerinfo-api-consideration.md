# API Design Consideration: PeerInfo Type Safety

**Date:** 2025-01-29
**Status:** Proposal - Not Implemented
**Impact:** Breaking change to public API

## Current Design

```go
type PeerInfo struct {
    // SPIFFEID is the verified SPIFFE ID from the peer's certificate.
    // Example: "spiffe://example.org/service"
    SPIFFEID string

    // TrustDomain is extracted from the SPIFFE ID.
    // Example: "example.org"
    TrustDomain string

    // ExpiresAt is when the peer's certificate expires.
    ExpiresAt time.Time
}
```

**Current usage:**
```go
peer, ok := e5s.PeerInfo(r)
if !ok {
    http.Error(w, "unauthorized", http.StatusUnauthorized)
    return
}

fmt.Fprintf(w, "Hello %s\n", peer.SPIFFEID)
log.Printf("Request from %s in trust domain %s", peer.SPIFFEID, peer.TrustDomain)
```

## Proposed Design

```go
type PeerInfo struct {
    // ID is the verified SPIFFE ID from the peer's certificate.
    // This provides strongly-typed access to both the full SPIFFE ID
    // and its components (trust domain, path).
    ID spiffeid.ID

    // ExpiresAt is when the peer's certificate expires.
    ExpiresAt time.Time
}
```

**Proposed usage:**
```go
peer, ok := e5s.PeerInfo(r)
if !ok {
    http.Error(w, "unauthorized", http.StatusUnauthorized)
    return
}

fmt.Fprintf(w, "Hello %s\n", peer.ID.String())
log.Printf("Request from %s in trust domain %s",
    peer.ID.String(), peer.ID.TrustDomain().Name())
```

## Benefits of Proposed Design

### 1. Type Safety
- `spiffeid.ID` is a strongly-typed value that guarantees valid SPIFFE IDs
- String fields can contain arbitrary invalid data
- Compile-time guarantees vs runtime validation

### 2. DRY (Don't Repeat Yourself)
- Current design stores redundant data (`TrustDomain` is already in `SPIFFEID`)
- Can cause inconsistencies if fields get out of sync
- Proposed design has single source of truth

### 3. SDK Alignment
- Matches go-spiffe SDK patterns (uses `spiffeid.ID` throughout)
- More idiomatic Go code
- Better integration with SDK functions

### 4. Access to SDK Methods
Users gain access to all `spiffeid.ID` methods:
```go
peer.ID.String()                    // Full SPIFFE ID
peer.ID.TrustDomain()              // Trust domain object
peer.ID.TrustDomain().Name()       // Trust domain string
peer.ID.Path()                     // Workload path
peer.ID.MemberOf(td)               // Trust domain membership check
```

### 5. Reduced Memory Footprint
- One `spiffeid.ID` instead of two strings
- No duplicate trust domain storage

## Drawbacks of Proposed Design

### 1. Breaking Change
**This is a breaking API change.** All existing code that accesses `PeerInfo` fields would need updates:

```go
// Before
peer.SPIFFEID      → // After: peer.ID.String()
peer.TrustDomain   → // After: peer.ID.TrustDomain().Name()
```

### 2. Slightly More Verbose
Common operations require method calls instead of direct field access:
```go
// Before: simple field access
log.Printf("Request from %s", peer.SPIFFEID)

// After: method call
log.Printf("Request from %s", peer.ID.String())
```

### 3. Learning Curve
Users need to understand `spiffeid.ID` API instead of working with plain strings.

## Migration Path

If we decide to implement this change, here's a gradual migration strategy:

### Phase 1: Add New Field, Deprecate Old Fields
```go
type PeerInfo struct {
    // ID is the verified SPIFFE ID (preferred).
    ID spiffeid.ID

    // Deprecated: Use ID.String() instead.
    // Will be removed in v2.0.0
    SPIFFEID string

    // Deprecated: Use ID.TrustDomain().Name() instead.
    // Will be removed in v2.0.0
    TrustDomain string

    ExpiresAt time.Time
}
```

### Phase 2: Update Internal Code
Update all examples and documentation to use `peer.ID` instead of deprecated fields.

### Phase 3: Announce Deprecation
Add deprecation notices in:
- CHANGELOG
- Release notes
- Migration guide

### Phase 4: Remove Deprecated Fields (v2.0.0)
In next major version, remove deprecated fields entirely.

## Alternative: Keep Current Design

**Reason to keep current design:**
- **No breaking changes** - Maintains backward compatibility
- **Simpler for beginners** - Strings are more approachable than typed IDs
- **Sufficient for most use cases** - Users rarely need SDK methods on the ID

**Compromise:**
Keep current `PeerInfo` struct but add helper methods:

```go
// ID returns the SPIFFE ID as a strongly-typed spiffeid.ID.
//
// This provides access to SDK methods like MemberOf(), Path(), etc.
func (p PeerInfo) ID() spiffeid.ID {
    id, _ := spiffeid.FromString(p.SPIFFEID)
    return id
}
```

This allows advanced users to opt into typed IDs without breaking existing code:
```go
// Simple usage (current)
log.Printf("Request from %s", peer.SPIFFEID)

// Advanced usage (new)
if peer.ID().MemberOf(someTrustDomain) {
    // ...
}
```

## Recommendation

**Recommendation: Keep current design with optional helper method**

**Reasoning:**
1. The library is already being used (examples exist in docs)
2. Breaking changes are disruptive to users
3. The current string-based API is more accessible to beginners
4. The redundant `TrustDomain` field provides convenience
5. Can add optional `ID()` helper method for advanced use cases without breaking compatibility

**Future consideration:**
- Revisit this in v2.0.0 if there's user demand for typed IDs
- Monitor user feedback and common pain points
- Consider making the change if type-related bugs emerge

## Decision Log

- **2025-01-29**: Documented this consideration after refactoring internal functions to use `spiffeid.ID`
- **Status**: No implementation planned for v1.x
- **Next review**: v2.0.0 planning or if user feedback indicates need

## Related Refactorings

This consideration came up during the type-safety refactoring documented in:
- `docs/refactor/spiffeid-type-safety.md` - Internal functions now use `spiffeid.ID`

The internal functions (`extractSPIFFEID`, `verifyClientIdentity`, etc.) have been refactored to use typed IDs, but the public API (`PeerInfo`) remains string-based for backward compatibility.

## References

- [go-spiffe spiffeid.ID documentation](https://pkg.go.dev/github.com/spiffe/go-spiffe/v2/spiffeid#ID)
- [Semantic Versioning 2.0.0](https://semver.org/) - Breaking changes require major version bump
