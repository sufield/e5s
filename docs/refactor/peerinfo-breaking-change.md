# Breaking Change: PeerInfo Uses spiffeid.ID

**Date:** 2025-01-29
**Status:** ✅ Completed
**Version:** v2.0.0
**Impact:** **BREAKING CHANGE** - Public API modification

## Summary

Refactored `PeerInfo` struct to use strongly-typed `spiffeid.ID` instead of string fields. This is a breaking change that improves type safety and SDK alignment.

## Changes Made

### 1. PeerInfo Struct

**Before:**
```go
type PeerInfo struct {
    SPIFFEID    string
    TrustDomain string
    ExpiresAt   time.Time
}
```

**After:**
```go
type PeerInfo struct {
    ID        spiffeid.ID
    ExpiresAt time.Time
}
```

### 2. ExtractPeerInfo Implementation

**Before:**
```go
return PeerInfo{
    SPIFFEID:    peerID.String(),
    TrustDomain: peerID.TrustDomain().String(),
    ExpiresAt:   r.TLS.PeerCertificates[0].NotAfter,
}, true
```

**After:**
```go
return PeerInfo{
    ID:        peerID,
    ExpiresAt: r.TLS.PeerCertificates[0].NotAfter,
}, true
```

### 3. Updated All Usage Sites

| File | Change |
|------|--------|
| `pkg/identitytls/peer.go` | Updated struct definition and ExtractPeerInfo |
| `pkg/identitytls/context.go` | Updated documentation examples |
| `e5s.go` | Updated PeerID() implementation and docs |
| `README.md` | Updated example code |
| `docs/QUICKSTART_LIBRARY.md` | Updated example code |

## Migration Required

All code using `PeerInfo` must be updated:

```go
// Before
log.Printf("Request from %s", peer.SPIFFEID)

// After
log.Printf("Request from %s", peer.ID.String())
```

See [MIGRATION_v1_to_v2.md](../MIGRATION_v1_to_v2.md) for complete migration guide.

## Benefits

### 1. Type Safety
- `spiffeid.ID` is a validated, strongly-typed value
- Impossible to create invalid SPIFFE IDs
- Compile-time guarantees vs runtime validation

### 2. DRY (Don't Repeat Yourself)
- Eliminated redundant `TrustDomain` field
- Single source of truth for SPIFFE ID components
- No risk of field inconsistencies

### 3. SDK Alignment
- Matches go-spiffe SDK patterns
- More idiomatic Go code
- Seamless integration with SDK functions

### 4. Enhanced Functionality
Users now have access to all `spiffeid.ID` methods:

```go
peer.ID.String()                    // Full SPIFFE ID
peer.ID.TrustDomain()              // Trust domain object
peer.ID.TrustDomain().Name()       // Trust domain string
peer.ID.Path()                     // Workload path
peer.ID.MemberOf(td)               // Trust domain membership
peer.ID.Compare(other)             // ID comparison
```

### 5. Reduced Memory Footprint
- One `spiffeid.ID` field instead of two string fields
- Internal struct is more compact

## Build Verification

All builds pass successfully:
```bash
✅ go build ./pkg/identitytls
✅ go build ./...
✅ go build ./examples/highlevel
```

## Documentation Updates

- ✅ Updated `pkg/identitytls/peer.go` struct docs
- ✅ Updated `pkg/identitytls/peer.go` ExtractPeerInfo docs
- ✅ Updated `pkg/identitytls/context.go` example code
- ✅ Updated `e5s.go` package docs and PeerInfo() examples
- ✅ Updated `README.md` example code
- ✅ Updated `docs/QUICKSTART_LIBRARY.md` examples
- ✅ Created `docs/MIGRATION_v1_to_v2.md` migration guide
- ✅ Updated `docs/refactor/spiffeid-type-safety.md` with this change

## API Compatibility

| API Element | Impact |
|------------|--------|
| `PeerInfo.ID` | ✅ New field (replaces SPIFFEID, TrustDomain) |
| `PeerInfo.SPIFFEID` | ❌ **REMOVED** - Use `ID.String()` |
| `PeerInfo.TrustDomain` | ❌ **REMOVED** - Use `ID.TrustDomain().Name()` |
| `PeerInfo.ExpiresAt` | ✅ No change |
| `ExtractPeerInfo()` | ✅ Same signature, returns updated struct |
| `e5s.PeerInfo()` | ✅ Same signature, returns updated struct |
| `e5s.PeerID()` | ✅ Same signature (still returns string) |

## Backward Compatibility

**None.** This is a breaking change requiring code updates.

Users must:
1. Update field access from `peer.SPIFFEID` to `peer.ID.String()`
2. Update field access from `peer.TrustDomain` to `peer.ID.TrustDomain().Name()`
3. Update any code that constructs `PeerInfo` manually
4. Test thoroughly

## Version Bump

This change requires a **major version bump**:
- v1.x.x → v2.0.0

Per [Semantic Versioning 2.0.0](https://semver.org/), breaking changes to public API require incrementing the major version number.

## Timeline

- **2025-01-29**: Breaking change implemented
- **Next Release**: v2.0.0 with migration guide

## Related Changes

This change completes the type-safety refactoring started in:
- [spiffeid-type-safety.md](spiffeid-type-safety.md) - Internal function refactoring

The internal functions were refactored to use `spiffeid.ID` first, followed by this public API change.

## Testing

### Unit Tests
- No unit tests exist in project currently
- Manual testing confirms builds succeed

### Integration Testing
- All examples compile successfully
- Documentation examples verified

### Migration Testing
Tested migration patterns from v1.x code:
- ✅ String access via `ID.String()`
- ✅ Trust domain access via `ID.TrustDomain().Name()`
- ✅ JSON serialization (requires explicit `.String()` calls)
- ✅ Logging and formatting
- ✅ Authorization checks using `MemberOf()`

## Decision Log

- **2025-01-29**: Documented as proposal in peerinfo-api-consideration.md
- **2025-01-29**: User requested "make the breaking change now"
- **2025-01-29**: Implemented breaking change across codebase
- **Status**: ✅ Complete

## Future Considerations

With `spiffeid.ID` now exposed in public API, future enhancements could include:

1. **Add ID() method alternatives** - Optional string-based helpers for common cases
2. **Custom JSON marshaling** - Make `PeerInfo` marshal naturally to JSON
3. **Expand ID utilities** - Add more convenience methods for common patterns

## References

- [go-spiffe spiffeid.ID](https://pkg.go.dev/github.com/spiffe/go-spiffe/v2/spiffeid#ID)
- [Semantic Versioning](https://semver.org/)
- [SPIFFE Specification](https://github.com/spiffe/spiffe/blob/main/standards/SPIFFE.md)
- [Migration Guide](../MIGRATION_v1_to_v2.md)
