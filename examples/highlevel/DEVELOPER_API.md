# High-Level API Example

**Application Developer Example** - Production behavior, simplest API.

This example demonstrates the high-level e5s API for building mTLS services with SPIRE. This is the recommended starting point for most users.

---

## New to SPIRE?

**[Start with the Tutorial →](TUTORIAL.md)**

The tutorial walks you through every step to get mTLS working in a development environment using Minikube. Perfect for developers who want to learn by doing.

**[Advanced Examples →](ADVANCED.md)**

See production patterns including environment variables, context timeouts, retry logic, circuit breakers, structured logging, and health checks.

## What's Here

- `e5s.yaml` - Configuration file with production-ready settings
- See [middleware example](../middleware/) for actual server implementation

## Features Demonstrated

- **Simple API**: `e5s.Start()` and `e5s.Client()` - no TLS code needed
- **Config-driven**: All SPIRE and mTLS settings in `e5s.yaml`
- **Identity extraction**: `e5s.PeerID()` in handlers
- **Automatic rotation**: SPIRE handles certificate renewal
- **Chi integration**: Works with any HTTP framework

## Prerequisites

You need a SPIRE deployment with registered workloads. See the [minikube-lowlevel example](../minikube-lowlevel/) for a complete SPIRE setup.

1. SPIRE Agent must be running and reachable at the socket path in `e5s.yaml`
2. The server workload must be registered with a SPIFFE ID in the trust domain allowed by `server.allowed_client_*`
3. The client workload must be registered with a SPIFFE ID in the trust domain (or specific ID) expected by `client.expected_server_*`

## Environment Variables

Both server and client support these environment variables:

- `E5S_CONFIG` - Path to config file (default: `e5s.yaml`)
- `SERVER_ADDR` - Server URL for client (default: `https://localhost:8443`)

Examples:

Use a different config file:
```bash
E5S_CONFIG=/etc/e5s/prod.yaml ./bin/highlevel-server
```

Connect to a remote server:
```bash
SERVER_ADDR=https://api.example.org:8443 ./bin/highlevel-client
```

Combine both:
```bash
E5S_CONFIG=./custom.yaml SERVER_ADDR=https://remote:8443 ./bin/highlevel-client
```

## Configuration

The `e5s.yaml` file configures both server and client:

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
  # Accept any client in this trust domain
  allowed_client_trust_domain: "example.org"

client:
  # Connect to any server in this trust domain
  expected_server_trust_domain: "example.org"
```

For production, use specific SPIFFE IDs instead of trust domains:

```yaml
server:
  allowed_client_spiffe_id: "spiffe://example.org/frontend"

client:
  expected_server_spiffe_id: "spiffe://example.org/api-server"
```

## Using This Configuration

This `e5s.yaml` provides production-ready configuration settings. To see actual working examples:

- **Server Example**: See [../middleware/](../middleware/) for a complete mTLS server implementation
- **Client Example**: See the main README for client code examples

The configuration in this directory demonstrates:
- **Production settings**: Conservative timeout, explicit socket paths
- **Authorization options**: Both trust domain and specific ID patterns
- **Flexibility**: Optional timeout configuration with sensible defaults

## Health Check Endpoints

The server provides health check endpoints for Kubernetes liveness/readiness probes:

Liveness probe:
```bash
curl -k https://localhost:8443/healthz
ok
```

Readiness probe:
```bash
curl -k https://localhost:8443/healthz/ready
ready
```

These endpoints:
- Are lightweight and fast for probes
- Are excluded from noisy request logging
- Can be moved to a separate unauthenticated port in production if needed

In this example, all endpoints (including `/healthz`) are served on the same mTLS listener. If your platform requires truly unauthenticated health checks, you'll need to run a separate HTTP listener on a different port.

## How It Works

### Server Code Example

```go
package main

import (
    "context"
    "fmt"
    "log"
    "net/http"
    "os/signal"
    "syscall"

    "github.com/go-chi/chi/v5"
    "github.com/sufield/e5s"
)

func main() {
    // Create context that listens for interrupt signals
    ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
    defer stop()

    r := chi.NewRouter()

    r.Get("/hello", func(w http.ResponseWriter, req *http.Request) {
        id, ok := e5s.PeerID(req)
        if !ok {
            http.Error(w, "unauthorized", http.StatusUnauthorized)
            return
        }
        fmt.Fprintf(w, "Hello, %s!\n", id)
    })

    shutdown, err := e5s.Start(r)
    if err != nil {
        log.Fatal(err)
    }
    defer shutdown()

    log.Println("Server running - press Ctrl+C to stop")

    // Wait for interrupt signal for graceful shutdown
    <-ctx.Done()
    stop()
    log.Println("Shutting down gracefully...")
}
```

The server:
1. Uses intelligent defaults (checks E5S_CONFIG env var, falls back to `e5s.yaml`)
2. Connects to SPIRE Agent
3. Starts mTLS server with automatic cert rotation
4. Injects peer identity into request context
5. Handlers use `e5s.PeerID()` to get authenticated caller
6. Gracefully shuts down on SIGINT/SIGTERM

### Client Code Example

```go
package main

import (
    "fmt"
    "io"
    "log"

    "github.com/sufield/e5s"
)

func main() {
    client, shutdown, err := e5s.Client("e5s.yaml")
    if err != nil {
        log.Fatal(err)
    }
    defer func() {
        if err := shutdown(); err != nil {
            log.Printf("Cleanup error: %v", err)
        }
    }()

    resp, err := client.Get("https://localhost:8443/hello")
    if err != nil {
        log.Fatal(err)
    }
    defer resp.Body.Close()

    body, _ := io.ReadAll(resp.Body)
    fmt.Println(string(body))
}
```

The client:
1. Loads configuration from `e5s.yaml` (must be explicitly specified)
2. Connects to SPIRE Agent
3. Returns standard `*http.Client` with mTLS
4. Automatically presents SPIFFE ID to servers
5. Verifies server identity per config policy

## What You Don't See

All of this is handled internally by e5s:

- Config file discovery (E5S_CONFIG env var or e5s.yaml)
- SPIRE Workload API connection
- Certificate fetching and rotation
- TLS 1.3 configuration
- mTLS handshake setup
- Trust bundle management
- SPIFFE ID verification
- Shutdown sequencing

You just use `e5s.Start()`, `e5s.Client()`, and `e5s.PeerID()`.

## Next Steps

- See [minikube-lowlevel example](../minikube-lowlevel/) for complete SPIRE cluster setup
- Read [API documentation](../../docs/reference/api.md) for lower-level usage
- Check [runtime security monitoring](../../docs/how-to/monitor-with-falco.md) for production hardening
