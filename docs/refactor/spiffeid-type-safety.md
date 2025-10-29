# Refactoring: Type-Safe SPIFFE ID Handling

**Date:** 2025-01-29
**Status:** Completed
**Impact:** Internal API improvement, no breaking changes to public API

## Summary

Refactored internal functions to use strongly-typed `spiffeid.ID` from the go-spiffe SDK instead of string-based SPIFFE IDs. This improves type safety, reduces string parsing overhead, and aligns with SDK best practices.

## Motivation

The codebase was decomposing `spiffeid.ID` objects into strings immediately after parsing, then re-parsing them later for authorization. This approach:

1. **Lost type safety** - Strings don't provide compile-time guarantees about SPIFFE ID validity
2. **Required redundant parsing** - Parse → string → parse again in verification functions
3. **Was not idiomatic** - The go-spiffe SDK uses typed IDs throughout
4. **Made errors harder to catch** - String comparisons can silently fail vs SDK's Matcher API

## Changes

### 1. `extractSPIFFEID()` - Return Typed ID

**Before:**
```go
func extractSPIFFEID(cert *x509.Certificate) (spiffeID string, trustDomain string, err error) {
    if cert == nil {
        return "", "", errors.New("certificate is nil")
    }

    id, err := x509svid.IDFromCert(cert)
    if err != nil {
        return "", "", fmt.Errorf("invalid peer certificate: %w", err)
    }

    return id.String(), id.TrustDomain().String(), nil
}
```

**After:**
```go
func extractSPIFFEID(cert *x509.Certificate) (spiffeid.ID, error) {
    if cert == nil {
        return spiffeid.ID{}, errors.New("certificate is nil")
    }

    return x509svid.IDFromCert(cert)
}
```

**Benefits:**
- Simpler signature (2 returns instead of 3)
- Returns strongly-typed `spiffeid.ID`
- Callers can access `id.String()`, `id.TrustDomain()`, `id.Path()` as needed
- No premature decomposition to strings

### 2. `verifyClientIdentity()` - Accept Typed Parameters

**Before:**
```go
func verifyClientIdentity(clientSPIFFEID, clientTrustDomain, serverTrustDomain string, cfg ServerConfig) error {
    // Parse client SPIFFE ID using SDK
    clientID, err := spiffeid.FromString(clientSPIFFEID)
    if err != nil {
        return fmt.Errorf("authorization failed: invalid client SPIFFE ID: %w", err)
    }

    // Parse server trust domain
    td, err := spiffeid.TrustDomainFromString(serverTrustDomain)
    if err != nil {
        return fmt.Errorf("authorization failed: invalid server trust domain: %w", err)
    }

    // ... build and apply matcher
}
```

**After:**
```go
func verifyClientIdentity(clientID spiffeid.ID, serverTrustDomain spiffeid.TrustDomain, cfg ServerConfig) error {
    // Build matcher based on policy
    var matcher spiffeid.Matcher

    // Policy 1: Exact SPIFFE ID match
    if cfg.AllowedClientID != "" {
        expectedID, err := spiffeid.FromString(cfg.AllowedClientID)
        if err != nil {
            return fmt.Errorf("authorization failed: invalid AllowedClientID config: %w", err)
        }
        matcher = spiffeid.MatchID(expectedID)
    } else if cfg.AllowedClientTrustDomain != "" {
        // Policy 2: Trust domain match (explicit)
        td, err := spiffeid.TrustDomainFromString(cfg.AllowedClientTrustDomain)
        if err != nil {
            return fmt.Errorf("authorization failed: invalid AllowedClientTrustDomain config: %w", err)
        }
        matcher = spiffeid.MatchMemberOf(td)
    } else {
        // Policy 3: Same trust domain as server (default)
        matcher = spiffeid.MatchMemberOf(serverTrustDomain)
    }

    // Apply matcher
    if err := matcher(clientID); err != nil {
        return fmt.Errorf("authorization failed: %w", err)
    }

    return nil
}
```

**Benefits:**
- No string parsing needed for already-parsed IDs
- Cleaner function signature
- Direct use of typed trust domain (no string-to-TrustDomain conversion needed)

### 3. `verifyServerIdentity()` - Client-Side Authorization

**Before:**
```go
func verifyServerIdentity(serverSPIFFEID, serverTrustDomain string, cfg ClientConfig) error {
    // Policy 1: Exact SPIFFE ID match
    if cfg.ExpectedServerID != "" {
        if serverSPIFFEID == cfg.ExpectedServerID {
            return nil // Allowed
        }
        return fmt.Errorf("authorization failed: server SPIFFE ID %q does not match expected ID %q",
            serverSPIFFEID, cfg.ExpectedServerID)
    }

    // Policy 2: Trust domain match
    if cfg.ExpectedServerTrustDomain != "" {
        if MatchesTrustDomain(serverSPIFFEID, cfg.ExpectedServerTrustDomain) {
            return nil // Allowed
        }
        return fmt.Errorf("authorization failed: server trust domain %q does not match expected trust domain %q",
            serverTrustDomain, cfg.ExpectedServerTrustDomain)
    }

    return errors.New("no server verification policy configured (internal misconfiguration)")
}
```

**After:**
```go
func verifyServerIdentity(serverID spiffeid.ID, cfg ClientConfig) error {
    // Build matcher based on policy
    var matcher spiffeid.Matcher

    // Policy 1: Exact SPIFFE ID match
    if cfg.ExpectedServerID != "" {
        expectedID, err := spiffeid.FromString(cfg.ExpectedServerID)
        if err != nil {
            return fmt.Errorf("authorization failed: invalid ExpectedServerID config: %w", err)
        }
        matcher = spiffeid.MatchID(expectedID)
    } else if cfg.ExpectedServerTrustDomain != "" {
        // Policy 2: Trust domain match
        td, err := spiffeid.TrustDomainFromString(cfg.ExpectedServerTrustDomain)
        if err != nil {
            return fmt.Errorf("authorization failed: invalid ExpectedServerTrustDomain config: %w", err)
        }
        matcher = spiffeid.MatchMemberOf(td)
    } else {
        return errors.New("no server verification policy configured (internal misconfiguration)")
    }

    // Apply matcher
    if err := matcher(serverID); err != nil {
        return fmt.Errorf("authorization failed: %w", err)
    }

    return nil
}
```

**Benefits:**
- Now uses SDK's `Matcher` API consistently with server-side
- Eliminates custom string comparison logic
- Better error messages from SDK matchers

### 4. Updated Call Sites

#### server.go - Extract Server SPIFFE ID
```go
// Before
_, serverTrustDomain, err := extractSPIFFEID(serverCert.Leaf)
serverTrustDomainStr := serverTrustDomain

// After
serverID, err := extractSPIFFEID(serverCert.Leaf)
serverTrustDomain := serverID.TrustDomain()
```

#### server.go - Verify Client Identity
```go
// Before
clientSPIFFEID, clientTrustDomain, err := extractSPIFFEID(leaf)
if err := verifyClientIdentity(clientSPIFFEID, clientTrustDomain, serverTrustDomain, cfg); err != nil {
    return err
}

// After
clientID, err := extractSPIFFEID(leaf)
if err := verifyClientIdentity(clientID, serverTrustDomain, cfg); err != nil {
    return err
}
```

#### client.go - Verify Server Identity
```go
// Before
serverSPIFFEID, serverTrustDomain, err := extractSPIFFEID(serverCert)
if err := verifyServerIdentity(serverSPIFFEID, serverTrustDomain, cfg); err != nil {
    return err
}

// After
serverID, err := extractSPIFFEID(serverCert)
if err := verifyServerIdentity(serverID, cfg); err != nil {
    return err
}
```

## SDK Alignment

This refactoring aligns with go-spiffe SDK patterns:

| SDK Function | What It Returns | How We Now Use It |
|-------------|-----------------|-------------------|
| `x509svid.IDFromCert(cert)` | `spiffeid.ID` | Return directly from `extractSPIFFEID()` |
| `spiffeid.MatchID(id)` | `Matcher` | Use for exact ID matching |
| `spiffeid.MatchMemberOf(td)` | `Matcher` | Use for trust domain matching |
| `id.TrustDomain()` | `spiffeid.TrustDomain` | Pass directly to verify functions |

## Testing

All builds pass successfully:
```bash
go build ./pkg/identitytls      # ✅
go build ./...                   # ✅
go build ./examples/highlevel    # ✅
```

No test files exist in the project currently, but the refactoring:
- Preserves all authorization logic
- Uses SDK functions that are battle-tested
- Only changes internal function signatures (no public API changes)

## Performance Impact

**Positive:**
- Eliminated redundant string parsing in authorization path
- Reduced allocations from string conversions

**Example flow before:**
1. Parse certificate → `spiffeid.ID`
2. Convert to string: `id.String()`
3. Pass string to verify function
4. Re-parse string: `spiffeid.FromString()`
5. Apply matcher

**Example flow after:**
1. Parse certificate → `spiffeid.ID`
2. Pass typed ID to verify function
3. Apply matcher directly

## Public API Impact

**No breaking changes.** This refactoring only affects internal functions:

- `extractSPIFFEID()` - internal, not exported
- `verifyClientIdentity()` - internal, not exported
- `verifyServerIdentity()` - internal, not exported

Public API remains unchanged:
- ✅ `identitytls.NewServerTLSConfig()` - same signature
- ✅ `identitytls.NewClientTLSConfig()` - same signature
- ✅ `identitytls.ExtractPeerInfo()` - same signature
- ✅ `e5s.Start()` - same signature
- ✅ `e5s.Client()` - same signature

## Lessons Learned

1. **Preserve SDK types as long as possible** - Don't convert to strings until absolutely necessary (e.g., for display/logging)

2. **Use Matcher API consistently** - The SDK's Matcher pattern (`MatchID`, `MatchMemberOf`) is more robust than custom string comparisons

3. **Type safety catches errors at compile time** - Using `spiffeid.ID` and `spiffeid.TrustDomain` makes invalid states unrepresentable

4. **Internal refactoring is safe** - When public API is stable, internal improvements can be made confidently

## Future Work

Potential next steps to further leverage SDK types:

1. **Consider exposing `spiffeid.ID` in public API** - `PeerInfo.SPIFFEID` could be `spiffeid.ID` instead of `string` (breaking change, needs careful consideration)

2. **Use `spiffeid.Matcher` in config** - Instead of `AllowedClientID string`, could accept `Matcher` directly (would require API redesign)

3. **Leverage more SDK functions** - Explore other SDK utilities we might be reimplementing

## References

- [go-spiffe SDK Documentation](https://pkg.go.dev/github.com/spiffe/go-spiffe/v2)
- [spiffeid Package](https://pkg.go.dev/github.com/spiffe/go-spiffe/v2/spiffeid)
- [x509svid Package](https://pkg.go.dev/github.com/spiffe/go-spiffe/v2/svid/x509svid)
- [SPIFFE Specification](https://github.com/spiffe/spiffe/blob/main/standards/SPIFFE.md)
