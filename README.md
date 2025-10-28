# e5s - SPIFFE/SPIRE mTLS Library

A lightweight Go library for building mutual TLS services with SPIFFE identity verification and automatic certificate rotation.

## Features

- **Provider-Agnostic Core** - Clean `CertSource` interface for any identity provider
- **SPIRE Adapter** - Production-ready SPIRE Workload API implementation
- **Automatic Rotation** - Zero-downtime certificate and trust bundle updates
- **SPIFFE ID Verification** - Policy-based peer authentication
- **TLS 1.3 Enforcement** - Strong cipher suites and security defaults
- **Thread-Safe** - Share sources across multiple servers and clients
- **Zero Dependencies** - Core library only depends on stdlib (SPIRE adapter uses `go-spiffe`)

## Quick Start

### Installation

```bash
go get github.com/sufield/e5s@latest
```

### Library Usage

**New to the library?** → See [docs/QUICKSTART_LIBRARY.md](docs/QUICKSTART_LIBRARY.md) for detailed examples and API documentation.

**Want a working demo?** → See [examples/minikube/](examples/minikube/) for a complete mTLS server/client with SPIRE deployment.

### Example: mTLS Server

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

    // Create server TLS config (accepts any client in same trust domain)
    tlsConfig, err := identitytls.NewServerTLSConfig(
        ctx,
        source,
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
        fmt.Fprintf(w, "Hello, %s!\n", peer.SPIFFEID)
    })

    // Start HTTPS server with mTLS
    server := &http.Server{
        Addr:      ":8443",
        TLSConfig: tlsConfig,
    }
    log.Fatal(server.ListenAndServeTLS("", ""))
}
```

### Example: mTLS Client

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

    // Create client TLS config
    tlsConfig, err := identitytls.NewClientTLSConfig(
        ctx,
        source,
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

## Architecture

- **Hexagonal architecture is about boundaries, not folders.** You can follow its principles without the typical `domain/ports/adapters` directories.
- **A library ≠ an application.** Libraries expose APIs; they don’t orchestrate or host servers, so they need fewer layers.
- **Your domain is in `pkg/identitytls`.** It defines rules for trust domains, SPIFFE IDs, and TLS configuration—pure business logic.
- **Your adapter is in `pkg/spire`.** It implements the boundary (`CertSource`) using SPIRE’s Workload API.
- **The `CertSource` interface is the port.** It cleanly separates your core logic from SPIRE.
- **Result:** You kept hexagonal *principles* (dependency inversion, clear boundaries) but dropped hexagonal *ceremony* (extra directories).
- **Outcome:** A lean, production-grade Go library that makes SPIRE easy to use without losing architectural integrity.

```
pkg/
├── identitytls/        # Core mTLS library (provider-agnostic)
│   ├── client.go       # Client TLS config builder
│   ├── server.go       # Server TLS config builder
│   ├── peer.go         # SPIFFE ID extraction/validation
│   └── source.go       # CertSource interface
└── spire/              # SPIRE Workload API adapter
    └── source.go       # Implements CertSource for SPIRE
```

**Clear separation:**
- `pkg/identitytls` - Defines interfaces and TLS policy (no SPIRE dependency)
- `pkg/spire` - Implements `CertSource` using SPIRE Workload API

You can implement custom `CertSource` adapters for other identity providers (Vault, cert-manager, etc.).

## Documentation

- **[Quick Start Guide](docs/QUICKSTART_LIBRARY.md)** - API usage and examples
- **[Example Application](examples/minikube/)** - Full mTLS demo with SPIRE cluster
- **[Security Posture](security/)** - Supply chain security and runtime monitoring

## Server Verification Policies

```go
// Accept any client in same trust domain (default)
identitytls.NewServerTLSConfig(ctx, source, identitytls.ServerConfig{})

// Accept specific trust domain
identitytls.NewServerTLSConfig(ctx, source, identitytls.ServerConfig{
    AllowedClientTrustDomain: "partner.example.org",
})

// Accept only specific SPIFFE ID
identitytls.NewServerTLSConfig(ctx, source, identitytls.ServerConfig{
    AllowedClientID: "spiffe://example.org/api-client",
})
```

## Client Verification Policies

```go
// Verify specific SPIFFE ID
identitytls.NewClientTLSConfig(ctx, source, identitytls.ClientConfig{
    ExpectedServerID: "spiffe://example.org/api-server",
})

// Accept any server in trust domain
identitytls.NewClientTLSConfig(ctx, source, identitytls.ClientConfig{
    ExpectedServerTrustDomain: "example.org",
})
```

## Development

```bash
# Run tests
make test

# Run tests with coverage
make test-coverage

# Build examples
make examples

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
