# e5s - Identity Based Authentication Library

[![OpenSSF Best Practices](https://www.bestpractices.dev/projects/11425/badge)](https://www.bestpractices.dev/projects/11425)
[![Go Report Card](https://goreportcard.com/badge/github.com/sufield/e5s)](https://goreportcard.com/report/github.com/sufield/e5s)

A Go library for building applications that use identity-based authentication instead of API keys.

Build mutual TLS services with SPIFFE identity verification and automatic certificate rotation with almost no boilerplate code. Built on the battle-tested go-spiffe SDK.

Eliminate API keys and plaintext secrets from your services, dramatically reducing the attack surface that comes with leaked credentials, secret sprawl, and rotation headaches.

# The Problem

## ❌ Before: Static API Keys

Every service call required a shared secret.

**Weather Server**

```go
func (s *WeatherService) GetForecast(w http.ResponseWriter, r *http.Request) {
    apiKey := r.Header.Get("Authorization")
    if apiKey != "Bearer weather-client-secret" {
        http.Error(w, "unauthorized", http.StatusUnauthorized)
        return
    }

    // business logic
    forecast := s.GetData()
    json.NewEncoder(w).Encode(forecast)
}
```

**Weather Client**

```go
func main() {
    apiKey := os.Getenv("WEATHER_API_KEY")
    req, _ := http.NewRequest("GET", "https://weather-service:8080/forecast", nil)
    req.Header.Set("Authorization", "Bearer "+apiKey)

    resp, err := http.DefaultClient.Do(req)
    if err != nil {
        log.Fatal(err)
    }
    defer resp.Body.Close()
}
```

**Problems**

* Secrets in code or environment variables
* Manual rotation and redeployment when keys change
* Leaks through logs, git, or config files
* Separate keys for every service pair

---

# The Solution
## ✅ After: Identity-Based Authentication with e5s

Each service has its own **cryptographic identity** (SPIFFE ID).
No API keys, no secrets, no manual rotation.

**Weather Server**

```go
func forecastHandler(w http.ResponseWriter, r *http.Request) {
    // Client already authenticated via mTLS
    forecast := map[string]string{"forecast": "Sunny"}
    json.NewEncoder(w).Encode(forecast)
}

func main() {
    http.HandleFunc("/forecast", forecastHandler)

    if err := e5s.Serve("server-config.yaml", http.DefaultServeMux); err != nil {
        log.Fatal(err)
    }
}
```

**server-config.yaml:**
```yaml
version: 1
spire:
  workload_socket: "unix:///tmp/spire-agent/public/api.sock"
server:
  listen_addr: ":8080"
  allowed_client_spiffe_id: "spiffe://prod.company.com/weather-client"
```

**Weather Client**

```go
func main() {
    client, cleanup, err := e5s.Client("client-config.yaml")
    if err != nil {
        log.Fatal(err)
    }
    defer cleanup()

    resp, err := client.Get("https://weather-service:8080/forecast")
    if err != nil {
        log.Fatal(err)
    }
    defer resp.Body.Close()

    var result map[string]string
    json.NewDecoder(resp.Body).Decode(&result)
    fmt.Println(result["forecast"])
}
```

**client-config.yaml:**
```yaml
version: 1
spire:
  workload_socket: "unix:///tmp/spire-agent/public/api.sock"
client:
  expected_server_spiffe_id: "spiffe://prod.company.com/weather-service"
```

---

## What Changed

| Aspect         | Before (API Keys)       | After (e5s)               |
| -------------- | ----------------------- | ------------------------- |
| Authentication | Shared secret in header | mTLS using SPIFFE IDs     |
| Rotation       | Manual                  | Automatic                 |
| Secret Storage | Env vars / config       | None                      |
| Breach Risk    | High (key leaks)        | Low (certs rotate hourly) |
| Setup          | Add key per service     | Register identity once    |
| Maintenance    | Continuous              | Zero-touch                |

---

## Example Setup

```bash
# One-time identity registration
spire-server entry create -spiffeID spiffe://prod.company.com/weather-client \
    -parentID spiffe://prod.company.com/spire-agent -selector unix:uid:1000

spire-server entry create -spiffeID spiffe://prod.company.com/weather-service \
    -parentID spiffe://prod.company.com/spire-agent -selector unix:uid:1000
```

Now, your services prove who they are through short-lived certificates—no shared keys, no dashboards, no secret rotation scripts.

## How does e5s work?

e5s solves the challenges of implementing secure, identity-based mutual TLS (mTLS) in distributed systems by adopting the proven SPIFFE standard. It simplifies 

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

- **[docs/](docs/)** - Documentation Table of Contents
- **[API Reference](https://pkg.go.dev/github.com/sufield/e5s)** - API docs on pkg.go.dev
- **[Examples](examples/)** - Working code for all use cases

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

Simple configuration approach - create `e5s.yaml` with your SPIRE socket path and authorization policy.

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

    // Start mTLS server - handles config, SPIRE, and graceful shutdown
    if err := e5s.Serve("e5s.yaml", http.DefaultServeMux); err != nil {
        log.Fatal(err)
    }
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

Then provide the config path:

**Simple approach - blocks with signal handling:**
```go
if err := e5s.Serve("e5s.yaml", handler); err != nil {
    log.Fatal(err)
}
```

**Advanced approach - custom shutdown:**
```go
shutdown, err := e5s.Start("e5s.yaml", handler)
if err != nil {
    log.Fatal(err)
}
defer shutdown()

// Your custom shutdown logic here
```

**Example config structure:**

See the complete annotated config file: **[examples/highlevel/e5s.yaml](examples/highlevel/e5s.yaml)**

This single file contains both server and client configuration with detailed comments explaining each option.

In your application, create separate config files (`e5s.dev.yaml`, `e5s.staging.yaml`, `e5s.prod.yaml`) with environment-specific values for socket paths, trust domains, and timeouts. These files are part of your application codebase, not the e5s library.

**For advanced usage** like environment variables, context timeouts, retry logic, and structured logging → See [examples/highlevel/ADVANCED.md](examples/highlevel/ADVANCED.md)

**For a production-ready example** → See [examples/highlevel/](examples/highlevel/) for a server with chi router, graceful shutdown, health checks, and structured logging.

### 2. Low-Level API

#### For Advanced Use Cases

The low-level API provides programmatic control over TLS configuration when you need customization beyond what the high-level API offers.

**Use the low-level API when:**
- Building libraries or frameworks on top of e5s
- Integrating with custom identity providers
- Need fine-grained control over TLS settings
- Building non-HTTP services

**Core packages:**

```go
import (
    "github.com/sufield/e5s/spire"       // SPIRE Workload API client
    "github.com/sufield/e5s/spiffehttp"  // mTLS TLS config builders
)
```

**Basic pattern:**

1. Create SPIRE identity source: `spire.NewIdentitySource(ctx, config)`
2. Build TLS config: `spiffehttp.NewServerTLSConfig()` or `spiffehttp.NewClientTLSConfig()`
3. Create `http.Server` or `http.Client` with the TLS config
4. Handle graceful shutdown manually

**Comparison to high-level API:**

| What | High-Level | Low-Level |
|------|-----------|-----------|
| Config | YAML file | Go code |
| Setup code | ~10 lines | ~75+ lines |
| Shutdown | Automatic | Manual |
| Hardcoded values | None | Ports, IDs, domains in code |

**Complete examples:**
- **API reference:** [docs/reference/api.md](docs/reference/api.md)
- **Working code:** [examples/middleware/](examples/middleware/)
- **Infrastructure setup:** [examples/minikube-lowlevel/](examples/minikube-lowlevel/)

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
make build                 # Build examples/basic-server and examples/basic-client
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
