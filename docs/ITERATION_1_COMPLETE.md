# Iteration 1: mTLS HTTP Server - COMPLETE ✅

## Overview

Iteration 1 implements the mTLS HTTP server (inbound adapter) as specified in [MTLS_IMPLEMENTATION.md](MTLS_IMPLEMENTATION.md). This server authenticates clients using X.509 SVIDs and exposes client identity to handlers via middleware.

## Implementation Status

### Files Created

1. **[internal/adapters/inbound/httpapi/server.go](../internal/adapters/inbound/httpapi/server.go)**
   - `HTTPServer` struct with mTLS configuration
   - `NewHTTPServer()` - Creates server with X509Source and authorizer
   - `Start()` - Starts HTTPS server with mTLS
   - `Stop()` - Graceful shutdown
   - `RegisterHandler()` - Registers handlers with middleware
   - `wrapHandler()` - Middleware to extract SPIFFE ID
   - Identity extraction utilities:
     - `GetSPIFFEID()` - Extracts client SPIFFE ID from context
     - `MustGetSPIFFEID()` - Panics if ID not present
     - `GetTrustDomain()` - Extracts trust domain
     - `GetPath()` - Extracts SPIFFE ID path

2. **[internal/adapters/inbound/httpapi/server_test.go](../internal/adapters/inbound/httpapi/server_test.go)**
   - Unit tests for configuration validation
   - Unit tests for identity extraction helpers
   - Example usage demonstrating handler registration

3. **[internal/adapters/inbound/httpapi/integration_test.go](../internal/adapters/inbound/httpapi/integration_test.go)**
   - `TestMTLSClientServer` - Full mTLS communication test
   - `TestMTLSClientServer_AuthorizationFailure` - Verifies wrong client ID rejected
   - `TestMTLSServer_HealthCheck` - Basic health endpoint test
   - Build tag: `//go:build integration` (requires SPIRE running)

## Key Features Implemented

### ✅ mTLS Server with Client Authentication
- Uses `workloadapi.X509Source` for automatic SVID fetching and rotation
- `tlsconfig.MTLSServerConfig` with go-spiffe authorizers
- Supports all go-spiffe built-in authorizers:
  - `AuthorizeAny()` - Any authenticated client
  - `AuthorizeID()` - Specific SPIFFE ID
  - `AuthorizeMemberOf()` - Trust domain membership
  - `AuthorizeOneOf()` - Multiple allowed IDs

### ✅ Identity Extraction Middleware
- Automatic extraction of client SPIFFE ID from TLS connection
- Adds identity to request context for handler access
- Safe extraction with error handling
- Validates TLS connection state

### ✅ Identity Extraction Utilities
- `GetSPIFFEID(r)` - Returns (id, ok) pattern
- `MustGetSPIFFEID(r)` - Panics if missing (for guaranteed contexts)
- `GetTrustDomain(r)` - Extracts trust domain from ID
- `GetPath(r)` - Extracts path component from ID

### ✅ Configuration Validation
- Required fields validated: address, socketPath, authorizer
- Reasonable timeout defaults
- Clear error messages

### ✅ Graceful Shutdown
- `Stop()` closes X509Source and shuts down HTTP server
- Proper resource cleanup

## Test Results

### Unit Tests (Without SPIRE)
```bash
$ go test ./internal/adapters/inbound/httpapi -v
=== RUN   TestNewHTTPServer_MissingAddress
--- PASS: TestNewHTTPServer_MissingAddress (0.00s)
=== RUN   TestNewHTTPServer_MissingSocketPath
--- PASS: TestNewHTTPServer_MissingSocketPath (0.00s)
=== RUN   TestNewHTTPServer_MissingAuthorizer
--- PASS: TestNewHTTPServer_MissingAuthorizer (0.00s)
=== RUN   TestGetSPIFFEID_Present
--- PASS: TestGetSPIFFEID_Present (0.00s)
=== RUN   TestGetSPIFFEID_NotPresent
--- PASS: TestGetSPIFFEID_NotPresent (0.00s)
=== RUN   TestMustGetSPIFFEID_Present
--- PASS: TestMustGetSPIFFEID_Present (0.00s)
=== RUN   TestMustGetSPIFFEID_Panics
--- PASS: TestMustGetSPIFFEID_Panics (0.00s)
=== RUN   TestGetTrustDomain_Present
--- PASS: TestGetTrustDomain_Present (0.00s)
=== RUN   TestGetTrustDomain_NotPresent
--- PASS: TestGetTrustDomain_NotPresent (0.00s)
=== RUN   TestGetPath_Present
--- PASS: TestGetPath_Present (0.00s)
=== RUN   TestGetPath_NotPresent
--- PASS: TestGetPath_NotPresent (0.00s)
PASS
```

**Coverage**: 41.1% of statements

### Integration Tests (Requires SPIRE)
```bash
$ go test -tags=integration ./internal/adapters/inbound/httpapi -v
# Tests require SPIRE agent running
# To run: make minikube-up && make register-mtls-workloads
```

## Usage Example

```go
package main

import (
    "context"
    "fmt"
    "net/http"

    "github.com/pocket/hexagon/spire/internal/adapters/inbound/httpapi"
    "github.com/spiffe/go-spiffe/v2/spiffeid"
    "github.com/spiffe/go-spiffe/v2/spiffetls/tlsconfig"
)

func main() {
    ctx := context.Background()

    // Create server with authentication (any client from trust domain)
    authorizer := tlsconfig.AuthorizeMemberOf(
        spiffeid.RequireTrustDomainFromString("example.org"),
    )

    server, err := httpapi.NewHTTPServer(
        ctx,
        ":8443",
        "unix:///tmp/spire-agent/public/api.sock",
        authorizer,
    )
    if err != nil {
        panic(err)
    }
    defer server.Stop(ctx)

    // Register handler that uses client identity
    server.RegisterHandler("/api/hello", func(w http.ResponseWriter, r *http.Request) {
        clientID, ok := httpapi.GetSPIFFEID(r)
        if !ok {
            http.Error(w, "No client identity", http.StatusInternalServerError)
            return
        }

        // Authentication done - application performs authorization
        fmt.Fprintf(w, "Hello, %s!\n", clientID.String())
    })

    server.Start(ctx)
    select {} // Block forever
}
```

## Architecture Compliance

### ✅ Authentication Only (No Authorization)
- Uses only go-spiffe built-in authorizers
- Exposes authenticated identity to handlers
- Application decides what authenticated identity can do

### ✅ Identity Extraction
- Client SPIFFE ID available in request context
- Safe extraction with error handling
- Helpers for common operations (trust domain, path)

### ✅ Automatic Certificate Rotation
- `X509Source` handles SVID rotation automatically
- Zero-downtime certificate updates
- No manual certificate management

### ✅ Standard HTTP Interface
- Compatible with `http.Handler` interface
- Works with standard HTTP middleware
- Familiar ServeMux pattern

## Verification Commands

```bash
# Build all packages
go build ./...

# Run unit tests
go test ./internal/adapters/inbound/httpapi -v

# Check test coverage
go test -cover ./internal/adapters/inbound/httpapi

# Run integration tests (requires SPIRE)
go test -tags=integration ./internal/adapters/inbound/httpapi -v
```

## Iteration 1 Checklist

- [x] Create HTTP server with mTLS configuration
- [x] Implement middleware to extract SPIFFE ID
- [x] Add context helper functions for SPIFFE ID access
- [x] Add identity extraction utilities (GetTrustDomain, GetPath)
- [x] Write unit tests with mock contexts
- [x] Write integration tests with real SPIRE
- [x] Configuration validation
- [x] Graceful shutdown
- [x] Documentation and examples

## Next Steps: Iteration 2

See [MTLS_IMPLEMENTATION.md](MTLS_IMPLEMENTATION.md) for Iteration 2:
- **mTLS HTTP Client (Outbound Adapter)**
- Create `httpclient` with SVID presentation
- Implement server identity verification
- Add standard HTTP methods (Get, Post, Do)
- Integration tests with server

## References

- [MTLS_IMPLEMENTATION.md](MTLS_IMPLEMENTATION.md) - Implementation plan
- [go-spiffe SDK](https://github.com/spiffe/go-spiffe)
- [SPIFFE Specification](https://github.com/spiffe/spiffe)
