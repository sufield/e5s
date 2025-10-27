Goal: make the library lightweight, framework-agnostic, and ready to drop into chi, gin, or plain net/http.
I’ll lay this out as a series of concrete refactor tasks you can apply directly in your repo.

---

## 0. Target shape (what we’re building toward)

We want a core package (I’ll call it `identitytls`) that gives you:

```go
// For servers
cfg, err := identitytls.NewServerTLSConfig(ctx, identitytls.ServerConfig{ /* ... */ })

// For clients
cfg, err := identitytls.NewClientTLSConfig(ctx, identitytls.ClientConfig{ /* ... */ })

// For request handlers (any framework)
id, ok := identitytls.PeerIdentityFromContext(ctx)
```

Everything else — routing, middleware stacks, health endpoints, HTTP server plumbing, etc. — should live in app code, not the library.

the refactor is about:

1. carving that out, and
2. deleting everything we don’t need in the core.

---

## 1. Create the new core package

Create a new package:
`internal/identitytls/` (or `pkg/identitytls/` if you want it publicly importable by other services).

Add `config.go`, `server.go`, `client.go`, `identity.go`, `spire.go`.

We’ll fill those in with minimal (compile-ready) skeletons so the rest of the codebase can start consuming them.

### 1.1 `config.go`

Purpose: shared types + constants.

```go
package identitytls

import "time"

type ServerConfig struct {
    // Required SPIFFE ID this server will present.
    // Example: "spiffe://example.org/api"
    ServerID string

    // Optional override to locate the SPIRE Workload API.
    // If empty we auto-discover (SPIFFE_ENDPOINT_SOCKET or well-known defaults).
    WorkloadSocketPath string
}

type ClientConfig struct {
    // Option A: exact ID the client expects the server to present.
    // Ex: "spiffe://example.org/api"
    ServerID string

    // Option B: trust domain match fallback.
    // Ex: "example.org"
    // If ServerID is empty and this is non-empty, we allow any SPIFFE ID
    // within that trust domain.
    ServerTrustDomain string

    // Optional override for Workload API socket.
    WorkloadSocketPath string
}

// PeerIdentity represents the authenticated caller's identity extracted from mTLS.
type PeerIdentity struct {
    SPIFFEID    string
    TrustDomain string
    ExpiresAt   time.Time
}
```

This file intentionally has no net/http symbols.

---

## 2. Move SPIRE / SVID logic into a helper (spire.go)

Right now you’ve got SPIRE identity logic living in your identity service (fetching SVIDs, figuring out trust domain, etc). We want that broken out into something `identitytls` can call without importing app code.

Add `spire.go`:

```go
package identitytls

import (
    "context"
    "time"
    // import SPIRE Workload API client you already use,
    // e.g. the adapter/agent client
)

// SVID represents whatever your SPIRE agent client returns (trim it down).
type SVID struct {
    SPIFFEID     string
    TrustDomain  string
    CertChain    [][]byte // leaf->root DER certs
    Key          interface{}
    ExpiresAt    time.Time
}

// fetchSVID should:
// - connect to SPIRE Workload API socket
// - fetch X509-SVID for this workload
// - return cert chain + key + metadata
//
// IMPORTANT: this MUST NOT be exported outside identitytls.
func fetchSVID(ctx context.Context, workloadSocketOverride string) (*SVID, error) {
    // This is mostly moving code you already have in IdentityServiceSPIRE.SnapshotData
    // and in the TLS bootstrap logic.
    //
    // Behavior:
    //   1. Resolve socket path: workloadSocketOverride, env var, fallback paths.
    //   2. Dial Workload API.
    //   3. Fetch X.509 SVID.
    //   4. Return parsed struct.
    return nil, nil // placeholder for now
}
```

Why:

* This lets both server and client callers reuse the same SPIRE bootstrap code.
* It also walls off SPIRE internals so chi/gin-facing code never has to know about it.

---

## 3. Produce TLS configs (server.go and client.go)

These are the workhorses. They’re what chi and gin will actually call under the hood through `http.Server.TLSConfig` or `http.Transport.TLSClientConfig`.

### 3.1 `server.go`

```go
package identitytls

import (
    "context"
    "crypto/tls"
    "crypto/x509"
    "errors"
    "time"
)

func NewServerTLSConfig(ctx context.Context, cfg ServerConfig) (*tls.Config, PeerIdentity, error) {
    // 1. pull SVID for THIS workload (the server)
    svid, err := fetchSVID(ctx, cfg.WorkloadSocketPath)
    if err != nil {
        return nil, PeerIdentity{}, err
    }

    if cfg.ServerID != "" && svid.SPIFFEID != cfg.ServerID {
        return nil, PeerIdentity{}, errors.New("server SPIFFE ID mismatch from config")
    }

    // 2. Build cert chain for tls.Config
    //    - leaf cert/key from SVID
    //    - intermediates/roots from SVID.CertChain
    //    (this is code you already have in zero-config server)
    cert := tls.Certificate{
        Certificate: svid.CertChain,
        PrivateKey:  svid.Key,
        Leaf:        nil, // we can lazily parse, or you already have parsed x509.Certificate
    }

    // 3. Build cert pool for client auth
    pool := x509.NewCertPool()
    // append trust bundle roots from SVID into pool
    // (you already have bundle retrieval logic — reuse that here)

    tlsCfg := &tls.Config{
        MinVersion: tls.VersionTLS13,

        Certificates: []tls.Certificate{cert},

        ClientAuth: tls.RequireAndVerifyClientCert,
        ClientCAs:  pool,

        // VerifyPeerCertificate: (optional override if you want to enforce SPIFFE IDs at handshake),
        // BUT: we’re moving that to request-level extraction instead of handshake rejection so we
        // can return 401 instead of hard-dropping the socket. For now, keep handshake basic.

        // Prefer to lock this to HTTP/1.1 and/or h2 as needed.
        NextProtos: []string{"h2", "http/1.1"},
    }

    me := PeerIdentity{
        SPIFFEID:    svid.SPIFFEID,
        TrustDomain: svid.TrustDomain,
        ExpiresAt:   svid.ExpiresAt,
    }

    // NOTE:
    // We are *not* adding any net/http handlers here.
    // chi/gin will use this TLS config with their own router/server.

    return tlsCfg, me, nil
}
```

What changed here compared to your current zero-config server:

* No routing.
* No `http.Server` creation.
* No health endpoint.
* No ReadHeaderTimeout defaults.
* Just: “give me a tls.Config for a mutual TLS server and tell me who I am.”

That makes this library small and reusable.

### 3.2 `client.go`

```go
package identitytls

import (
    "context"
    "crypto/tls"
    "crypto/x509"
    "errors"
)

func NewClientTLSConfig(ctx context.Context, cfg ClientConfig) (*tls.Config, error) {
    // 1. Fetch the client's own SVID
    svid, err := fetchSVID(ctx, cfg.WorkloadSocketPath)
    if err != nil {
        return nil, err
    }

    // 2. Build client cert chain
    cert := tls.Certificate{
        Certificate: svid.CertChain,
        PrivateKey:  svid.Key,
        Leaf:        nil,
    }

    // 3. Build RootCAs for verifying the server
    roots := x509.NewCertPool()
    // fill roots with trust bundle from SPIRE

    verifyServerID := cfg.ServerID
    trustDomain := cfg.ServerTrustDomain

    if verifyServerID == "" && trustDomain == "" {
        return nil, errors.New("must configure ServerID or ServerTrustDomain")
    }

    tlsCfg := &tls.Config{
        MinVersion: tls.VersionTLS13,

        Certificates: []tls.Certificate{cert},

        // We *do* custom-verify the server cert against SPIFFE ID after the standard chain check.
        RootCAs: roots,

        InsecureSkipVerify: true, // we’ll do our own verify below
        VerifyPeerCertificate: func(rawCerts [][]byte, verifiedChains [][]*x509.Certificate) error {
            // 1. Parse server leaf cert SPIFFE ID
            // 2. Ensure either exact match ServerID OR trust-domain match ServerTrustDomain
            // 3. Fail otherwise
            //
            // You already have this logic in your zero-config client today.
            // Bring it here.
            return nil
        },
    }

    return tlsCfg, nil
}
```

Again:

* No `http.Client` wrapper.
* No `Get()` helper.
* We just hand you a `*tls.Config`, you plug that into your own transport.

App code can then do:

```go
tlsCfg, _ := identitytls.NewClientTLSConfig(ctx, clientCfg)

transport := &http.Transport{
    TLSClientConfig: tlsCfg,
}

httpClient := &http.Client{
    Transport: transport,
    Timeout:   5 * time.Second,
}
```

Your lib no longer decides timeout, retry, tracing, etc. That’s good.

---

## 4. Peer identity extraction (identity.go)

We need a way for handlers to know “who is calling me?” without any chi/gin types.

Add `identity.go`:

```go
package identitytls

import (
    "context"
    "crypto/x509"
    "errors"
    "net/http"
    "time"
)

type peerIdentityKey struct{}

func WithPeerIdentity(ctx context.Context, id PeerIdentity) context.Context {
    return context.WithValue(ctx, peerIdentityKey{}, id)
}

func PeerIdentityFromContext(ctx context.Context) (PeerIdentity, bool) {
    v := ctx.Value(peerIdentityKey{})
    if v == nil {
        return PeerIdentity{}, false
    }
    id, ok := v.(PeerIdentity)
    return id, ok
}

// ExtractPeerIdentity inspects the verified mTLS connection on the request
// and returns the caller's SPIFFE ID and cert metadata.
// This does NOT modify the request; frameworks can either:
//
//   a) call this at the top of each handler, OR
//   b) wrap it in their own middleware that stores results in context.
//
// We do NOT ship middleware here. We only provide the primitive.
func ExtractPeerIdentity(r *http.Request) (PeerIdentity, error) {
    connState := r.TLS
    if connState == nil {
        return PeerIdentity{}, errors.New("no TLS connection state")
    }
    if len(connState.PeerCertificates) == 0 {
        return PeerIdentity{}, errors.New("no peer certificate presented")
    }

    leaf := connState.PeerCertificates[0]

    spiffeID, trustDomain, err := spiffeFromCert(leaf)
    if err != nil {
        return PeerIdentity{}, err
    }

    id := PeerIdentity{
        SPIFFEID:    spiffeID,
        TrustDomain: trustDomain,
        ExpiresAt:   leaf.NotAfter,
    }

    return id, nil
}

// helper - parse SPIFFE ID from SAN URI(s)
func spiffeFromCert(cert *x509.Certificate) (id string, trustDomain string, err error) {
    // You already have code that extracts SPIFFE IDs (SAN URI like spiffe://example.org/service).
    // Reuse it here.
    //
    // Parse cert.URIs, pick the SPIFFE URI, split "spiffe://<trustDomain>/<rest>"
    // Return id, trustDomain.
    return "", "", nil
}
```

Key point:

* This code uses only `net/http` and `crypto/tls` types — both are universal to chi, gin, stdlib.
* chi route handlers and gin handlers can both call this.

This is the flexibility you were aiming for: you’re not forcing a middleware contract, you’re returning a primitive.

---

## 5. Delete / move high-level server code

Now that `identitytls` exposes “give me a tls.Config”, we no longer want the library to:

* stand up its own `http.Server`
* mount `/health`
* manage route maps
* hold read/write timeouts
* expose helper `zerotrustserver.Serve`

That should move to *examples*.

Concretely:

1. In `pkg/zerotrustserver` (or wherever Serve() lives):

   * Mark `Serve()` as deprecated in a comment, or remove the package entirely if you’re comfortable with the breaking change.
   * Same for `PeerIdentity()` there: replace calls with `identitytls.ExtractPeerIdentity(r)` or `identitytls.PeerIdentityFromContext(r.Context())`.

2. Remove any code that:

   * Owns a `http.Server` lifecycle
   * Injects handlers into `mux`
   * Sets ReadHeaderTimeout, IdleTimeout, etc.

Those now belong in the chi/gin example apps, not in core.

If you can’t rip it right now (because other services depend on it), do this:

```go
// Deprecated: Prefer building your own http.Server and using identitytls.NewServerTLSConfig.
// This helper will be removed in a future release.
func Serve(...) error {
    // existing code
}
```

That keeps consumers compiling while you introduce the better API.

---

## 6. Produce new example apps

We’ll make two new directories (not in the library’s import path, so they don’t become part of the public API surface):

* `examples/chi-server/`
* `examples/gin-server/`
* `examples/mtls-client/`

### 6.1 chi server skeleton

```go
package main

import (
    "context"
    "log"
    "net/http"
    "time"

    "github.com/go-chi/chi/v5"
    "github.com/pocket/hexagon/spire/internal/identitytls"
)

func main() {
    ctx := context.Background()

    tlsCfg, me, err := identitytls.NewServerTLSConfig(ctx, identitytls.ServerConfig{
        ServerID: "spiffe://example.org/api",
    })
    if err != nil {
        log.Fatalf("tls config: %v", err)
    }

    log.Printf("serving as %s (trust domain %s)", me.SPIFFEID, me.TrustDomain)

    r := chi.NewRouter()

    // simple auth gate at handler level
    r.Get("/secure", func(w http.ResponseWriter, r *http.Request) {
        caller, err := identitytls.ExtractPeerIdentity(r)
        if err != nil {
            http.Error(w, "unauthorized", http.StatusUnauthorized)
            return
        }
        w.WriteHeader(http.StatusOK)
        _, _ = w.Write([]byte("hello " + caller.SPIFFEID + "\n"))
    })

    srv := &http.Server{
        Addr:              ":8443",
        Handler:           r,
        TLSConfig:         tlsCfg,
        ReadHeaderTimeout: 2 * time.Second,
        IdleTimeout:       30 * time.Second,
        MaxHeaderBytes:    8 << 10,
    }

    log.Fatal(srv.ListenAndServeTLS("", "")) // key/cert come from tlsCfg
}
```

Note what happened:

* chi owns routing.
* we own TLS config.
* no middleware required.
* we call `ExtractPeerIdentity` right in the handler.

### 6.2 gin server skeleton

```go
package main

import (
    "context"
    "log"
    "net/http"
    "time"

    "github.com/gin-gonic/gin"
    "github.com/pocket/hexagon/spire/internal/identitytls"
)

func main() {
    ctx := context.Background()

    tlsCfg, me, err := identitytls.NewServerTLSConfig(ctx, identitytls.ServerConfig{
        ServerID: "spiffe://example.org/api",
    })
    if err != nil {
        log.Fatalf("tls config: %v", err)
    }

    log.Printf("serving as %s (trust domain %s)", me.SPIFFEID, me.TrustDomain)

    r := gin.New()

    r.GET("/secure", func(c *gin.Context) {
        caller, err := identitytls.ExtractPeerIdentity(c.Request)
        if err != nil {
            c.String(http.StatusUnauthorized, "unauthorized")
            return
        }
        c.String(http.StatusOK, "hello "+caller.SPIFFEID+"\n")
    })

    srv := &http.Server{
        Addr:              ":8443",
        Handler:           r,
        TLSConfig:         tlsCfg,
        ReadHeaderTimeout: 2 * time.Second,
        IdleTimeout:       30 * time.Second,
        MaxHeaderBytes:    8 << 10,
    }

    log.Fatal(srv.ListenAndServeTLS("", ""))
}
```

Again: gin owns routing, we just feed in TLSConfig.

### 6.3 mtls client skeleton

```go
package main

import (
    "context"
    "io"
    "log"
    "net/http"
    "time"

    "github.com/pocket/hexagon/spire/internal/identitytls"
)

func main() {
    ctx := context.Background()

    tlsCfg, err := identitytls.NewClientTLSConfig(ctx, identitytls.ClientConfig{
        ServerID: "spiffe://example.org/api",
    })
    if err != nil {
        log.Fatalf("client tls: %v", err)
    }

    transport := &http.Transport{
        TLSClientConfig: tlsCfg,
    }

    client := &http.Client{
        Transport: transport,
        Timeout:   5 * time.Second,
    }

    resp, err := client.Get("https://localhost:8443/secure")
    if err != nil {
        log.Fatalf("GET: %v", err)
    }
    defer resp.Body.Close()

    body, _ := io.ReadAll(resp.Body)
    log.Printf("status=%d body=%s", resp.StatusCode, body)
}
```

These examples become the “realistic” demos you’ll later tie into minikube / SPIRE (e.g. via a deployment manifest and SPIRE workload registration). We don't need to wire minikube yet — we’ve just isolated the surfaces so we *can*.

---

## 7. Summary of required improvements

Here are the concrete actions you should take now:

1. **Create `identitytls` package** with:

   * `config.go` (ServerConfig, ClientConfig, PeerIdentity)
   * `spire.go` (fetchSVID + trust bundle helpers, pulled from existing SPIRE adapter)
   * `server.go` (NewServerTLSConfig)
   * `client.go` (NewClientTLSConfig)
   * `identity.go` (ExtractPeerIdentity, PeerIdentityFromContext, WithPeerIdentity)

2. **Move/inline SPIRE bootstrap logic** from your current `zerotrustserver` / `zerotrustclient` into `fetchSVID()` + trust bundle helpers. Those should NOT depend on net/http.

3. **Remove HTTP server orchestration from the core library**:

   * no `http.Server` constructors
   * no mux / router creation
   * no health endpoint
   * no timeouts
   * no goroutine lifecycle management
   * mark those helpers deprecated or delete them (based on downstream usage tolerance)

4. **Stop exporting helper methods that wrap `http.Client`** like `client.Get(ctx, url)` if they exist. Instead, return `*tls.Config` and let callers wire their own `http.Transport`.

5. **Add `examples/`** showing:

   * chi secured service
   * gin secured service
   * mTLS client talking to them
     These will eventually become your minikube demo.

6. **Start updating internal callers** (your own services) to:

   * call `identitytls.NewServerTLSConfig` and set `http.Server.TLSConfig`
   * call `identitytls.NewClientTLSConfig` and set `http.Transport.TLSClientConfig`
   * call `identitytls.ExtractPeerIdentity(r)` in request handlers

After this, the library will:

* be framework-agnostic,
* be much lighter (no routing / server lifecycle),
* and ready for minikube SPIRE demo in the next step.

