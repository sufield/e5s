# go-spiffe SDK Migration Guide

## Overview

This codebase uses the `IdentityClient` interface to abstract SPIRE Workload API operations. The interface is designed to match `go-spiffe` SDK's signature exactly, enabling seamless migration from in-memory implementation to real SDK.

## Current Architecture

### Client-Side Interface (What Workloads Use)

**Location**: `internal/ports/inbound.go`

```go
// IdentityClient is the main entrypoint for workloads to fetch their SVID
// Matches go-spiffe SDK's workloadapi.Client for seamless transition
type IdentityClient interface {
    // FetchX509SVID fetches an X.509 SVID for the calling workload
    // Signature matches: (*workloadapi.Client).FetchX509SVID(ctx) (*x509svid.SVID, error)
    FetchX509SVID(ctx context.Context) (*Identity, error)
}
```

**Design Decisions**:
- ✅ No `callerIdentity` parameter - extracted automatically from connection
- ✅ Context-only signature matches SDK exactly
- ✅ Returns `*Identity` (maps to `*x509svid.SVID` in SDK)

### Server-Side Service (Internal Implementation)

**Location**: `internal/app/workload_api.go`

```go
// IdentityClientService implements server-side SVID issuance logic
// The server adapter extracts credentials and calls this service
type IdentityClientService struct {
    agent ports.Agent
}

func (s *IdentityClientService) FetchX509SVIDForCaller(
    ctx context.Context,
    callerIdentity ports.ProcessIdentity,
) (*ports.Identity, error)
```

**Architecture**:
1. Server adapter extracts Unix socket peer credentials (UID/PID/GID)
2. Calls `FetchX509SVIDForCaller()` with extracted credentials
3. Service delegates to agent for attestation → matching → issuance

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

// FetchX509SVID implements ports.IdentityClient interface
func (c *SDKIdentityClient) FetchX509SVID(ctx context.Context) (*ports.Identity, error) {
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
func convertSDKSVIDToIdentity(svid *x509svid.SVID) *ports.Identity {
    // TODO: Map x509svid.SVID fields to ports.Identity
    // - svid.ID.String() → IdentityNamespace
    // - svid.Certificates → IdentityDocument
    // - svid.PrivateKey → IdentityDocument
    return &ports.Identity{
        // ... mapping logic
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
    svid, err := client.FetchX509SVID(ctx)
    if err != nil {
        log.Fatalf("Failed to fetch SVID: %v", err)
    }

    fmt.Printf("SPIFFE ID: %s\n", svid.IdentityNamespace.String())
}
```

#### Step 4: Server-Side (Use Real SPIRE Server)

For server-side, you would:
1. Run real SPIRE Server binary
2. Run real SPIRE Agent binary (connects to server)
3. Remove `cmd/agent/` (or keep as demonstration)
4. Configure real agent with registration entries

## Type Mapping

| Walking Skeleton Type | go-spiffe SDK Type | Notes |
|-----------------------|-------------------|-------|
| `ports.Identity` | `x509svid.SVID` | Contains cert chain + private key |
| `ports.IdentityNamespace` | `spiffeid.ID` | SPIFFE ID parsing/validation |
| `domain.TrustDomain` | `spiffeid.TrustDomain` | Trust domain validation |
| `ports.ProcessIdentity` | N/A (server-side only) | Used for attestation |

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
func TestFetchSVID(t *testing.T) {
    // Mock IdentityClient for testing
    mockClient := &MockIdentityClient{
        svid: &ports.Identity{
            IdentityNamespace: testNamespace,
            IdentityDocument: testDoc,
        },
    }

    // Test code works with interface
    svid, err := mockClient.FetchX509SVID(context.Background())
    require.NoError(t, err)
    assert.Equal(t, testNamespace, svid.IdentityNamespace)
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

    svid, err := client.FetchX509SVID(ctx)
    require.NoError(t, err)
    assert.NotNil(t, svid)
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
