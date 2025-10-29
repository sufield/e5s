# HTTP Middleware Examples

This directory contains **reference implementations** of HTTP middleware patterns for SPIFFE mTLS.

**Important:** This is NOT part of the core `spiffehttp` library. These are examples showing how to build your own middleware with full control over behavior.

## Why Not Include Middleware in the Library?

The `spiffehttp` package provides **primitives**, not opinionated wrappers:

- `PeerFromRequest()` - Extract peer identity from TLS connection
- `PeerFromContext()` - Retrieve peer from context
- `WithPeer()` - Attach peer to context

You should build middleware specific to your needs rather than using a one-size-fits-all solution.

## What's Included

### `authMiddleware`
Basic example that extracts peer identity and attaches it to context.

**Use this when:** You want simple authentication without authorization logic.

```go
protectedHandler := authMiddleware(http.HandlerFunc(myHandler))
```

### `trustDomainMiddleware`
Enforces trust domain boundaries with custom error messages.

**Use this when:** You accept mTLS from multiple trust domains but need per-route policies.

```go
td, _ := spiffeid.TrustDomainFromString("example.org")
restrictedHandler := trustDomainMiddleware(td)(http.HandlerFunc(myHandler))
```

### `certExpiryWarningMiddleware`
Logs warnings for certificates expiring soon.

**Use this when:** Debugging certificate rotation issues or implementing monitoring.

```go
monitoredHandler := certExpiryWarningMiddleware(5*time.Minute)(http.HandlerFunc(myHandler))
```

## Running the Example

This example requires a running SPIRE agent. See the [minikube-lowlevel example](../minikube-lowlevel/) for a complete SPIRE setup.

### With SPIRE Running Locally

```bash
# Start the server
go run main.go

# From another terminal with mTLS client configured:
curl https://localhost:8443/api/hello
```

### Testing Different Endpoints

```bash
# Public endpoint (no auth required)
curl https://localhost:8443/health

# Basic auth (any valid SPIFFE peer)
curl --cert client.crt --key client.key https://localhost:8443/api/hello

# Trust domain restricted (example.org only)
curl --cert client.crt --key client.key https://localhost:8443/api/restricted

# With cert expiry monitoring
curl --cert client.crt --key client.key https://localhost:8443/api/monitored
```

## Customization Ideas

### Custom Error Responses

Return JSON instead of plain text:

```go
func authMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        peer, ok := spiffehttp.PeerFromRequest(r)
        if !ok {
            w.Header().Set("Content-Type", "application/json")
            w.WriteHeader(http.StatusUnauthorized)
            json.NewEncoder(w).Encode(map[string]string{
                "error": "unauthorized",
                "message": "valid SPIFFE identity required",
            })
            return
        }
        ctx := spiffehttp.WithPeer(r.Context(), peer)
        next.ServeHTTP(w, r.WithContext(ctx))
    })
}
```

### Path-Based Authorization

Check SPIFFE ID path components:

```go
func pathAuthMiddleware(requiredPath string) func(http.Handler) http.Handler {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            peer, ok := spiffehttp.PeerFromRequest(r)
            if !ok || peer.ID.Path() != requiredPath {
                http.Error(w, "Forbidden", http.StatusForbidden)
                return
            }
            ctx := spiffehttp.WithPeer(r.Context(), peer)
            next.ServeHTTP(w, r.WithContext(ctx))
        })
    }
}

// Only allow spiffe://example.org/api-client
handler := pathAuthMiddleware("/api-client")(http.HandlerFunc(myHandler))
```

### Metrics and Logging

Track authentication by trust domain:

```go
var authCounter = promauto.NewCounterVec(
    prometheus.CounterOpts{
        Name: "spiffe_auth_total",
        Help: "Total authentication attempts by trust domain",
    },
    []string{"trust_domain", "status"},
)

func metricsMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        peer, ok := spiffehttp.PeerFromRequest(r)
        if !ok {
            authCounter.WithLabelValues("unknown", "failed").Inc()
            http.Error(w, "Unauthorized", http.StatusUnauthorized)
            return
        }

        authCounter.WithLabelValues(peer.ID.TrustDomain().Name(), "success").Inc()
        ctx := spiffehttp.WithPeer(r.Context(), peer)
        next.ServeHTTP(w, r.WithContext(ctx))
    })
}
```

### Rate Limiting by SPIFFE ID

```go
var limiters = make(map[string]*rate.Limiter)
var mu sync.Mutex

func rateLimitMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        peer, ok := spiffehttp.PeerFromRequest(r)
        if !ok {
            http.Error(w, "Unauthorized", http.StatusUnauthorized)
            return
        }

        id := peer.ID.String()
        mu.Lock()
        limiter, exists := limiters[id]
        if !exists {
            limiter = rate.NewLimiter(10, 100) // 10 req/sec, burst 100
            limiters[id] = limiter
        }
        mu.Unlock()

        if !limiter.Allow() {
            http.Error(w, "Rate limit exceeded", http.StatusTooManyRequests)
            return
        }

        ctx := spiffehttp.WithPeer(r.Context(), peer)
        next.ServeHTTP(w, r.WithContext(ctx))
    })
}
```

## Best Practices

1. **Keep middleware focused** - One middleware per concern (auth, logging, metrics)
2. **Chain middleware** - Compose multiple simple middleware instead of one complex one
3. **Use context for peer info** - Extract once in middleware, retrieve in handlers
4. **Log authentication** - Always log who's accessing what for security auditing
5. **Handle errors gracefully** - Return appropriate HTTP status codes
6. **Don't assume context has peer** - Always check the `ok` return value

## Integration with Frameworks

### Chi Router

```go
r := chi.NewRouter()
r.Use(authMiddleware)
r.Get("/api/users", userHandler)
```

### Gin

```go
func authMiddlewareGin() gin.HandlerFunc {
    return func(c *gin.Context) {
        peer, ok := spiffehttp.PeerFromRequest(c.Request)
        if !ok {
            c.AbortWithStatusJSON(401, gin.H{"error": "unauthorized"})
            return
        }
        c.Request = c.Request.WithContext(spiffehttp.WithPeer(c.Request.Context(), peer))
        c.Next()
    }
}

r := gin.Default()
r.Use(authMiddlewareGin())
```

### Echo

```go
func authMiddlewareEcho(next echo.HandlerFunc) echo.HandlerFunc {
    return func(c echo.Context) error {
        peer, ok := spiffehttp.PeerFromRequest(c.Request())
        if !ok {
            return echo.NewHTTPError(http.StatusUnauthorized, "unauthorized")
        }
        c.SetRequest(c.Request().WithContext(spiffehttp.WithPeer(c.Request().Context(), peer)))
        return next(c)
    }
}
```

## See Also

- [Core library documentation](../../docs/QUICKSTART_LIBRARY.md)
- [High-level API example](../highlevel/)
- [SPIRE deployment example](../minikube-lowlevel/)
