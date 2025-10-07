# Backward Compatibility Removal

All backward compatibility type aliases have been removed from the codebase. The code now exclusively uses the unified configuration types.

## Changes Made

### Type Aliases Removed

The following type aliases have been completely removed from `internal/ports/identityserver.go`:

```go
// REMOVED - No longer present
type (
    ServerConfig       = MTLSConfig
    ClientConfig       = MTLSConfig
    SPIFFEServerConfig = SPIFFEConfig
    SPIFFEClientConfig = SPIFFEConfig
    HTTPServerConfig   = HTTPConfig
    HTTPClientConfig   = HTTPConfig
)
```

### Updated Type References

All references throughout the codebase now use the canonical types:

**internal/ports/identityserver.go:**
- Only `MTLSConfig`, `SPIFFEConfig`, `HTTPConfig` remain

**internal/adapters/inbound/identityserver/spiffe_server.go:**
- Function signature: `NewSPIFFEServer(ctx context.Context, cfg ports.MTLSConfig)`
- Struct field: `cfg ports.MTLSConfig`

**internal/adapters/inbound/identityserver/spiffe_server_test.go:**
- All tests use `ports.MTLSConfig`
- All SPIFFE config uses `ports.SPIFFEConfig`
- All HTTP config uses `ports.HTTPConfig`

**internal/config/mtls.go:**
- `ToServerConfig() ports.MTLSConfig` (comment updated)
- `ToClientConfig() ports.MTLSConfig` (comment updated)

**examples/identityserver-example/main.go:**
- Variable declaration: `var cfg ports.MTLSConfig`

## Verification

All code builds successfully and all tests pass:

```bash
$ go test ./...
ok      github.com/pocket/hexagon/spire/internal/adapters/inbound/identityserver
ok      github.com/pocket/hexagon/spire/internal/config
ok      github.com/pocket/hexagon/spire/internal/ports
# ... all other packages ...

$ go build ./...
# Success - no errors

$ go build ./examples/identityserver-example/
# Success - no errors
```

## Benefits

1. **Cleaner API**: No deprecated aliases cluttering the API surface
2. **Clear Intent**: Only one way to refer to configuration types
3. **Simpler Maintenance**: No need to maintain compatibility layer
4. **Better Documentation**: Documentation is clearer without migration guides

## Impact

This is a **breaking change** for any external code that was using the old type aliases. However, migration is straightforward:

### Migration Required

Replace old type names with new unified types:

```go
// Old code (no longer works)
var cfg ports.ServerConfig
cfg.SPIFFE = ports.SPIFFEServerConfig{
    AllowedClientID: "spiffe://example.org/client",
}

// New code (required)
var cfg ports.MTLSConfig
cfg.SPIFFE = ports.SPIFFEConfig{
    AllowedPeerID: "spiffe://example.org/client",
}
```

## Files Modified

1. `internal/ports/identityserver.go` - Removed type aliases
2. `internal/adapters/inbound/identityserver/spiffe_server.go` - Uses `MTLSConfig`
3. `internal/adapters/inbound/identityserver/spiffe_server_test.go` - Uses `MTLSConfig`, `SPIFFEConfig`, `HTTPConfig`
4. `internal/config/mtls.go` - Updated return types and comments
5. `examples/identityserver-example/main.go` - Uses `MTLSConfig`
6. `docs/UNIFIED_CONFIG_IMPROVEMENTS.md` - Removed backward compatibility sections

## Related Documentation

- [Unified Configuration Improvements](UNIFIED_CONFIG_IMPROVEMENTS.md)
- [Port-Based Improvements](PORT_BASED_IMPROVEMENTS.md)
- [Improvements Applied](IMPROVEMENTS_APPLIED.md)
