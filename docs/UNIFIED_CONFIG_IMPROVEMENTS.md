# Unified Configuration Improvements

**Type**: Architecture Decision Record (ADR)

## Overview

This document explains why the mTLS configuration was unified into a single `MTLSConfig` structure and why generic "peer" terminology was adopted instead of separate "client" and "server" configurations. 

## Changes Made

### 1. Unified Configuration Structure

**Before:**
```go
// Separate server and client configs
type ServerConfig struct {
    WorkloadAPI WorkloadAPIConfig
    SPIFFE      SPIFFEServerConfig
    HTTP        HTTPServerConfig
}

type ClientConfig struct {
    WorkloadAPI WorkloadAPIConfig
    SPIFFE      SPIFFEClientConfig
    HTTP        HTTPClientConfig
}
```

**After:**
```go
// Single unified config
type MTLSConfig struct {
    WorkloadAPI WorkloadAPIConfig
    SPIFFE      SPIFFEConfig
    HTTP        HTTPConfig
}
```

### 2. Generic Peer Terminology

**Before:**
```go
type SPIFFEServerConfig struct {
    AllowedClientID string  // Server-specific
}

type SPIFFEClientConfig struct {
    ExpectedServerID string  // Client-specific
}
```

**After:**
```go
type SPIFFEConfig struct {
    AllowedPeerID string  // Generic, works for both
}
```

### 3. Simplified Interface Comments

**Before:**
```go
// MTLSServer is the stable interface for an mTLS HTTP server.
// It provides identity-based authentication using SPIFFE/SPIRE...
// [long detailed explanation]
```

**After:**
```go
// MTLSServer is the stable interface for an mTLS HTTP server.
// It provides identity-based authentication using SPIFFE/SPIRE.
type MTLSServer interface {
    // Handle registers an HTTP handler (same semantics as http.ServeMux).
    // Handlers receive requests with authenticated SPIFFE ID in context.
    Handle(pattern string, handler http.Handler)
    // ...
}
```

### 4. Updated Error Messages

All error messages now use generic "peer" terminology:

```go
// Before
if cfg.SPIFFE.AllowedClientID == "" {
    return nil, fmt.Errorf("spiffe allowed client id is required")
}

// After
if cfg.SPIFFE.AllowedPeerID == "" {
    return nil, fmt.Errorf("spiffe allowed peer id is required")
}
```

## Files Modified

1. **internal/ports/identityserver.go**
   - Introduced `MTLSConfig` with unified structure
   - Changed `AllowedClientID` to `AllowedPeerID`
   - Simplified interface comments

2. **internal/adapters/inbound/identityserver/spiffe_server.go**
   - Updated to use `cfg.SPIFFE.AllowedPeerID`
   - Updated error messages to use "peer" terminology
   - Updated variable names (`clientID` variable name kept for SPIRE SDK compatibility)

3. **internal/adapters/inbound/identityserver/spiffe_server_test.go**
   - Updated all tests to use `AllowedPeerID`
   - Updated test assertions to expect "peer" in error messages
   - Added `contains()` helper function for better error message checking

4. **internal/config/mtls.go**
   - Updated `ToServerConfig()` to use `AllowedPeerID`
   - Updated `ToClientConfig()` to use `AllowedPeerID`

5. **examples/identityserver-example/main.go**
   - Updated to use `cfg.SPIFFE.AllowedPeerID`
   - Updated environment variable usage

## Benefits

### 1. Simpler Configuration

A single configuration structure works for both clients and servers, reducing duplication and making the API easier to understand.

### 2. Generic Terminology

"Peer" is more accurate than "client" or "server" in mTLS contexts where both parties authenticate each other. It's clearer that the configuration specifies the *other* party in the connection.

### 3. Consistent API

The same field name (`AllowedPeerID`) works whether you're:
- A server checking the client's identity
- A client checking the server's identity

## Usage

Use the unified configuration structure:

```go
cfg := ports.MTLSConfig{
    WorkloadAPI: ports.WorkloadAPIConfig{
        SocketPath: "unix:///tmp/spire-agent/public/api.sock",
    },
    SPIFFE: ports.SPIFFEConfig{
        AllowedPeerID: "spiffe://example.org/peer",
    },
    HTTP: ports.HTTPConfig{
        Address: ":8443",
    },
}
```

## Testing

All tests pass with the new unified configuration:

```bash
$ go test ./internal/adapters/inbound/identityserver/...
PASS
ok      github.com/pocket/hexagon/spire/internal/adapters/inbound/identityserver    30.011s
```

Tests verify:
- Configuration validation
- Error messages use "peer" terminology
- SPIFFE ID extraction from requests

## Design Rationale

### Why "Peer" instead of "Client"?

In mTLS, both parties authenticate each other. From the server's perspective, it's validating the *client*. From the client's perspective, it's validating the *server*. Using "peer" makes it clear we're talking about the *other party* in the connection, regardless of which side you're on.

### Why Unified Config?

The server and client configurations were largely identical. Unifying them:
1. Reduces code duplication
2. Makes it easier to create configurations that work for both
3. Simplifies the API surface
4. Aligns with how SPIFFE/SPIRE works (symmetric authentication)

### Why Keep Variable Name `clientID`?

In the server implementation, we kept the variable name `clientID` when interacting with the SPIRE SDK because:
1. From the server's perspective, it *is* checking the client
2. It matches SPIRE's terminology (e.g., `tlsconfig.AuthorizeID`)
3. It doesn't leak into the public API

## Related Documentation

- [PORT_CONTRACTS.md](PORT_CONTRACTS.md) - Complete port interface contracts
- [MANUAL_TESTING_GUIDE.md](MANUAL_TESTING_GUIDE.md) - Manual testing procedures with examples
