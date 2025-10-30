# e5s - SPIFFE/SPIRE mTLS Library

A lightweight Go library for building mutual TLS services with SPIFFE identity verification and automatic certificate rotation.

## Features

- **Simple High-Level API** - Config-driven with `e5s.Start()` and `e5s.Client()`
- **Low-Level Control** - Direct access to `pkg/spiffehttp` and `pkg/spire` for custom use cases
- **SDK-Based Implementation** - Uses official go-spiffe SDK API
- **SPIRE Adapter** - Production-ready SPIRE Workload API implementation
- **Automatic Rotation** - Zero-downtime certificate and trust bundle updates
- **SPIFFE ID Verification** - Policy-based peer authentication
- **TLS 1.3 Enforcement** - Strong cipher suites and security defaults
- **Thread-Safe** - Share sources across multiple servers and clients
- **Minimal Dependencies** 
    - Core (`pkg/spiffehttp`): stdlib only. 
    - SPIRE adapter (`pkg/spire`): `go-spiffe/v2`. 
    - High-level API (`e5s.go`): adds `yaml.v3`.
    - Examples add `chi` (see `go.mod` for details)

## Quick Start

### Installation

```bash
go get github.com/sufield/e5s@latest
```

## Two Ways to Use e5s

We provide a **high-level** and a **low-level** APIs because they serve different developer roles and abstraction levels:

### High-Level API (`e5s.Start`, `e5s.Client`)

**For:** Application developers
**Goal:** Make identity-based mTLS work with one line of code

- Handles configuration, SPIRE connection, certificate rotation, and verification internally
- Reads `e5s.yaml` → starts server/client automatically
- Ideal when you just want secure communication without caring how certificates or trust domains are wired
- Example use: web apps, microservices, APIs

**Benefits:** Zero boilerplate, hard to misuse, easy to run locally and in production with the same config

### Low-Level API (`pkg/spiffehttp`, `pkg/spire`)

**For:** Infrastructure/platform teams
**Goal:** Allow full control over mTLS internals

- Lets you build custom TLS configs and integrate with the go-spiffe SDK directly
- Exposes `spiffehttp.NewServerTLSConfig` and `spire.NewSource`
- Ideal for customizing certificate rotation intervals, trust domain logic, or integrating into non-HTTP systems

**Benefits:** Extensible for advanced use cases, can plug in custom identity providers, useful for testing/debugging or building frameworks

### Which example should I look at?

| Developer Type | API | Use Case |
|----------------|-----|----------|
| **Application Developer** | High-Level (`e5s.Start`) | Secure HTTP services quickly |
| **Platform/Infra Engineer** | Low-Level (`pkg/spiffehttp`, `pkg/spire`) | Build custom SPIRE integrations or non-HTTP services |

- **`examples/highlevel/`** - Start here for application development (production behavior, minimal code)
- **`examples/minikube-lowlevel/`** - Platform/infrastructure example (full SPIRE + mTLS stack in Kubernetes)

### 1. High-Level API (Recommended for Most Users)

Config-driven approach - no TLS code needed. Just create an `e5s.yaml` file and call `e5s.Start()`.

**Example Server:**

```go
package main

import (
    "context"
    "fmt"
    "log"
    "net/http"
    "os/signal"
    "syscall"

    "github.com/sufield/e5s"
)

func main() {
    // Create context that listens for interrupt signals
    ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
    defer stop()

    http.HandleFunc("/hello", func(w http.ResponseWriter, r *http.Request) {
        id, ok := e5s.PeerID(r)
        if !ok {
            http.Error(w, "unauthorized", http.StatusUnauthorized)
            return
        }
        fmt.Fprintf(w, "Hello, %s!\n", id)
    })

    shutdown, err := e5s.Start("e5s.yaml", http.DefaultServeMux)
    if err != nil {
        log.Fatal(err)
    }
    defer shutdown()

    log.Println("Server running - press Ctrl+C to stop")

    // Wait for interrupt signal for graceful shutdown
    <-ctx.Done()
    stop() // Stop receiving signals
    log.Println("Shutting down gracefully...")
}
```

**Example Client:**

```go
package main

import (
    "context"
    "io"
    "log"
    "net/http"
    "os"
    "time"

    "github.com/sufield/e5s"
)

func main() {
    // Create context with timeout for the request
    ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
    defer cancel()

    client, shutdown, err := e5s.Client("e5s.yaml")
    if err != nil {
        log.Fatal(err)
    }
    defer shutdown()

    // Create request with context
    req, err := http.NewRequestWithContext(ctx, "GET", "https://localhost:8443/hello", nil)
    if err != nil {
        log.Fatal(err)
    }

    resp, err := client.Do(req)
    if err != nil {
        log.Fatal(err)
    }
    defer resp.Body.Close()

    if _, err := io.Copy(os.Stdout, resp.Body); err != nil {
        log.Fatal(err)
    }
}
```

**Config File (e5s.yaml):**

```yaml
spire:
  # Path to SPIRE Agent's Workload API socket
  workload_socket: unix:///tmp/spire-agent/public/api.sock

  # (Optional) How long to wait for identity from SPIRE before failing startup
  # Format: Go duration (e.g. "5s", "30s", "1m")
  # Default: 30s if not specified
  # Set higher in dev (agent may start slowly), lower in prod (fail fast)
  initial_fetch_timeout: 30s

server:
  listen_addr: ":8443"

  # Allow any client in this trust domain
  allowed_client_trust_domain: "example.org"

  # Or allow only a specific client SPIFFE ID
  # allowed_client_spiffe_id: "spiffe://example.org/client"

client:
  # Allow any server in this trust domain
  expected_server_trust_domain: "example.org"

  # Or require a specific server SPIFFE ID
  # expected_server_spiffe_id: "spiffe://example.org/server"
```

**For a production-ready example** → See [examples/highlevel/](examples/highlevel/) for a complete server with chi router, graceful shutdown, health checks, and structured logging.

### 2. Low-Level API (For Advanced Use Cases)

Direct control over TLS configuration for custom scenarios.

**Example: mTLS Server (Low-Level)**

```go
package main

import (
    "context"
    "fmt"
    "log"
    "net/http"
    "os/signal"
    "syscall"
    "time"

    "github.com/sufield/e5s/pkg/spiffehttp"
    "github.com/sufield/e5s/pkg/spire"
)

func main() {
    // Create context that listens for interrupt signals
    ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
    defer stop()

    // Create SPIRE certificate source
    source, err := spire.NewSource(ctx, spire.Config{})
    if err != nil {
        log.Fatal(err)
    }
    defer source.Close()

    // Get SDK X509Source for TLS config
    x509Source := source.X509Source()

    // Create server TLS config (accepts any client in same trust domain)
    tlsConfig, err := spiffehttp.NewServerTLSConfig(
        ctx,
        x509Source,
        x509Source,
        spiffehttp.ServerConfig{},
    )
    if err != nil {
        log.Fatal(err)
    }

    // HTTP handler that extracts peer identity
    http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
        peer, ok := spiffehttp.PeerFromRequest(r)
        if !ok {
            http.Error(w, "unauthorized", http.StatusUnauthorized)
            return
        }
        fmt.Fprintf(w, "Hello, %s!\n", peer.ID.String())
    })

    // Start HTTPS server with mTLS
    server := &http.Server{
        Addr:      ":8443",
        TLSConfig: tlsConfig,
    }

    // Start server in a goroutine
    go func() {
        log.Println("Server listening on :8443")
        if err := server.ListenAndServeTLS("", ""); err != nil && err != http.ErrServerClosed {
            log.Fatalf("Server error: %v", err)
        }
    }()

    // Wait for interrupt signal for graceful shutdown
    <-ctx.Done()
    stop()

    // Graceful shutdown with timeout
    shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()

    if err := server.Shutdown(shutdownCtx); err != nil {
        log.Fatalf("Server shutdown error: %v", err)
    }

    log.Println("Server stopped gracefully")
}
```

**Example: mTLS Client (Low-Level)**

```go
package main

import (
    "context"
    "io"
    "log"
    "net/http"
    "os"
    "time"

    "github.com/sufield/e5s/pkg/spiffehttp"
    "github.com/sufield/e5s/pkg/spire"
)

func main() {
    ctx := context.Background()

    // Create SPIRE certificate source
    source, err := spire.NewSource(ctx, spire.Config{})
    if err != nil {
        log.Fatal(err)
    }
    defer source.Close()

    // Get SDK X509Source for TLS config
    x509Source := source.X509Source()

    // Create client TLS config
    tlsConfig, err := spiffehttp.NewClientTLSConfig(
        ctx,
        x509Source,
        x509Source,
        spiffehttp.ClientConfig{
            ExpectedServerTrustDomain: "example.org",
        },
    )
    if err != nil {
        log.Fatal(err)
    }

    // Create HTTP client with mTLS
    client := &http.Client{
        Transport: &http.Transport{
            TLSClientConfig: tlsConfig,
        },
    }

    // Create context with timeout for the request
    reqCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
    defer cancel()

    // Create request with context
    req, err := http.NewRequestWithContext(reqCtx, "GET", "https://server.example.org:8443", nil)
    if err != nil {
        log.Fatal(err)
    }

    // Make mTLS request
    resp, err := client.Do(req)
    if err != nil {
        log.Fatal(err)
    }
    defer resp.Body.Close()

    if _, err := io.Copy(os.Stdout, resp.Body); err != nil {
        log.Fatal(err)
    }
}
```

**Want more control?** → See [examples/minikube-lowlevel/](examples/minikube-lowlevel/) for low-level API usage with full SPIRE cluster setup.

## Architecture

```
e5s.go                  # High-level config-driven API
├── e5s.Start()         # Server with config file
├── e5s.Client()        # Client with config file
└── e5s.PeerID()        # Extract authenticated peer

pkg/
├── spiffehttp/        # Core mTLS library (provider-agnostic)
│   ├── client.go       # Client TLS config builder
│   ├── server.go       # Server TLS config builder
│   ├── peer.go         # SPIFFE ID extraction/validation
│   └── context.go      # Context helpers for peer info
└── spire/              # SPIRE Workload API adapter
    └── source.go       # SPIRE Workload API client

internal/config/        # Config file loading (not exported)
├── config.go           # Config structs
├── load.go             # YAML loader
└── validate.go         # Config validation
```

**Two-tier architecture:**
1. **High-level** (`e5s.go`) - Config-driven, minimal code, works with any HTTP framework
2. **Low-level** (`pkg/spiffehttp` + `pkg/spire`) - Full control over TLS, rotation, verification

**Clear separation:**
- `pkg/spiffehttp` - TLS configuration using go-spiffe SDK (no SPIRE dependency)
- `pkg/spire` - SPIRE Workload API client
- `e5s.go` - Wires everything together based on config file

The examples are separate modules (each has its own `go.mod`) so you can vendor/copy them without pulling extra dependencies into your service. The core library has minimal dependencies.

## Documentation

- **[High-Level Example](examples/highlevel/)** - Application developer example (production behavior, simplest API)
- **[Minikube Low-Level Example](examples/minikube-lowlevel/)** - Platform / infrastructure example (full SPIRE + mTLS stack in local Kubernetes)
- **[API Documentation](docs/QUICKSTART_LIBRARY.md)** - Low-level API usage
- **[Security Posture](security/)** - Supply chain security and runtime monitoring

## Development

```bash
# Run tests
make test

# Run tests with coverage
make test-coverage

# Build all examples (creates 4 binaries in bin/)
make examples
# Builds:
#   bin/highlevel-server    - High-level API example with chi
#   bin/highlevel-client    - High-level API client
#   bin/minikube-server     - Low-level API example
#   bin/minikube-client     - Low-level API client

# Build specific examples
make example-highlevel-server   # Application developer example
make example-highlevel-client
make example-minikube-server    # Platform/infra example
make example-minikube-client

# Run security checks
make sec-all
```

## License

MIT License - See [LICENSE](LICENSE) file for details.

## Contributing

Contributions welcome! Please ensure:
- Tests pass: `make test`
- Security checks pass: `make sec-all`
- Examples build: `make examples`
