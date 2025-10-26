# Identity Based Authentication Library

An identity based authentication library using SPIFFE/SPIRE for service-to-service communication, built with hexagonal architecture.

## Overview

This is a mTLS library using `go-spiffe` SDK v2.6.0 for identity-based authentication. It also includes an in-memory SPIRE implementation for development and testing purposes.

### Features

The library provides:
- **Zero-Config API**: One-call setup with automatic socket and trust domain detection
- **Automatic Certificate Management**: Zero-downtime certificate rotation via SPIRE
- **mTLS Authentication**: Both client and server authenticate each other
- **Identity Extraction**: SPIFFE ID available to application handlers
- **Standard HTTP**: Compatible with Go's standard `http` package
- **Authentication Only**: No authorization logic - app decides access
- **Production Ready**: Comprehensive tests (unit + integration)
- **Simple API**: Structured configuration with sensible defaults
- **Thread-Safe**: Proper shutdown and resource management

### Inmemory Implementation

An in-memory SPIRE implementation demonstrates:
- SPIRE Workload API concepts
- Agent server and workload attestation flow
- Used for development and testing

**Hexagonal Architecture**: Clear separation between domain, ports, and adapters allows both implementations to coexist.

## Quick Start

### Zero-Config mTLS Server

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

// rootHandler returns "Success!" only if the request context carries an identity.
func rootHandler(w http.ResponseWriter, r *http.Request) {
    id, ok := zerotrustserver.PeerIdentity(r.Context())
    if !ok {
        http.Error(w, "unauthorized", http.StatusUnauthorized)
        return
    }

    fmt.Fprintf(w, "Success! Authenticated as: %s\n", id.SPIFFEID)
}

func main() {
    // Cancel on SIGINT/SIGTERM for graceful shutdown.
    ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
    defer stop()

    routes := map[string]http.Handler{
        "/": http.HandlerFunc(rootHandler),
    }

    if err := zerotrustserver.Serve(ctx, routes); err != nil {
        stop() // Ensure cleanup before exit
        //nolint:gocritic // exitAfterDefer: stop() called explicitly before Fatal
        log.Fatalf("server error: %v", err)
    }
}
```

**What's auto-detected?**
- SPIRE agent socket (checks `SPIFFE_ENDPOINT_SOCKET` env var and common paths)
- Trust domain (extracted from workload's SVID)
- TLS configuration (enforces TLS 1.3+ with mTLS)
- Health endpoint (auto-mounted at `/health`)
- HTTP timeouts (sensible defaults)

### Zero-Config mTLS Client

The simplest way to create an mTLS client - just specify the server's identity:

```go
package main

import (
    "context"
    "fmt"
    "io"
    "log"

    "github.com/pocket/hexagon/spire/pkg/zerotrustclient"
)

func main() {
    ctx := context.Background()

    // Create zero-config client - only specify the server's SPIFFE ID
    client, err := zerotrustclient.New(ctx, &zerotrustclient.Config{
        ServerID: "spiffe://example.org/server",
    })
    if err != nil {
        log.Fatalf("Failed to create client: %v", err)
    }
    defer client.Close()

    // Make a GET request (hostname doesn't matter - SPIFFE ID is verified)
    resp, err := client.Get(ctx, "https://localhost:8443/api/hello")
    if err != nil {
        log.Fatalf("Request failed: %v", err)
    }
    defer resp.Body.Close()

    body, _ := io.ReadAll(resp.Body)
    fmt.Printf("Response: %s\n", body)
}
```

**What's auto-detected?**
- SPIRE agent socket (checks `SPIFFE_ENDPOINT_SOCKET` env var and common paths)
- TLS configuration (enforces TLS 1.3+ with mTLS)
- HTTP timeouts (sensible defaults: 10s read/write, 120s idle)
- Certificate rotation (automatic via SPIRE)

**Server verification options:**
```go
// Option 1: Exact server ID (recommended for production)
Config{ServerID: "spiffe://example.org/server"}

// Option 2: Accept any server in trust domain
Config{ServerTrustDomain: "example.org"}
```

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
        id, ok := ports.PeerIdentity(r.Context())
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

### Production Configuration (YAML + Environment Variables)

For production deployments, use configuration files with environment variable overrides:

**config.yaml**:
```yaml
spire:
  socket_path: unix:///tmp/spire-agent/public/api.sock
  trust_domain: example.org

http:
  address: :8443
  read_header_timeout: 10s
  read_timeout: 30s
  write_timeout: 30s
  idle_timeout: 120s
  authentication:
    peer_verification: trust-domain  # Options: any, trust-domain, specific-id, one-of
    trust_domain: example.org        # Required when peer_verification=trust-domain
    # allowed_ids:                   # Required when peer_verification=specific-id or one-of
    #   - spiffe://example.org/client
```

**Application code**:
```go
package main

import (
    "context"
    "fmt"
    "log"
    "net/http"

    "github.com/pocket/hexagon/spire/internal/config"
    "github.com/pocket/hexagon/spire/internal/adapters/inbound/identityserver"
    "github.com/pocket/hexagon/spire/internal/adapters/outbound/httpclient"
    "github.com/pocket/hexagon/spire/internal/ports"
)

func main() {
    ctx := context.Background()

    // Load configuration from file with env variable overrides
    // Supports: config.Load("config.yaml"), config.Load("-") for stdin, config.Load("") for env-only
    cfg, err := config.Load("config.yaml")
    if err != nil {
        log.Fatalf("Failed to load config: %v", err)
    }

    // Create server from config
    serverCfg := cfg.ToServerConfig()
    server, err := identityserver.New(ctx, serverCfg)
    if err != nil {
        log.Fatalf("Failed to create server: %v", err)
    }
    defer server.Close()

    // Register handlers
    server.Handle("/api/hello", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        id, ok := ports.PeerIdentity(r.Context())
        if !ok {
            http.Error(w, "Unauthorized", http.StatusUnauthorized)
            return
        }
        fmt.Fprintf(w, "Hello, %s!\n", id.SPIFFEID)
    }))

    log.Printf("Server listening on %s", cfg.HTTP.Address)

    // Start server (blocks until shutdown)
    if err := server.Start(ctx); err != nil {
        log.Fatalf("Server error: %v", err)
    }
}
```

**Environment variable overrides**:
```bash
# Override SPIRE socket path
export SPIRE_AGENT_SOCKET=unix:///var/run/spire/sockets/agent.sock

# Override SPIRE trust domain
export SPIRE_TRUST_DOMAIN=production.example.org

# Override HTTP address
export HTTP_ADDRESS=:9443

# Override authentication settings
export AUTH_PEER_VERIFICATION=specific-id
export ALLOWED_ID=spiffe://production.example.org/client

# Override timeouts
export HTTP_READ_TIMEOUT=60s
export HTTP_WRITE_TIMEOUT=60s

# Run with overrides
go run main.go
```

**Benefits**:
- **Separation of Concerns**: Config externalized from code
- **Environment-Specific**: Different configs for dev/staging/prod
- **Secret Management**: Override sensitive values via env vars or secrets manager
- **Validation**: Config is validated on load with clear error messages
- **Defaults**: Sensible defaults applied automatically
- **YAML Strictness**: Unknown keys rejected to catch typos

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

### Hexagonal Architecture Overview

This project follows **Hexagonal Architecture** (Ports & Adapters pattern):

```
┌─────────────────────────────────────────────────────────┐
│             🔵 INBOUND ADAPTERS (Drivers)                │
│           How external actors interact with us           │
├─────────────────────────────────────────────────────────┤
│  • identityserver/  → HTTP server exposing mTLS API     │
│  • cli/             → Command-line interface            │
│  • zerotrustserver/ → Zero-config API wrapper (pkg/)    │
└─────────────────────┬───────────────────────────────────┘
                      │
                 ┌────▼────┐
                 │  PORTS  │  ← Interfaces/Contracts
                 └────┬────┘
                      │
┌─────────────────────▼───────────────────────────────────┐
│              🟢 DOMAIN (Core Business Logic)             │
│                  Pure Go, No Dependencies                │
├─────────────────────────────────────────────────────────┤
│  • domain/          → Entities (TrustDomain, SVID, etc.)│
│  • app/             → Business logic & orchestration    │
└─────────────────────┬───────────────────────────────────┘
                      │
                 ┌────▼────┐
                 │  PORTS  │  ← Interfaces/Contracts
                 └────┬────┘
                      │
┌─────────────────────▼───────────────────────────────────┐
│             🟠 OUTBOUND ADAPTERS (Driven)                │
│         How we interact with external systems            │
├─────────────────────────────────────────────────────────┤
│  • spire/       → SPIRE Workload API (go-spiffe SDK)    │
│  • httpclient/  → mTLS HTTP client                      │
│  • helm/        → Kubernetes/Helm deployment (dev)      │
│  • inmemory/    → In-memory impl for testing (dev)      │
│  • compose/     → Dependency injection                  │
└─────────────────────────────────────────────────────────┘
```

**Key Principle**: Domain never depends on adapters. Adapters depend on ports.

### Directory Structure Mapped to Hexagonal Layers

```
📦 Public API Layer
pkg/
└── zerotrustserver/     # 🔵 INBOUND: Zero-config mTLS server API
    ├── server.go        #    Serve() - one-call server setup
    ├── defaults.go      #    Auto-detection (socket, trust domain)
    ├── identity.go      #    Identity extraction helper
    └── doc.go           #    Package documentation

📦 Core Application Layer
internal/
├── 🟢 DOMAIN LAYER (Pure Business Logic)
│   ├── domain/          # Entities: TrustDomain, IdentityCredential, SVID, Selector
│   └── app/             # Business logic & orchestration
│
├── ⚪ PORTS LAYER (Contracts/Interfaces)
│   └── ports/           # All interfaces that adapters must implement
│       ├── inbound.go   #   Inbound ports (IdentityProvider, CLI)
│       ├── outbound.go  #   Outbound ports (Agent, Parsers, Validators)
│       ├── identityserver.go # MTLSServer, MTLSClient interfaces
│       └── types.go     #   Shared types (Identity, ProcessIdentity)
│
├── 🔵 INBOUND ADAPTERS (How external actors interact)
│   └── adapters/inbound/
│       ├── identityserver/ # Production mTLS HTTP server (go-spiffe SDK)
│       └── cli/            # CLI demonstration adapter
│
├── 🟠 OUTBOUND ADAPTERS (How we interact with external systems)
│   └── adapters/outbound/
│       ├── spire/       # Real SPIRE Workload API (production)
│       ├── httpclient/  # mTLS HTTP client (production)
│       ├── helm/        # Kubernetes/Helm deployment (dev-only)
│       ├── inmemory/    # In-memory SPIRE (dev/testing)
│       └── compose/     # Dependency injection factory
│
└── config/              # Configuration loading (YAML + env vars)

📦 Entry Points
cmd/
├── main.go              # 🔵 INBOUND: CLI demo (dev-only, uses inmemory)
├── main_prod.go         # Production entrypoint (uses real SPIRE)
└── cp-minikube/         # 🔵 INBOUND: Minikube control plane CLI (dev-only)

📦 Examples & Deployment
examples/
├── zeroconfig-example/  # Complete working example (recommended)
│   ├── main.go          # Server using pkg/zerotrustserver
│   └── Dockerfile       # Production container image
├── test-client.go       # Infrastructure testing tool
├── mtls-server.yaml     # Kubernetes deployment manifests
└── README.md            # Deployment guide
```

**Legend:**
- 🔵 **Inbound Adapters**: External → Application (HTTP server, CLI)
- 🟢 **Domain**: Pure business logic (no external dependencies)
- ⚪ **Ports**: Interfaces between layers
- 🟠 **Outbound Adapters**: Application → External (SPIRE, Helm, HTTP client)

### Layer Dependencies (Dependency Rule)

```
┌─────────────────────────────────────────┐
│         Dependencies Flow Inward         │
│         (Outer layers depend on inner)   │
└─────────────────────────────────────────┘

Adapters ────depends on────> Ports ────depends on────> Domain

✅ Allowed:  Adapter imports Port
✅ Allowed:  Port imports Domain
❌ Forbidden: Domain imports Port
❌ Forbidden: Domain imports Adapter
❌ Forbidden: Port imports Adapter
```

**Real examples from this codebase:**

```go
// ✅ GOOD: Adapter depends on Port
package identityserver
import "github.com/pocket/hexagon/spire/internal/ports"

// ✅ GOOD: Port depends on Domain
package ports
import "github.com/pocket/hexagon/spire/internal/domain"

// ❌ BAD: Domain depending on Port (NEVER)
package domain
import "github.com/pocket/hexagon/spire/internal/ports"  // ← FORBIDDEN

// ❌ BAD: Domain depending on Adapter (NEVER)
package domain
import "github.com/pocket/hexagon/spire/internal/adapters/outbound/spire"  // ← FORBIDDEN
```

### Swappable Implementations (Why Hexagonal?)

The hexagonal architecture enables **swapping implementations** without changing domain code:

```go
// Production: Use real SPIRE
factory := compose.NewSPIREAdapterFactory(ctx, &spire.Config{
    SocketPath: "/tmp/spire-agent/public/api.sock",
})

// Development: Use in-memory (no SPIRE needed)
factory := compose.NewInMemoryAdapterFactory()

// Same domain code works with both!
application := app.Bootstrap(ctx, configLoader, factory)
```

**Benefits:**
- ✅ Test domain logic without infrastructure
- ✅ Develop locally without external dependencies
- ✅ Easy to add new adapters (e.g., Vault, AWS Secrets Manager)
- ✅ Domain remains pure and testable

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

// PeerIdentity retrieves the Identity from the request context
// Returns (identity, true) if present, (zero, false) otherwise
func PeerIdentity(ctx context.Context) (Identity, bool)

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
    certificateChain   []*x509.Certificate
    expiresAt          time.Time
}
```

**Why X.509-only?** Focus on simplicity and the primary use case (mTLS). JWT can be added via adapters if needed without changing the domain model.

**Private Key Management**: Private keys are managed separately by adapters (e.g., in `x509svid.SVID` or `dto.Identity`), not in the domain entity. This keeps the domain model purely descriptive and separates sensitive key material from identity metadata.

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
        id, ok := ports.PeerIdentity(r.Context())
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

    // Create mTLS client using httpclient adapter
    clientCfg := &ports.MTLSConfig{
        WorkloadAPI: ports.WorkloadAPIConfig{
            SocketPath: "unix:///tmp/spire-agent/public/api.sock",
        },
        SPIFFE: ports.SPIFFEConfig{
            AllowedTrustDomain: "example.org",
        },
        HTTP: ports.HTTPConfig{
            ReadTimeout:  10 * time.Second,
            WriteTimeout: 10 * time.Second,
        },
    }

    client, err := httpclient.New(ctx, clientCfg)
    require.NoError(t, err)
    defer client.Close()

    // Make request
    req, err := http.NewRequest("GET", "https://localhost:8443/test", http.NoBody)
    require.NoError(t, err)

    resp, err := client.Do(ctx, req)
    require.NoError(t, err)
    defer resp.Body.Close()

    assert.Equal(t, http.StatusOK, resp.StatusCode)
}
```

## Documentation

### Core Documentation

- [docs/PRODUCTION_VS_DEVELOPMENT.md](docs/PRODUCTION_VS_DEVELOPMENT.md) - Production vs Development architecture
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
go run -tags=dev ./cmd

# This uses the in-memory implementation for learning purposes
# No external SPIRE infrastructure required - all components run in-process
```

## Design Decisions

### 1. Hexagonal Architecture

Consists of domain, port interfaces, swappable adapters:
- Production implementation uses real `go-spiffe` SDK
- In-memory implementation for development/testing
- No domain coupling to infrastructure

### 2. Config Structs for Grouped Parameters

APIs use config structs for maintainability and extensibility. This allows adding new fields without breaking existing code and provides clear documentation of related settings.

```go
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
    id, ok := ports.PeerIdentity(r.Context())
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

1. **Config Structs**: APIs use config structs for maintainability
2. **Proper Validation**: Required fields validated with clear error messages
3. **Resource Management**: Proper cleanup with defer, separate Shutdown/Close
4. **Thread Safety**: Mutex protects shared state, sync.Once for initialization
5. **Graceful Shutdown**: Separate shutdown context with timeout
6. **Error Wrapping**: Context preserved with `fmt.Errorf("%w", err)`
7. **Test Coverage**: Unit tests (validation) + Integration tests (mTLS)
8. **Documentation**: Inline docs, comprehensive guides, examples

### Security

This project implements defense-in-depth security with multiple layers:

**Build-Time Security (Static Analysis)**:
- **gosec**: Go code security scanning (0 issues)
- **golangci-lint**: 22+ security-focused linters
- **govulncheck**: Dependency vulnerability scanning
- **Trivy**: Container image scanning

**Deploy-Time Security (Kubernetes)**:
- **Pod Security Context**: runAsNonRoot, capabilities dropped, seccomp
- **Network Policies**: mTLS-only traffic
- **RBAC**: Minimal permissions
- **Distroless Images**: Minimal attack surface

**Runtime Security (Falco)**:
- **Syscall Monitoring**: Real-time threat detection with eBPF
- **SPIRE Socket Protection**: Detect unauthorized Workload API access
- **Container Behavior Analysis**: Shell spawning, file tampering, network anomalies
- **Certificate Monitoring**: Detect unauthorized cert modifications

**Application Security (mTLS)**:
1. **mTLS Required**: All connections use mutual TLS
2. **Identity-Based**: Authentication via SPIFFE IDs, not passwords
3. **Certificate Rotation**: Automatic via SPIRE (zero downtime)
4. **No Authorization**: Library only authenticates - app decides access
5. **Timeout Configuration**: All operations have configurable timeouts
6. **TLS 1.3**: Minimum TLS version enforced
7. **SPIFFE Verification**: Server identity verified via SPIFFE ID, not DNS hostname

**Security Tools & Documentation**:
- [security/](security/) - Security tools and Falco integration
- [security/FALCO_GUIDE.md](security/FALCO_GUIDE.md) - Runtime security monitoring guide
- [security/README.md](security/README.md) - Complete security overview

**Quick Security Check**:
```bash
# Run all security scans
gosec ./...                    # Go code security (0 issues expected)
govulncheck ./...              # Dependency vulnerabilities
golangci-lint run              # Comprehensive linting

# Install and monitor with Falco (requires sudo)
sudo bash security/install-falco.sh
sudo journalctl -u falco -f   # View runtime alerts
```

## SPIRE Integration

The project uses the real `go-spiffe` SDK v2.6.0 for production deployments:

**Public APIs**:
- pkg/zerotrustserver` - Zero-config mTLS server (recommended for most users)
- `pkg/zerotrustclient` - Zero-config mTLS client (recommended for most users)

**Production adapters**:
- `internal/adapters/inbound/identityserver` - mTLS server using go-spiffe SDK
- `internal/adapters/outbound/spire` - SPIRE Workload API client adapters
- `internal/adapters/outbound/httpclient` - mTLS HTTP client using go-spiffe SDK
- Integration tests - Full mTLS with real SPIRE agent

**Development adapters**:
- `internal/adapters/outbound/inmemory` - In-memory SPIRE implementation for learning
- Used by `cmd/main.go` for CLI demonstrations

## Documentation

Comprehensive documentation is organized by audience and purpose:

- **[docs/guide/](docs/guide/)** - User guides and tutorials
  - [QUICKSTART.md](docs/guide/QUICKSTART.md) - Get started quickly
  - [TROUBLESHOOTING.md](docs/guide/TROUBLESHOOTING.md) - Common issues
  - [BUILD_MODES.md](docs/guide/BUILD_MODES.md) - Dev vs prod builds

- **[docs/architecture/](docs/architecture/)** - Design and technical reference
  - [ARCHITECTURE.md](docs/architecture/ARCHITECTURE.md) - System design
  - [PORT_CONTRACTS.md](docs/architecture/PORT_CONTRACTS.md) - API contracts
  - [DOMAIN.md](docs/architecture/DOMAIN.md) - Domain model

- **[docs/engineering/](docs/engineering/)** - Testing and verification
  - [TESTING.md](docs/engineering/TESTING.md) - Testing strategy
  - [VERIFICATION.md](docs/engineering/VERIFICATION.md) - Quality assurance

- **[docs/roadmap/](docs/roadmap/)** - Future direction and plans

See [docs/README.md](docs/README.md) for the complete documentation index.

## References

- [SPIFFE Specification](https://github.com/spiffe/spiffe)
- [SPIRE Documentation](https://spiffe.io/docs/latest/spire/)
- [go-spiffe SDK](https://github.com/spiffe/go-spiffe)
- [Hexagonal Architecture](https://alistair.cockburn.us/hexagonal-architecture/)
