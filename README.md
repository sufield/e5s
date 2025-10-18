# SPIRE mTLS Library

An mTLS authentication library using SPIFFE/SPIRE for service-to-service communication, built with hexagonal architecture.

## Overview

This is an mTLS library using the real `go-spiffe` SDK v2.6.0 for identity-based authentication. It also includes an **in-memory SPIRE implementation** for development and testing purposes.

### mTLS Library

The library provides:
- ✅ **Zero-Config API**: One-call setup with automatic socket and trust domain detection
- ✅ **Automatic Certificate Management**: Zero-downtime certificate rotation via SPIRE
- ✅ **mTLS Authentication**: Both client and server authenticate each other
- ✅ **Identity Extraction**: SPIFFE ID available to application handlers
- ✅ **Standard HTTP**: Compatible with Go's standard `http` package
- ✅ **Authentication Only**: No authorization logic - app decides access
- ✅ **Production Ready**: Comprehensive tests (unit + integration)
- ✅ **Simple API**: Structured configuration with sensible defaults
- ✅ **Thread-Safe**: Proper shutdown and resource management

### Inmemory Implementation

An in-memory SPIRE implementation demonstrates:
- SPIRE Workload API concepts
- Agent server and workload attestation flow
- Used for development and testing

**Hexagonal Architecture**: Clear separation between domain, ports, and adapters allows both implementations to coexist.

## Quick Start

### Zero-Config mTLS Server (Recommended)

The simplest way to create an mTLS server - everything is auto-detected:

```go
package main

import (
    "context"
    "fmt"
    "log"
    "net/http"
    "os/signal"
    "syscall"

    "github.com/pocket/hexagon/spire/pkg/zerotrustserver"
)

func main() {
    ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
    defer stop()

    err := zerotrustserver.Serve(ctx, map[string]http.Handler{
        "/": http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            id, ok := zerotrustserver.Identity(r.Context())
            if !ok {
                http.Error(w, "unauthorized", http.StatusUnauthorized)
                return
            }
            fmt.Fprintf(w, "Success! Authenticated as: %s\n", id.SPIFFEID)
        }),
    })
    if err != nil {
        log.Fatal(err)
    }
}
```

**What's auto-detected?**
- SPIRE agent socket (checks `SPIFFE_ENDPOINT_SOCKET` env var and common paths)
- Trust domain (extracted from workload's SVID)
- TLS configuration (enforces TLS 1.3+ with mTLS)
- Health endpoint (auto-mounted at `/health`)
- HTTP timeouts (sensible defaults)

### Advanced Configuration

For fine-grained control, use the lower-level adapter API:

```go
package main

import (
    "context"
    "fmt"
    "log"
    "net/http"
    "time"

    "github.com/pocket/hexagon/spire/internal/adapters/inbound/identityserver"
    "github.com/pocket/hexagon/spire/internal/ports"
)

func main() {
    ctx := context.Background()

    // Configure the mTLS server
    var cfg ports.MTLSConfig
    cfg.WorkloadAPI.SocketPath = "unix:///tmp/spire-agent/public/api.sock"
    cfg.SPIFFE.AllowedPeerID = "spiffe://example.org/client"  // Or use AllowedTrustDomain
    cfg.HTTP.Address = ":8443"
    cfg.HTTP.ReadHeaderTimeout = 10 * time.Second
    cfg.HTTP.WriteTimeout = 30 * time.Second

    // Create the mTLS server
    server, err := identityserver.New(ctx, cfg)
    if err != nil {
        log.Fatalf("Failed to create server: %v", err)
    }
    defer server.Close()

    // Register handlers
    server.Handle("/api/hello", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        id, ok := ports.IdentityFrom(r.Context())
        if !ok {
            http.Error(w, "Unauthorized", http.StatusUnauthorized)
            return
        }
        fmt.Fprintf(w, "Hello, %s!\n", id.SPIFFEID)
    }))

    log.Println("Server listening on :8443")

    // Start server (blocks until shutdown)
    if err := server.Start(ctx); err != nil {
        log.Fatalf("Server error: %v", err)
    }
}
```

### mTLS Client

> **Note**: The `httpclient` adapter is planned but not yet implemented. The example below shows raw SDK usage.

```go
package main

import (
    "context"
    "crypto/tls"
    "fmt"
    "io"
    "log"
    "net/http"
    "time"

    "github.com/spiffe/go-spiffe/v2/spiffeid"
    "github.com/spiffe/go-spiffe/v2/spiffetls/tlsconfig"
    "github.com/spiffe/go-spiffe/v2/workloadapi"
)

func main() {
    ctx := context.Background()

    // Create X.509 source for automatic SVID rotation
    x509Source, err := workloadapi.NewX509Source(
        ctx,
        workloadapi.WithClientOptions(workloadapi.WithAddr("unix:///tmp/spire-agent/public/api.sock")),
    )
    if err != nil {
        log.Fatalf("Failed to create X.509 source: %v", err)
    }
    defer x509Source.Close()

    // Authorize specific server SPIFFE ID
    serverID, err := spiffeid.FromString("spiffe://example.org/server")
    if err != nil {
        log.Fatalf("Failed to parse server SPIFFE ID: %v", err)
    }

    // Create mTLS HTTP client
    tlsConfig := tlsconfig.MTLSClientConfig(x509Source, x509Source, tlsconfig.AuthorizeID(serverID))
    tlsConfig.MinVersion = tls.VersionTLS13 // Enforce TLS 1.3
    httpClient := &http.Client{
        Transport: &http.Transport{
            TLSClientConfig: tlsConfig,
        },
        Timeout: 10 * time.Second,
    }

    // Make request
    // Note: SPIFFE authentication verifies the server's SPIFFE ID (via AuthorizeID),
    // not the DNS hostname. Using "localhost" here is fine.
    resp, err := httpClient.Get("https://localhost:8443/api/hello")
    if err != nil {
        log.Fatalf("Request failed: %v", err)
    }
    defer resp.Body.Close()

    body, _ := io.ReadAll(resp.Body)
    fmt.Printf("Response: %s\n", body)
}
```

### Configuration Options

```go
// MTLSConfig holds all configuration
type MTLSConfig struct {
    WorkloadAPI WorkloadAPIConfig
    SPIFFE      SPIFFEConfig
    HTTP        HTTPConfig
}

// WorkloadAPI configuration
type WorkloadAPIConfig struct {
    SocketPath string // e.g., "unix:///tmp/spire-agent/public/api.sock"
}

// SPIFFE authorization configuration
type SPIFFEConfig struct {
    AllowedPeerID      string // Exact SPIFFE ID match (e.g., "spiffe://example.org/client")
    AllowedTrustDomain string // Any ID in trust domain (e.g., "example.org")
}

// HTTP server configuration
type HTTPConfig struct {
    Address           string        // Server address (e.g., ":8443")
    ReadHeaderTimeout time.Duration // Prevents Slowloris attacks (default: 10s)
    ReadTimeout       time.Duration // Default: 30s
    WriteTimeout      time.Duration // Default: 30s
    IdleTimeout       time.Duration // Default: 120s
}
```

#### Configuration Precedence and Validation

**Authorization Policy** (`SPIFFEConfig`):
- **Exactly one** of `AllowedPeerID` or `AllowedTrustDomain` must be set
- Both empty: Returns validation error (deny-all not supported)
- Both set: Returns validation error (ambiguous policy)
- `AllowedPeerID`: Exact match against a specific SPIFFE ID (e.g., `spiffe://example.org/client`)
- `AllowedTrustDomain`: Allow any ID in the trust domain (e.g., any `spiffe://example.org/*`)

**Socket Path** (`WorkloadAPIConfig.SocketPath`):
- Must be non-empty
- Must use `unix://` scheme (e.g., `unix:///tmp/spire-agent/public/api.sock`)
- Invalid scheme returns error

**HTTP Timeouts** (`HTTPConfig`):
- All timeouts are optional; adapters apply sensible defaults if unset/zero
- Defaults (from `internal/config`):
  - `ReadHeaderTimeout`: 10 seconds (prevents Slowloris)
  - `ReadTimeout`: 30 seconds
  - `WriteTimeout`: 30 seconds
  - `IdleTimeout`: 120 seconds

## Architecture

### Directory Structure

```
pkg/
└── zerotrustserver/     # Zero-config mTLS server API (public)
    ├── server.go        # Serve() - one-call server
    ├── defaults.go      # Auto-detection logic
    ├── identity.go      # Identity helper
    └── doc.go           # Package documentation

internal/
├── domain/              # Domain entities (TrustDomain, IdentityCredential, etc.)
├── ports/               # Port interfaces (contracts between layers)
│   ├── inbound.go       # IdentityProvider, CLI interfaces
│   ├── outbound.go      # Agent, parsers, validators, factories
│   ├── identityserver.go # MTLSServer, MTLSClient, MTLSConfig
│   └── types.go         # Shared types (Identity, ProcessIdentity, etc.)
├── app/                 # Application services (business logic)
├── config/              # Configuration (YAML + env fallback)
├── controlplane/        # Control plane for SPIRE deployment
└── adapters/            # Infrastructure implementations
    ├── inbound/
    │   ├── identityserver/ # Production mTLS server (go-spiffe SDK)
    │   └── cli/            # CLI demonstration
    └── outbound/
        ├── spire/          # Production SPIRE adapters (go-spiffe SDK)
        ├── inmemory/       # In-memory SPIRE implementation (dev/learning)
        └── compose/        # Dependency injection factory

cmd/
├── main.go              # CLI demonstration tool (uses in-memory)
├── main_prod.go         # Production entrypoint (uses real SPIRE)
└── cp-minikube/         # Control plane for Minikube deployment

examples/
├── zeroconfig-example/  # Zero-config server example (recommended)
├── test-client.go       # Infrastructure testing tool (verifies SPIRE setup and mTLS)
├── mtls-server.yaml     # Kubernetes deployment manifest
├── test-client.yaml     # Test client deployment manifest
└── README.md            # Kubernetes deployment guide
```

### Hexagonal Architecture

```
┌─────────────────────────────────────────────────────────┐
│                    Inbound Adapters                      │
│  ┌────────────────┐              ┌─────────────────┐    │
│  │IdentityServer  │              │ CLI Demo        │    │
│  │ (mTLS HTTP)    │              │ Adapter         │    │
│  └────────┬───────┘              └────────┬────────┘    │
│           │                               │             │
│           └───────────────┬───────────────┘             │
│                           │                             │
│  ┌────────────────────────▼─────────────────────────┐   │
│  │              Ports (Interfaces)                   │   │
│  │  • MTLSServer     • MTLSClient                   │   │
│  │  • Agent          • IdentityProvider             │   │
│  │  • Parsers        • Validators                   │   │
│  └────────────────────────┬─────────────────────────┘   │
│                           │                             │
│  ┌────────────────────────▼─────────────────────────┐   │
│  │              Domain Entities                      │   │
│  │  • TrustDomain  • IdentityCredential              │   │
│  │  • IdentityDocument  • Selector                  │   │
│  └───────────────────────────────────────────────────┘   │
│                           │                             │
│  ┌────────────────────────▼─────────────────────────┐   │
│  │            Outbound Adapters                      │   │
│  │  • SPIREAgent     • HTTPClient                   │   │
│  │  • InMemoryAgent  • InMemoryServer (dev)         │   │
│  └───────────────────────────────────────────────────┘   │
└─────────────────────────────────────────────────────────┘
```

## Interfaces

### MTLSServer (Production Interface)

**Location**: `internal/ports/identityserver.go`

```go
// MTLSServer is the stable interface for an mTLS HTTP server.
// It provides identity-based authentication using SPIFFE/SPIRE.
type MTLSServer interface {
    // Handle registers an HTTP handler
    // Handlers receive requests with authenticated SPIFFE ID in context
    Handle(pattern string, handler http.Handler) error

    // Start begins serving HTTPS with identity-based mTLS (blocks until shutdown)
    Start(ctx context.Context) error

    // Shutdown gracefully stops the server, waiting for active connections
    Shutdown(ctx context.Context) error

    // Close releases resources (X509Source, connections, etc.)
    Close() error
}
```

### MTLSClient (Production Interface)

**Location**: `internal/ports/identityserver.go`

```go
// MTLSClient is the stable interface for an mTLS HTTP client.
// It provides identity-based authentication and server verification using SPIFFE/SPIRE.
type MTLSClient interface {
    // Do executes an HTTP request using identity-based mTLS
    Do(ctx context.Context, req *http.Request) (*http.Response, error)

    // Close releases resources (X509Source, connections, etc.)
    Close() error
}
```

### Identity Extraction

**Location**: `internal/ports/identity.go`

Handlers access authenticated identity using port-level abstractions:

```go
// Identity represents an authenticated workload identity (port-level abstraction)
type Identity struct {
    SPIFFEID    string  // e.g., "spiffe://example.org/client"
    TrustDomain string  // e.g., "example.org"
    Path        string  // e.g., "/client"
}

// IdentityFrom retrieves the Identity from the request context
// Returns (identity, true) if present, (zero, false) otherwise
func IdentityFrom(ctx context.Context) (Identity, bool)

// WithIdentity stores an Identity in the context (used by adapters)
func WithIdentity(ctx context.Context, id Identity) context.Context
```

The adapter automatically injects `ports.Identity` into the request context during mTLS authentication. Handlers depend on ports, not on adapter-specific code.

## Domain Entities

### IdentityCredential (SPIFFE ID)

**Location**: `internal/domain/identity_credential.go`

```go
// IdentityCredential represents a SPIFFE ID: spiffe://<trust-domain>/<path>
type IdentityCredential struct {
    trustDomain *TrustDomain
    path        string
}
```

**Examples**:
- `spiffe://example.org/host` (agent)
- `spiffe://example.org/server` (server workload)
- `spiffe://example.org/client` (client workload)

### IdentityDocument (SVID)

**Location**: `internal/domain/identity_document.go`

```go
// IdentityDocument represents an X.509 SVID
type IdentityDocument struct {
    identityCredential *IdentityCredential
    certificate        *x509.Certificate
    privateKey         crypto.PrivateKey // ⚠️ Planned for removal (see TODO in source)
    certificateChain   []*x509.Certificate
    expiresAt          time.Time
}
```

**Why X.509-only?** Focus on simplicity and the primary use case (mTLS). JWT can be added via adapters if needed without changing the domain model.

> **Note**: The `privateKey` field is planned for removal to keep the domain entity purely descriptive. Private keys will be managed by adapters. See TODO in `identity_document.go` for migration plan.

### Selector

**Location**: `internal/domain/selector.go`

```go
// Selector represents a workload attribute used for attestation
// Format: type:key:value
type Selector struct {
    selectorType SelectorType // e.g., "unix" | "workload" | "k8s"
    key          string       // e.g., "uid", "namespace"
    value        string       // e.g., "1000" (value MAY contain colons)
    formatted    string       // Cached "type:key:value" representation
}
```

**Examples**:
- `unix:uid:1000` → type="unix", key="uid", value="1000"
- `k8s:namespace:production` → type="k8s", key="namespace", value="production"
- `k8s:pod:ns:default:name` → type="k8s", key="pod", value="ns:default:name" (multi-colon value)

## Testing

The project has comprehensive test coverage with both unit and integration tests. See [docs/TEST_ARCHITECTURE.md](docs/TEST_ARCHITECTURE.md) for complete testing guide.

### Quick Test Commands

```bash
# Run unit tests (fast, no dependencies)
make test

# Run integration tests (automatic - checks SPIRE, registers workloads, runs tests)
make minikube-up         # Start SPIRE infrastructure (once)
make test-integration    # Run integration tests

# Run all tests with coverage
go test -cover ./internal/...
```

### Unit Tests

Mock the interfaces:

```go
// Mock MTLSServer for testing
type MockMTLSServer struct {
    handlers map[string]http.Handler
}

func (m *MockMTLSServer) Handle(pattern string, handler http.Handler) error {
    m.handlers[pattern] = handler
    return nil
}
```

### Integration Tests

Use real SPIRE:

```go
func TestMTLSAuthentication(t *testing.T) {
    ctx := context.Background()

    // Create server
    var serverCfg ports.MTLSConfig
    serverCfg.WorkloadAPI.SocketPath = "unix:///tmp/spire-agent/public/api.sock"
    serverCfg.SPIFFE.AllowedTrustDomain = "example.org"
    serverCfg.HTTP.Address = ":8443"
    server, err := identityserver.New(ctx, serverCfg)
    require.NoError(t, err)
    defer server.Close()

    // Register handler
    server.Handle("/test", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        id, ok := ports.IdentityFrom(r.Context())
        if !ok {
            http.Error(w, "Unauthorized", http.StatusUnauthorized)
            return
        }
        fmt.Fprintf(w, "Hello, %s", id.SPIFFEID)
    }))

    // Start server in goroutine (blocks until shutdown)
    go func() {
        server.Start(ctx)
    }()

    // Create client (using raw SDK until httpclient adapter is implemented)
    x509Source, err := workloadapi.NewX509Source(ctx, workloadapi.WithClientOptions(workloadapi.WithAddr("unix:///tmp/spire-agent/public/api.sock")))
    require.NoError(t, err)
    defer x509Source.Close()

    trustDomain, err := spiffeid.TrustDomainFromString("example.org")
    require.NoError(t, err)

    tlsConfig := tlsconfig.MTLSClientConfig(x509Source, x509Source, tlsconfig.AuthorizeMemberOf(trustDomain))
    tlsConfig.MinVersion = tls.VersionTLS13 // Enforce TLS 1.3
    httpClient := &http.Client{
        Transport: &http.Transport{
            TLSClientConfig: tlsConfig,
        },
        Timeout: 10 * time.Second,
    }

    // Make request
    resp, err := httpClient.Get("https://localhost:8443/test")
    require.NoError(t, err)
    defer resp.Body.Close()

    assert.Equal(t, http.StatusOK, resp.StatusCode)
}
```

## Documentation

### Core Documentation

- [docs/PROJECT_STATUS.md](docs/PROJECT_STATUS.md) - Current state: Production vs Educational
- [docs/TEST_ARCHITECTURE.md](docs/TEST_ARCHITECTURE.md) - Testing strategy and best practices
- [docs/CONTROL_PLANE.md](docs/CONTROL_PLANE.md) - SPIRE deployment and control plane
- [docs/ARCHITECTURE_REVIEW.md](docs/ARCHITECTURE_REVIEW.md) - Port placement and design decisions

### Examples

- [examples/README.md](examples/README.md) - Kubernetes/Minikube deployment guide
- [examples/zeroconfig-example/](examples/zeroconfig-example/) - Zero-config server (recommended for all users)

## Running the Examples

> **For Kubernetes/Minikube deployment**: See [examples/README.md](examples/README.md) for a complete guide on deploying to Kubernetes with SPIRE.

### Prerequisites

- Go 1.25.1 or higher
- SPIRE Agent running locally (for production examples)
- Or Minikube with SPIRE (for integration tests)

### Run Zero-Config Server Example

```bash
# The zero-config example auto-detects everything
go run ./examples/zeroconfig-example

# SPIRE agent must be running and accessible via:
# - SPIFFE_ENDPOINT_SOCKET env var, or
# - Common paths: /tmp/spire-agent/public/api.sock, /var/run/spire/sockets/agent.sock

# Output:
# Server starting on :8443 with zero-trust mTLS
# Auto-detected socket: unix:///tmp/spire-agent/public/api.sock
# Auto-detected trust domain: example.org
# Server listening on :8443
```

### Run Infrastructure Testing Tool

The `examples/test-client.go` tool verifies that SPIRE infrastructure is working correctly:

```bash
# Run the infrastructure testing tool
go run ./examples/test-client.go

# What it does:
# 1. Connects to SPIRE Workload API
# 2. Obtains client X.509 SVID
# 3. Tests mTLS connectivity to the server
# 4. Reports results for each endpoint

# When to use:
# - After deploying SPIRE to Kubernetes
# - To verify workload registration is correct
# - For troubleshooting mTLS connectivity issues
# - As a reference for building SPIFFE clients

# See examples/README.md for full Kubernetes deployment guide
```

### Run CLI Demo (In-Memory)

```bash
# Run full demonstration using in-memory SPIRE
IDP_MODE=inmem go run ./cmd

# This uses the in-memory implementation for learning purposes
```

## Design Decisions

### 1. Hexagonal Architecture

Consists of domain, port interfaces, swappable adapters:
- Production implementation uses real `go-spiffe` SDK
- In-memory implementation for development/testing
- No domain coupling to infrastructure

### 2. Config Structs over Multiple Parameters

```go
// ✅ Good: Grouped parameters with defaults
server, err := identityserver.New(ctx, ports.MTLSConfig{
    WorkloadAPI: ports.WorkloadAPIConfig{
        SocketPath: socketPath,
    },
    SPIFFE: ports.SPIFFEConfig{
        AllowedPeerID: "spiffe://example.org/client",
    },
    HTTP: ports.HTTPConfig{
        Address: ":8443",
    },
})

// ❌ Bad: Too many parameters
server, err := NewServer(ctx, socketPath, allowedID, ":8443", 10*time.Second, ...)
```

### 3. Separate Shutdown and Close

```go
// Graceful shutdown
shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
defer cancel()
server.Shutdown(shutdownCtx)  // Wait for connections to drain

// Release resources
server.Close()  // Close X509Source, sockets, etc.
```

### 4. Authentication Only (No Authorization)

The library only authenticates clients via SPIFFE IDs. Authorization decisions are left to the application:

```go
server.Handle("/admin", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
    id, ok := ports.IdentityFrom(r.Context())
    if !ok {
        http.Error(w, "Unauthorized", http.StatusUnauthorized)
        return
    }

    // Application decides access control
    if !isAdmin(id.SPIFFEID) {
        http.Error(w, "Forbidden", http.StatusForbidden)
        return
    }

    // Handle admin request
}))
```

## Quality and Best Practices

This implementation follows Go best practices and production-ready patterns:

1. **✅ Config Structs**: APIs use config structs for maintainability
2. **✅ Proper Validation**: Required fields validated with clear error messages
3. **✅ Resource Management**: Proper cleanup with defer, separate Shutdown/Close
4. **✅ Thread Safety**: Mutex protects shared state, sync.Once for initialization
5. **✅ Graceful Shutdown**: Separate shutdown context with timeout
6. **✅ Error Wrapping**: Context preserved with `fmt.Errorf("%w", err)`
7. **✅ Test Coverage**: Unit tests (validation) + Integration tests (mTLS)
8. **✅ Documentation**: Inline docs, comprehensive guides, examples

### Security Considerations

1. **mTLS Required**: All connections must use mutual TLS
2. **Identity-Based**: Authentication via SPIFFE IDs, not passwords
3. **Certificate Rotation**: Automatic via SPIRE (zero downtime)
4. **No Authorization**: Library only authenticates - app decides access
5. **Timeout Configuration**: All operations have configurable timeouts
6. **TLS 1.3**: Minimum TLS version enforced; cipher suites negotiated per TLS 1.3
7. **SPIFFE Verification**: Server identity verified via SPIFFE ID, not DNS hostname

## SPIRE Integration

The project uses the real `go-spiffe` SDK v2.6.0 for production deployments:

**Public APIs**:
- ✅ `pkg/zerotrustserver` - Zero-config mTLS server (recommended for most users)

**Production adapters**:
- ✅ `internal/adapters/inbound/identityserver` - mTLS server using go-spiffe SDK
- ✅ `internal/adapters/outbound/spire` - SPIRE Workload API client adapters
- ✅ Integration tests - Full mTLS with real SPIRE agent
- ⏳ `internal/adapters/outbound/httpclient` - mTLS HTTP client (planned)

**Development adapters**:
- `internal/adapters/outbound/inmemory` - In-memory SPIRE implementation for learning
- Used by `cmd/main.go` for CLI demonstrations

## References

- [SPIFFE Specification](https://github.com/spiffe/spiffe)
- [SPIRE Documentation](https://spiffe.io/docs/latest/spire/)
- [go-spiffe SDK](https://github.com/spiffe/go-spiffe)
- [Hexagonal Architecture](https://alistair.cockburn.us/hexagonal-architecture/)
