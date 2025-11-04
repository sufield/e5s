# Frequently Asked Questions (FAQ)

## General Questions

### What is e5s?

e5s (Easy SPIFFE Services) is a Go library that simplifies building HTTP services with SPIFFE/SPIRE-based mTLS authentication. It provides a high-level API for creating secure microservices without dealing with low-level TLS configuration details.

### How does e5s relate to SPIFFE/SPIRE?

e5s is built on top of the official go-spiffe SDK and integrates with SPIRE for workload identity management. It uses the same underlying security mechanisms but provides a simpler, configuration-driven API specifically for HTTP services.

### Is e5s production-ready?

Yes. e5s enforces security best practices by default (TLS 1.3, mTLS, SPIFFE ID verification) and handles certificate rotation automatically. It's designed for production use with proper error handling, graceful shutdown, and security hardening.

---

## Architecture and Design

### Does e5s use Go's standard library HTTP features?

**Yes, absolutely.** e5s uses Go's standard library `net/http` package for all HTTP functionality. It does not replace or reimplement HTTP.

**For Clients:**
- `e5s.Client()` returns a standard `*http.Client` type
- You can use all standard methods: `client.Get()`, `client.Post()`, `client.Do()`
- Response handling uses standard `http.Response` with `resp.Body`, `resp.StatusCode`, etc.

**For Servers:**
- `e5s.Start()` creates a standard `http.Server` internally
- You use standard `http.HandleFunc()` and `http.Handler` interface for routing
- Handlers receive standard `http.ResponseWriter` and `*http.Request` parameters

**What e5s actually does:**
1. Connects to SPIRE Workload API to obtain X.509 certificates
2. Creates a `tls.Config` with mTLS settings
3. Injects that config into standard `http.Client` or `http.Server`
4. Handles automatic certificate rotation in the background

The HTTP layer remains 100% standard library - e5s only simplifies the SPIRE integration and TLS configuration.

### Can I use e5s with existing HTTP routers and middleware?

Yes! Since e5s uses the standard `http.Handler` interface, it works with any Go HTTP router or middleware:

```go
import (
    "github.com/gorilla/mux"
    "github.com/sufield/e5s"
)

func main() {
    // Use your favorite router
    r := mux.NewRouter()
    r.HandleFunc("/api/users", handleUsers)
    r.HandleFunc("/api/products", handleProducts)

    // Start with e5s
    shutdown, err := e5s.Start("config.yaml", r)
    // ... handle error and shutdown
}
```

You can also chain middleware:

```go
handler := loggingMiddleware(authMiddleware(myHandler))
shutdown, err := e5s.Start("config.yaml", handler)
```

### What's the difference between e5s and using go-spiffe SDK directly?

See [comparison.md](./comparison.md) for a detailed side-by-side comparison. In summary:

| Aspect | e5s | go-spiffe SDK |
|--------|-----|---------------|
| **Use Case** | HTTP/REST services | Any protocol (HTTP, gRPC, TCP, etc.) |
| **Configuration** | YAML files | Hardcoded in Go |
| **Code Size** | ~36 lines (client), ~31 lines (server) | ~60 lines (client), ~57 lines (server) |
| **Learning Curve** | Low (standard HTTP) | Medium (SPIFFE concepts) |
| **Flexibility** | HTTP only | Full control, any protocol |
| **Security Defaults** | Automatic (TLS 1.3, validation) | Manual setup required |

**Use e5s if:** You're building HTTP services and want simplicity
**Use go-spiffe SDK if:** You need custom protocols or maximum control

---

## Configuration

### Can I override configuration with environment variables?

Yes, using the `IDP_MODE` environment variable for testing:

```bash
# Use in-memory identity provider (no SPIRE agent needed)
IDP_MODE=inmem go run ./cmd/server

# Use in-memory mode for tests
IDP_MODE=inmem go test ./...
```

For production configuration, use YAML files which support different environments (dev, staging, prod).

### What's the difference between `allowed_client_spiffe_id` and `allowed_client_trust_domain`?

**`allowed_client_spiffe_id`** - Exact SPIFFE ID matching:
```yaml
server:
  allowed_client_spiffe_id: "spiffe://example.org/client"
```
Only allows connections from workloads with this exact ID.

**`allowed_client_trust_domain`** - Trust domain matching:
```yaml
server:
  allowed_client_trust_domain: "example.org"
```
Allows connections from any workload in the trust domain (more permissive).

You must set **exactly one** of these options. Setting both or neither will cause a validation error.

### How do I configure different settings for dev/staging/prod?

Use separate configuration files:

```
config/
├── dev.yaml      # Development (relaxed settings)
├── staging.yaml  # Staging (production-like)
└── prod.yaml     # Production (hardened)
```

Then load the appropriate config:

```go
env := os.Getenv("ENVIRONMENT")
configPath := fmt.Sprintf("config/%s.yaml", env)
client, shutdown, err := e5s.Client(configPath)
```

Or use Helm values files for Kubernetes deployments:
```bash
helmfile -e dev apply     # Uses values-dev.yaml
helmfile -e prod apply    # Uses values-prod.yaml
```

---

## Security

### Does e5s enforce TLS 1.3?

Yes, e5s automatically enforces TLS 1.3 as the minimum version. This is a security best practice that you don't need to configure manually.

### How does certificate rotation work?

Certificate rotation is handled automatically by the SPIRE agent and go-spiffe SDK:

1. SPIRE agent continuously monitors certificate expiration
2. Before expiration, it automatically fetches new certificates
3. The SDK updates the TLS config without dropping connections
4. This happens transparently - no code changes or restarts needed

e5s inherits this zero-downtime rotation from go-spiffe SDK.

### Can I use custom authorization logic beyond SPIFFE ID matching?

Yes! Use the `spiffehttp.PeerFromRequest()` function to extract authenticated peer information and implement custom logic:

```go
import "github.com/sufield/e5s/spiffehttp"

func handler(w http.ResponseWriter, r *http.Request) {
    peer, ok := spiffehttp.PeerFromRequest(r)
    if !ok {
        http.Error(w, "Unauthorized", 401)
        return
    }

    // Custom authorization logic
    if !peer.ID.MemberOf(requiredTrustDomain) {
        http.Error(w, "Forbidden", 403)
        return
    }

    if !hasRole(peer.ID, "admin") {
        http.Error(w, "Forbidden", 403)
        return
    }

    // Handle request...
}
```

See [examples/middleware/main.go](../examples/middleware/main.go) for complete examples.

### Is e5s vulnerable to Slowloris attacks?

No. e5s sets `ReadHeaderTimeout: 10 * time.Second` by default, which protects against Slowloris and similar slow-read attacks. This timeout limits how long the server waits for request headers.

---

## Testing

### Can I test e5s-based services without running SPIRE?

Yes! Use the in-memory identity provider:

```bash
IDP_MODE=inmem go test ./...
IDP_MODE=inmem go run ./cmd/server
```

This creates mock SPIFFE identities without requiring a SPIRE agent, perfect for:
- Unit tests
- Local development
- CI/CD pipelines
- Quick prototyping

### How do I test client and server interaction?

See the integration test examples:

1. **Unit tests** - Use `IDP_MODE=inmem` to test without SPIRE
2. **Integration tests** - Run full SPIRE stack in Kubernetes
3. **Local testing** - Use scripts in `examples/minikube-lowlevel/`

Example integration test setup:
```bash
# Start Minikube with SPIRE
cd examples/minikube-lowlevel
./setup.sh

# Run integration tests
./scripts/run-integration-tests.sh
```

---

## Deployment

### Can I use e5s in Kubernetes?

Yes! e5s is designed for Kubernetes deployments with SPIRE. See the complete examples in `examples/minikube-lowlevel/` which include:

- SPIRE server and agent deployment (Helm)
- Workload registration
- mTLS client and server deployment
- Zero-trust security demonstration

### How do I configure the SPIRE socket path in containers?

The default path is `/tmp/spire-agent/public/api.sock`. In Kubernetes, mount the SPIRE agent socket into your pod:

```yaml
apiVersion: v1
kind: Pod
spec:
  containers:
  - name: my-app
    volumeMounts:
    - name: spire-agent-socket
      mountPath: /tmp/spire-agent/public
      readOnly: true
  volumes:
  - name: spire-agent-socket
    hostPath:
      path: /run/spire/agent-sockets
      type: Directory
```

Then configure e5s:
```yaml
spire:
  workload_socket: "unix:///tmp/spire-agent/public/api.sock"
```

### What are the resource requirements?

**Minimal requirements for e5s itself:**
- CPU: ~10m (0.01 cores) idle, ~100m under load
- Memory: ~20-50 MB

**SPIRE agent overhead:**
- CPU: ~50m per agent pod
- Memory: ~100-200 MB per agent pod

These are typical values - actual usage depends on your workload. See `examples/minikube-lowlevel/deploy/values/` for production resource configurations.

---

## Troubleshooting

### "unable to create X509Source: connection refused"

This means e5s can't connect to the SPIRE agent socket. Check:

1. SPIRE agent is running: `kubectl get pods -n spire-system`
2. Socket path is correct in config: `workload_socket: "unix:///tmp/spire-agent/public/api.sock"`
3. Socket is mounted in your pod (Kubernetes deployments)
4. Workload is registered with SPIRE: `kubectl exec -n spire-system spire-server-0 -- spire-server entry show`

### "invalid server_spiffe_id: trust domain is empty"

Your SPIFFE ID is malformed. Valid format: `spiffe://trust-domain/path`

Examples:
- ✅ `spiffe://example.org/server`
- ✅ `spiffe://example.org/ns/default/sa/server`
- ❌ `example.org/server` (missing `spiffe://`)
- ❌ `spiffe:///server` (empty trust domain)

### "tls: bad certificate" or "certificate signed by unknown authority"

This usually means:

1. **Client not registered** - The client doesn't have a SPIFFE identity from SPIRE
2. **Trust domain mismatch** - Client and server are in different trust domains
3. **Wrong socket path** - Client is not connecting to SPIRE agent properly

Debug steps:
```bash
# Check if workload is registered
kubectl exec -n spire-system spire-server-0 -- \
  spire-server entry show

# Check SPIRE agent logs
kubectl logs -n spire-system -l app=spire-agent

# Check your app logs
kubectl logs <your-pod>
```

### How do I debug TLS handshake failures?

Enable debug logging in SPIRE agent:

```yaml
# In SPIRE agent config
agent:
  log_level: "DEBUG"
```

For e5s applications, add logging to see peer information:

```go
import "github.com/sufield/e5s/spiffehttp"

func handler(w http.ResponseWriter, r *http.Request) {
    peer, ok := spiffehttp.PeerFromRequest(r)
    if !ok {
        log.Println("ERROR: No peer certificate found")
        http.Error(w, "Unauthorized", 401)
        return
    }
    log.Printf("Authenticated request from: %s", peer.ID.String())
    // Handle request...
}
```

---

## Performance

### Does e5s add latency to HTTP requests?

No. Once the initial SPIRE connection is established:
- Per-request overhead: ~0 microseconds (same as standard TLS)
- Certificate rotation overhead: ~1 microsecond (happens in background)
- No additional serialization or proxy layers

e5s has the same runtime performance as using go-spiffe SDK directly because it uses the same underlying implementation.

### How many requests per second can e5s handle?

e5s doesn't limit throughput - it's bounded by your standard HTTP server performance:
- Same as `http.Server` with TLS (no additional bottleneck)
- Typical: 10,000+ req/sec on modest hardware
- Scales with number of CPU cores

Performance is identical to using go-spiffe SDK directly.

### Does certificate rotation cause connection drops?

No. Certificate rotation in SPIRE/go-spiffe is zero-downtime:
1. New certificate is fetched before old one expires
2. Both certificates are valid during overlap period
3. New connections use new certificate
4. Existing connections continue with old certificate until complete
5. No dropped requests or connection resets

---

## Migration

### I'm currently using go-spiffe SDK. How do I migrate to e5s?

See [comparison.md](./comparison.md) for the full migration guide. Quick steps:

1. Extract hardcoded config values to YAML:
```yaml
spire:
  workload_socket: "unix:///tmp/spire-agent/public/api.sock"
server:
  listen_addr: ":8443"
  allowed_client_spiffe_id: "spiffe://example.org/client"
```

2. Replace source creation:
```go
// Before
source, err := workloadapi.NewX509Source(ctx, ...)
defer source.Close()
tlsConfig := tlsconfig.MTLSServerConfig(source, source, ...)
srv := &http.Server{...}

// After
shutdown, err := e5s.Start("config.yaml", handler)
defer shutdown()
```

3. No changes needed to HTTP handlers (same `http.Handler` interface)

### Can I use both e5s and go-spiffe SDK in the same application?

Yes! Use each for what it's best at:
- e5s for HTTP endpoints
- go-spiffe SDK for gRPC or custom protocols

Example:
```go
// HTTP server with e5s
go func() {
    shutdown, _ := e5s.Start("config.yaml", httpHandler)
    defer shutdown()
}()

// gRPC server with go-spiffe SDK
source, _ := workloadapi.NewX509Source(ctx)
creds := grpccredentials.MTLSServerCredentials(source, source, ...)
grpcServer := grpc.NewServer(grpc.Creds(creds))
```

---

## Contributing

### How do I report a bug?

Use the [bug report template](.github/ISSUE_TEMPLATE/bug_report.yml) on GitHub. Include:
- Go version
- e5s version
- SPIRE version
- Steps to reproduce
- Expected vs actual behavior
- Relevant logs

### How do I request a feature?

Open a GitHub issue with the feature request template. Include:
- Clear description of the feature
- Use case and motivation
- Example API or configuration
- Alternative solutions considered

### Can I contribute code?

Yes! See [CONTRIBUTING.md](../CONTRIBUTING.md) for guidelines. Key points:
- Fork the repository
- Write tests for your changes
- Run linters: `golangci-lint run`
- Run security checks: `gosec ./...` and `govulncheck ./...`
- Sign your commits: `git commit -s`
- Follow conventional commit format

---

## Additional Resources

- [Quickstart Guide](./QUICKSTART_LIBRARY.md) - Get started in 5 minutes
- [API Reference](./API.md) - Complete API documentation
- [Comparison Guide](./comparison.md) - e5s vs go-spiffe SDK
- [Integration Tests](./integration-tests.md) - Testing with SPIRE
- [Examples](../examples/) - Working code examples
- [SPIRE Documentation](https://spiffe.io/docs/latest/spire/) - Official SPIRE docs
- [go-spiffe SDK](https://pkg.go.dev/github.com/spiffe/go-spiffe/v2) - Underlying SDK

---

## Still Have Questions?

- Open a [GitHub Discussion](https://github.com/sufield/e5s/discussions)
- File an [issue](https://github.com/sufield/e5s/issues) if you found a bug
- Check the [examples directory](../examples/) for working code
