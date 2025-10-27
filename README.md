# Identity Based Authentication Library

An identity based authentication library using SPIFFE/SPIRE for service-to-service communication, built with hexagonal architecture.

## Overview

This is a mTLS library using `go-spiffe` SDK v2.6.0 for identity-based authentication. It includes an in-memory SPIRE implementation for development and testing purposes.

### Features

- **Zero-Config API**: One-call setup with automatic socket and trust domain detection
- **Automatic Certificate Management**: Zero-downtime certificate rotation via SPIRE
- **mTLS Authentication**: Both client and server authenticate each other
- **Identity Extraction**: SPIFFE ID available to application handlers
- **Standard HTTP**: Compatible with Go's standard `http` package
- **Authentication Only**: No authorization logic - app decides access
- **Production Ready**: Comprehensive tests (unit + integration + property-based + fuzz)
- **Simple API**: Structured configuration with sensible defaults
- **Thread-Safe**: Proper shutdown and resource management

### Inmemory Implementation

An in-memory SPIRE implementation demonstrates:
- SPIRE Workload API concepts
- Agent server and workload attestation flow
- Used for development and testing

**Hexagonal Architecture**: Clear separation between domain, ports, and adapters allows both implementations to coexist.

## Getting Started

**👉 New to this library?** Start with the [Quick Start Guide](docs/tutorials/QUICKSTART.md) for step-by-step instructions to deploy SPIRE and run examples.

The guide covers:
- Deploying SPIRE infrastructure (Minikube)
- Running the example server and client
- Verifying mTLS authentication
- Troubleshooting common issues

## API Examples

Once you have SPIRE running (see [Quick Start Guide](docs/tutorials/QUICKSTART.md)), here's how to use the API:

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

func rootHandler(w http.ResponseWriter, r *http.Request) {
    id, ok := zerotrustserver.PeerIdentity(r.Context())
    if !ok {
        http.Error(w, "unauthorized", http.StatusUnauthorized)
        return
    }

    fmt.Fprintf(w, "Success! Authenticated as: %s\n", id.SPIFFEID)
}

func main() {
    ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
    defer stop()

    routes := map[string]http.Handler{
        "/": http.HandlerFunc(rootHandler),
    }

    if err := zerotrustserver.Serve(ctx, routes); err != nil {
        stop()
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
    cfg.SPIFFE.AllowedPeerID = "spiffe://example.org/client"
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
    peer_verification: trust-domain
    trust_domain: example.org
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
    "github.com/pocket/hexagon/spire/internal/ports"
)

func main() {
    ctx := context.Background()

    // Load configuration from file with env variable overrides
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
```

## Configuration Options

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
    AllowedPeerID      string // Exact SPIFFE ID match
    AllowedTrustDomain string // Any ID in trust domain
}

// HTTP server configuration
type HTTPConfig struct {
    Address           string        // Server address (e.g., ":8443")
    ReadHeaderTimeout time.Duration // Prevents Slowloris attacks
    ReadTimeout       time.Duration
    WriteTimeout      time.Duration
    IdleTimeout       time.Duration
}
```

### Authorization Policy

- **Exactly one** of `AllowedPeerID` or `AllowedTrustDomain` must be set
- `AllowedPeerID`: Exact match against a specific SPIFFE ID
- `AllowedTrustDomain`: Allow any ID in the trust domain

## Architecture

This project applies **Hexagonal Architecture** (Ports & Adapters pattern):

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

Domain never depends on adapters. Adapters depend on ports.

See [docs/explanation/ARCHITECTURE.md](docs/explanation/ARCHITECTURE.md) for detailed architecture documentation.

## Domain Entities

### IdentityCredential (SPIFFE ID)

```go
// IdentityCredential represents a SPIFFE ID: spiffe://<trust-domain>/<path>
type IdentityCredential struct {
    trustDomain *TrustDomain
    path        string
}
```

**Examples**: `spiffe://example.org/server`, `spiffe://example.org/client`

### IdentityDocument (SVID)

```go
// IdentityDocument represents an X.509 SVID
type IdentityDocument struct {
    identityCredential *IdentityCredential
    certificate        *x509.Certificate
    certificateChain   []*x509.Certificate
    expiresAt          time.Time
}
```

See [docs/reference/DOMAIN.md](docs/reference/DOMAIN.md) for complete domain model documentation.

## Testing

The project has:

- **Unit tests**: Fast, no dependencies
- **Integration tests**: Real SPIRE infrastructure
- **Property-based tests**: Algebraic properties (10k iterations)
- **Fuzz tests**: Edge cases and invalid inputs

```bash
# Run unit tests
make test

# Run integration tests (requires SPIRE)
make minikube-up
make test-integration

# Run property-based tests
PBT_MAX_COUNT=10000 go test -v -run "Properties" ./internal/...

# Run fuzz tests
go test -fuzz=FuzzNormalizePath -fuzztime=30s ./internal/domain
```

See [docs/reference/TESTING.md](docs/reference/TESTING.md) for complete testing guide.

## Security

This project implements defense-in-depth security:

**Application Security (mTLS)**:
- mTLS required for all connections
- Identity-based authentication via SPIFFE IDs
- Automatic certificate rotation via SPIRE
- TLS 1.3 minimum version enforced
- SPIFFE verification (not DNS hostname)

**Build-Time Security**:
- gosec: Go code security scanning
- golangci-lint: 22+ security-focused linters
- govulncheck: Dependency vulnerability scanning
- Trivy: Container image scanning

**Runtime Security (Falco)**:
- Syscall monitoring with eBPF
- SPIRE socket protection
- Container behavior analysis

See [security/README.md](security/README.md) for complete security documentation.

## Documentation

This project uses the [Diátaxis framework](https://diataxis.fr/) for clear, user-focused documentation.

**Start here**: [Documentation Index](docs/README.md)

### Quick Links by Purpose

- 🎓 **[Tutorials](docs/tutorials/)** - Learn by doing
  - [Quick Start Guide](docs/tutorials/QUICKSTART.md) - Get up and running ⭐
  - [Editor Setup](docs/tutorials/EDITOR_SETUP.md) - Configure your IDE
  - [Examples](docs/tutorials/examples/) - Hands-on code examples

- 🔧 **[How-To Guides](docs/how-to-guides/)** - Solve specific problems
  - [Production Deployment](docs/how-to-guides/PRODUCTION_WORKLOAD_API.md) - Deploy with kernel attestation
  - [Troubleshooting](docs/how-to-guides/TROUBLESHOOTING.md) - Debug common issues
  - [Security Tools](docs/how-to-guides/security-tools.md) - Set up security scanning

- 📖 **[Reference](docs/reference/)** - Technical specifications
  - [Port Contracts](docs/reference/PORT_CONTRACTS.md) - Interface definitions
  - [Domain Model](docs/reference/DOMAIN.md) - Core domain types
  - [Testing Guide](docs/reference/TESTING.md) - Comprehensive testing docs

- 💡 **[Explanation](docs/explanation/)** - Understand the design
  - [Architecture](docs/explanation/ARCHITECTURE.md) - System design rationale
  - [Design by Contract](docs/explanation/DESIGN_BY_CONTRACT.md) - Why we use contracts
  - [SPIFFE ID Refactoring](docs/explanation/SPIFFE_ID_REFACTORING.md) - Design evolution

See [docs/README.md](docs/README.md) for the complete documentation index.

## Design Decisions

### 1. Hexagonal Architecture

Consists of domain, port interfaces, swappable adapters:
- Production implementation uses real `go-spiffe` SDK
- In-memory implementation for development/testing
- No domain coupling to infrastructure

### 2. Config Structs for Grouped Parameters

APIs use config structs for maintainability and extensibility.

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

The library only authenticates clients via SPIFFE IDs. Authorization decisions are left to the application.

## Quality and Best Practices

Follows Go best practices:

1. **Config Structs**: APIs use config structs for maintainability
2. **Proper Validation**: Required fields validated with clear error messages
3. **Resource Management**: Proper cleanup with defer, separate Shutdown/Close
4. **Thread Safety**: Mutex protects shared state, sync.Once for initialization
5. **Graceful Shutdown**: Separate shutdown context with timeout
6. **Error Wrapping**: Context preserved with `fmt.Errorf("%w", err)`
7. **Test Coverage**: Unit + Integration + Property-based + Fuzz tests
8. **Documentation**: Inline docs, comprehensive guides, examples

## SPIRE Integration

Production deployments use `go-spiffe` SDK v2.6.0:

**Public APIs**:
- `pkg/zerotrustserver` - Zero-config mTLS server (recommended)
- `pkg/zerotrustclient` - Zero-config mTLS client (recommended)

**Production adapters**:
- `internal/adapters/inbound/identityserver` - mTLS server
- `internal/adapters/outbound/spire` - SPIRE Workload API client
- `internal/adapters/outbound/httpclient` - mTLS HTTP client

**Development adapters**:
- `internal/adapters/outbound/inmemory` - In-memory SPIRE for the ability to run the application in headless and tailess mode.

## References

- [SPIFFE Specification](https://github.com/spiffe/spiffe)
- [SPIRE Documentation](https://spiffe.io/docs/latest/spire/)
- [go-spiffe SDK](https://github.com/spiffe/go-spiffe)
- [Hexagonal Architecture](https://alistair.cockburn.us/hexagonal-architecture/)
