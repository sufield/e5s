# e5s - SPIFFE/SPIRE mTLS Library

A lightweight Go library for building mutual TLS services with SPIFFE identity verification and automatic certificate rotation.

## Features

- **Simple High-Level API** - Config-driven with `e5s.Start()` and `e5s.Client()`
- **Low-Level Control** - Direct access to `pkg/identitytls` and `pkg/spire` for custom use cases
- **SDK-Based Implementation** - Uses official go-spiffe SDK patterns
- **SPIRE Adapter** - Production-ready SPIRE Workload API implementation
- **Automatic Rotation** - Zero-downtime certificate and trust bundle updates
- **SPIFFE ID Verification** - Policy-based peer authentication
- **TLS 1.3 Enforcement** - Strong cipher suites and security defaults
- **Thread-Safe** - Share sources across multiple servers and clients
- **Minimal Dependencies** - Core (`pkg/identitytls`): stdlib only. SPIRE adapter (`pkg/spire`): `go-spiffe/v2`. High-level API (`e5s.go`): adds `yaml.v3`. Examples add `chi` (see `go.mod` for details)

## Quick Start

### Installation

```bash
go get github.com/sufield/e5s@latest
```

## Two Ways to Use e5s

We provide both **high-level** and **low-level** APIs because they serve different developer roles and abstraction levels:

### High-Level API (`e5s.Start`, `e5s.Client`)

**For:** Application developers
**Goal:** Make identity-based mTLS work with one line of code

- Handles configuration, SPIRE connection, certificate rotation, and verification internally
- Reads `e5s.yaml` → starts server/client automatically
- Ideal when you just want secure communication without caring how certificates or trust domains are wired
- Example use: web apps, microservices, APIs

✅ **Benefits:** Zero boilerplate, hard to misuse, easy to run locally and in production with the same config

### Low-Level API (`pkg/identitytls`, `pkg/spire`)

**For:** Infrastructure/platform teams
**Goal:** Allow full control over mTLS internals

- Lets you build custom TLS configs and integrate with the go-spiffe SDK directly
- Exposes `identitytls.NewServerTLSConfig` and `spire.NewSource`
- Ideal for customizing certificate rotation intervals, trust domain logic, or integrating into non-HTTP systems

✅ **Benefits:** Extensible for advanced use cases, can plug in custom identity providers, useful for testing/debugging or building frameworks

### Which example should I look at?

| Developer Type | API | Use Case |
|----------------|-----|----------|
| **Application Developer** | High-Level (`e5s.Start`) | Secure HTTP services quickly |
| **Platform/Infra Engineer** | Low-Level (`pkg/identitytls`, `pkg/spire`) | Build custom SPIRE integrations or non-HTTP services |

- **`examples/highlevel/`** - Start here for application development (production behavior, minimal code)
- **`examples/minikube-lowlevel/`** - Platform/infrastructure example (full SPIRE + mTLS stack in Kubernetes)

### 1. High-Level API (Recommended for Most Users)

Config-driven approach - no TLS code needed. Just create an `e5s.yaml` file and call `e5s.Start()`.

**Example Server:**

```go
package main

import (
    "fmt"
    "log"
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

    shutdown, err := e5s.Start("e5s.yaml", http.DefaultServeMux)
    if err != nil {
        log.Fatal(err)
    }
    defer shutdown()

    log.Println("Server running - press Ctrl+C to stop")
    select {} // block forever
}
```

**Example Client:**

```go
package main

import (
    "io"
    "log"
    "os"

    "github.com/sufield/e5s"
)

func main() {
    client, shutdown, err := e5s.Client("e5s.yaml")
    if err != nil {
        log.Fatal(err)
    }
    defer shutdown()

    resp, err := client.Get("https://localhost:8443/hello")
    if err != nil {
        log.Fatal(err)
    }
    defer resp.Body.Close()

    io.Copy(os.Stdout, resp.Body)
}
```

**Config File (e5s.yaml):**

```yaml
spire:
  workload_socket: unix:///tmp/spire-agent/public/api.sock

server:
  listen_addr: ":8443"
  allowed_client_trust_domain: "example.org"

client:
  expected_server_trust_domain: "example.org"
```

**Want a production-ready example?** → See [examples/highlevel/](examples/highlevel/) for a complete server with chi router, graceful shutdown, health checks, and structured logging.

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

    "github.com/sufield/e5s/pkg/identitytls"
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

    // Create server TLS config (accepts any client in same trust domain)
    tlsConfig, err := identitytls.NewServerTLSConfig(
        ctx,
        x509Source,
        x509Source,
        identitytls.ServerConfig{},
    )
    if err != nil {
        log.Fatal(err)
    }

    // HTTP handler that extracts peer identity
    http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
        peer, ok := identitytls.ExtractPeerInfo(r)
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
    log.Fatal(server.ListenAndServeTLS("", ""))
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

    "github.com/sufield/e5s/pkg/identitytls"
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
    tlsConfig, err := identitytls.NewClientTLSConfig(
        ctx,
        x509Source,
        x509Source,
        identitytls.ClientConfig{
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

    // Make mTLS request
    resp, err := client.Get("https://server.example.org:8443")
    if err != nil {
        log.Fatal(err)
    }
    defer resp.Body.Close()

    io.Copy(os.Stdout, resp.Body)
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
├── identitytls/        # Core mTLS library (provider-agnostic)
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
2. **Low-level** (`pkg/identitytls` + `pkg/spire`) - Full control over TLS, rotation, verification

**Clean separation:**
- `pkg/identitytls` - TLS configuration using go-spiffe SDK (no SPIRE dependency)
- `pkg/spire` - SPIRE Workload API client
- `e5s.go` - Wires everything together based on config file

**Note:** The examples are separate modules (each has its own `go.mod`) so you can vendor/copy them without pulling extra dependencies into your service. The core library has minimal dependencies.

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
