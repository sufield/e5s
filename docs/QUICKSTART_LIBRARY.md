# e5s Library Quickstart

`github.com/sufield/e5s` is a lightweight Go library for SPIFFE/SPIRE-based mTLS. It provides type-safe abstractions for building mutual TLS connections with automatic certificate rotation.

## Installation

```bash
go get github.com/sufield/e5s@latest
```

## Main Packages

The library has two main packages:

- **`spiffehttp`** - Provider-agnostic mTLS primitives and policy
- **`spire`** - SPIRE Workload API client

## Example mTLS Server

```go
package main

import (
    "context"
    "fmt"
    "log"
    "net/http"

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

    // Create server TLS config
    // Accepts any client in the same trust domain by default
    tlsConfig, err := spiffehttp.NewServerTLSConfig(
        ctx,
        x509Source,
        x509Source,
        spiffehttp.ServerConfig{},
    )
    if err != nil {
        log.Fatal(err)
    }

    // Create HTTP handler that extracts peer identity
    http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
        peer, ok := spiffehttp.PeerFromRequest(r)
        if !ok {
            http.Error(w, "unauthorized", http.StatusUnauthorized)
            return
        }
        fmt.Fprintf(w, "Hello, %s!\n", peer.ID.String())
    })

    // Start HTTPS server
    server := &http.Server{
        Addr:      ":8443",
        TLSConfig: tlsConfig,
    }
    log.Fatal(server.ListenAndServeTLS("", ""))
}
```

## Example mTLS Client

```go
package main

import (
    "context"
    "fmt"
    "io"
    "log"
    "net/http"

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
    // Accepts any server in the specified trust domain
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

    // Create HTTP client
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

    body, err := io.ReadAll(resp.Body)
    if err != nil {
        log.Fatal(err)
    }
    fmt.Println(string(body))
}
```

## Server Identity Verification Policies

### Accept any client in same trust domain (default)

```go
x509Source := source.X509Source()
tlsConfig, err := spiffehttp.NewServerTLSConfig(
    ctx, x509Source, x509Source,
    spiffehttp.ServerConfig{},
)
```

### Accept specific trust domain

```go
x509Source := source.X509Source()
tlsConfig, err := spiffehttp.NewServerTLSConfig(
    ctx, x509Source, x509Source,
    spiffehttp.ServerConfig{
        AllowedClientTrustDomain: "partner.example.org",
    },
)
```

### Accept only specific SPIFFE ID

```go
x509Source := source.X509Source()
tlsConfig, err := spiffehttp.NewServerTLSConfig(
    ctx, x509Source, x509Source,
    spiffehttp.ServerConfig{
        AllowedClientID: "spiffe://example.org/api-client",
    },
)
```

## Client Server Verification Policies

### Verify specific SPIFFE ID

```go
x509Source := source.X509Source()
tlsConfig, err := spiffehttp.NewClientTLSConfig(
    ctx, x509Source, x509Source,
    spiffehttp.ClientConfig{
        ExpectedServerID: "spiffe://example.org/api-server",
    },
)
```

### Accept any server in trust domain

```go
x509Source := source.X509Source()
tlsConfig, err := spiffehttp.NewClientTLSConfig(
    ctx, x509Source, x509Source,
    spiffehttp.ClientConfig{
        ExpectedServerTrustDomain: "example.org",
    },
)
```

## Extracting Peer Identity

On the server side, extract the authenticated client's identity:

```go
func handler(w http.ResponseWriter, r *http.Request) {
    peer, ok := spiffehttp.PeerFromRequest(r)
    if !ok {
        http.Error(w, "unauthorized", http.StatusUnauthorized)
        return
    }

    // Use peer.ID for authorization
    log.Printf("Request from: %s (trust domain: %s)",
        peer.ID.String(), peer.ID.TrustDomain().Name())

    // Check certificate expiry
    if time.Until(peer.ExpiresAt) < 5*time.Minute {
        log.Printf("Warning: client cert expires soon: %s", peer.ExpiresAt)
    }
}
```

## SPIRE Socket Configuration

The `spire.Source` auto-detects the SPIRE Workload API socket in this order:

1. `Config.WorkloadSocket` (if provided)
2. `SPIFFE_ENDPOINT_SOCKET` environment variable
3. `/tmp/spire-agent/public/api.sock` (common default)
4. `/var/run/spire/sockets/agent.sock` (alternate location)

Explicit configuration:

```go
source, err := spire.NewIdentitySource(ctx, spire.Config{
    WorkloadSocket: "unix:///custom/path/to/agent.sock",
})
```

TCP endpoints are also supported for remote SPIRE agents:

```go
source, err := spire.NewIdentitySource(ctx, spire.Config{
    WorkloadSocket: "tcp://spire-agent.example.org:8081",
})
```

## Certificate Rotation

Certificate rotation is automatic. The `spire.Source` maintains a live connection to the SPIRE Workload API and updates certificates before they expire. No restart required.

## Thread Safety

All types are safe for concurrent use. You can share a single `spire.Source` across multiple servers and clients.

## Lifecycle Management

```go
// Create once per process
source, err := spire.NewIdentitySource(ctx, spire.Config{})
if err != nil {
    log.Fatal(err)
}

// Share across multiple TLS configs
x509Source := source.X509Source()
serverTLS, _ := spiffehttp.NewServerTLSConfig(ctx, x509Source, x509Source, ...)
clientTLS, _ := spiffehttp.NewClientTLSConfig(ctx, x509Source, x509Source, ...)

// Close when shutting down
defer source.Close()
```

The `context` passed to `NewIdentitySource` / `NewServerTLSConfig` / `NewClientTLSConfig` is only used for initial validation. To actually shut down the source, you **must** call `source.Close()`. Canceling the context does NOT stop background rotation.
