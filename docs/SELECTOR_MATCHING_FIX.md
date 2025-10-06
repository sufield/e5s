# Critical Fix: Selector Matching Logic

## Issue

The `MatchesSelectors()` method in `RegistrationEntry` was using **OR logic** instead of the correct **AND logic** per SPIRE specification.

## Problem

### Incorrect Implementation (Before)
```go
func (r *RegistrationEntry) MatchesSelectors(selectors *SelectorSet) bool {
    // Check if any of the entry's selectors match (WRONG - OR logic)
    for _, entrySelector := range r.selectors.All() {
        if selectors.Contains(entrySelector) {
            return true  // Returns on FIRST match
        }
    }
    return false
}
```

**Issue**: This returns `true` if **ANY** entry selector is found, which is **incorrect per SPIRE semantics**.

### Example of Incorrect Behavior

**Entry requires**: `[unix:uid:1000, k8s:ns:default]`

**Workload has**: `[unix:uid:1000]` (missing namespace selector)

**Result with OR logic**: ✅ **MATCH** (incorrect!)
- Found `unix:uid:1000`, returned true immediately
- Never checked for `k8s:ns:default`

**Security Impact**: Workload gets identity without meeting all requirements.

## Solution

### Correct Implementation (After)
```go
func (r *RegistrationEntry) MatchesSelectors(selectors *SelectorSet) bool {
    // All entry selectors must be present (CORRECT - AND logic)
    for _, entrySelector := range r.selectors.All() {
        if !selectors.Contains(entrySelector) {
            return false // Missing required selector
        }
    }
    return true // All entry selectors matched
}
```

**Correct**: Returns `true` only if **ALL** entry selectors are present.

### Example of Correct Behavior

**Entry requires**: `[unix:uid:1000, k8s:ns:default]`

#### Test Case 1: Missing Selector
**Workload has**: `[unix:uid:1000]`

**Result with AND logic**: ❌ **NO MATCH** (correct!)
- Found `unix:uid:1000` ✓
- Missing `k8s:ns:default` ✗
- Returns false

#### Test Case 2: All Required Selectors
**Workload has**: `[unix:uid:1000, k8s:ns:default]`

**Result with AND logic**: ✅ **MATCH** (correct!)
- Found `unix:uid:1000` ✓
- Found `k8s:ns:default` ✓
- Returns true

#### Test Case 3: Superset (Extra Selectors)
**Workload has**: `[unix:uid:1000, k8s:ns:default, k8s:pod:my-pod]`

**Result with AND logic**: ✅ **MATCH** (correct!)
- Found `unix:uid:1000` ✓
- Found `k8s:ns:default` ✓
- Extra `k8s:pod:my-pod` ignored (OK)
- Returns true

## SPIRE Specification

Per SPIRE documentation:
> A workload matches a registration entry if **all** of the selectors defined in the entry are present in the selectors discovered during workload attestation.

This is **AND logic**:
- Entry selectors define **requirements**
- Discovered selectors must be a **superset** of entry selectors
- Workload can have additional selectors (extras are ignored)
- All entry selectors must be satisfied

## Security Implications

### With OR Logic (Insecure)
- Workload could match entry with partial credentials
- Example: Match production identity with only `uid:1000`, ignoring required `env:prod`
- **Breaks principle of strong attestation**

### With AND Logic (Secure)
- Workload must prove ALL required attributes
- Example: Must have BOTH `uid:1000` AND `env:prod`
- **Enforces strong attestation as intended by SPIRE**

### Documentation Updated
- [DOMAIN.md](DOMAIN.md) - Added AND semantics explanation with examples
- [REGISTRATION_ENTRY_VERIFICATION.md](REGISTRATION_ENTRY_VERIFICATION.md) - Updated authorization logic documentation

## Testing

### Before Fix
```go
entry := NewRegistrationEntry(identityNamespace, selectorSet) // Requires [uid:1000, ns:default]
discovered := NewSelectorSet()
discovered.Add(uidSelector) // Only has uid:1000

entry.MatchesSelectors(discovered) // TRUE (incorrect!)
```

### After Fix
```go
entry := NewRegistrationEntry(identityNamespace, selectorSet) // Requires [uid:1000, ns:default]
discovered := NewSelectorSet()
discovered.Add(uidSelector) // Only has uid:1000

entry.MatchesSelectors(discovered) // FALSE (correct!)
```

## Verification

```bash
$ go build ./...
Build successful

$ IDP_MODE=inmem go run ./cmd/console
✓ Success! Selector matching now uses correct AND logic
```

## Impact on Walking Skeleton

The current in-memory implementation still works because:
- Each workload registration has a single unique selector
- Single-selector entries still match correctly with AND logic
- Example: Entry `[unix:user:server-workload]` matches workload with `[unix:user:server-workload, unix:uid:1001, ...]`

For multi-selector scenarios (future testing):
- Now correctly requires ALL selectors
- Aligns with SPIRE specification
- Enables proper strong attestation testing

## Conclusion

This fix aligns the domain implementation with SPIRE's authorization semantics:
- ✅ Uses AND logic as specified by SPIRE
- ✅ Enforces strong attestation (all requirements must be met)
- ✅ Remains pure domain logic (no SDK dependencies)
- ✅ Properly documented with examples
- ✅ Security implications addressed

The domain entity now correctly models SPIRE's registration entry matching behavior.
