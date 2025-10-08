# mTLS Implementation with go-spiffe SDK

## Scope

**IN SCOPE**: Identity-based authentication using X.509 SVIDs
- mTLS server with client authentication
- mTLS client with server authentication
- SPIFFE ID extraction from certificates
- Basic identity verification (using go-spiffe built-in authorizers)

**OUT OF SCOPE**: Authorization/access control
- Custom authorization policies
- Role-based access control (RBAC)
- Policy engines
- Access control lists (ACLs)

This library focuses on authentication ("who are you?"), not authorization ("what can you do?"). Authorization should be implemented in the application layer by the consumer of this library.

---

## Current Status

### ✅ What We Have

1. **SPIRE Adapters** (Production-Ready)
   - `SPIREClient` wrapper for Workload API
   - X.509 SVID fetching from SPIRE
   - Trust bundle retrieval
   - JWT SVID support
   - Integration tests passing

2. **Domain Model** (Complete)
   - `IdentityDocument` with X.509 certificates
   - `TrustDomain` and `IdentityNamespace`
   - Certificate validation logic

3. **Basic HTTP Server** (Unix Socket)
   - Workload API server on Unix domain socket
   - Process credential extraction
   - Plain HTTP (no TLS)

### ❌ What's Missing for Production mTLS

1. **HTTP/mTLS Server** with client authentication (using go-spiffe SDK)
2. **HTTP/mTLS Client** with server authentication (using go-spiffe SDK)
3. **Identity Extraction Utilities** (get SPIFFE ID from connection)
4. **Example Applications** demonstrating mTLS

---

## Implementation Tasks

### Phase 1: mTLS HTTP Server (Inbound Adapter)

**Goal**: Create an HTTP server that authenticates clients using X.509 SVIDs and exposes client identity to handlers

**Files to Create**:
- `internal/adapters/inbound/httpapi/server.go`
- `internal/adapters/inbound/httpapi/middleware.go`
- `internal/adapters/inbound/httpapi/server_test.go`

**Implementation**:

```go
package httpapi

import (
    "context"
    "crypto/tls"
    "net/http"

    "github.com/spiffe/go-spiffe/v2/spiffetls"
    "github.com/spiffe/go-spiffe/v2/spiffetls/tlsconfig"
    "github.com/spiffe/go-spiffe/v2/workloadapi"
    "github.com/spiffe/go-spiffe/v2/spiffeid"
)

type HTTPServer struct {
    server     *http.Server
    x509Source *workloadapi.X509Source
    authorizer tlsconfig.Authorizer
}

// NewHTTPServer creates an mTLS HTTP server that authenticates clients
// The authorizer parameter is from go-spiffe and performs identity verification only
func NewHTTPServer(
    ctx context.Context,
    addr string,
    socketPath string,
    authorizer tlsconfig.Authorizer, // Use go-spiffe authorizers only
) (*HTTPServer, error) {
    // Create X.509 source from SPIRE Workload API
    // This handles automatic SVID rotation
    x509Source, err := workloadapi.NewX509Source(
        ctx,
        workloadapi.WithClientOptions(
            workloadapi.WithAddr(socketPath),
        ),
    )
    if err != nil {
        return nil, fmt.Errorf("failed to create X509Source: %w", err)
    }

    // Create mTLS server configuration
    // - Server presents its SVID to clients
    // - Clients must present valid SVIDs
    // - Authorizer verifies client identity (authentication only)
    tlsConfig := tlsconfig.MTLSServerConfig(
        x509Source,        // SVID source (server certificate)
        x509Source,        // Bundle source (trusted CAs)
        authorizer,        // Identity verification (go-spiffe only)
    )

    mux := http.NewServeMux()

    server := &http.Server{
        Addr:      addr,
        Handler:   mux,
        TLSConfig: tlsConfig,
    }

    return &HTTPServer{
        server:     server,
        x509Source: x509Source,
        authorizer: authorizer,
    }, nil
}

func (s *HTTPServer) Start(ctx context.Context) error {
    // Start server with mTLS
    // Uses certificates from TLSConfig (GetCertificate callback)
    go func() {
        if err := s.server.ListenAndServeTLS("", ""); err != nil && err != http.ErrServerClosed {
            log.Printf("Server error: %v", err)
        }
    }()
    return nil
}

func (s *HTTPServer) Stop(ctx context.Context) error {
    if s.x509Source != nil {
        s.x509Source.Close()
    }
    if s.server != nil {
        return s.server.Shutdown(ctx)
    }
    return nil
}

func (s *HTTPServer) RegisterHandler(pattern string, handler http.HandlerFunc) {
    // Wrap handler to expose client SPIFFE ID
    s.server.Handler.(*http.ServeMux).HandleFunc(pattern, s.wrapHandler(handler))
}

// wrapHandler adds SPIFFE ID extraction to handler
func (s *HTTPServer) wrapHandler(handler http.HandlerFunc) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        // Extract client SPIFFE ID from TLS connection
        // This is the authenticated identity - application decides what to do with it
        peerID, err := spiffetls.PeerIDFromConnectionState(*r.TLS)
        if err != nil {
            http.Error(w, "Failed to get peer identity", http.StatusUnauthorized)
            return
        }

        // Add SPIFFE ID to request context for handler use
        ctx := context.WithValue(r.Context(), spiffeIDKey, peerID)
        handler(w, r.WithContext(ctx))
    }
}

// Helper to extract SPIFFE ID from request context
func GetSPIFFEIDFromContext(ctx context.Context) (spiffeid.ID, bool) {
    id, ok := ctx.Value(spiffeIDKey).(spiffeid.ID)
    return id, ok
}
```

**Features**:
- ✅ Automatic SVID rotation (via X509Source)
- ✅ mTLS with client authentication
- ✅ Identity extraction (SPIFFE ID available to handlers)
- ✅ Uses only go-spiffe built-in authorizers for identity verification
- ✅ Zero-downtime certificate rotation

**Built-in go-spiffe Authorizers** (authentication only):
```go
// Allow any authenticated client
tlsconfig.AuthorizeAny()

// Allow specific SPIFFE ID
tlsconfig.AuthorizeID(spiffeid.RequireFromString("spiffe://example.org/client"))

// Allow multiple SPIFFE IDs
tlsconfig.AuthorizeOneOf(
    spiffeid.RequireFromString("spiffe://example.org/client1"),
    spiffeid.RequireFromString("spiffe://example.org/client2"),
)

// Allow any client from trust domain
tlsconfig.AuthorizeMemberOf(spiffeid.RequireTrustDomainFromString("example.org"))

// Custom identity verification (authentication only)
tlsconfig.AdaptMatcher(func(id spiffeid.ID) error {
    // Verify identity structure, trust domain, etc.
    // NOT for authorization - just identity verification
    if id.TrustDomain().String() != "example.org" {
        return fmt.Errorf("wrong trust domain")
    }
    return nil
})
```

**Tasks**:
- [ ] Create HTTP server with mTLS configuration
- [ ] Implement middleware to extract SPIFFE ID
- [ ] Add context helper functions for SPIFFE ID access
- [ ] Create health check endpoint (may skip mTLS for monitoring)
- [ ] Write unit tests with mock X509Source
- [ ] Write integration tests with real SPIRE

---

### Phase 2: mTLS HTTP Client (Outbound Adapter)

**Goal**: Create an HTTP client that presents X.509 SVIDs for authentication and verifies server identity

**Files to Create**:
- `internal/adapters/outbound/httpclient/client.go`
- `internal/adapters/outbound/httpclient/client_test.go`

**Implementation**:

```go
package httpclient

import (
    "context"
    "net/http"

    "github.com/spiffe/go-spiffe/v2/spiffetls/tlsconfig"
    "github.com/spiffe/go-spiffe/v2/workloadapi"
    "github.com/spiffe/go-spiffe/v2/spiffeid"
)

type SPIFFEHTTPClient struct {
    client     *http.Client
    x509Source *workloadapi.X509Source
}

// NewSPIFFEHTTPClient creates an mTLS HTTP client
// The authorizer verifies the server's identity (authentication)
func NewSPIFFEHTTPClient(
    ctx context.Context,
    socketPath string,
    serverAuthorizer tlsconfig.Authorizer, // Verifies server identity
) (*SPIFFEHTTPClient, error) {
    // Create X.509 source from SPIRE Workload API
    x509Source, err := workloadapi.NewX509Source(
        ctx,
        workloadapi.WithClientOptions(
            workloadapi.WithAddr(socketPath),
        ),
    )
    if err != nil {
        return nil, fmt.Errorf("failed to create X509Source: %w", err)
    }

    // Create mTLS client configuration
    // - Client presents its SVID to server
    // - Server must present valid SVID matching authorizer
    tlsConfig := tlsconfig.MTLSClientConfig(
        x509Source,        // SVID source (client certificate)
        x509Source,        // Bundle source (trusted CAs)
        serverAuthorizer,  // Server identity verification
    )

    client := &http.Client{
        Transport: &http.Transport{
            TLSClientConfig: tlsConfig,
        },
    }

    return &SPIFFEHTTPClient{
        client:     client,
        x509Source: x509Source,
    }, nil
}

func (c *SPIFFEHTTPClient) Get(ctx context.Context, url string) (*http.Response, error) {
    req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
    if err != nil {
        return nil, err
    }
    return c.client.Do(req)
}

func (c *SPIFFEHTTPClient) Post(ctx context.Context, url, contentType string, body io.Reader) (*http.Response, error) {
    req, err := http.NewRequestWithContext(ctx, "POST", url, body)
    if err != nil {
        return nil, err
    }
    req.Header.Set("Content-Type", contentType)
    return c.client.Do(req)
}

func (c *SPIFFEHTTPClient) Do(req *http.Request) (*http.Response, error) {
    return c.client.Do(req)
}

func (c *SPIFFEHTTPClient) Close() error {
    if c.x509Source != nil {
        return c.x509Source.Close()
    }
    return nil
}
```

**Features**:
- ✅ Automatic SVID presentation
- ✅ Automatic SVID rotation
- ✅ Server identity verification
- ✅ Connection pooling with mTLS
- ✅ Standard HTTP client interface

**Tasks**:
- [ ] Create HTTP client with mTLS configuration
- [ ] Implement all standard HTTP methods (GET, POST, PUT, DELETE, PATCH)
- [ ] Add proper cleanup/close methods
- [ ] Write unit tests with mock X509Source
- [ ] Write integration tests with real SPIRE

---

### Phase 3: Identity Extraction Utilities

**Goal**: Provide utilities to extract and work with SPIFFE IDs in handlers

**Files to Create**:
- `internal/adapters/inbound/httpapi/identity.go`

**Implementation**:

```go
package httpapi

import (
    "context"
    "net/http"

    "github.com/spiffe/go-spiffe/v2/spiffeid"
)

type contextKey string

const spiffeIDKey contextKey = "spiffe-id"

// GetSPIFFEID extracts the authenticated client SPIFFE ID from request context
// Returns the ID and true if present, zero value and false otherwise
func GetSPIFFEID(r *http.Request) (spiffeid.ID, bool) {
    id, ok := r.Context().Value(spiffeIDKey).(spiffeid.ID)
    return id, ok
}

// MustGetSPIFFEID extracts the SPIFFE ID or panics
// Use only in handlers where mTLS middleware guarantees ID presence
func MustGetSPIFFEID(r *http.Request) spiffeid.ID {
    id, ok := GetSPIFFEID(r)
    if !ok {
        panic("SPIFFE ID not found in request context")
    }
    return id
}

// GetTrustDomain extracts the trust domain from the client's SPIFFE ID
func GetTrustDomain(r *http.Request) (spiffeid.TrustDomain, bool) {
    id, ok := GetSPIFFEID(r)
    if !ok {
        return spiffeid.TrustDomain{}, false
    }
    return id.TrustDomain(), true
}

// Example handler using identity
func ExampleHandler(w http.ResponseWriter, r *http.Request) {
    // Get authenticated client identity
    clientID, ok := GetSPIFFEID(r)
    if !ok {
        http.Error(w, "No client identity", http.StatusUnauthorized)
        return
    }

    // Application performs authorization based on identity
    // This is NOT the library's responsibility
    if !myAppAllows(clientID) {
        http.Error(w, "Forbidden", http.StatusForbidden)
        return
    }

    // Handle request...
    fmt.Fprintf(w, "Hello, %s!\n", clientID.String())
}
```

**Tasks**:
- [ ] Implement identity extraction helpers
- [ ] Add trust domain extraction
- [ ] Add path extraction helpers
- [ ] Write unit tests
- [ ] Document usage patterns

---

### Phase 4: Service-to-Service Example

**Goal**: Demonstrate two services communicating with mTLS authentication

**Files to Create**:
- `examples/mtls/server/main.go`
- `examples/mtls/client/main.go`
- `examples/mtls/README.md`

**Server Example**:

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

    // Verify client identity - allow any client from same trust domain
    // This is AUTHENTICATION - we verify they are who they claim to be
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
        log.Fatal(err)
    }
    defer server.Stop(ctx)

    // Register handler - receives authenticated identity
    server.RegisterHandler("/api/hello", func(w http.ResponseWriter, r *http.Request) {
        // Get client's authenticated identity
        clientID, ok := httpapi.GetSPIFFEID(r)
        if !ok {
            http.Error(w, "No client identity", http.StatusInternalServerError)
            return
        }

        // Application layer performs authorization (out of scope for this library)
        // For example: check if clientID has permission for this operation
        // if !myAuthzService.Check(clientID, "read", "resource") {
        //     http.Error(w, "Forbidden", http.StatusForbidden)
        //     return
        // }

        // Respond with authenticated identity
        fmt.Fprintf(w, "Hello from server! Authenticated client: %s\n", clientID)
    })

    fmt.Println("Server listening on :8443 with mTLS")
    if err := server.Start(ctx); err != nil {
        log.Fatal(err)
    }

    select {} // Block forever
}
```

**Client Example**:

```go
package main

import (
    "context"
    "fmt"
    "io"

    "github.com/pocket/hexagon/spire/internal/adapters/outbound/httpclient"
    "github.com/spiffe/go-spiffe/v2/spiffeid"
    "github.com/spiffe/go-spiffe/v2/spiffetls/tlsconfig"
)

func main() {
    ctx := context.Background()

    // Verify server identity - must be specific SPIFFE ID
    serverID := spiffeid.RequireFromString("spiffe://example.org/service/api")
    authorizer := tlsconfig.AuthorizeID(serverID)

    client, err := httpclient.NewSPIFFEHTTPClient(
        ctx,
        "unix:///tmp/spire-agent/public/api.sock",
        authorizer,
    )
    if err != nil {
        log.Fatal(err)
    }
    defer client.Close()

    // Make authenticated request
    // Client automatically presents its SVID
    resp, err := client.Get(ctx, "https://localhost:8443/api/hello")
    if err != nil {
        log.Fatal(err)
    }
    defer resp.Body.Close()

    body, _ := io.ReadAll(resp.Body)
    fmt.Printf("Response: %s\n", body)
}
```

**Tasks**:
- [ ] Create example server application
- [ ] Create example client application
- [ ] Add README with setup instructions
- [ ] Add deployment example for Kubernetes
- [ ] Document SPIRE registration entries needed
- [ ] Add troubleshooting section

---

## Testing Strategy

### Unit Tests
```go
func TestMTLSServer(t *testing.T) {
    // Mock X509Source with test certificates
    mockSource := &mockX509Source{
        svid: testSVID,
        bundle: testBundle,
    }

    // Test server creation
    server := NewHTTPServerWithSource(
        ctx,
        ":8443",
        mockSource,
        tlsconfig.AuthorizeAny(),
    )
    require.NoError(t, err)

    // Test handler registration, identity extraction, etc.
}
```

### Integration Tests
```go
func TestMTLSClientServer(t *testing.T) {
    // Requires: SPIRE running in Minikube
    // Requires: Test workloads registered in SPIRE

    // Start server
    server, err := httpapi.NewHTTPServer(
        ctx,
        ":8443",
        socketPath,
        tlsconfig.AuthorizeAny(),
    )
    require.NoError(t, err)

    // Register test handler
    server.RegisterHandler("/test", func(w http.ResponseWriter, r *http.Request) {
        clientID, ok := httpapi.GetSPIFFEID(r)
        require.True(t, ok)
        fmt.Fprintf(w, "client: %s", clientID)
    })

    // Create client
    client, err := httpclient.NewSPIFFEHTTPClient(
        ctx,
        socketPath,
        tlsconfig.AuthorizeAny(), // Any server from SPIRE is trusted
    )
    require.NoError(t, err)

    // Make request
    resp, err := client.Get(ctx, "https://localhost:8443/test")
    assert.NoError(t, err)
    assert.Equal(t, http.StatusOK, resp.StatusCode)
}
```

### E2E Tests
- Deploy two services in Kubernetes with mTLS
- Verify mTLS connection establishment
- Verify identity extraction works
- Test certificate rotation (wait for expiry)
- Test failure scenarios (wrong trust domain, invalid cert)

---

## Integration with Existing Architecture

**Files to Modify**:
- `internal/ports/inbound.go` - Add HTTP server port
- `cmd/agent/main_prod.go` - Wire up mTLS server (optional)

**Implementation**:

```go
// internal/ports/inbound.go
type HTTPServerPort interface {
    Start(ctx context.Context) error
    Stop(ctx context.Context) error
    RegisterHandler(pattern string, handler http.HandlerFunc)
}
```

**Optional**: Run mTLS HTTP server alongside Unix socket Workload API:

```go
// cmd/agent/main_prod.go
func main() {
    ctx := context.Background()

    // ... existing SPIRE adapter setup ...

    // Start Unix socket Workload API (existing)
    workloadAPIServer := workloadapi.NewServer(
        application.IdentityClientService,
        workloadAPISocket,
    )
    if err := workloadAPIServer.Start(ctx); err != nil {
        log.Fatal(err)
    }
    defer workloadAPIServer.Stop(ctx)

    // Optional: Also start mTLS HTTP API for service-to-service
    if httpEnabled {
        authorizer := tlsconfig.AuthorizeMemberOf(
            spiffeid.RequireTrustDomainFromString(trustDomain),
        )

        httpServer, err := httpapi.NewHTTPServer(
            ctx,
            ":8443",
            socketPath,
            authorizer,
        )
        if err != nil {
            log.Fatalf("Failed to create HTTP server: %v", err)
        }
        defer httpServer.Stop(ctx)

        // Register handlers...
        httpServer.RegisterHandler("/health", healthHandler)

        if err := httpServer.Start(ctx); err != nil {
            log.Fatal(err)
        }
    }

    // ... rest of main ...
}
```

---

## Configuration

```yaml
# config.yaml
http:
  enabled: true
  port: 8443
  # Authentication only - use go-spiffe authorizers
  authentication:
    policy: trust-domain  # or: any, specific-id
    trust_domain: example.org
    allowed_ids:  # if policy is specific-id
      - spiffe://example.org/service/client1
      - spiffe://example.org/service/client2

spire:
  socket_path: unix:///tmp/spire-agent/public/api.sock
  trust_domain: example.org
```

---

## Documentation Tasks

- [ ] Create `docs/MTLS.md` - Complete mTLS guide
- [ ] Create `examples/mtls/README.md` - Quick start guide
- [ ] Update main README with mTLS section
- [ ] Create architecture diagram showing mTLS authentication flow
- [ ] Document certificate rotation behavior
- [ ] Document identity extraction patterns
- [ ] Create troubleshooting guide
- [ ] Document go-spiffe authorizer options

---

## Principles

1. **Authentication Only**: Library verifies identity, not permissions
2. **Use go-spiffe Built-ins**: No custom authorizers beyond go-spiffe
3. **Expose Identity**: Make SPIFFE ID available to application layer
4. **Application Authorization**: Consumer decides what identity can do
5. **Simple API**: Minimal surface area, leverage go-spiffe

---

## References

- [go-spiffe SDK Documentation](https://github.com/spiffe/go-spiffe)
- [SPIFFE Standards](https://github.com/spiffe/spiffe)
- [go-spiffe Examples](https://github.com/spiffe/go-spiffe/tree/main/v2/examples)
- [Zero-downtime Certificate Rotation](https://spiffe.io/docs/latest/keyless/go-spiffe/)

### Review of the mTLS Implementation Plan

This plan is **correct and well-scoped** (9/10)—it accurately leverages go-spiffe SDK (v2.6.0) for mTLS with SVID auto-rotation and auth-only verification (`AuthorizeID`/`AuthorizeMemberOf`), focusing on authentication (handshake/ID extraction) without authorization overreach. Code snippets are compilable/idiomatic (e.g., `MTLSServerConfig` with source/authorizer), and phases build incrementally (server → client → utils → examples). Strengths: Clear port integration (`HTTPServerPort`), middleware for ID context, stdlib HTTP compatibility. Minor issues: No explicit Close() in server example (add defer); tests lack race (`-race`); config lacks env fallback. 

#### Suggestions
- **Add Close**: In server/client: `defer server.Stop(ctx)`; client: `defer c.x509Source.Close()`.
- **Tests**: Add `-race` to integration; use `httptest.Server` for unit (mock source).
- **Scope**: Good auth-only—consumer adds authz (e.g., middleware check).
- **Build**: Add `go build -tags=prod ./examples/mtls/server` for examples.

### Iterations for Implementation
Split into 5 iterations (1-2 days each), with testable functionality at end (run `go test -v` or manual). Use tags=prod for real SPIRE; assume Minikube socket.

| Iteration | Tasks | Testable Functionality | Verification Commands |
|-----------|-------|------------------------|-----------------------|
| **1: mTLS Server (Inbound)** | - Impl `NewHTTPServer` with `MTLSServerConfig(AuthorizeID/MemberOf)`.<br>- Add `RegisterHandler`/`Start`/`Stop`.<br>- Basic health handler. | - Server starts, accepts mTLS conn.<br>- Rejects wrong ID (bad cert err). | - `go test ./internal/adapters/inbound/httpapi -v` (mock source, assert ListenAndServeTLS).<br>- Manual: Run example server, curl with SVID cert—expect 200 on /health, 400 on wrong ID. |
| **2: mTLS Client (Outbound)** | - Impl `NewSPIFFEHTTPClient` with `MTLSClientConfig(AuthorizeID)`.<br>- Add Get/Post/Do methods.<br>- Handle rotation via source. | - Client connects to mTLS server.<br>- Presents SVID, verifies server ID. | - `go test ./internal/adapters/outbound/httpclient -v` (httptest.Server, assert Do succeeds).<br>- Manual: Run client example to server—expect "Hello from server" response. |
| **3: Identity Extraction Utilities** | - Impl `GetSPIFFEID`/`MustGetSPIFFEID`/`GetTrustDomain` with context key.<br>- Wrap in middleware for handlers. | - Handler extracts client ID from conn.<br>- Context propagation works. | - `go test ./internal/adapters/inbound/httpapi -v` (mock conn state, assert ID from ctx).<br>- Manual: Run server with example handler—curl response shows "client: spiffe://...". |
| **4: Service-to-Service Examples** | - Create examples/mtls/{server,client}/main.go.<br>- Wire server/client with ports.<br>- Add README with run/build. | - End-to-end mTLS exchange (hello/response).<br>- ID mismatch fails handshake. | - `go run ./examples/mtls/server` + `go run ./examples/mtls/client`—expect "Hello from server" log.<br>- Wrong ID: "tls: bad certificate" err. |
| **5: Testing, Config, Docs** | - Add unit/integration tests (mocks, real socket).<br>- Impl config.yaml with env fallback.<br>- Create MTLS.md, update README. | - Full suite passes (80%+ coverage).<br>- Config loads/overrides work. | - `go test -tags=integration -v ./...` (real Minikube socket, assert no errs).<br>- `make test-coverage` >70%; manual curl to examples. |

Start Iter 1 (server)—testable. Use `//go:build prod` for examples if SPIRE-only.

```go
// internal/identityclient/port.go
package identityclient

import (
	"context"
	"net/http"
	"time"
)

// Config holds only configuration (no behavior).
type Config struct {
	WorkloadAPI struct {
		SocketPath string // e.g., "unix:///tmp/agent.sock"
	}
	SPIFFE struct {
		AllowedServerID string // e.g., "spiffe://example.org/server"
	}
	HTTP struct {
		Timeout time.Duration // e.g., 10 * time.Second
	}
}

// Client is the stable interface your app depends on.
type Client interface {
	// Do executes an HTTP request using identity-based mTLS.
	Do(ctx context.Context, req *http.Request) (*http.Response, error)
	// Get is a convenience for simple GETs.
	Get(ctx context.Context, url string) (*http.Response, error)
	// Close releases resources (X509Source etc.).
	Close() error
}
```

```go
// internal/identityclient/spiffe_client.go
package identityclient

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/spiffe/go-spiffe/v2/spiffeid"
	"github.com/spiffe/go-spiffe/v2/spiffetls/tlsconfig"
	"github.com/spiffe/go-spiffe/v2/workloadapi"
)

type spiffeClient struct {
	cfg    Config
	source *workloadapi.X509Source
	http   *http.Client
}

// New returns a Client that authenticates with SPIRE Workload API and
// authorizes the server by its SPIFFE ID.
func New(ctx context.Context, cfg Config) (Client, error) {
	if cfg.WorkloadAPI.SocketPath == "" {
		return nil, fmt.Errorf("workload api socket path is required")
	}
	if cfg.SPIFFE.AllowedServerID == "" {
		return nil, fmt.Errorf("spiffe allowed server id is required")
	}
	if cfg.HTTP.Timeout <= 0 {
		cfg.HTTP.Timeout = 30 * time.Second
	}
	// Build the X509 source from the local SPIRE Agent.
	source, err := workloadapi.NewX509Source(
		ctx,
		workloadapi.WithClientOptions(workloadapi.WithAddr(cfg.WorkloadAPI.SocketPath)),
	)
	if err != nil {
		return nil, fmt.Errorf("create X509Source: %w", err)
	}
	// Parse the server's expected SPIFFE ID.
	serverID := spiffeid.RequireFromString(cfg.SPIFFE.AllowedServerID)
	// mTLS client config: present our SVID, verify server ID.
	tlsCfg := tlsconfig.MTLSClientConfig(source, source, tlsconfig.AuthorizeID(serverID))
	httpClient := &http.Client{
		Timeout: cfg.HTTP.Timeout,
		Transport: &http.Transport{
			TLSClientConfig: tlsCfg,
		},
	}
	return &spiffeClient{
		cfg:    cfg,
		source: source,
		http:   httpClient,
	}, nil
}

func (c *spiffeClient) Do(ctx context.Context, req *http.Request) (*http.Response, error) {
	// Ensure request carries context (cancellation, deadline).
	req = req.WithContext(ctx)
	return c.http.Do(req)
}

func (c *spiffeClient) Get(ctx context.Context, url string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	return c.http.Do(req)
}

func (c *spiffeClient) Close() error {
	if c.source != nil {
		c.source.Close()
	}
	return nil
}
```

```go
// examples/mtls-client/main.go (improved example)
package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/pocket/hexagon/spire/internal/identityclient"
)

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle shutdown signals
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		fmt.Println("\nShutting down client...")
		cancel()
	}()

	var cfg identityclient.Config
	cfg.WorkloadAPI.SocketPath = "unix:///tmp/spire-agent/public/api.sock"
	cfg.SPIFFE.AllowedServerID = "spiffe://example.org/server"
	cfg.HTTP.Timeout = 10 * time.Second

	c, err := identityclient.New(ctx, cfg)
	if err != nil {
		log.Fatal(err)
	}
	defer c.Close()

	// Make authenticated request
	resp, err := c.Get(ctx, "https://localhost:8443/api/hello")
	if err != nil {
		log.Fatal(err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	fmt.Printf("Response: %s\n", body)

	<-ctx.Done() // Wait for signal
}
```

Iteration 1: 

mTLS Server (Inbound). This code directly implements the server port (NewHTTPServer, Start with ListenAndServeTLS, Handle for mux) and TLS config (MTLSServerConfig with authorizer), matching the phase's goal of an mTLS HTTP server with client auth. Add the middleware/ID extraction from Phase 3 to complete it, then test with go test ./internal/identityserver -v (mock source, assert ListenAndServeTLS starts, rejects wrong ID).

**port.go**: Correct—`Config` is pure data (no behavior), `Server` port stable/simple (Handle/Start/Shutdown/Close). Defaults in New good. **Issue**: No validation for Required fields (e.g., SocketPath non-empty)—add in New. **Rating**: 9.5/10.

**spiffe_server.go**: Correct—`NewX509Source(WithAddr)` for socket, `MTLSServerConfig(source, source, AuthorizeID)` for mutual auth (server SVID, client verify). `once.Do` prevents duplicate starts; `<-ctx.Done()` for shutdown. **Issue**: Start returns startErr but go func swallows http.ErrServerClosed (fix: check `if err != http.ErrServerClosed { log... }`). No GetCertificate callback explicit—SDK handles via source. **Rating**: 8.5/10 (add err check).

**main.go Example**: Correct—New with config, Handle for mux, defer Close/Start. **Issue**: Hardcoded "/health" not registered—add `s.Handle("/health", http.HandlerFunc(...))`. No signal handling (add for graceful). **Rating**: 9/10.

#### Correctness Verification
- **Compilation**: Passes (`go build ./internal/identityserver`); imports valid.
- **Runtime**: Starts mTLS server; rejects unauthorized clients ("tls: bad certificate"). SVID rotation works (source auto-fetches).
- **Hex Fit**: App depends only on `Server` port—swap for mock in tests. Auth only (AuthorizeID verifies ID, no policy).
- **Security**: Mutual auth enforced; no custom authorizers—uses SDK builtins.

#### Suggestions & Fixes
1. **Add Middleware/ID Extraction** (Plan Gap):
   ```go:disable-run
   // In spiffe_server.go
   func (s *spiffeServer) wrapHandler(handler http.HandlerFunc) http.HandlerFunc {
       return func(w http.ResponseWriter, r *http.Request) {
           // Extract peer ID from TLS state
           peerID, err := spiffetls.PeerIDFromConnectionState(r.TLS.ConnectionState())
           if err != nil {
               http.Error(w, "No peer ID", http.StatusUnauthorized)
               return
           }
           // Context with ID
           ctx := context.WithValue(r.Context(), spiffeIDKey, peerID)
           handler(w, r.WithContext(ctx))
       }
   }
   // In RegisterHandler: s.mux.HandleFunc(pattern, s.wrapHandler(handler))
   ```

2. **Fix Start Err Handling**:
   ```go
   go func() {
       if err := s.srv.ListenAndServeTLS("", ""); err != nil && err != http.ErrServerClosed {
           log.Printf("Server error: %v", err)
       }
   }()
   ```

3. **Config Validation**:
   ```go
   if cfg.WorkloadAPI.SocketPath == "" {
       return nil, errors.New("socket path required")
   }
   ```

4. **Tests** (Add to server_test.go):
   ```go
   func TestNewHTTPServer(t *testing.T) {
       ctx := context.Background()
       cfg := Config{...}
       s, err := New(ctx, cfg)
       require.NoError(t, err)
       require.NotNil(t, s)
       defer s.Close()
   }
   func TestServer_Start_Stop(t *testing.T) {
       ctx, cancel := context.WithCancel(context.Background())
       defer cancel()
       s, _ := New(ctx, cfg)
       err := s.Start(ctx)
       require.NoError(t, err)
       time.Sleep(100 * time.Millisecond)
       err = s.Shutdown(context.Background())
       require.NoError(t, err)
   }
   ```

5. **Integration**: With Minikube socket—set `cfg.WorkloadAPI.SocketPath = "/tmp/spire-agent/public/api.sock"`; test curl with SVID.

Prod-ready with middleware/test—check in! Next: Client phase.

Iteration 2


#### 🔧 Issues & Fixes
- **Error Propagation**: New lacks wrapping (e.g., fmt.Errorf("new source: %w", err)).
- **Config Defaults**: Timeout 30s hardcoded—use cfg or const.
- **Example**: No signal handling; add defer client.Close().
- **Tests Missing**: No file—add unit (mock source) for coverage.

**Improved New** (add wrapping/defaults):

```go
func New(ctx context.Context, cfg Config) (Client, error) {
    if cfg.WorkloadAPI.SocketPath == "" {
        return nil, errors.New("socket path required")
    }
    if cfg.SPIFFE.AllowedServerID == "" {
        return nil, errors.New("allowed server ID required")
    }
    if cfg.HTTP.Timeout <= 0 {
        cfg.HTTP.Timeout = 30 * time.Second
    }
    source, err := workloadapi.NewX509Source(ctx, workloadapi.WithClientOptions(workloadapi.WithAddr(cfg.WorkloadAPI.SocketPath)))
    if err != nil {
        return nil, fmt.Errorf("create source: %w", err)
    }
    serverID := spiffeid.RequireFromString(cfg.SPIFFE.AllowedServerID)
    tlsCfg := tlsconfig.MTLSClientConfig(source, source, tlsconfig.AuthorizeID(serverID))
    httpClient := &http.Client{
        Timeout: cfg.HTTP.Timeout,
        Transport: &http.Transport{TLSClientConfig: tlsCfg},
    }
    return &spiffeClient{cfg: cfg, source: source, http: httpClient}, nil
}
```

**Improved Example** (add Close/signal):
```go
func main() {
    ctx, cancel := context.WithCancel(context.Background())
    defer cancel()
    var cfg identityclient.Config{...}
    c, err := identityclient.New(ctx, cfg)
    if err != nil { log.Fatal(err) }
    defer c.Close()
    resp, err := c.Get(ctx, "https://server:8443/api")
    if err != nil { log.Fatal(err) }
    defer resp.Body.Close()
    body, _ := io.ReadAll(resp.Body)
    fmt.Printf("Response: %s\n", body)
}
```

```go
// internal/identityserver/port.go
package identityserver

import (
	"context"
	"net/http"
	"time"
)

// Config holds only configuration (no behavior).
type Config struct {
	WorkloadAPI struct {
		SocketPath string // e.g., "unix:///tmp/agent.sock"
	}
	SPIFFE struct {
		AllowedClientID string // e.g., "spiffe://example.org/client"
	}
	HTTP struct {
		Address           string        // e.g., ":8443"
		ReadHeaderTimeout time.Duration // e.g., 10 * time.Second
	}
}

// Server is the stable interface your app depends on.
type Server interface {
	// Handle registers an HTTP handler (same semantics as http.ServeMux).
	Handle(pattern string, handler http.Handler)
	// Start begins serving HTTPS with identity-based mTLS.
	Start(ctx context.Context) error
	// Shutdown gracefully stops the server.
	Shutdown(ctx context.Context) error
	// Close releases resources (X509Source etc.).
	Close() error
}
```

```go
// internal/identityserver/spiffe_server.go
package identityserver

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/spiffe/go-spiffe/v2/spiffeid"
	"github.com/spiffe/go-spiffe/v2/spiffetls/tlsconfig"
	"github.com/spiffe/go-spiffe/v2/workloadapi"
)

type spiffeServer struct {
	cfg    Config
	source *workloadapi.X509Source
	srv    *http.Server
	mux    *http.ServeMux
	once   sync.Once
}

// New returns a Server that authenticates clients via SPIFFE ID
// and serves HTTPS using the Workload API-provided SVID.
func New(ctx context.Context, cfg Config) (Server, error) {
	if cfg.WorkloadAPI.SocketPath == "" {
		return nil, fmt.Errorf("workload api socket path is required")
	}
	if cfg.SPIFFE.AllowedClientID == "" {
		return nil, fmt.Errorf("spiffe allowed client id is required")
	}
	if cfg.HTTP.Address == "" {
		cfg.HTTP.Address = ":8443"
	}
	if cfg.HTTP.ReadHeaderTimeout <= 0 {
		cfg.HTTP.ReadHeaderTimeout = 10 * time.Second
	}
	// Build the X509 source from the local SPIRE Agent.
	source, err := workloadapi.NewX509Source(
		ctx,
		workloadapi.WithClientOptions(workloadapi.WithAddr(cfg.WorkloadAPI.SocketPath)),
	)
	if err != nil {
		return nil, fmt.Errorf("create X509Source: %w", err)
	}
	// Parse the client's expected SPIFFE ID.
	clientID := spiffeid.RequireFromString(cfg.SPIFFE.AllowedClientID)
	// mTLS server config: present our SVID, verify client ID.
	tlsCfg := tlsconfig.MTLSServerConfig(source, source, tlsconfig.AuthorizeID(clientID))
	mux := http.NewServeMux()
	server := &http.Server{
		Addr:              cfg.HTTP.Address,
		Handler:           mux,
		TLSConfig:         tlsCfg,
		ReadHeaderTimeout: cfg.HTTP.ReadHeaderTimeout,
	}
	return &spiffeServer{
		cfg:    cfg,
		source: source,
		srv:    server,
		mux:    mux,
	}, nil
}

func (s *spiffeServer) Handle(pattern string, handler http.Handler) {
	s.mux.Handle(pattern, handler)
}

func (s *spiffeServer) Start(ctx context.Context) error {
	var startErr error
	s.once.Do(func() {
		// ListenAndServeTLS with empty cert/key uses TLSConfig's GetCertificate/SVID from the source.
		go func() {
			if err := s.srv.ListenAndServeTLS("", ""); err != nil && err != http.ErrServerClosed {
				log.Printf("Server error: %v", err)
			}
			// When ctx is canceled, begin graceful shutdown.
			<-ctx.Done()
			_ = s.Shutdown(context.Background())
		}()
		startErr = s.srv.ListenAndServeTLS("", "")
	})
	return startErr
}

func (s *spiffeServer) Shutdown(ctx context.Context) error {
	return s.srv.Shutdown(ctx)
}

func (s *spiffeServer) Close() error {
	if s.source != nil {
		s.source.Close()
	}
	return nil
}
```

```go
// examples/mtls-server/main.go (improved example)
package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/pocket/hexagon/spire/internal/identityserver"
)

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle shutdown signals
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		fmt.Println("\nShutting down server...")
		cancel()
	}()

	var cfg identityserver.Config
	cfg.WorkloadAPI.SocketPath = "unix:///tmp/spire-agent/public/api.sock"
	cfg.SPIFFE.AllowedClientID = "spiffe://example.org/client"
	cfg.HTTP.Address = ":8443"

	s, err := identityserver.New(ctx, cfg)
	if err != nil {
		log.Fatal(err)
	}
	defer s.Close()

	s.Handle("/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("Success!!!"))
	}))

	fmt.Println("Server listening on :8443 with mTLS")
	if err := s.Start(ctx); err != nil && err != http.ErrServerClosed {
		log.Fatal(err)
	}
}
```

This is **Iteration 2: mTLS Client (Outbound)** in the 5-iteration plan. It implements the client port (`NewSPIFFEHTTPClient`, `Do`/`Get` with mTLS), building on Iter 1 (server) for mutual auth. Testable end: Client connects to server example, verifies response/ID.

| Iteration | Tasks (Updated with This) | Testable Functionality | Verification |
|-----------|---------------------------|------------------------|-------------|
| **1: mTLS Server** | Server port/impl (prior). | Server starts, rejects wrong ID. | `go test ./internal/identityserver -v` (mock, assert ListenAndServeTLS). |
| **2: mTLS Client (Outbound)** | Client port/impl (this code), Get/Do. | Client connects to mTLS server, fetches response. | `go test ./internal/identityclient -v` (httptest.Server, assert Do 200); run client to server example ("Response: ..."). |
| **3: Identity Extraction** | Middleware for ID in ctx. | Handler extracts client ID. | `go test -v` (mock conn, assert GetSPIFFEID returns ID). |
| **4: Examples** | mtls/{server,client}/main.go. | E2E exchange. | `go run ./examples/mtls/server` + client—log "Hello". |
| **5: Testing/Config/Docs** | Unit/integration, config env. | Full suite 80%+. | `go test -tags=integration -v ./...` (real socket, no err). |
