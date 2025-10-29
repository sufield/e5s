# Migration Guide: v1.x to v2.0

**BREAKING CHANGES**: This guide covers breaking changes introduced in v2.0.0.

## PeerInfo Structure Change

### What Changed

The `PeerInfo` struct now uses strongly-typed `spiffeid.ID` instead of string fields:

**Before (v1.x):**
```go
type PeerInfo struct {
    SPIFFEID    string
    TrustDomain string
    ExpiresAt   time.Time
}
```

**After (v2.0):**
```go
type PeerInfo struct {
    ID        spiffeid.ID
    ExpiresAt time.Time
}
```

### Why This Change?

1. **Type Safety** - `spiffeid.ID` guarantees valid SPIFFE IDs at compile time
2. **DRY Principle** - No redundant trust domain storage
3. **SDK Alignment** - Matches go-spiffe SDK patterns
4. **More Functionality** - Access to all SDK methods (`MemberOf()`, `Path()`, etc.)

### Migration Steps

#### 1. Update Field Access

**Before:**
```go
peer, ok := e5s.PeerInfo(r)
if !ok {
    return
}

log.Printf("Request from %s", peer.SPIFFEID)
log.Printf("Trust domain: %s", peer.TrustDomain)
```

**After:**
```go
peer, ok := e5s.PeerInfo(r)
if !ok {
    return
}

log.Printf("Request from %s", peer.ID.String())
log.Printf("Trust domain: %s", peer.ID.TrustDomain().Name())
```

#### 2. String Formatting

**Before:**
```go
fmt.Fprintf(w, "Hello %s!\n", peer.SPIFFEID)
```

**After:**
```go
fmt.Fprintf(w, "Hello %s!\n", peer.ID.String())
```

#### 3. Trust Domain Checks

**Before:**
```go
if peer.TrustDomain == "example.org" {
    // allow
}
```

**After (Option 1 - String comparison):**
```go
if peer.ID.TrustDomain().Name() == "example.org" {
    // allow
}
```

**After (Option 2 - SDK method, recommended):**
```go
td, _ := spiffeid.TrustDomainFromString("example.org")
if peer.ID.MemberOf(td) {
    // allow
}
```

#### 4. Struct Construction (If Manually Creating)

**Before:**
```go
peer := PeerInfo{
    SPIFFEID:    "spiffe://example.org/service",
    TrustDomain: "example.org",
    ExpiresAt:   time.Now().Add(time.Hour),
}
```

**After:**
```go
id, _ := spiffeid.FromString("spiffe://example.org/service")
peer := PeerInfo{
    ID:        id,
    ExpiresAt: time.Now().Add(time.Hour),
}
```

## Quick Migration Checklist

- [ ] Replace `peer.SPIFFEID` with `peer.ID.String()`
- [ ] Replace `peer.TrustDomain` with `peer.ID.TrustDomain().Name()`
- [ ] Update any trust domain comparisons to use `ID.MemberOf()` (optional but recommended)
- [ ] Build and test your code
- [ ] Update any persisted data structures that stored `PeerInfo`

## New Capabilities

With `spiffeid.ID`, you now have access to:

```go
peer, ok := e5s.PeerInfo(r)

// Full SPIFFE ID
spiffeID := peer.ID.String()  // "spiffe://example.org/service"

// Trust domain (object)
td := peer.ID.TrustDomain()

// Trust domain (string)
tdName := peer.ID.TrustDomain().Name()  // "example.org"

// Workload path
path := peer.ID.Path()  // "/service"

// Trust domain membership check
if peer.ID.MemberOf(someTrustDomain) {
    // allow
}

// Comparison
if peer.ID.Compare(otherID) == 0 {
    // equal
}
```

## Common Patterns

### Pattern 1: Authorization by Trust Domain

**Before:**
```go
func authorize(peer PeerInfo) bool {
    return peer.TrustDomain == "example.org"
}
```

**After:**
```go
func authorize(peer PeerInfo) bool {
    td, _ := spiffeid.TrustDomainFromString("example.org")
    return peer.ID.MemberOf(td)
}
```

### Pattern 2: Logging

**Before:**
```go
log.Printf("Request from %s in %s", peer.SPIFFEID, peer.TrustDomain)
```

**After:**
```go
log.Printf("Request from %s in %s",
    peer.ID.String(), peer.ID.TrustDomain().Name())
```

### Pattern 3: JSON Serialization

**Before:**
```go
type Response struct {
    CallerID    string `json:"caller_id"`
    TrustDomain string `json:"trust_domain"`
}

resp := Response{
    CallerID:    peer.SPIFFEID,
    TrustDomain: peer.TrustDomain,
}
```

**After:**
```go
type Response struct {
    CallerID    string `json:"caller_id"`
    TrustDomain string `json:"trust_domain"`
}

resp := Response{
    CallerID:    peer.ID.String(),
    TrustDomain: peer.ID.TrustDomain().Name(),
}
```

## Migration Tools

### Automated Find & Replace

Use these sed commands (review changes before applying):

```bash
# Replace peer.SPIFFEID with peer.ID.String()
find . -name "*.go" -type f -exec sed -i.bak 's/peer\.SPIFFEID/peer.ID.String()/g' {} \;

# Replace peer.TrustDomain with peer.ID.TrustDomain().Name()
find . -name "*.go" -type f -exec sed -i.bak 's/peer\.TrustDomain/peer.ID.TrustDomain().Name()/g' {} \;

# Clean up backups after verifying changes
find . -name "*.go.bak" -delete
```

**Warning:** Always review automated changes before committing!

## Rollback Strategy

If you need to temporarily stay on v1.x:

```bash
# Pin to last v1.x version in go.mod
go get github.com/sufield/e5s@v1.x.x

# Or use a version constraint
require github.com/sufield/e5s v1.x.x
```

## Support

If you encounter issues during migration:

1. Check examples in `examples/` directory
2. Read updated documentation in `docs/`
3. Report issues at https://github.com/sufield/e5s/issues

## Related Documentation

- [Type Safety Refactoring](refactor/spiffeid-type-safety.md) - Technical details of this change
- [PeerInfo API Consideration](refactor/peerinfo-api-consideration.md) - Design rationale
- [go-spiffe spiffeid.ID docs](https://pkg.go.dev/github.com/spiffe/go-spiffe/v2/spiffeid#ID)
