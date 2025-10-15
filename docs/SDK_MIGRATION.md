# go-spiffe SDK Integration Guide

> **✅ STATUS: Migration Complete!**
> The mTLS adapters (`httpapi` and `httpclient`) already use the real `go-spiffe` SDK v2.6.0.
> This guide explains the dual-mode architecture: production SDK vs educational in-memory implementation.

## Overview

This codebase demonstrates **two implementations** side-by-side:

1. **Production mTLS** (`httpapi`/`httpclient`) - Uses real `go-spiffe` SDK v2.6.0
2. **Educational demo** (`cmd/agent`/`cmd/workload`) - Uses in-memory implementation

The `IdentityClient` interface abstracts SPIRE Workload API operations, allowing both implementations to coexist through hexagonal architecture.

## Current Architecture

### Client-Side Interface (What Workloads Use)

**Location**: `internal/ports/inbound.go`

```go
// IdentityClient is the main entrypoint for workloads to fetch their SVID
// Matches go-spiffe SDK's workloadapi.Client for seamless transition
type IdentityClient interface {
    // FetchX509SVID fetches an X.509 SVID for the calling workload
    // Signature matches: (*workloadapi.Client).FetchX509SVID(ctx) (*x509svid.SVID, error)
    FetchIdentity(ctx context.Context) (*dto.Identity, error)
}
```

**Design Decisions**:
- ✅ No `callerIdentity` parameter - extracted automatically from connection
- ✅ Context-only signature matches SDK exactly
- ✅ Returns `*dto.Identity` (transport DTO wrapping domain objects)

### Server-Side Service (Internal Implementation)

**Location**: `internal/app/workload_api.go`

```go
// IdentityClientService implements server-side SVID issuance logic
// The server adapter extracts credentials and calls this service
type IdentityClientService struct {
    agent ports.Agent
}

func (s *IdentityClientService) IssueIdentity(
    ctx context.Context,
    workload *domain.Workload,
) (*dto.Identity, error)
```

**Architecture**:
1. Server adapter extracts Unix socket peer credentials (UID/PID/GID)
2. Creates `domain.Workload` object with extracted credentials
3. Calls `IssueIdentity()` with workload information
4. Service delegates to agent for attestation → matching → issuance

## Migration Path

### Phase 1: Current In-Memory Implementation

**Client**: `internal/adapters/outbound/workloadapi/client.go`
- HTTP client over Unix domain socket
- Sends credentials in headers (demonstration only)
- Implements `IdentityClient` interface

**Server**: `internal/adapters/inbound/workloadapi/server.go`
- HTTP server on Unix socket
- Extracts credentials from headers (demonstration only)
- Production would use SO_PEERCRED syscall

### Phase 2: Migrate to Real go-spiffe SDK

#### Step 1: Add Dependency

```bash
go get github.com/spiffe/go-spiffe/v2
```

#### Step 2: Create SDK Adapter (Client-Side)

**Location**: `internal/adapters/outbound/spiffe/client.go`

```go
package spiffe

import (
    "context"

    "github.com/spiffe/go-spiffe/v2/svid/x509svid"
    "github.com/spiffe/go-spiffe/v2/workloadapi"

    "github.com/pocket/hexagon/spire/internal/ports"
)

// SDKIdentityClient wraps go-spiffe SDK's workloadapi.Client
type SDKIdentityClient struct {
    client *workloadapi.Client
}

// NewSDKIdentityClient creates a new SDK-based identity client
func NewSDKIdentityClient(ctx context.Context, opts ...workloadapi.ClientOption) (*SDKIdentityClient, error) {
    // Default option: connect to Unix socket
    if len(opts) == 0 {
        opts = []workloadapi.ClientOption{
            workloadapi.WithAddr("unix:///tmp/spire-agent/public/api.sock"),
        }
    }

    client, err := workloadapi.New(ctx, opts...)
    if err != nil {
        return nil, err
    }

    return &SDKIdentityClient{client: client}, nil
}

// FetchIdentity implements ports.IdentityClient interface
func (c *SDKIdentityClient) FetchIdentity(ctx context.Context) (*dto.Identity, error) {
    // Call real SDK method
    svid, err := c.client.FetchX509SVID(ctx)
    if err != nil {
        return nil, err
    }

    // Convert SDK SVID to our domain type
    return convertSDKSVIDToIdentity(svid), nil
}

// Close closes the SDK client connection
func (c *SDKIdentityClient) Close() error {
    return c.client.Close()
}

// convertSDKSVIDToIdentity converts SDK types to domain types
func convertSDKSVIDToIdentity(svid *x509svid.SVID) *dto.Identity {
    // Parse SPIFFE ID to domain IdentityCredential
    identityCredential := domain.NewIdentityCredentialFromURI(svid.ID.String())

    // Build domain IdentityDocument
    doc := domain.NewIdentityDocumentFromComponents(
        identityCredential,
        svid.Certificates[0],      // leaf cert
        svid.PrivateKey,            // private key
        svid.Certificates[1:],      // CA chain
        svid.Certificates[0].NotAfter, // expiration
    )

    return &dto.Identity{
        IdentityCredential: identityCredential,
        IdentityDocument:   doc,
    }
}
```

#### Step 3: Wire SDK Client in Workload Command

**Location**: `cmd/workload/main.go`

```go
package main

import (
    "context"
    "fmt"
    "log"
    "os"

    "github.com/pocket/hexagon/spire/internal/adapters/outbound/spiffe"
)

func main() {
    ctx := context.Background()

    // Use environment variable to switch implementations
    useSDK := os.Getenv("USE_SDK") == "true"

    var client ports.IdentityClient
    if useSDK {
        // Real SDK implementation
        sdkClient, err := spiffe.NewSDKIdentityClient(ctx)
        if err != nil {
            log.Fatalf("Failed to create SDK client: %v", err)
        }
        defer sdkClient.Close()
        client = sdkClient
    } else {
        // In-memory implementation
        client = workloadapi.NewClient("/tmp/spire-agent/public/api.sock")
    }

    // Same code works for both implementations!
    identity, err := client.FetchIdentity(ctx)
    if err != nil {
        log.Fatalf("Failed to fetch identity: %v", err)
    }

    fmt.Printf("SPIFFE ID: %s\n", identity.IdentityCredential.String())
}
```

#### Step 4: Server-Side (Use Real SPIRE Server)

For server-side, you would:
1. Run real SPIRE Server binary
2. Run real SPIRE Agent binary (connects to server)
3. Remove `cmd/agent/` (or keep as demonstration)
4. Configure real agent with registration entries

## Type Mapping

| Application Type | go-spiffe SDK Type | Notes |
|-----------------------|-------------------|-------|
| `dto.Identity` | `x509svid.SVID` | Transport DTO containing domain objects |
| `domain.IdentityCredential` | `spiffeid.ID` | SPIFFE ID parsing/validation |
| `domain.TrustDomain` | `spiffeid.TrustDomain` | Trust domain validation |
| `domain.Workload` | N/A (server-side only) | Used for attestation |
| `domain.IdentityDocument` | X.509 cert + key | Rich domain object wrapping crypto material |

## Environment Variables

**Current In-Memory**:
- `SPIRE_AGENT_SOCKET`: Path to Unix socket (default: `/tmp/spire-agent/public/api.sock`)
- `IDP_MODE=inmem`: Use in-memory implementation

**With SDK**:
- `SPIFFE_ENDPOINT_SOCKET`: Standard SDK env var for socket path
- `USE_SDK=true`: Switch to real SDK implementation

## Testing Strategy

### Unit Tests

```go
func TestFetchIdentity(t *testing.T) {
    // Mock IdentityClient for testing
    mockClient := &MockIdentityClient{
        identity: &dto.Identity{
            IdentityCredential: testCredential,
            IdentityDocument: testDoc,
        },
    }

    // Test code works with interface
    identity, err := mockClient.FetchIdentity(context.Background())
    require.NoError(t, err)
    assert.Equal(t, testCredential, identity.IdentityCredential)
}
```

### Integration Tests

```go
func TestRealSDKIntegration(t *testing.T) {
    if os.Getenv("RUN_SDK_TESTS") != "true" {
        t.Skip("Skipping SDK integration tests")
    }

    ctx := context.Background()
    client, err := spiffe.NewSDKIdentityClient(ctx)
    require.NoError(t, err)
    defer client.Close()

    identity, err := client.FetchIdentity(ctx)
    require.NoError(t, err)
    assert.NotNil(t, identity)
}
```

## Benefits of This Approach

1. **Interface Compatibility**: `IdentityClient` interface exactly matches SDK signature
2. **Easy Testing**: Mock implementations for unit tests, in-memory for integration
3. **Gradual Migration**: Switch implementations via DI, no code changes in consumers
4. **Zero Lock-In**: Not coupled to in-memory implementation or SDK
5. **Production Ready**: When ready, just swap the adapter

## Implementation Status

1. ✅ Interface designed to match SDK
2. ✅ In-memory implementation working
3. ✅ SDK adapter implementation complete (`internal/adapters/outbound/spire/`)
   - `client.go` - SPIRE Workload API client wrapper
   - `agent.go` - Production agent delegating to external SPIRE
   - `server.go` - Production server using SPIRE CA certificates
   - `identity_provider.go` - X.509/JWT SVID operations
   - `bundle_provider.go` - Trust bundle management
   - `attestor.go` - Workload attestation via SPIRE
4. ✅ Type conversion utilities complete (`translation.go`)
   - Domain ↔ go-spiffe SDK type conversions
   - Uses `spiffeid` package for SPIFFE ID handling
   - Converts `x509svid.SVID` to domain `IdentityDocument`
5. ✅ Integration tests with real SPIRE (`integration_test.go`)
   - X.509 SVID fetching tests
   - JWT SVID validation tests
   - Bundle management tests
   - Workload attestation tests
6. ✅ Deployment configurations complete
   - Production Helm values: `deploy/values/values-prod.yaml`
   - Development Minikube setup: `infra/dev/minikube/`
   - Adapter factory: `internal/adapters/outbound/compose/spire.go`

**Status**: All planned work completed. Production SPIRE adapters fully functional.
