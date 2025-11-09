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

Yes, e5s uses Go's standard library `net/http` package for all HTTP functionality. It does not replace or reimplement HTTP.

**For Clients:**
- `e5s.Client()` returns a standard `*http.Client` type
- You can use all standard methods: `client.Get()`, `client.Post()`, `client.Do()`
- Response handling uses standard `http.Response` with `resp.Body`, `resp.StatusCode`, etc.

**For Servers:**
- `e5s.Start()` creates a standard `http.Server` internally
- You use standard `http.HandleFunc()` and `http.Handler` interface for routing
- Handlers receive standard `http.ResponseWriter` and `*http.Request` parameters

**What e5s does:**
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

Configuration must be provided via YAML files. There is no environment variable override support. Use different YAML files for different environments (dev, staging, prod).

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

Apply dev environment (uses values-dev.yaml):
```bash
helmfile -e dev apply
```

Apply prod environment (uses values-prod.yaml):
```bash
helmfile -e prod apply
```

### Why does the server use `:8443` but the client uses `localhost:8443`?

This is the **correct and standard pattern** for server/client configuration. They serve different purposes:

**Server `listen_addr: ":8443"`**

The server uses **`:8443`** which means "listen on ALL network interfaces":
- `localhost` (127.0.0.1) for local connections
- External IPs (e.g., 192.168.1.100) for remote connections
- Container IPs in Kubernetes
- Any other network interface on the machine

This is called a "bind address" - it specifies where the server accepts connections from.

**Example:**

```yaml
server:
  listen_addr: ":8443"  # Listen on all interfaces, port 8443
```

**Why this is correct for servers:**

- ✅ Allows local testing (`localhost` connections work)
- ✅ Allows production deployment (external connections work)
- ✅ Works in Kubernetes (pod-to-pod connections work)
- ✅ Flexible - no changes needed between environments

**Client `server_url: "https://localhost:8443/time"`**

The client uses a full URL specifying:

1. **Protocol** - `https://` (TLS encrypted)
2. **Hostname** - `localhost` (where to connect)
3. **Port** - `8443` (which port)
4. **Path** - `/time` (which endpoint to request)

**Example:**

```yaml
client:
  server_url: "https://localhost:8443/time"  # For local development
```

In production/Kubernetes, override with environment variable:

```yaml
# k8s deployment
env:
- name: SERVER_URL
  value: "https://e5s-server:8443/time"  # Kubernetes service name
```

**Comparison:**

```
Local Development (same machine):
┌─────────────────────────────────┐
│  Server                         │
│  listen_addr: ":8443"           │
│  (Listens on all interfaces)    │
│         ↑                       │
│         │ connects via loopback │
│         │                       │
│  Client                         │
│  server_url:                    │
│  "https://localhost:8443/time"  │
└─────────────────────────────────┘

Kubernetes (separate pods):
┌────────────────┐         ┌────────────────┐
│  Client Pod    │─────────│  Server Pod    │
│                │ network │                │
│  server_url:   │         │  listen_addr:  │
│  https://e5s-  │         │  ":8443"       │
│  server:8443   │         │  (all ifaces)  │
│  (via env var) │         │                │
└────────────────┘         └────────────────┘
```

**What if server used `localhost:8443`?**

If the server configured `listen_addr: "localhost:8443"`, it would **only** listen on the loopback interface:
- ✅ Local clients could connect
- ❌ Kubernetes pods could NOT connect
- ❌ Other machines could NOT connect
- ❌ Production deployments would fail

**Summary:**
- **Server `:8443`** = "I'm available to everyone on all network interfaces"
- **Client `localhost:8443`** = "Connect to the local machine for testing"
- **Client `e5s-server:8443`** (prod) = "Connect to the e5s-server service in Kubernetes"

This pattern allows the same server configuration to work in both development and production, while clients specify exactly where to connect based on their environment.

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

### How do I test client and server interaction?

See the integration test examples:

1. **Integration tests** - Run full SPIRE stack in Kubernetes (see `docs/INTEGRATION_TESTING.md`)
2. **Local testing** - Use scripts in `examples/minikube-lowlevel/`

Example integration test setup:

Start Minikube with SPIRE:
```bash
cd examples/minikube-lowlevel
./setup.sh
```

Run integration tests:
```bash
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

Check if workload is registered:
```bash
kubectl exec -n spire-system spire-server-0 -- \
  spire-server entry show
```

Check SPIRE agent logs:
```bash
kubectl logs -n spire-system -l app=spire-agent
```

Check your app logs:
```bash
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
import (
    "context"
    "os"
    "os/signal"
    "syscall"

    "github.com/spiffe/go-spiffe/v2/spiffegrpc/grpccredentials"
    "github.com/spiffe/go-spiffe/v2/workloadapi"
    "github.com/sufield/e5s"
    "google.golang.org/grpc"
)

func main() {
    ctx, cancel := context.WithCancel(context.Background())
    defer cancel()

    // Start HTTP server with e5s
    httpShutdown, err := e5s.Start("config.yaml", httpHandler)
    if err != nil {
        log.Fatal(err)
    }
    defer httpShutdown()

    // Start gRPC server with go-spiffe SDK
    source, err := workloadapi.NewX509Source(ctx)
    if err != nil {
        log.Fatal(err)
    }
    defer source.Close()

    creds := grpccredentials.MTLSServerCredentials(source, source,
        tlsconfig.AuthorizeAny())
    grpcServer := grpc.NewServer(grpc.Creds(creds))

    // Register gRPC services here
    // pb.RegisterYourServiceServer(grpcServer, &yourService{})

    listener, err := net.Listen("tcp", ":50051")
    if err != nil {
        log.Fatal(err)
    }

    go func() {
        if err := grpcServer.Serve(listener); err != nil {
            log.Printf("gRPC server error: %v", err)
        }
    }()

    // Wait for interrupt signal
    sigChan := make(chan os.Signal, 1)
    signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
    <-sigChan

    // Graceful shutdown
    grpcServer.GracefulStop()
    httpShutdown()
}
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

## Still Have Questions?

- Open a [GitHub Discussion](https://github.com/sufield/e5s/discussions)
- File an [issue](https://github.com/sufield/e5s/issues) if you found a bug
