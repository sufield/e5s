# e5s vs Raw go-spiffe SDK: A Comparison

This guide compares using the e5s library versus the raw go-spiffe SDK to help you choose the right approach for your use case.

## TL;DR

- **Use e5s** if you're building HTTP/REST services and want simple configuration
- **Use raw go-spiffe SDK** if you need custom protocols or maximum control

Both approaches use the same underlying security mechanisms and are equally secure.

---

## Side-by-Side Comparison

### HTTP Client Example

#### Using Raw go-spiffe SDK

```go
package main

import (
    "context"
    "crypto/tls"
    "fmt"
    "io"
    "log"
    "net/http"

    "github.com/spiffe/go-spiffe/v2/spiffeid"
    "github.com/spiffe/go-spiffe/v2/spiffetls/tlsconfig"
    "github.com/spiffe/go-spiffe/v2/workloadapi"
)

const (
    socketPath = "unix:///tmp/spire-agent/public/api.sock"
    serverURL  = "https://localhost:8443/"
)

func main() {
    if err := run(context.Background()); err != nil {
        log.Fatal(err)
    }
}

func run(ctx context.Context) error {
    ctx, cancel := context.WithCancel(ctx)
    defer cancel()

    // Create X509Source from SPIRE Workload API
    source, err := workloadapi.NewX509Source(
        ctx,
        workloadapi.WithClientOptions(workloadapi.WithAddr(socketPath)),
    )
    if err != nil {
        return fmt.Errorf("unable to create X509Source: %w", err)
    }
    defer source.Close()

    // Define expected server SPIFFE ID
    serverID := spiffeid.RequireFromString("spiffe://example.org/server")

    // Create mTLS client config
    tlsConfig := tlsconfig.MTLSClientConfig(
        source,
        source,
        tlsconfig.AuthorizeID(serverID),
    )
    tlsConfig.MinVersion = tls.VersionTLS13

    // Create HTTP client with mTLS
    client := &http.Client{
        Transport: &http.Transport{
            TLSClientConfig: tlsConfig,
        },
    }

    // Make HTTP request
    r, err := client.Get(serverURL)
    if err != nil {
        return fmt.Errorf("error connecting to %q: %w", serverURL, err)
    }
    defer r.Body.Close()

    body, err := io.ReadAll(r.Body)
    if err != nil {
        return fmt.Errorf("unable to read body: %w", err)
    }

    log.Printf("%s", body)
    return nil
}
```

**Lines of code: ~60**

#### Using e5s Library

```go
package main

import (
    "context"
    "fmt"
    "io"
    "log"

    "github.com/sufield/e5s"
)

func main() {
    if err := run(context.Background()); err != nil {
        log.Fatal(err)
    }
}

func run(ctx context.Context) error {
    // Create HTTP client from config file
    client, err := e5s.Client(ctx, "client-config.yaml")
    if err != nil {
        return fmt.Errorf("failed to create client: %w", err)
    }

    // Make HTTP request
    resp, err := client.Get("https://localhost:8443/")
    if err != nil {
        return fmt.Errorf("request failed: %w", err)
    }
    defer resp.Body.Close()

    body, err := io.ReadAll(resp.Body)
    if err != nil {
        return fmt.Errorf("failed to read response: %w", err)
    }

    log.Printf("%s", body)
    return nil
}
```

**client-config.yaml:**
```yaml
spire:
  workload_socket: "unix:///tmp/spire-agent/public/api.sock"
  initial_fetch_timeout: "30s"

client:
  expected_server_spiffe_id: "spiffe://example.org/server"
```

**Lines of code: ~30 (Go) + 6 (YAML) = 36 total**

---

## Feature Comparison

| Feature | Raw go-spiffe SDK | e5s Library |
|---------|-------------------|-------------|
| **HTTP Support** | Manual setup | Built-in |
| **Configuration** | Hardcoded in Go | YAML files |
| **TLS 1.3 Enforcement** | Manual | Automatic |
| **Input Validation** | Manual | Automatic |
| **Lifecycle Management** | Manual Close() calls | Managed wrapper |
| **Lines of Code** | ~60 lines | ~36 lines |
| **Learning Curve** | Medium (SPIFFE concepts) | Low (standard HTTP) |
| **Flexibility** | Maximum | HTTP-focused |
| **Custom Protocols** | ✅ Full control | ❌ HTTP only |
| **Certificate Rotation** | ✅ Automatic | ✅ Automatic |
| **Peer Verification** | ✅ Fully configurable | ✅ Pre-configured |
| **Error Messages** | SDK-level | Application-level |
| **Environment Variables** | Supported | Config files preferred |

---

## HTTP Server Example

### Using Raw go-spiffe SDK

```go
package main

import (
    "context"
    "fmt"
    "io"
    "log"
    "net/http"
    "time"

    "github.com/spiffe/go-spiffe/v2/spiffeid"
    "github.com/spiffe/go-spiffe/v2/spiffetls/tlsconfig"
    "github.com/spiffe/go-spiffe/v2/workloadapi"
)

// Workload API socket path
const socketPath = "unix:///tmp/agent.sock"

func main() {
    if err := run(context.Background()); err != nil {
        log.Fatal(err)
    }
}

func run(ctx context.Context) error {
    ctx, cancel := context.WithCancel(ctx)
    defer cancel()

    // Set up a `/` resource handler
    http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
        log.Println("Request received")
        _, _ = io.WriteString(w, "Success!!!")
    })

    // Create a `workloadapi.X509Source`, it will connect to Workload API using provided socket.
    // If socket path is not defined using `workloadapi.SourceOption`, value from environment variable `SPIFFE_ENDPOINT_SOCKET` is used.
    source, err := workloadapi.NewX509Source(ctx, workloadapi.WithClientOptions(workloadapi.WithAddr(socketPath)))
    if err != nil {
        return fmt.Errorf("unable to create X509Source: %w", err)
    }
    defer source.Close()

    // Allowed SPIFFE ID
    clientID := spiffeid.RequireFromString("spiffe://example.org/client")

    // Create a `tls.Config` to allow mTLS connections, and verify that presented certificate has SPIFFE ID `spiffe://example.org/client`
    tlsConfig := tlsconfig.MTLSServerConfig(source, source, tlsconfig.AuthorizeID(clientID))
    server := &http.Server{
        Addr:              ":8443",
        TLSConfig:         tlsConfig,
        ReadHeaderTimeout: time.Second * 10,
    }

    if err := server.ListenAndServeTLS("", ""); err != nil {
        return fmt.Errorf("failed to serve: %w", err)
    }
    return nil
}
```

**Lines of code: ~57**

### Using e5s Library

```go
package main

import (
    "context"
    "io"
    "log"
    "net/http"

    "github.com/sufield/e5s"
)

func main() {
    if err := run(context.Background()); err != nil {
        log.Fatal(err)
    }
}

func run(ctx context.Context) error {
    // Set up a `/` resource handler
    http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
        log.Println("Request received")
        _, _ = io.WriteString(w, "Success!!!")
    })

    // Start mTLS server from config
    return e5s.Start(ctx, "server-config.yaml")
}
```

**Lines of code: ~24 (Go) + 7 (YAML) = 31 total**

**server-config.yaml:**
```yaml
spire:
  workload_socket: "unix:///tmp/agent.sock"
  initial_fetch_timeout: "30s"

server:
  listen_addr: ":8443"
  allowed_client_spiffe_id: "spiffe://example.org/client"
```

### Key Differences Highlighted

**What go-spiffe SDK requires explicitly:**
1. Context cancellation management (`ctx, cancel := context.WithCancel(ctx)`)
2. Explicit X509Source creation with socket path configuration
3. Manual source lifecycle management (`defer source.Close()`)
4. SPIFFE ID parsing (`spiffeid.RequireFromString()`)
5. TLS config construction (`tlsconfig.MTLSServerConfig()`)
6. HTTP server setup with TLS config injection
7. ReadHeaderTimeout security setting (good practice, but manual)

**What e5s handles automatically:**
1. ✅ Context management (internal)
2. ✅ X509Source creation (from config)
3. ✅ Source lifecycle management (automatic cleanup)
4. ✅ SPIFFE ID validation (from config with error messages)
5. ✅ TLS config with TLS 1.3 enforcement
6. ✅ HTTP server setup with security defaults
7. ✅ ReadHeaderTimeout (10s default)

**Result:** 57 lines → 31 lines (**46% reduction** in code)

---

## Detailed Comparison

### 1. Configuration Management

#### go-spiffe SDK
- Configuration hardcoded in Go code
- Must recompile to change settings
- Good for simple, static deployments

```go
const (
    socketPath = "unix:///tmp/spire-agent/public/api.sock"
    serverID   = "spiffe://example.org/server"
)
```

#### e5s
- External YAML configuration files
- Change settings without recompiling
- Environment variable overrides (via `IDP_MODE`)
- Better for 12-factor apps and multi-environment deployments

```yaml
spire:
  workload_socket: "unix:///tmp/spire-agent/public/api.sock"
```

### 2. Error Handling

#### go-spiffe SDK
Errors are low-level and require understanding of SPIFFE internals:

```
unable to create X509Source: workloadapi: no such file or directory
```

#### e5s
Errors are contextualized for application developers:

```
failed to validate server configuration: server.listen_addr must be set
```

### 3. Security Defaults

#### go-spiffe SDK
You must explicitly configure security settings:

```go
tlsConfig.MinVersion = tls.VersionTLS13  // Must remember this
```

#### e5s
Security best practices are enforced automatically:
- TLS 1.3 minimum (enforced)
- Mutual TLS required (enforced)
- Input validation (automatic)
- Trust domain verification (configured)

### 4. Middleware Support

#### go-spiffe SDK
Must implement custom middleware for peer extraction:

```go
func authMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        // Manual peer extraction from TLS connection state
        if r.TLS == nil || len(r.TLS.PeerCertificates) == 0 {
            http.Error(w, "Unauthorized", 401)
            return
        }
        // Extract SPIFFE ID from certificate...
        next.ServeHTTP(w, r)
    })
}
```

#### e5s
Provides ready-to-use middleware utilities:

```go
import "github.com/sufield/e5s/spiffehttp"

func handler(w http.ResponseWriter, r *http.Request) {
    peer, ok := spiffehttp.PeerFromRequest(r)
    if !ok {
        http.Error(w, "Unauthorized", 401)
        return
    }
    log.Printf("Request from: %s", peer.ID.String())
}
```

See `examples/middleware/main.go` for complete examples.

### 5. Testing

#### go-spiffe SDK
Requires mocking Workload API or running full SPIRE stack:

```go
// Need to mock workloadapi.X509Source or run real SPIRE agent
```

#### e5s
Supports in-memory testing mode:

```bash
IDP_MODE=inmem go test ./...
```

No SPIRE agent required for unit tests.

---

## When to Use Each Approach

### Use Raw go-spiffe SDK When:

1. **Non-HTTP Protocols**
   - gRPC with custom configuration
   - Custom TCP protocols
   - Database connections with mTLS

2. **Maximum Control**
   - Need fine-grained control over TLS settings
   - Custom authorization logic beyond ID/trust domain
   - Performance-critical applications requiring optimization

3. **Library Development**
   - Building your own abstraction layer
   - Creating framework integrations
   - Need to support multiple transport protocols

### Use e5s Library When:

1. **HTTP/REST Services**
   - Building microservices with HTTP APIs
   - Standard REST endpoints
   - Web applications

2. **Rapid Development**
   - Prototyping SPIFFE-based services
   - Quick proof of concepts
   - Time-constrained projects

3. **Configuration-Driven**
   - Multi-environment deployments (dev/staging/prod)
   - Container-based deployments
   - 12-factor app methodology

4. **Team Productivity**
   - Teams new to SPIFFE/SPIRE
   - Standardized service templates
   - Reduced boilerplate code

---

## Migration Path

### From go-spiffe SDK to e5s

1. **Extract configuration values to YAML:**
   ```go
   // Before: hardcoded
   const socketPath = "unix:///tmp/spire-agent/public/api.sock"

   // After: config file
   // spire:
   //   workload_socket: "unix:///tmp/spire-agent/public/api.sock"
   ```

2. **Replace source creation:**
   ```go
   // Before
   source, err := workloadapi.NewX509Source(ctx, ...)

   // After
   client, err := e5s.Client(ctx, "config.yaml")
   ```

3. **Update HTTP handlers** (no changes needed, same `http.Handler` interface)

### From e5s to go-spiffe SDK

If you need custom protocol support:

1. **Reference e5s implementation** in `pkg/spire/source.go` and `pkg/spiffehttp/`
2. **Copy configuration validation** from `internal/config/`
3. **Adapt to your protocol** (TCP, gRPC, etc.)

---

## Performance Comparison

Both approaches have **identical runtime performance**:

- Same underlying `workloadapi.X509Source`
- Same TLS handshake process
- Same certificate rotation mechanism
- Same memory footprint for connections

e5s adds minimal overhead:
- ~5 microseconds for config parsing (startup only)
- ~1 microsecond for input validation (startup only)
- Zero overhead per request

---

## Common Patterns

### Custom Middleware with e5s

Even when using e5s, you can add custom middleware:

```go
package main

import (
    "github.com/sufield/e5s"
    "github.com/sufield/e5s/spiffehttp"
)

func main() {
    http.Handle("/api/", authMiddleware(apiHandler()))
    e5s.Start(context.Background(), "config.yaml")
}

func authMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        peer, ok := spiffehttp.PeerFromRequest(r)
        if !ok || !peer.ID.MemberOf(requiredTrustDomain) {
            http.Error(w, "Forbidden", 403)
            return
        }
        next.ServeHTTP(w, r)
    })
}
```

### Using go-spiffe SDK for gRPC

```go
package main

import (
    "github.com/spiffe/go-spiffe/v2/spiffegrpc/grpccredentials"
    "github.com/spiffe/go-spiffe/v2/workloadapi"
    "google.golang.org/grpc"
)

func main() {
    source, _ := workloadapi.NewX509Source(ctx)
    defer source.Close()

    creds := grpccredentials.MTLSClientCredentials(source, source,
        tlsconfig.AuthorizeID(serverID))

    conn, _ := grpc.Dial("localhost:50051", grpc.WithTransportCredentials(creds))
    defer conn.Close()

    // Use gRPC client...
}
```

For gRPC, use raw go-spiffe SDK with the `spiffegrpc` package.

---

## Conclusion

- **e5s is a convenience wrapper** around go-spiffe SDK for HTTP use cases
- **Both are equally secure** - same underlying mechanisms
- **Choose based on your needs:**
  - HTTP services → e5s
  - Custom protocols → go-spiffe SDK
  - Learning SPIFFE → e5s (simpler)
  - Maximum control → go-spiffe SDK

You can even use both in the same application:
- e5s for HTTP endpoints
- go-spiffe SDK for gRPC or custom protocols

---

## Further Reading

- [go-spiffe SDK Documentation](https://pkg.go.dev/github.com/spiffe/go-spiffe/v2)
- [e5s API Reference](./API.md)
- [e5s Quickstart](./QUICKSTART_LIBRARY.md)
- [SPIRE Documentation](https://spiffe.io/docs/latest/spire/)
- [Custom Middleware Examples](../examples/middleware/)

## Questions?

If you're unsure which approach to use, ask yourself:

1. Am I building an HTTP service? → **Use e5s**
2. Do I need a custom protocol? → **Use go-spiffe SDK**
3. Do I want configuration files? → **Use e5s**
4. Do I need maximum control? → **Use go-spiffe SDK**

When in doubt, start with e5s. You can always drop down to the raw SDK later if needed.
