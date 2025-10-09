# mTLS Architecture Comparison

This document compares two approaches to implementing mTLS with SPIFFE/SPIRE:

1. **My Approach** (MTLS_IMPLEMENTATION.md): Detailed phases with explicit adapters
2. **Your Approach**: Identity server abstraction hiding SPIFFE details

---

## Architecture Comparison

### My Approach: Hexagonal Architecture (Ports & Adapters)

```
┌─────────────────────────────────────────────────────┐
│                 Application Layer                    │
│  (Handlers that use httpapi.GetSPIFFEID())          │
└─────────────────────┬───────────────────────────────┘
                      │
┌─────────────────────▼───────────────────────────────┐
│             Inbound Port (Implicit)                  │
│  - RegisterHandler(pattern, handler)                 │
│  - Start(ctx)                                        │
│  - Stop(ctx)                                         │
└─────────────────────┬───────────────────────────────┘
                      │
┌─────────────────────▼───────────────────────────────┐
│        Adapter: httpapi.HTTPServer                   │
│  - Wraps go-spiffe workloadapi.X509Source           │
│  - Wraps http.Server with mTLS config               │
│  - Exposes SPIFFE details to application            │
│    (via httpapi.GetSPIFFEID(r))                     │
└─────────────────────┬───────────────────────────────┘
                      │
┌─────────────────────▼───────────────────────────────┐
│             go-spiffe SDK                            │
│  - workloadapi.X509Source                           │
│  - tlsconfig.MTLSServerConfig                       │
│  - spiffetls.PeerIDFromConnectionState              │
└──────────────────────────────────────────────────────┘
```

**Characteristics**:
- ❌ **No explicit port interface** - application depends directly on httpapi package
- ❌ **SPIFFE details leak** - application uses `httpapi.GetSPIFFEID(r)`
- ❌ **Hard to swap implementations** - tightly coupled to go-spiffe
- ✅ **Flexible handler registration** - standard http.Handler
- ⚠️ **Multiple files** - server.go, middleware.go, identity.go

---

### Your Approach: Clean Identity Server Abstraction

```
┌─────────────────────────────────────────────────────┐
│                 Application Layer                    │
│  (Handlers that just handle http.Request)           │
└─────────────────────┬───────────────────────────────┘
                      │
┌─────────────────────▼───────────────────────────────┐
│           identityserver.Server (PORT)               │
│  type Server interface {                            │
│    Handle(pattern string, handler http.Handler)     │
│    Start(ctx context.Context) error                 │
│    Shutdown(ctx context.Context) error              │
│    Close() error                                    │
│  }                                                   │
└─────────────────────┬───────────────────────────────┘
                      │
┌─────────────────────▼───────────────────────────────┐
│      Adapter: identityserver.spiffeServer            │
│  - Implements Server interface                       │
│  - Hides ALL SPIFFE/SPIRE details                   │
│  - Application never sees spiffeid.ID                │
│  - Config-driven (no code coupling)                  │
└─────────────────────┬───────────────────────────────┘
                      │
┌─────────────────────▼───────────────────────────────┐
│             go-spiffe SDK                            │
│  (completely encapsulated)                           │
└──────────────────────────────────────────────────────┘
```

**Characteristics**:
- ✅ **Explicit port interface** - clean boundary
- ✅ **Zero SPIFFE leakage** - application is SPIFFE-agnostic
- ✅ **Easy to swap** - mock Server for tests
- ✅ **Minimal API surface** - 4 methods only
- ✅ **Single file per adapter** - port.go + spiffe_server.go
- ✅ **True hexagonal architecture** - inversion of control

---

## Detailed Comparison

### 1. Separation of Concerns

| Aspect | My Approach | Your Approach |
|--------|-------------|---------------|
| **Port Definition** | Implicit, no interface | ✅ Explicit `identityserver.Server` interface |
| **SPIFFE Visibility** | Application sees `spiffeid.ID` | ✅ Application never sees SPIFFE types |
| **Configuration** | Mixed with code | ✅ Pure data `Config` struct |
| **Dependency Direction** | App → httpapi → go-spiffe | ✅ App → Server ← spiffeServer → go-spiffe |

**Example of Leakage in My Approach**:
```go
// Application code directly depends on SPIFFE types
import "github.com/pocket/hexagon/spire/internal/adapters/inbound/httpapi"

func handler(w http.ResponseWriter, r *http.Request) {
    clientID, ok := httpapi.GetSPIFFEID(r)  // SPIFFE leakage!
    // Application must understand spiffeid.ID
}
```

**Clean Separation in Your Approach**:
```go
// Application code is SPIFFE-agnostic
func handler(w http.ResponseWriter, r *http.Request) {
    // Authentication already done by identityserver
    // Just handle the request
    w.Write([]byte("Success!!!"))
}
```

---

### 2. Testability

#### My Approach: Difficult to Mock
```go
// To test, you need:
// 1. Mock workloadapi.X509Source
// 2. Mock entire httpapi.HTTPServer
// 3. Still couple to httpapi package

type MockHTTPServer struct {
    // Must implement all httpapi internals
}
```

#### Your Approach: Trivial to Mock
```go
// Just implement the interface
type MockServer struct {
    handlers map[string]http.Handler
}

func (m *MockServer) Handle(pattern string, h http.Handler) {
    m.handlers[pattern] = h
}

func (m *MockServer) Start(ctx context.Context) error { return nil }
func (m *MockServer) Shutdown(ctx context.Context) error { return nil }
func (m *MockServer) Close() error { return nil }

// Use in tests
func TestApp(t *testing.T) {
    server := &MockServer{handlers: make(map[string]http.Handler)}
    app := NewApp(server)  // No SPIFFE setup needed!
    // Test app logic independently
}
```

---

### 3. Configuration

#### My Approach: Scattered Configuration
```go
// Configuration mixed with implementation
server, err := httpapi.NewHTTPServer(
    ctx,
    ":8443",                                    // Hardcoded
    "unix:///tmp/spire-agent/public/api.sock", // Hardcoded
    tlsconfig.AuthorizeMemberOf(               // Code-level config
        spiffeid.RequireTrustDomainFromString("example.org"),
    ),
)
```

**Problems**:
- ❌ Can't configure without changing code
- ❌ Mix of data and behavior
- ❌ Authorization policy is code, not data

#### Your Approach: Pure Data Configuration
```go
// Configuration as pure data
var cfg identityserver.Config
cfg.WorkloadAPI.SocketPath = "unix:///tmp/agent.sock"
cfg.SPIFFE.AllowedClientID = "spiffe://example.org/client"
cfg.HTTP.Address = ":8443"
cfg.HTTP.ReadHeaderTimeout = 10 * time.Second

server, err := identityserver.New(ctx, cfg)
```

**Benefits**:
- ✅ Load from file/env/consul
- ✅ Pure data (can serialize to JSON/YAML)
- ✅ Change config without code changes
- ✅ Validate config before runtime

---

### 4. Identity Access Patterns

#### My Approach: Application Extracts Identity
```go
// Pattern: Application must extract identity from request
func handler(w http.ResponseWriter, r *http.Request) {
    clientID, ok := httpapi.GetSPIFFEID(r)
    if !ok {
        http.Error(w, "Unauthorized", 401)
        return
    }

    // Now check authorization
    if clientID.String() != "spiffe://example.org/admin" {
        http.Error(w, "Forbidden", 403)
        return
    }

    // Handle request
}
```

**Problems**:
- ❌ Every handler repeats identity extraction
- ❌ Application couples to SPIFFE ID type
- ❌ No standard pattern for authorization

#### Your Approach: Authentication Done, Identity Implicit
```go
// Pattern: If handler is called, client is authenticated
func handler(w http.ResponseWriter, r *http.Request) {
    // Client already authenticated by identityserver
    // Identity verification was: AuthorizeID(clientID)

    // Just handle the business logic
    w.Write([]byte("Success!!!"))
}
```

**Benefits**:
- ✅ Authentication at edge (identity server)
- ✅ Handlers focus on business logic
- ✅ Authorization is separate concern (if needed)

**For cases needing identity**:
```go
// If you need identity, add middleware
func withIdentity(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        // Extract from TLS state once, put in context
        peerID, _ := spiffetls.PeerIDFromConnectionState(*r.TLS)
        ctx := context.WithValue(r.Context(), "identity", peerID)
        next.ServeHTTP(w, r.WithContext(ctx))
    })
}

// Then your app can get it from context (optional)
func handler(w http.ResponseWriter, r *http.Request) {
    id := r.Context().Value("identity").(spiffeid.ID)
    // Use id if needed for logging, etc.
}
```

---

### 5. File Organization

#### My Approach: Multiple Files per Concern
```
internal/adapters/inbound/httpapi/
├── server.go          (HTTPServer struct, Start, Stop)
├── middleware.go      (wrapHandler, SPIFFE ID extraction)
├── identity.go        (GetSPIFFEID, MustGetSPIFFEID, GetTrustDomain)
└── server_test.go     (tests)
```

**Problems**:
- ❌ Scattered implementation
- ❌ No single source of truth
- ❌ Multiple imports needed

#### Your Approach: Minimal Files
```
internal/identityserver/
├── port.go            (Server interface + Config struct)
└── spiffe_server.go   (spiffeServer implementation)
```

**Benefits**:
- ✅ Interface and config together
- ✅ One implementation per file
- ✅ Single import: `identityserver`

---

### 6. Extensibility

#### My Approach: Add Authorization in Application
```go
// Application handles authorization after authentication
func handler(w http.ResponseWriter, r *http.Request) {
    clientID, ok := httpapi.GetSPIFFEID(r)
    if !ok {
        http.Error(w, "Unauthorized", http.StatusUnauthorized)
        return
    }

    // Application authorization (not in library)
    if !myAuthz.Check(clientID, "read", "resource") {
        http.Error(w, "Forbidden", http.StatusForbidden)
        return
    }

    // Handle request
}
```

#### Your Approach: Same Pattern
```go
// Authentication at edge, authorization in app
func handler(w http.ResponseWriter, r *http.Request) {
    // Client authenticated by identityserver.Server

    // If you need authorization, extract identity
    id := r.Context().Value("identity").(spiffeid.ID)
    if !myAuthz.Check(id, "read", "resource") {
        http.Error(w, "Forbidden", 403)
        return
    }

    // Handle request
}
```

**Both approaches keep authorization out of the library** ✅

---

## Differences Summary

| Feature | My Approach | Your Approach | Winner |
|---------|-------------|---------------|--------|
| **Port Interface** | Implicit | ✅ Explicit `Server` | Your Approach |
| **SPIFFE Encapsulation** | Leaks to app | ✅ Fully hidden | Your Approach |
| **Configuration** | Mixed with code | ✅ Pure data | Your Approach |
| **Testability** | Hard to mock | ✅ Trivial to mock | Your Approach |
| **File Count** | 3-4 files | ✅ 2 files | Your Approach |
| **Dependency Direction** | App → Adapter | ✅ App → Port ← Adapter | Your Approach |
| **Handler Pattern** | Extract identity | ✅ Identity implicit | Your Approach |
| **Hexagonal Purity** | Partial | ✅ True hexagonal | Your Approach |

---

## Recommendation

**Your approach is superior for this library** because:

1. **True Hexagonal Architecture**
   - Clean port interface (`identityserver.Server`)
   - Proper inversion of control
   - Application depends on abstraction, not implementation

2. **Zero SPIFFE Leakage**
   - Application never imports go-spiffe
   - No `spiffeid.ID` in application code
   - True encapsulation

3. **Trivial Testing**
   - Mock `Server` interface in 5 lines
   - No SPIRE infrastructure needed for unit tests
   - Fast, isolated tests

4. **Configuration as Data**
   - `Config` struct is pure data
   - Load from file, env, or any source
   - No code changes to reconfigure

5. **Minimal API Surface**
   - 4 methods: `Handle`, `Start`, `Shutdown`, `Close`
   - 1 config struct
   - 2 files total

---

## Recommended Changes to My Document

### 1. Add Port Definition (Phase 0)

```go
// internal/identityserver/port.go
package identityserver

type Server interface {
    Handle(pattern string, handler http.Handler)
    Start(ctx context.Context) error
    Shutdown(ctx context.Context) error
    Close() error
}

type Config struct {
    WorkloadAPI struct {
        SocketPath string
    }
    SPIFFE struct {
        AllowedClientID string
    }
    HTTP struct {
        Address           string
        ReadHeaderTimeout time.Duration
    }
}
```

### 2. Simplify Phase 1 (Server Implementation)

Replace my complex `httpapi` package with:

```go
// internal/identityserver/spiffe_server.go
package identityserver

func New(ctx context.Context, cfg Config) (Server, error) {
    // Single constructor
    // Returns interface, not concrete type
}
```

### 3. Remove Phase 3 (Identity Utilities)

Not needed! Authentication is done at edge. If application needs identity:

```go
// Optional middleware (not in library)
func withIdentity(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        peerID, _ := spiffetls.PeerIDFromConnectionState(*r.TLS)
        ctx := context.WithValue(r.Context(), identityKey, peerID)
        next.ServeHTTP(w, r.WithContext(ctx))
    })
}
```

### 4. Update Examples

```go
// Simple usage
cfg := identityserver.Config{
    WorkloadAPI: struct{ SocketPath string }{"unix:///tmp/agent.sock"},
    SPIFFE:      struct{ AllowedClientID string }{"spiffe://example.org/client"},
    HTTP:        struct{ Address string; ReadHeaderTimeout time.Duration }{":8443", 10 * time.Second},
}

server, err := identityserver.New(ctx, cfg)
if err != nil {
    log.Fatalf("Failed to create server: %v", err)
}
defer server.Close()

server.Handle("/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
    w.Write([]byte("Success!!!"))
}))

if err := server.Start(ctx); err != nil {
    log.Fatalf("Failed to start server: %v", err)
}
```

---

## Migration Path

To adopt your approach:

1. **Create `internal/identityserver/`**
   - Copy your `port.go`
   - Copy your `spiffe_server.go`

2. **Delete from my plan**:
   - ❌ `internal/adapters/inbound/httpapi/middleware.go`
   - ❌ `internal/adapters/inbound/httpapi/identity.go`
   - Simplify `server.go` to match your `spiffe_server.go`

3. **Update examples**
   - Use `identityserver.Server` interface
   - Config-driven initialization
   - No SPIFFE types in application

4. **Update tests**
   - Mock `Server` interface
   - No go-spiffe imports in tests

---

## Conclusion

Your approach is the correct hexagonal architecture pattern.

My approach was on the right track (authentication only, no authorization) but:
- ❌ Missing explicit port interface
- ❌ Leaked SPIFFE types to application
- ❌ Too complex (3-4 files vs 2 files)
- ❌ Configuration mixed with code

Your approach:
- ✅ True hexagonal architecture
- ✅ Perfect encapsulation
- ✅ Minimal API surface
- ✅ Trivial to test
- ✅ Config as data
