# e5s - Identity Based Authentication Library

[![OpenSSF Best Practices](https://www.bestpractices.dev/projects/11425/badge)](https://www.bestpractices.dev/projects/11425)
[![Go Report Card](https://goreportcard.com/badge/github.com/sufield/e5s)](https://goreportcard.com/report/github.com/sufield/e5s)

A Go library for building mutual TLS services with SPIFFE identity verification and automatic certificate rotation based on go-spiffe SDK.

## What Problem Does This Solve?

e5s solves the challenges of implementing secure, identity-based mutual TLS (mTLS) in distributed systems. It simplifies 

- SPIFFE-based authentication
- Automates certificate rotation without downtime
- Enforces peer ID verification
- Minimizes manual certificate management

Ideal for microservices in zero-trust environments. It reduces security risks from expired certs or weak auth, while offering high-level APIs for ease and low-level controls for customization.

## Why Not Use go-spiffe SDK Directly?

Using e5s over the go-spiffe SDK directly offers these advantages for developers building mTLS services:

- **Simpler Abstraction**: The high-level API (e.g., `e5s.Run()`) handles configuration, SPIRE connections, certificate rotation, and verification with minimal code—often one line—versus manual SDK setup and boilerplate.
- **Config-Driven**: YAML-based config (`e5s.dev.yaml` for dev, `e5s.prod.yaml` for prod) streamlines setup for servers/clients, including timeouts and trust domains, without custom coding.
- **Built-in Features**: Automatic zero-downtime rotation, policy-based SPIFFE ID verification, TLS 1.3 enforcement, graceful shutdown, health checks, structured logging and thread-safety are ready out-of-the-box.
- **Low-Level Flexibility**: Direct access to pkg/spiffehttp and pkg/spire for customization, minimizing dependencies (core uses stdlib only).
- **Ease of Adoption**: Comprehensive docs, quickstarts, and examples reduce integration time compared to raw SDK usage.

Use go-spiffe directly only if you need non-HTTP services or custom workflows.

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

## Documentation

- **[API Reference](https://pkg.go.dev/github.com/sufield/e5s)** - Complete API documentation on pkg.go.dev
- **[API Guide](docs/API.md)** - Detailed guide with examples and patterns
- **[Quickstart](docs/QUICKSTART_LIBRARY.md)** - Get started in 5 minutes

## Quick Start

### Installation

```bash
go get github.com/sufield/e5s@latest
```

## Two Ways to Use e5s

A **high-level** and a **low-level** APIs are provided because they serve different developer roles and abstraction levels:

### High-Level API (`e5s.Start`, `e5s.Client`)

**For:** Application developers
**Use Case:** Secure HTTP services quickly
**Goal:** Make identity-based mTLS work with one line of code

- Handles configuration, SPIRE connection, certificate rotation, and verification internally
- Reads `e5s.dev.yaml` (default) or specify via `-config` flag or `E5S_CONFIG` env var
- Ideal when you just want secure communication without caring how certificates or trust domains are wired
- Example use: web apps, microservices, APIs

`examples/highlevel/`** - Start here for application development (production behavior, minimal code)

**Benefits:** Zero boilerplate, hard to misuse, easy to run locally and in production with the same config

### Low-Level API (`pkg/spiffehttp`, `pkg/spire`)

**For:** Platform/Infra Engineer
**Use Case:** Build custom SPIRE integrations or non-HTTP services
**Goal:** Allow full control over mTLS internals

- Build custom TLS configs and integrate with the go-spiffe SDK directly
- Exposes `spiffehttp.NewServerTLSConfig` and `spire.NewIdentitySource`
- Ideal for customizing certificate rotation intervals, trust domain logic, or integrating into non-HTTP systems

**Benefits:** Extensible for advanced use cases, can plug in custom identity providers, useful for testing/debugging or building frameworks

**`examples/minikube-lowlevel/`** - Platform/infrastructure example (full SPIRE + mTLS stack in Kubernetes)

### 1. High-Level API

#### Recommended for Most Users

Simple configuration approach - create `e5s.dev.yaml` for development, `e5s.prod.yaml` for production, and call `e5s.Run()`.

**Example Server:**

```go
package main

import (
    "fmt"
    "net/http"

    "github.com/sufield/e5s"
)

func main() {
    http.HandleFunc("/hello", func(w http.ResponseWriter, r *http.Request) {
        id, ok := e5s.PeerID(r)
        if !ok {
            http.Error(w, "unauthorized", http.StatusUnauthorized)
            return
        }
        fmt.Fprintf(w, "Hello, %s!\n", id)
    })

    // Start mTLS server - handles config, SPIRE, and graceful shutdown
    e5s.Run(http.DefaultServeMux)
}
```

**Example Client:**

```go
package main

import (
    "fmt"
    "io"
    "log"

    "github.com/sufield/e5s"
)

func main() {
    // Create mTLS HTTP client
    client, cleanup, err := e5s.Client("e5s.yaml")
    if err != nil {
        log.Fatal(err)
    }
    defer func() {
        if err := cleanup(); err != nil {
            log.Printf("Cleanup error: %v", err)
        }
    }()

    // Perform mTLS GET request
    resp, err := client.Get("https://localhost:8443/hello")
    if err != nil {
        log.Fatal(err)
    }
    defer resp.Body.Close()

    body, _ := io.ReadAll(resp.Body)
    fmt.Println(string(body))
}
```

**Config File:**

The e5s library requires explicit configuration and never assumes a default environment.

**Config files live in YOUR application codebase, not in the e5s library.**

Copy the example config from `examples/highlevel/e5s.yaml` to your project and customize it:

In your application directory:

```bash
cp path/to/e5s/examples/highlevel/e5s.yaml ./e5s.yaml
```

Then edit for your environment (e5s.dev.yaml, e5s.prod.yaml, etc.)

Then provide the config path explicitly:

1. **Explicit path** (recommended):
   ```go
   shutdown, err := e5s.Start("e5s.prod.yaml", handler)
   ```

2. **Via E5S_CONFIG environment variable**:
   ```bash
   export E5S_CONFIG=/etc/myapp/e5s.prod.yaml
   ```
   ```go
   shutdown, err := e5s.StartServer(handler)  // Uses E5S_CONFIG
   ```

**Example config structure:**

See the complete annotated config file: **[examples/highlevel/e5s.yaml](examples/highlevel/e5s.yaml)**

This single file contains both server and client configuration with detailed comments explaining each option.

In your application, create separate config files (`e5s.dev.yaml`, `e5s.staging.yaml`, `e5s.prod.yaml`) with environment-specific values for socket paths, trust domains, and timeouts. These files are part of your application codebase, not the e5s library.

**For advanced usage** like environment variables, context timeouts, retry logic, and structured logging → See [examples/highlevel/ADVANCED.md](examples/highlevel/ADVANCED.md)

**For a production-ready example** → See [examples/highlevel/](examples/highlevel/) for a server with chi router, graceful shutdown, health checks, and structured logging.

### 2. Low-Level API

#### For Advanced Use Cases

Direct control over TLS configuration for custom scenarios.

**Example: mTLS Server**

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

    "github.com/sufield/e5s/spiffehttp"
    "github.com/sufield/e5s/spire"
)

func main() {
    // Create context that listens for interrupt signals
    ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
    defer stop()

    // Create SPIRE certificate source
    source, err := spire.NewIdentitySource(ctx, spire.Config{})
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

**Example: mTLS Client**

```go
package main

import (
    "context"
    "io"
    "log"
    "net/http"
    "os"
    "time"

    "github.com/sufield/e5s/spiffehttp"
    "github.com/sufield/e5s/spire"
)

func main() {
    ctx := context.Background()

    // Create SPIRE certificate source
    source, err := spire.NewIdentitySource(ctx, spire.Config{})
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

- **[Tutorial](examples/highlevel/TUTORIAL.md)** - Build your first mTLS app (start here!)
- **[Quick Start: Testing](examples/highlevel/QUICK_START_PRERELEASE.md)** - ⚡ For library developers (3 commands)
- **[Examples](examples/)** - High-level, middleware, and infrastructure examples
- **[API Docs](docs/API.md)** - Complete API reference

## Development

```bash
# Run all CI checks locally before pushing
make ci

# Individual checks
make lint          # Run golangci-lint (what CI runs)
make vet           # Run go vet
make fmt           # Format code
make test          # Run tests
make build         # Build example binaries
```

### Common Tasks

```bash
# Testing
make test                  # Run tests quickly
make test-race             # Run with race detector (recommended before push)
make test-coverage         # Generate coverage report
make test-coverage-html    # Open coverage in browser

# Code Quality
make lint                  # Lint code (matches CI)
make lint-fix              # Auto-fix linting issues
make fmt                   # Format all code
make fmt-check             # Check if code is formatted
make vet                   # Run go vet
make tidy                  # Tidy go modules
make verify                # Verify go modules

# Building
make build                 # Build cmd/example-server and cmd/example-client
make examples              # Build all examples (4 binaries)
make example-highlevel-server   # Application developer example
make example-highlevel-client
make example-minikube-server    # Platform/infra example
make example-minikube-client

# Security
make sec-all               # Run all security checks
make sec-deps              # Check for vulnerabilities
make sec-lint              # Security-focused static analysis

# All available targets
make help
```

## Security

e5s implements defense-in-depth security with multiple layers:

- **Static Analysis**: gosec, golangci-lint, govulncheck, gitleaks
- **Container Security**: Pinned digests, non-root users, minimal images
- **mTLS Enforcement**: TLS 1.3, SPIFFE identity verification, certificate rotation
- **Runtime Monitoring**: Optional Falco integration for threat detection
- **Supply Chain**: Signed releases with Cosign/Sigstore

**For detailed security information:**
- [Security Tools & Documentation](security/) - Comprehensive security guide
- [Report Security Issues](https://github.com/sufield/e5s/security/advisories/new) - Private vulnerability disclosure
- [Security Policy](.github/SECURITY.md) - Vulnerability disclosure policy

**Run security scans:**
```bash
make sec-all  # Run all security checks
```

## License

MIT License - See [LICENSE](LICENSE) file for details.
