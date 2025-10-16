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

This library provides mTLS (Mutual TLS) authentication using SPIFFE/SPIRE for service-to-service communication. It follows hexagonal architecture principles and focuses solely on **authentication** (verifying identity), leaving **authorization** (access control) to the application layer.

### Features

- ✅ **Automatic Certificate Management**: Zero-downtime certificate rotation via SPIRE
- ✅ **mTLS Authentication**: Both client and server authenticate each other using X.509 SVIDs
- ✅ **Identity Extraction**: SPIFFE ID available to application handlers through simple APIs
- ✅ **Standard HTTP**: Compatible with Go's standard `http` package
- ✅ **Authentication Only**: No authorization logic - application decides access control
- ✅ **Production Ready**: Battle-tested with comprehensive tests
- ✅ **Clean Architecture**: Port interfaces with adapter implementations

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

1. Go 1.21+
2. SPIRE server and agent running
3. Workload registrations created

### Server Example

```go
package main

import (
    "context"
    "fmt"
    "net/http"

    "github.com/pocket/hexagon/spire/internal/adapters/inbound/identityserver"
    "github.com/pocket/hexagon/spire/internal/ports"
)

func main() {
    ctx := context.Background()

    // Create server with mTLS authentication
    server, err := identityserver.New(ctx, ports.MTLSConfig{
        WorkloadAPI: ports.WorkloadAPIConfig{
            SocketPath: "unix:///tmp/spire-agent/public/api.sock",
        },
        SPIFFE: ports.SPIFFEConfig{
            AllowedTrustDomain: "example.org", // Allow any client from this domain
        },
        HTTP: ports.HTTPConfig{
            Address: ":8443",
        },
    })
    if err != nil {
        panic(err)
    }
    defer server.Close()

    // Register handler
    server.Handle("/api/hello", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        clientID, ok := identityserver.GetIdentity(r)
        if !ok {
            http.Error(w, "No identity", http.StatusUnauthorized)
            return
        }

        fmt.Fprintf(w, "Hello, %s!\n", clientID.String())
    }))

    // Start server (blocks)
    server.Start(ctx)
}
```

### Client Example

```go
package main

import (
    "context"
    "io"
    "os"

    "github.com/pocket/hexagon/spire/examples/httpclient"
)

func main() {
    ctx := context.Background()

    // Create client with mTLS authentication
    client, err := httpclient.New(ctx, httpclient.Config{
        WorkloadAPI: httpclient.WorkloadAPIConfig{
            SocketPath: "unix:///tmp/spire-agent/public/api.sock",
        },
        SPIFFE: httpclient.SPIFFEConfig{
            ExpectedServerID: "spiffe://example.org/server", // Or leave empty for any
        },
        HTTP: httpclient.HTTPClientConfig{
            Timeout: 30 * time.Second,
        },
    })
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
│         Port Layer (Dependency Inversion)               │
│                                                          │
│  - ports.MTLSServer interface                           │
│  - ports.MTLSClient interface                           │
│  - ports.MTLSConfig (pure data)                         │
└──────────────────┬──────────────────────────────────────┘
                   │
                   ▼
┌─────────────────────────────────────────────────────────┐
│           Adapter Layer (Implementations)               │
│                                                          │
│  Inbound: identityserver    Outbound: httpclient        │
│  - spiffeServer             - spiffeClient              │
│  - Identity extraction      - All HTTP methods          │
│  - Helper utilities         - Server verification       │
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

The server uses the `ports.MTLSServer` interface, implemented by `identityserver.New()`:

```go
import (
    "context"
    "time"

    "github.com/pocket/hexagon/spire/internal/adapters/inbound/identityserver"
    "github.com/pocket/hexagon/spire/internal/ports"
)

func main() {
    ctx := context.Background()

    server, err := identityserver.New(ctx, ports.MTLSConfig{
        WorkloadAPI: ports.WorkloadAPIConfig{
            SocketPath: "unix:///tmp/spire-agent/public/api.sock",
        },
        SPIFFE: ports.SPIFFEConfig{
            // Choose ONE of these authorization policies:
            AllowedPeerID:      "spiffe://example.org/client",  // Specific client
            // OR
            AllowedTrustDomain: "example.org",                  // Any from domain
        },
        HTTP: ports.HTTPConfig{
            Address:           ":8443",
            ReadHeaderTimeout: 10 * time.Second,
            ReadTimeout:       30 * time.Second,
            WriteTimeout:      30 * time.Second,
            IdleTimeout:       60 * time.Second,
        },
    })
    if err != nil {
        panic(err)
    }
    defer server.Close()

    // Register handlers...
    server.Start(ctx)
}
```

### Authorization Policies

The server configuration requires **exactly one** of these policies:

#### 1. AllowedPeerID - Specific Client Identity

```go
SPIFFE: ports.SPIFFEConfig{
    AllowedPeerID: "spiffe://example.org/client",
}
```

**Use for**: Point-to-point connections, dedicated client-server pairs.

#### 2. AllowedTrustDomain - Any Client from Domain

```go
SPIFFE: ports.SPIFFEConfig{
    AllowedTrustDomain: "example.org",
}
```

**Use for**: Production environments with multiple services in the same trust domain.

### Registering Handlers

```go
// Register handler - returns error if called after Start()
err := server.Handle("/api/resource", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
    // Handler automatically receives authenticated client identity in context
    clientID, ok := identityserver.GetIdentity(r)
    if !ok {
        http.Error(w, "No identity", http.StatusInternalServerError)
        return
    }

    // Your handler logic here
    fmt.Fprintf(w, "Authenticated as: %s\n", clientID.String())
}))
if err != nil {
    log.Fatal(err)
}
```

### Graceful Shutdown

```go
import (
    "os"
    "os/signal"
    "syscall"
)

// Start server in goroutine
go func() {
    if err := server.Start(ctx); err != nil {
        log.Fatal(err)
    }
}()

// Setup signal handling
sigChan := make(chan os.Signal, 1)
signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
<-sigChan

// Shutdown gracefully
shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
defer cancel()

if err := server.Shutdown(shutdownCtx); err != nil {
    log.Printf("Shutdown error: %v", err)
}

// Close resources
server.Close()
```

---

## Client Implementation

### Creating an mTLS Client

The client is provided as an example implementation in `examples/httpclient`:

```go
import (
    "context"
    "time"

    "github.com/pocket/hexagon/spire/examples/httpclient"
)

func main() {
    ctx := context.Background()

    client, err := httpclient.New(ctx, httpclient.Config{
        WorkloadAPI: httpclient.WorkloadAPIConfig{
            SocketPath: "unix:///tmp/spire-agent/public/api.sock",
        },
        SPIFFE: httpclient.SPIFFEConfig{
            // Optional: Verify specific server identity
            ExpectedServerID: "spiffe://example.org/server",
            // OR verify any server from specific trust domain
            ExpectedTrustDomain: "example.org",
            // OR leave both empty to accept any server from client's trust domain
        },
        HTTP: httpclient.HTTPClientConfig{
            Timeout:             30 * time.Second,
            MaxIdleConns:        100,
            MaxIdleConnsPerHost: 10,
            IdleConnTimeout:     90 * time.Second,
        },
    })
    if err != nil {
        panic(err)
    }
    defer client.Close()

    // Use client...
}
```

### HTTP Methods

```go
// GET request
resp, err := client.Get(ctx, "https://server:8443/api/resource")

// POST request
body := strings.NewReader(`{"key":"value"}`)
resp, err := client.Post(ctx, url, "application/json", body)

// Custom request with Do()
req, _ := http.NewRequestWithContext(ctx, "PUT", url, body)
req.Header.Set("Content-Type", "application/json")
resp, err := client.Do(req)
```

**Note**: The client currently supports `Get`, `Post`, and `Do` methods. For other HTTP methods (PUT, DELETE, PATCH), use `Do()` with a custom request.

---

## Identity Extraction

The library provides simple identity extraction functions through the `identityserver` package:

### Basic Extraction (Recommended)

```go
import "github.com/pocket/hexagon/spire/internal/adapters/inbound/identityserver"

func handler(w http.ResponseWriter, r *http.Request) {
    // Get SPIFFE ID - returns (spiffeid.ID, bool)
    clientID, ok := identityserver.GetIdentity(r)
    if !ok {
        http.Error(w, "No identity", http.StatusUnauthorized)
        return
    }

    // Use the identity
    log.Printf("Request from: %s", clientID.String())
    log.Printf("Trust domain: %s", clientID.TrustDomain().String())
    log.Printf("Path: %s", clientID.Path())
}
```

### Error-Based Extraction

```go
// Get SPIFFE ID with error - returns (spiffeid.ID, error)
clientID, err := identityserver.RequireIdentity(r)
if err != nil {
    http.Error(w, "Unauthorized", http.StatusUnauthorized)
    return
}
```

### Context-Based Extraction

```go
// Extract from context (useful for non-HTTP code)
clientID, ok := identityserver.IdentityFromContext(r.Context())
if !ok {
    // No identity in context
}
```

### Available Identity Methods

Once you have the `spiffeid.ID`, you can use these methods:

```go
clientID, _ := identityserver.GetIdentity(r)

// Get components
fullID := clientID.String()              // "spiffe://example.org/service/frontend"
domain := clientID.TrustDomain().String() // "example.org"
path := clientID.Path()                   // "/service/frontend"

// Check properties
isZero := clientID.IsZero()              // false for valid IDs

// Compare trust domains
sameDomain := clientID.TrustDomain() == otherID.TrustDomain()
```

### Advanced Path-Based Checks

For more advanced path-based authorization, use the `httpcontext` package (internal implementation detail):

```go
import "github.com/pocket/hexagon/spire/internal/adapters/inbound/identityserver/httpcontext"

// Check path prefix
if httpcontext.HasPathPrefix(r, "/service/") {
    // Client is a service workload
}

// Check path suffix
if httpcontext.HasPathSuffix(r, "/admin") {
    // Client has admin role (application-defined)
}

// Get path segments
segments, ok := httpcontext.GetPathSegments(r)
// For spiffe://example.org/ns/prod/service/api
// segments = []string{"ns", "prod", "service", "api"}

// Use segments for routing
if len(segments) >= 2 {
    namespace := segments[0]    // "ns"
    environment := segments[1]  // "prod"
}
```

### Middleware (Advanced)

The `httpcontext` package provides middleware for common patterns:

```go
import "github.com/pocket/hexagon/spire/internal/adapters/inbound/identityserver/httpcontext"

// Require authentication
mux.Handle("/api/", httpcontext.Authenticated(apiHandler))

// Require specific trust domain
expectedTD := spiffeid.RequireTrustDomainFromString("example.org")
handler := httpcontext.RequireTrustDomainID(expectedTD, apiHandler)

// Require path prefix
handler := httpcontext.RequirePathPrefix("/service/", apiHandler)

// Log all identities (debugging)
mux.Handle("/api/", httpcontext.LogIdentity(apiHandler))

// Chain middleware
handler := httpcontext.Authenticated(
    httpcontext.RequireTrustDomainID(expectedTD,
        httpcontext.LogIdentity(apiHandler),
    ),
)
```

---

## Configuration

### Server Configuration

```go
server, err := identityserver.New(ctx, ports.MTLSConfig{
    WorkloadAPI: ports.WorkloadAPIConfig{
        SocketPath: "unix:///tmp/spire-agent/public/api.sock",
    },
    SPIFFE: ports.SPIFFEConfig{
        // REQUIRED: Choose ONE
        AllowedPeerID:      "spiffe://example.org/client",  // Specific client
        AllowedTrustDomain: "example.org",                  // Any from domain
    },
    HTTP: ports.HTTPConfig{
        Address:           ":8443",           // Required
        ReadHeaderTimeout: 10 * time.Second,  // Optional, has defaults
        ReadTimeout:       30 * time.Second,  // Optional
        WriteTimeout:      30 * time.Second,  // Optional
        IdleTimeout:       60 * time.Second,  // Optional
    },
})
```

### Client Configuration

```go
client, err := httpclient.New(ctx, httpclient.Config{
    WorkloadAPI: httpclient.WorkloadAPIConfig{
        SocketPath: "unix:///tmp/spire-agent/public/api.sock",  // Required
    },
    SPIFFE: httpclient.SPIFFEConfig{
        // OPTIONAL: All can be empty for "any server from client's trust domain"
        ExpectedServerID:    "spiffe://example.org/server",  // Specific server
        ExpectedTrustDomain: "example.org",                  // Any from domain
    },
    HTTP: httpclient.HTTPClientConfig{
        Timeout:             30 * time.Second,  // Required
        MaxIdleConns:        100,               // Optional
        MaxIdleConnsPerHost: 10,                // Optional
        IdleConnTimeout:     90 * time.Second,  // Optional
    },
})
```

### Environment Variables

The examples support these environment variables:

**Server**:
- `SPIRE_AGENT_SOCKET` - Socket path (default: `unix:///tmp/spire-agent/public/api.sock`)
- `SERVER_ADDRESS` - Listen address (default: `:8443`)
- `ALLOWED_CLIENT_ID` - Restrict to specific client (optional)

**Client**:
- `SPIRE_AGENT_SOCKET` - Socket path (default: `unix:///tmp/spire-agent/public/api.sock`)
- `SERVER_URL` - Server URL (default: `https://localhost:8443`)
- `EXPECTED_SERVER_ID` - Expected server identity (optional)

Example:
```bash
# Server
SPIRE_AGENT_SOCKET="unix:///var/run/spire/agent.sock" \
SERVER_ADDRESS=":9443" \
ALLOWED_CLIENT_ID="spiffe://example.org/client" \
./bin/mtls-server

# Client
SPIRE_AGENT_SOCKET="unix:///var/run/spire/agent.sock" \
SERVER_URL="https://api-server:8443" \
EXPECTED_SERVER_ID="spiffe://example.org/server" \
./bin/mtls-client
```

---

## Authentication vs Authorization

This library handles **authentication only**. Your application must implement **authorization**.

### Authentication (This Library) ✅

```go
func handler(w http.ResponseWriter, r *http.Request) {
    // AUTHENTICATION: Verify WHO the client is
    clientID, ok := identityserver.GetIdentity(r)
    if !ok {
        http.Error(w, "Unauthorized", http.StatusUnauthorized)
        return
    }

    // At this point, we know WHO the client is (authenticated)
    log.Printf("Authenticated: %s", clientID.String())
}
```

### Authorization (Your Application) 🔒

```go
func handler(w http.ResponseWriter, r *http.Request) {
    // AUTHENTICATION: Verify WHO the client is
    clientID, ok := identityserver.GetIdentity(r)
    if !ok {
        http.Error(w, "Unauthorized", http.StatusUnauthorized)
        return
    }

    // AUTHORIZATION: Decide WHAT the client can do
    if !myAuthzService.IsAllowed(clientID.String(), "read", "resource") {
        http.Error(w, "Forbidden", http.StatusForbidden)
        return
    }

    // Handle authorized request
    fmt.Fprintf(w, "Success!\n")
}
```

### Common Authorization Patterns

#### 1. Path-Based Roles

```go
import "github.com/pocket/hexagon/spire/internal/adapters/inbound/identityserver/httpcontext"

// Application defines roles via SPIFFE ID path
if httpcontext.HasPathSuffix(r, "/admin") {
    // Client has admin role
} else if httpcontext.HasPathSuffix(r, "/readonly") {
    // Client has readonly role
} else {
    http.Error(w, "Forbidden", http.StatusForbidden)
    return
}
```

#### 2. Service Type Detection

```go
segments, _ := httpcontext.GetPathSegments(r)
if len(segments) > 0 {
    switch segments[0] {
    case "service":
        // Service-to-service request
    case "user":
        // User request
    case "system":
        // System request
    default:
        http.Error(w, "Forbidden", http.StatusForbidden)
        return
    }
}
```

#### 3. External Authorization Service

```go
clientID, _ := identityserver.GetIdentity(r)

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
- **Rotation**: Happens automatically before expiry (typically ~30 minutes before)
- **Zero Downtime**: New connections use new cert, existing connections continue
- **No Application Code Required**: Completely transparent to your application

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

**Note**: The exact rotation timing is controlled by SPIRE and may vary based on configuration. The X509Source monitors expiry and fetches new certificates automatically.

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

See [examples/mtls-adapters/KUBERNETES.md](../examples/mtls-adapters/KUBERNETES.md) for complete Kubernetes deployment guide including:
- Deployment manifests
- Building images for Minikube and remote registries
- Workload registration with SPIRE
- Advanced configuration

Quick start:
```bash
# Build images
docker build -t mtls-server:latest -f examples/mtls-adapters/server/Dockerfile .
docker build -t mtls-client:latest -f examples/mtls-adapters/client/Dockerfile .

# Deploy
kubectl apply -f examples/mtls-adapters/k8s/
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
- Verify socket permissions: `stat /tmp/spire-agent/public/api.sock`
- Check `SPIRE_AGENT_SOCKET` environment variable

### 2. "TLS handshake failed"

**Problem**: mTLS authentication failed.

**Possible Causes**:
- Client and server in different trust domains
- Server's `AllowedPeerID` doesn't match client
- Client's `ExpectedServerID` doesn't match server
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

### 5. "cannot register handler after Start"

**Problem**: Tried to call `Handle()` after `Start()`.

**Solution**: Register all handlers **before** calling `Start()`:
```go
server.Handle("/api/hello", handler1)
server.Handle("/api/goodbye", handler2)
// Now start
server.Start(ctx)
```

---

## Best Practices

### 1. Use Specific Authorization Policies in Production

```go
// ❌ Too permissive for production
SPIFFE: ports.SPIFFEConfig{
    AllowedTrustDomain: "dev.example.org",
}

// ✅ Specific client in production
SPIFFE: ports.SPIFFEConfig{
    AllowedPeerID: "spiffe://prod.example.org/payment-service",
}

// ✅ Or production trust domain
SPIFFE: ports.SPIFFEConfig{
    AllowedTrustDomain: "prod.example.org",
}
```

### 2. Always Close Resources

```go
server, err := identityserver.New(ctx, config)
if err != nil {
    return err
}
defer server.Close()  // ✅ Always defer Close

client, err := httpclient.New(ctx, config)
if err != nil {
    return err
}
defer client.Close()  // ✅ Always defer Close
```

### 3. Use Environment Variables for Configuration

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
// Start server in goroutine
go func() {
    server.Start(ctx)
}()

// Wait for signal
sigChan := make(chan os.Signal, 1)
signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
<-sigChan

// Graceful shutdown with timeout
shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
defer cancel()
server.Shutdown(shutdownCtx)  // ✅ Graceful shutdown
server.Close()                // ✅ Then release resources
```

### 5. Handle Errors Properly

```go
// ✅ Check errors from Handle()
if err := server.Handle("/api/hello", handler); err != nil {
    log.Fatalf("Failed to register handler: %v", err)
}

// ✅ Check identity extraction
clientID, ok := identityserver.GetIdentity(r)
if !ok {
    http.Error(w, "Unauthorized", http.StatusUnauthorized)
    return
}
```

### 6. Log Identity for Audit

```go
import "github.com/pocket/hexagon/spire/internal/adapters/inbound/identityserver/httpcontext"

// ✅ Log all authenticated requests for audit
handler := httpcontext.LogIdentity(apiHandler)
```

### 7. Don't Leak Identity in Errors

```go
// ❌ Don't leak identity information
http.Error(w, fmt.Sprintf("User %s not authorized", clientID), 403)

// ✅ Use generic messages
http.Error(w, "Forbidden", http.StatusForbidden)
log.Printf("Authorization failed for %s", clientID) // Log separately
```

---

## Examples

Complete working examples are available in [`examples/mtls-adapters/`](../examples/mtls-adapters/):

### Available Examples

1. **Server Example** ([server/main.go](../examples/mtls-adapters/server/main.go))
   - 4 endpoints: /api/hello, /api/echo, /api/identity, /health
   - Demonstrates identity extraction
   - Graceful shutdown
   - Environment-based configuration

2. **Client Example** ([client/main.go](../examples/mtls-adapters/client/main.go))
   - Makes requests to all server endpoints
   - Demonstrates Get and Post methods
   - Proper error handling and resource cleanup

3. **Kubernetes Deployment** ([k8s/](../examples/mtls-adapters/k8s/))
   - Complete manifests for server and client
   - Health probes and resource limits
   - SPIRE integration

### Running Examples

```bash
# Local
go run ./examples/mtls-adapters/server
go run ./examples/mtls-adapters/client

# Kubernetes
kubectl apply -f examples/mtls-adapters/k8s/
```

---

## References

### Project Documentation

- [examples/mtls-adapters/README.md](../examples/mtls-adapters/README.md) - Example usage guide
- [examples/mtls-adapters/KUBERNETES.md](../examples/mtls-adapters/KUBERNETES.md) - Kubernetes deployment
- [examples/mtls-adapters/TROUBLESHOOTING.md](../examples/mtls-adapters/TROUBLESHOOTING.md) - Troubleshooting guide
- [PROJECT_STATUS.md](PROJECT_STATUS.md) - Current project status and architecture

### External Resources

- [SPIFFE Specification](https://github.com/spiffe/spiffe) - SPIFFE ID and SVID specs
- [SPIRE Documentation](https://spiffe.io/docs/) - SPIRE server and agent setup
- [go-spiffe SDK](https://github.com/spiffe/go-spiffe) - Go SDK documentation
- [go-spiffe Examples](https://github.com/spiffe/go-spiffe/tree/main/v2/examples) - Official SDK examples

### API Reference

- `internal/ports/identityserver.go` - Port interfaces (MTLSServer, MTLSClient, MTLSConfig)
- `internal/adapters/inbound/identityserver/` - Server implementation
- `examples/httpclient/` - Client implementation (example code)

---

## Support

For issues and questions:

- **Examples**: See [examples/mtls-adapters/README.md](../examples/mtls-adapters/README.md)
- **Kubernetes**: See [examples/mtls-adapters/KUBERNETES.md](../examples/mtls-adapters/KUBERNETES.md)
- **Troubleshooting**: See [examples/mtls-adapters/TROUBLESHOOTING.md](../examples/mtls-adapters/TROUBLESHOOTING.md)
- **SPIRE Issues**: [spiffe/spire GitHub](https://github.com/spiffe/spire/issues)
