# mTLS Authentication Guide

## Table of Contents

1. [Overview](#overview)
2. [Quick Start](#quick-start)
3. [Architecture](#architecture)
4. [Server Implementation](#server-implementation)
5. [Client Implementation](#client-implementation)
6. [Identity Extraction](#identity-extraction)
7. [Configuration](#configuration)
8. [Authentication vs Authorization](#authentication-vs-authorization)
9. [Certificate Rotation](#certificate-rotation)
10. [Deployment](#deployment)
11. [Troubleshooting](#troubleshooting)
12. [Best Practices](#best-practices)
13. [Examples](#examples)

---

## Overview

This library provides mTLS (Mutual TLS) authentication using SPIFFE/SPIRE for service-to-service communication. It implements the **adapter pattern** and focuses solely on **authentication** (verifying identity), leaving **authorization** (access control) to the application layer.

### Key Features

- ✅ **Automatic Certificate Management**: Zero-downtime certificate rotation via SPIRE
- ✅ **mTLS Authentication**: Both client and server authenticate each other
- ✅ **Identity Extraction**: SPIFFE ID available to application handlers
- ✅ **Standard HTTP**: Compatible with Go's standard `http` package
- ✅ **Authentication Only**: No authorization logic - app decides access
- ✅ **Production Ready**: Battle-tested with comprehensive tests
- ✅ **Configuration Flexible**: YAML files with environment variable overrides

### What This Library Does

| ✅ Handles (Authentication) | ❌ Does NOT Handle (Authorization) |
|----------------------------|-----------------------------------|
| Identity verification via mTLS | Role-based access control (RBAC) |
| SVID validation | Resource-level permissions |
| Trust domain verification | Policy enforcement |
| Certificate rotation | Access control lists |
| Identity extraction | Business logic authorization |

---

## Quick Start

### Prerequisites

1. Go 1.25+
2. SPIRE server and agent running
3. Workload registrations created

### Server Example

```go
package main

import (
    "context"
    "fmt"
    "net/http"

    "github.com/pocket/hexagon/spire/internal/adapters/inbound/httpapi"
    "github.com/spiffe/go-spiffe/v2/spiffeid"
    "github.com/spiffe/go-spiffe/v2/spiffetls/tlsconfig"
)

func main() {
    ctx := context.Background()

    // Create server with trust domain authentication
    authorizer := tlsconfig.AuthorizeMemberOf(
        spiffeid.RequireTrustDomainFromString("example.org"),
    )

    server, err := httpapi.NewHTTPServer(
        ctx,
        ":8443",
        "unix:///tmp/spire-agent/public/api.sock",
        authorizer,
    )
    if err != nil {
        panic(err)
    }
    defer server.Stop(ctx)

    // Register handler
    server.RegisterHandler("/api/hello", func(w http.ResponseWriter, r *http.Request) {
        clientID, ok := httpapi.GetSPIFFEID(r)
        if !ok {
            http.Error(w, "No identity", http.StatusUnauthorized)
            return
        }

        fmt.Fprintf(w, "Hello, %s!\n", clientID.String())
    })

    server.Start(ctx)
    select {} // Block forever
}
```

### Client Example

```go
package main

import (
    "context"
    "io"
    "os"

    "github.com/pocket/hexagon/spire/internal/adapters/outbound/httpclient"
    "github.com/spiffe/go-spiffe/v2/spiffeid"
    "github.com/spiffe/go-spiffe/v2/spiffetls/tlsconfig"
)

func main() {
    ctx := context.Background()

    // Create client with server identity verification
    serverID := spiffeid.RequireFromString("spiffe://example.org/server")
    client, err := httpclient.NewSPIFFEHTTPClient(
        ctx,
        "unix:///tmp/spire-agent/public/api.sock",
        tlsconfig.AuthorizeID(serverID),
    )
    if err != nil {
        panic(err)
    }
    defer client.Close()

    // Make authenticated request
    resp, err := client.Get(ctx, "https://localhost:8443/api/hello")
    if err != nil {
        panic(err)
    }
    defer resp.Body.Close()

    io.Copy(os.Stdout, resp.Body)
}
```

### Run the Examples

```bash
# Terminal 1: Start server
go run ./examples/mtls-adapters/server

# Terminal 2: Run client
go run ./examples/mtls-adapters/client
```

---

## Architecture

### Component Diagram

```
┌─────────────────────────────────────────────────────────┐
│                  Application Layer                       │
│              (Your Business Logic)                       │
│                                                          │
│  - Receives authenticated SPIFFE ID                     │
│  - Performs authorization (RBAC, ABAC, etc.)            │
│  - Executes business logic                              │
└──────────────────┬──────────────────────────────────────┘
                   │
                   ▼
┌─────────────────────────────────────────────────────────┐
│           mTLS Adapter Layer (This Library)             │
│                                                          │
│  Inbound: httpapi                  Outbound: httpclient │
│  - HTTPServer                      - SPIFFEHTTPClient   │
│  - Identity middleware             - All HTTP methods   │
│  - Identity utilities              - Server verification│
└──────────────────┬──────────────────────────────────────┘
                   │
                   ▼
┌─────────────────────────────────────────────────────────┐
│              Infrastructure Layer                        │
│                                                          │
│  - go-spiffe SDK v2.6.0                                 │
│  - workloadapi.X509Source (auto SVID rotation)          │
│  - tlsconfig.MTLSServerConfig/MTLSClientConfig          │
│  - SPIRE Workload API (Unix socket)                     │
└─────────────────────────────────────────────────────────┘
```

### mTLS Handshake Flow

```
Client                    SPIRE Agent              Server
  │                            │                       │
  │ 1. Request SVID            │                       │
  ├───────────────────────────>│                       │
  │                            │                       │
  │ 2. Validate workload       │                       │
  │    Return X.509 SVID       │                       │
  │<───────────────────────────┤                       │
  │                            │                       │
  │ 3. Initiate TLS            │                       │
  │    Present client cert     │                       │
  ├───────────────────────────────────────────────────>│
  │                            │                       │
  │                            │   4. Request SVID     │
  │                            │<──────────────────────┤
  │                            │                       │
  │                            │   5. Validate workload│
  │                            │      Return SVID      │
  │                            ├──────────────────────>│
  │                            │                       │
  │ 6. Verify server cert      │   7. Verify client    │
  │    Check trust bundle      │      Check trust      │
  │    Validate SPIFFE ID      │      Validate ID      │
  │                            │                       │
  │ 8. mTLS connection established                     │
  │<───────────────────────────────────────────────────┤
  │                            │                       │
  │ 9. Extract peer SPIFFE ID  │                       │
  │    Add to request context  │                       │
  │                            │                       │
  │ 10. Handle request         │                       │
  │     (Authorization by app) │                       │
  │                            │                       │
  │ 11. Response               │                       │
  │<───────────────────────────────────────────────────┤
```

---

## Server Implementation

### Creating an mTLS Server

```go
import (
    "context"
    "github.com/pocket/hexagon/spire/internal/adapters/inbound/httpapi"
    "github.com/spiffe/go-spiffe/v2/spiffetls/tlsconfig"
)

func main() {
    ctx := context.Background()

    server, err := httpapi.NewHTTPServer(
        ctx,
        ":8443",                                    // Listen address
        "unix:///tmp/spire-agent/public/api.sock", // SPIRE socket
        tlsconfig.AuthorizeAny(),                  // Client authorizer
    )
    if err != nil {
        panic(err)
    }
    defer server.Stop(ctx)

    // Register handlers...
    server.Start(ctx)
}
```

### Authorizer Options

The server uses go-spiffe's built-in authorizers to verify client identity during the TLS handshake:

#### 1. AuthorizeAny() - Accept Any Client

```go
authorizer := tlsconfig.AuthorizeAny()
```

Use for: Development, internal networks where all authenticated clients are trusted.

#### 2. AuthorizeID() - Specific Client ID

```go
clientID := spiffeid.RequireFromString("spiffe://example.org/client")
authorizer := tlsconfig.AuthorizeID(clientID)
```

Use for: Point-to-point connections, dedicated client-server pairs.

#### 3. AuthorizeMemberOf() - Trust Domain

```go
trustDomain := spiffeid.RequireTrustDomainFromString("example.org")
authorizer := tlsconfig.AuthorizeMemberOf(trustDomain)
```

Use for: Production environments, multi-service deployments.

#### 4. AuthorizeOneOf() - Multiple Allowed IDs

```go
authorizer := tlsconfig.AuthorizeOneOf(
    spiffeid.RequireFromString("spiffe://example.org/client1"),
    spiffeid.RequireFromString("spiffe://example.org/client2"),
    spiffeid.RequireFromString("spiffe://example.org/client3"),
)
```

Use for: Load balancers, multiple client instances.

### Registering Handlers

```go
server.RegisterHandler("/api/resource", func(w http.ResponseWriter, r *http.Request) {
    // Handler automatically receives authenticated client identity in context
    clientID, ok := httpapi.GetSPIFFEID(r)
    if !ok {
        http.Error(w, "No identity", http.StatusInternalServerError)
        return
    }

    // Your handler logic here
})
```

### Graceful Shutdown

```go
import (
    "os"
    "os/signal"
    "syscall"
)

// Setup signal handling
sigChan := make(chan os.Signal, 1)
signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

// Start server
go func() {
    if err := server.Start(ctx); err != nil {
        log.Fatal(err)
    }
}()

// Wait for signal
<-sigChan

// Shutdown gracefully
shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
defer cancel()
server.Stop(shutdownCtx)
```

---

## Client Implementation

### Creating an mTLS Client

```go
import (
    "context"
    "github.com/pocket/hexagon/spire/internal/adapters/outbound/httpclient"
    "github.com/spiffe/go-spiffe/v2/spiffeid"
    "github.com/spiffe/go-spiffe/v2/spiffetls/tlsconfig"
)

func main() {
    ctx := context.Background()

    // Verify server identity
    serverID := spiffeid.RequireFromString("spiffe://example.org/server")
    client, err := httpclient.NewSPIFFEHTTPClient(
        ctx,
        "unix:///tmp/spire-agent/public/api.sock",
        tlsconfig.AuthorizeID(serverID),
    )
    if err != nil {
        panic(err)
    }
    defer client.Close()

    // Use client...
}
```

### HTTP Methods

```go
// GET
resp, err := client.Get(ctx, "https://server:8443/api/resource")

// POST
body := strings.NewReader(`{"key":"value"}`)
resp, err := client.Post(ctx, url, "application/json", body)

// PUT
body := strings.NewReader(`{"key":"updated"}`)
resp, err := client.Put(ctx, url, "application/json", body)

// DELETE
resp, err := client.Delete(ctx, "https://server:8443/api/resource/123")

// PATCH
body := strings.NewReader(`{"key":"patched"}`)
resp, err := client.Patch(ctx, url, "application/json", body)

// Custom request
req, _ := http.NewRequestWithContext(ctx, "OPTIONS", url, nil)
resp, err := client.Do(req)
```

### Client Configuration

```go
// Set timeout
client.SetTimeout(60 * time.Second)

// Access underlying http.Client for advanced configuration
httpClient := client.GetHTTPClient()
transport := httpClient.Transport.(*http.Transport)
transport.MaxIdleConns = 200
```

---

## Identity Extraction

The library provides 15+ helper functions for working with SPIFFE identities in request handlers.

### Core Extraction

```go
// Get SPIFFE ID with error handling
clientID, ok := httpapi.GetSPIFFEID(r)
if !ok {
    http.Error(w, "No identity", http.StatusUnauthorized)
    return
}

// Get SPIFFE ID or panic (for guaranteed contexts)
clientID := httpapi.MustGetSPIFFEID(r)

// Get ID as string
idStr := httpapi.GetIDString(r)  // "spiffe://example.org/service"
```

### Trust Domain Operations

```go
// Extract trust domain
td, ok := httpapi.GetTrustDomain(r)

// Check trust domain match
if httpapi.MatchesTrustDomain(r, "example.org") {
    // Client from example.org
}
```

### Path Operations

SPIFFE IDs have a hierarchical path structure: `spiffe://trust-domain/path/to/workload`

```go
// Extract path
path, ok := httpapi.GetPath(r)  // "/service/frontend"

// Check path prefix (useful for service type detection)
if httpapi.HasPathPrefix(r, "/service/") {
    // Client is a service workload
}

// Check path suffix (useful for role-like patterns)
if httpapi.HasPathSuffix(r, "/admin") {
    // Client has admin role (application-defined)
}

// Get path segments
segments, ok := httpapi.GetPathSegments(r)
// For spiffe://example.org/ns/prod/service/api
// segments = []string{"ns", "prod", "service", "api"}
```

### Identity Matching

```go
// Exact ID match
if httpapi.MatchesID(r, "spiffe://example.org/service/frontend") {
    // Specific service identity
}
```

### Middleware

```go
import "github.com/pocket/hexagon/spire/internal/adapters/inbound/httpapi"

// Require authentication
mux.Handle("/api/", httpapi.RequireAuthentication(apiHandler))

// Require specific trust domain
handler := httpapi.RequireTrustDomain("example.org", apiHandler)

// Require path prefix
handler := httpapi.RequirePathPrefix("/service/", apiHandler)

// Log all identities (for debugging)
mux.Handle("/api/", httpapi.LogIdentity(apiHandler))

// Chain middleware
handler := httpapi.RequireAuthentication(
    httpapi.RequireTrustDomain("example.org",
        httpapi.RequirePathPrefix("/service/",
            httpapi.LogIdentity(apiHandler),
        ),
    ),
)
```

---

## Configuration

### YAML Configuration File

Create `config.yaml`:

```yaml
http:
  enabled: true
  address: ":8443"
  timeout: 30s
  authentication:
    policy: trust-domain
    trust_domain: example.org

spire:
  socket_path: unix:///tmp/spire-agent/public/api.sock
  trust_domain: example.org
```

Load in code:

```go
import "github.com/pocket/hexagon/spire/internal/config"

cfg, err := config.LoadFromFile("config.yaml")
if err != nil {
    panic(err)
}

if err := cfg.Validate(); err != nil {
    panic(err)
}

// Use config...
```

### Environment Variables

Environment variables override YAML values:

```bash
export SPIRE_AGENT_SOCKET="unix:///custom/socket"
export SPIRE_TRUST_DOMAIN="production.example.org"
export HTTP_ADDRESS=":9443"
export AUTH_POLICY="specific-id"
export ALLOWED_CLIENT_ID="spiffe://example.org/client"
```

Load environment-only:

```go
cfg := config.LoadFromEnv()
```

### Configuration Options

| YAML Key | Env Var | Description | Default |
|----------|---------|-------------|---------|
| `spire.socket_path` | `SPIRE_AGENT_SOCKET` | SPIRE agent socket | `unix:///tmp/spire-agent/public/api.sock` |
| `spire.trust_domain` | `SPIRE_TRUST_DOMAIN` | Trust domain | `example.org` |
| `http.address` | `HTTP_ADDRESS` | Listen address | `:8443` |
| `http.enabled` | `HTTP_ENABLED` | Enable HTTP | `false` |
| `http.timeout` | - | Request timeout | `30s` |
| `http.authentication.policy` | `AUTH_POLICY` | Auth policy | `any` |
| `http.authentication.allowed_id` | `ALLOWED_CLIENT_ID` or `EXPECTED_SERVER_ID` | Single allowed ID | - |
| `http.authentication.trust_domain` | `AUTH_TRUST_DOMAIN` | Auth trust domain | Same as `spire.trust_domain` |

---

## Authentication vs Authorization

This library handles **authentication only**. Your application must implement **authorization**.

### Authentication (This Library) ✅

```go
func handler(w http.ResponseWriter, r *http.Request) {
    // AUTHENTICATION: Verify who the client is
    clientID, ok := httpapi.GetSPIFFEID(r)
    if !ok {
        http.Error(w, "Unauthorized", http.StatusUnauthorized)
        return
    }

    // At this point, we know WHO the client is (authenticated)
    // ...
}
```

### Authorization (Your Application) 🔒

```go
func handler(w http.ResponseWriter, r *http.Request) {
    // AUTHENTICATION: Verify who the client is
    clientID, ok := httpapi.GetSPIFFEID(r)
    if !ok {
        http.Error(w, "Unauthorized", http.StatusUnauthorized)
        return
    }

    // AUTHORIZATION: Decide what the client can do
    if !myAuthzService.IsAllowed(clientID, "read", "resource") {
        http.Error(w, "Forbidden", http.StatusForbidden)
        return
    }

    // Handle authorized request
    // ...
}
```

### Common Authorization Patterns

#### 1. Path-Based Roles

```go
// Application defines roles via SPIFFE ID path
if httpapi.HasPathSuffix(r, "/admin") {
    // Client has admin role
} else if httpapi.HasPathSuffix(r, "/readonly") {
    // Client has readonly role
}
```

#### 2. Service Type Detection

```go
segments, _ := httpapi.GetPathSegments(r)
if len(segments) > 0 {
    switch segments[0] {
    case "service":
        // Service-to-service request
    case "user":
        // User request
    case "system":
        // System request
    }
}
```

#### 3. External Authorization Service

```go
clientID, _ := httpapi.GetSPIFFEID(r)

// Call external authz service (OPA, Casbin, etc.)
decision, err := opaClient.Evaluate(clientID.String(), resource, action)
if err != nil || !decision.Allowed {
    http.Error(w, "Forbidden", http.StatusForbidden)
    return
}
```

---

## Certificate Rotation

### Automatic Rotation

The library uses `workloadapi.X509Source` which automatically rotates certificates:

- **Default SVID TTL**: 1 hour
- **Rotation**: Happens automatically ~30 minutes before expiry
- **Zero Downtime**: New connections use new cert, existing connections continue

### Monitoring Rotation

```go
// X509Source handles rotation internally
// No application code required

// Optional: Log rotation events (advanced)
source.WatchX509Context(ctx, &x509ContextWatcher{
    OnX509ContextUpdate: func(ctx *workloadapi.X509Context) {
        log.Printf("Certificate rotated: %s", ctx.DefaultSVID().ID)
    },
})
```

### Rotation Timeline

```
Time:     0m          30m         60m         90m
          │           │           │           │
SVID 1:   │═══════════│═══════════│           │
          │           │           │           │
SVID 2:   │           │═══════════│═══════════│
          │           │           │           │
          │           ▲           │           │
          │      Rotation      Expiry        │
          │      Triggered                   │
```

---

## Deployment

### Local Development

```bash
# 1. Start SPIRE
# See: https://spiffe.io/docs/latest/spire/installing/

# 2. Register workloads
spire-server entry create \
  -spiffeID spiffe://example.org/server \
  -parentID spiffe://example.org/agent \
  -selector unix:uid:$(id -u)

spire-server entry create \
  -spiffeID spiffe://example.org/client \
  -parentID spiffe://example.org/agent \
  -selector unix:uid:$(id -u)

# 3. Run server
go run ./examples/mtls-adapters/server

# 4. Run client
go run ./examples/mtls-adapters/client
```

### Kubernetes

See [examples/mtls-adapters/README.md](../examples/mtls-adapters/README.md) for complete Kubernetes deployment guide.

```bash
# Build images
docker build -t mtls-server:latest -f examples/mtls-adapters/server/Dockerfile .
docker build -t mtls-client:latest -f examples/mtls-adapters/client/Dockerfile .

# Register workloads
./examples/mtls-adapters/k8s/spire-registrations.sh

# Deploy
kubectl apply -f examples/mtls-adapters/k8s/server-deployment.yaml
kubectl apply -f examples/mtls-adapters/k8s/client-job.yaml
```

### Docker

```bash
# Run with SPIRE agent socket mounted
docker run -d \
  --name mtls-server \
  -v /tmp/spire-agent/public:/spire-agent-socket:ro \
  -p 8443:8443 \
  mtls-server:latest
```

---

## Troubleshooting

### 1. "Failed to create X509Source: context deadline exceeded"

**Problem**: Cannot connect to SPIRE agent socket.

**Solutions**:
- Verify SPIRE agent is running: `ps aux | grep spire-agent`
- Check socket exists: `ls -la /tmp/spire-agent/public/api.sock`
- Verify socket permissions
- Check `SPIRE_AGENT_SOCKET` environment variable

### 2. "TLS handshake failed"

**Problem**: mTLS authentication failed.

**Possible Causes**:
- Client and server in different trust domains
- Server's `ALLOWED_CLIENT_ID` doesn't match client
- Client's `EXPECTED_SERVER_ID` doesn't match server
- Workload not registered in SPIRE

**Solutions**:
```bash
# Verify registrations
spire-server entry show

# Check both can fetch SVIDs
spire-agent api fetch x509
```

### 3. "No identity issued"

**Problem**: Workload not registered.

**Solution**:
```bash
spire-server entry create \
  -spiffeID spiffe://example.org/myservice \
  -parentID spiffe://example.org/agent \
  -selector unix:uid:$(id -u)
```

### 4. "Connection refused"

**Problem**: Server not reachable.

**Solutions**:
- Verify server is running
- Check address/port match
- Check firewall rules
- In Kubernetes, verify Service is created

---

## Best Practices

### 1. Use Specific Authorizers in Production

```go
// ❌ Don't use in production
authorizer := tlsconfig.AuthorizeAny()

// ✅ Use specific trust domain
authorizer := tlsconfig.AuthorizeMemberOf(
    spiffeid.RequireTrustDomainFromString("production.example.org"),
)

// ✅ Or specific IDs
authorizer := tlsconfig.AuthorizeID(
    spiffeid.RequireFromString("spiffe://production.example.org/client"),
)
```

### 2. Always Close Resources

```go
server, err := httpapi.NewHTTPServer(ctx, addr, socket, authorizer)
if err != nil {
    return err
}
defer server.Stop(ctx)  // ✅ Always defer Stop

client, err := httpclient.NewSPIFFEHTTPClient(ctx, socket, authorizer)
if err != nil {
    return err
}
defer client.Close()  // ✅ Always defer Close
```

### 3. Use Environment Variables for Secrets

```go
// ❌ Don't hardcode
socketPath := "unix:///tmp/spire-agent/public/api.sock"

// ✅ Use environment variables
socketPath := os.Getenv("SPIRE_AGENT_SOCKET")
if socketPath == "" {
    socketPath = "unix:///tmp/spire-agent/public/api.sock"  // Default
}
```

### 4. Implement Graceful Shutdown

```go
sigChan := make(chan os.Signal, 1)
signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

go func() {
    server.Start(ctx)
}()

<-sigChan

shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
defer cancel()
server.Stop(shutdownCtx)  // ✅ Graceful shutdown
```

### 5. Use Middleware for Common Patterns

```go
// ✅ Reusable authentication middleware
requireAuth := httpapi.RequireAuthentication(
    httpapi.RequireTrustDomain("example.org", nil),
)

mux.Handle("/api/", requireAuth)
```

### 6. Log Identity for Audit

```go
// ✅ Log all authenticated requests
handler := httpapi.LogIdentity(apiHandler)
```

### 7. Validate Configuration

```go
cfg, err := config.LoadFromFile("config.yaml")
if err != nil {
    return err
}

if err := cfg.Validate(); err != nil {  // ✅ Always validate
    return err
}
```

---

## Examples

Complete working examples are available in [`examples/mtls-adapters/`](../examples/mtls-adapters/):

### Available Examples

1. **Server Example** ([server/main.go](../examples/mtls-adapters/server/main.go))
   - 4 endpoints: /api/hello, /api/echo, /api/identity, /health
   - Demonstrates all identity utilities
   - Graceful shutdown
   - Environment-based configuration

2. **Client Example** ([client/main.go](../examples/mtls-adapters/client/main.go))
   - Makes requests to all server endpoints
   - Demonstrates all HTTP methods
   - Proper error handling

3. **Kubernetes Deployment** ([k8s/](../examples/mtls-adapters/k8s/))
   - Complete manifests for server and client
   - Automated workload registration script
   - Health probes and resource limits

### Running Examples

```bash
# Local
go run ./examples/mtls-adapters/server
go run ./examples/mtls-adapters/client

# Kubernetes
./examples/mtls-adapters/k8s/spire-registrations.sh
kubectl apply -f examples/mtls-adapters/k8s/
```

---

## References

### Documentation

- [MTLS_IMPLEMENTATION.md](MTLS_IMPLEMENTATION.md) - Implementation plan
- [ITERATIONS_COMPLETE_SUMMARY.md](ITERATIONS_COMPLETE_SUMMARY.md) - Complete implementation summary
- [examples/mtls-adapters/README.md](../examples/mtls-adapters/README.md) - Example usage guide

### External Resources

- [SPIFFE Specification](https://github.com/spiffe/spiffe)
- [SPIRE Documentation](https://spiffe.io/docs/)
- [go-spiffe SDK](https://github.com/spiffe/go-spiffe)
- [go-spiffe Examples](https://github.com/spiffe/go-spiffe/tree/main/v2/examples)

### API Reference

- [httpapi Package](../internal/adapters/inbound/httpapi/) - Server implementation
- [httpclient Package](../internal/adapters/outbound/httpclient/) - Client implementation
- [config Package](../internal/config/) - Configuration support

---

## Support

For issues and questions:

- **Examples**: See [examples/mtls-adapters/README.md](../examples/mtls-adapters/README.md)
- **Troubleshooting**: See [Troubleshooting](#troubleshooting) section above
- **SPIRE Issues**: [spiffe/spire GitHub](https://github.com/spiffe/spire/issues)

---

**Last Updated**: 2025-10-07
**Version**: 1.0.0
