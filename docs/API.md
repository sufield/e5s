# API Documentation

API reference for the e5s SPIRE mTLS library with all package, function, and type details is available at:

**https://pkg.go.dev/github.com/sufield/e5s**

This generated documentation includes:
- Exported functions, types, and constants
- Function signatures and return values
- Usage examples
- Cross-referenced type definitions
- Source code links

## Viewing Documentation Locally

Use the `go doc` command:

View package-level documentation

```bash 
go doc github.com/sufield/e5s
```

View specific function documentation

```bash
go doc github.com/sufield/e5s.Run
go doc github.com/sufield/e5s.Start
go doc github.com/sufield/e5s.Client
```

View all exported symbols

```bash
go doc -all github.com/sufield/e5s
```

Or run a local documentation server:

Install godoc (if not already installed)

```bash
go install golang.org/x/tools/cmd/godoc@latest
```

Start local documentation server

```bash
godoc -http=:6060
```

Visit http://localhost:6060/pkg/github.com/sufield/e5s/

## API Overview

The e5s library provides two levels of API:

### 1. High-Level API (e5s package)

**For:** Application developers building mTLS services

#### Server Functions

- **`e5s.Run(handler http.Handler)`**
  Convention-over-configuration server that starts and blocks until Ctrl+C

- **`e5s.Start(configPath string, handler http.Handler) (shutdown func() error, error)`**
  Config-based server with explicit lifecycle management

- **`e5s.StartServer(handler http.Handler) (shutdown func() error, error)`**
  Environment-variable-based server with defaults

- **`e5s.PeerID(r *http.Request) (string, bool)`**
  Extract authenticated peer's SPIFFE ID from request

- **`e5s.PeerInfo(r *http.Request) (Peer, bool)`**
  Extract full peer information (ID + certificates) from request

#### Client Functions

- **`e5s.Client(configPath string) (*http.Client, func() error, error)`**
  Create an HTTP client configured for mTLS with SPIRE

**Example:** See [QUICKSTART_LIBRARY.md](QUICKSTART_LIBRARY.md)

### 2. Low-Level API (pkg/* packages)

**For:** Library developers needing fine-grained control

#### pkg/spiffehttp

TLS configuration for HTTP mTLS using SPIFFE identities.

**Types:**
- `ServerConfig` - Server authorization configuration
- `ClientConfig` - Client server verification configuration
- `Peer` - Authenticated peer information

**Functions:**
- `NewServerTLSConfig()` - Create server TLS config with client verification
- `NewClientTLSConfig()` - Create client TLS config with server verification
- `PeerFromContext()` - Extract peer info from request context

**Documentation:** https://pkg.go.dev/github.com/sufield/e5s/spiffehttp

#### pkg/spire

SPIRE Workload API integration.

**Types:**
- `Source` - SPIRE X.509 certificate source with auto-rotation
- `Config` - SPIRE connection configuration

**Functions:**
- `NewIdentitySource()` - Connect to SPIRE Workload API and fetch identities

**Documentation:** https://pkg.go.dev/github.com/sufield/e5s/spire

## Configuration

### Configuration File

**Standard config file:** [examples/highlevel/e5s.yaml](../examples/highlevel/e5s.yaml)

This single annotated config file contains both server and client configuration sections with detailed comments.

**Environment-specific files:** Create `e5s.dev.yaml`, `e5s.staging.yaml`, `e5s.prod.yaml` in your application with environment-specific values.

**Server configuration options:**
- `spire.workload_socket` - Path to SPIRE agent socket
- `spire.initial_fetch_timeout` - Timeout for fetching initial identity (default: 30s)
- `server.listen_addr` - Address to listen on (e.g., `:8443`)
- `server.allowed_client_spiffe_id` - Allow specific client SPIFFE ID
- `server.allowed_client_trust_domain` - Allow any client in trust domain

**Client configuration options:**
- `client.server_url` - Server URL to connect to
- `client.expected_server_spiffe_id` - Require specific server SPIFFE ID
- `client.expected_server_trust_domain` - Trust any server in domain

**Environment Variables (for `StartServer` or `Run`):**

- `E5S_CONFIG` - Path to config file (if not passed as parameter)
- `SPIRE_WORKLOAD_SOCKET` - Path to SPIRE agent socket (default: `unix:///tmp/spire-agent/public/api.sock`)
- `LISTEN_ADDR` - Server listen address (default: `:8443`)
- `ALLOWED_CLIENT_SPIFFE_ID` - Specific client SPIFFE ID to allow
- `ALLOWED_CLIENT_TRUST_DOMAIN` - Trust domain to allow (alternative to specific ID)
- `INITIAL_FETCH_TIMEOUT` - Timeout for initial certificate fetch (default: `30s`)

## Common Usage

### 1. Convention-Over-Configuration Server

Minimal code, intelligent defaults from environment:

```go
package main

import (
    "fmt"
    "net/http"

    "github.com/sufield/e5s"
)

func main() {
    http.HandleFunc("/api", func(w http.ResponseWriter, r *http.Request) {
        id, ok := e5s.PeerID(r)
        if !ok {
            http.Error(w, "unauthorized", http.StatusUnauthorized)
            return
        }
        fmt.Fprintf(w, "Hello %s\n", id)
    })

    // Loads config from environment, blocks until Ctrl+C
    e5s.Run(http.DefaultServeMux)
}
```

### 2. Explicit Configuration Server

Full control over configuration and lifecycle:

```go
shutdown, err := e5s.Start("e5s.dev.yaml", myHandler)
if err != nil {
    log.Fatal(err)
}
defer shutdown()

// Run until interrupt signal
sigChan := make(chan os.Signal, 1)
signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
<-sigChan

if err := shutdown(); err != nil {
    log.Printf("Shutdown error: %v", err)
}
```

### 3. HTTP Client with mTLS

```go
client, shutdown, err := e5s.Client("e5s.dev.yaml")
if err != nil {
    log.Fatal(err)
}
defer shutdown()

resp, err := client.Get("https://secure-service:8443/api")
if err != nil {
    log.Fatal(err)
}
defer resp.Body.Close()

body, _ := io.ReadAll(resp.Body)
fmt.Println(string(body))
```

## Error Handling

All functions return detailed errors with context:

```go
shutdown, err := e5s.Start("e5s.dev.yaml", handler)
if err != nil {
    // Error will include context: config loading, SPIRE connection, server startup
    log.Fatalf("Failed to start server: %v", err)
}
```

Common error scenarios:
- **Config errors**: Invalid YAML, missing required fields, invalid SPIFFE IDs
- **SPIRE errors**: Cannot connect to agent socket, identity not registered, timeout
- **TLS errors**: Certificate validation failed, unsupported protocol
- **Server errors**: Port already in use, permission denied

## Type Reference

### spiffehttp.Peer

Information about an authenticated peer:

```go
type Peer struct {
    ID    spiffeid.ID              // SPIFFE ID of the peer
    Certs []*x509.Certificate      // Peer's certificate chain
}
```

Access in handlers:

```go
peer, ok := e5s.PeerInfo(r)
if ok {
    fmt.Printf("Peer ID: %s\n", peer.ID)
    fmt.Printf("Certificate CN: %s\n", peer.Certs[0].Subject.CommonName)
}
```

## Security Considerations

### TLS Configuration

The library enforces secure defaults:
- **TLS 1.3 only** (TLS 1.2 and below are NOT allowed)
- **Mutual TLS** required (both parties present certificates)
- **Automatic certificate rotation** (zero downtime)
- **SPIFFE ID verification** per configuration

### Authorization

Choose the appropriate authorization model:

1. **Specific SPIFFE ID** - Most restrictive, allows only one identity
   ```yaml
   allowed_client_spiffe_id: "spiffe://example.org/specific-client"
   ```

2. **Trust Domain** - Allow any identity from the trust domain
   ```yaml
   allowed_client_trust_domain: "example.org"
   ```

### Deployment

- **Never** run with elevated privileges unless required
- **Always** use Unix domain sockets for SPIRE agent communication when possible
- **Monitor** certificate expiration and rotation in production
- **Implement** proper logging and observability

## Testing

### Unit Tests

Mock the SPIRE Workload API for unit testing:

```go
// Use dependency injection and mock the underlying SPIRE source
```

### Integration Tests

See [integration tests documentation](integration-tests.md) for running tests against real SPIRE deployments.

### Local Testing

Use the provided scripts:
- `scripts/quick-test-spire.sh` - Fast local test with SPIRE
- `scripts/test-prod-binary-minikube.sh` - Full integration test in Kubernetes

## Performance Considerations

### Certificate Rotation

- Certificates rotate automatically in the background
- No application downtime during rotation
- Rotation triggered by SPIRE agent updates

### Connection Pooling

The HTTP client uses connection pooling by default:
- Connections are reused across requests
- TLS handshake overhead minimized
- Configure `http.Transport` if custom tuning needed

### Resource Usage

- **Memory**: Modest footprint, primarily certificate storage
- **CPU**: TLS operations use CPU, but modern hardware handles this well
- **Network**: Only SPIRE agent socket connection (local Unix domain socket)

## Migration Guide

### From Raw TLS to e5s

**Before:**
```go
cert, _ := tls.LoadX509KeyPair("server.crt", "server.key")
server := &http.Server{
    TLSConfig: &tls.Config{Certificates: []tls.Certificate{cert}},
}
server.ListenAndServeTLS("", "")
```

**After:**
```go
e5s.Run(myHandler)  // Automatic mTLS with SPIRE
```

### From go-spiffe SDK to e5s

**Before:**
```go
source, _ := workloadapi.NewX509Source(ctx)
listener, _ := spiffetls.Listen(ctx, "tcp", ":8443", source)
server := &http.Server{Handler: myHandler}
server.Serve(listener)
```

**After:**
```go
e5s.Run(myHandler)  // Same result, less code
```

## Troubleshooting

### Common Issues

**"Cannot connect to SPIRE agent"**
- Check SPIRE agent is running
- Verify socket path in configuration
- Check file permissions on socket

**"Identity not found"**
- Ensure workload is registered in SPIRE
- Verify SPIFFE ID matches registration
- Check SPIRE agent logs

**"TLS handshake failed"**
- Verify both parties have valid certificates
- Check SPIFFE ID authorization configuration
- Ensure clocks are synchronized

**"Certificate has expired"**
- This shouldn't happen with automatic rotation
- Check SPIRE agent connectivity
- Verify agent is receiving updates

### Debug Logging

Enable debug logging:

```go
log.SetFlags(log.Ldate | log.Ltime | log.Lshortfile)
```

## Support and Resources

- **Documentation**: https://github.com/sufield/e5s/tree/main/docs
- **Examples**: See `examples/` directory
- **Issues**: https://github.com/sufield/e5s/issues
- **Discussions**: https://github.com/sufield/e5s/discussions
- **SPIRE Documentation**: https://spiffe.io/docs/latest/spire/

## Related Documentation

- [Quickstart Guide](QUICKSTART_LIBRARY.md) - Get started in 5 minutes
- [Integration Tests](integration-tests.md) - Testing guide
- [Architecture](e5s.md) - Design decisions and internals
