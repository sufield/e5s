# e5s Troubleshooting Guide

This guide is useful to diagnose and resolve issues with e5s, focusing on debugging workflows and concurrency-related problems.

## Quick Start: Running Examples

The fastest way to verify e5s is working:

### 1. Start Example Server (Normal Mode)

```bash
./scripts/run-example-server.sh
```

This runs the server in normal multi-threaded mode with an HTTP server goroutine.

### 2. Start Example Server (Debug Mode)

```bash
./scripts/run-example-server.sh --debug
```

This runs the server in single-threaded mode, eliminating e5s's HTTP server goroutine. Useful for:
- Step debugging without goroutine interference
- Race detector with `GOMAXPROCS=1`
- Isolating whether issues are concurrency-related

### 3. Run Example Client

In another terminal:

```bash
./scripts/run-example-client.sh
```

You should see:
- Server logs showing authenticated request
- Client logs showing successful response
- Current server time returned

---

## Debug Mode: When and Why

### What is Debug Mode?

e5s offers two ways to start a server:

**Normal mode** (`Start()`):
- Spawns HTTP server in a goroutine
- Returns immediately with a shutdown function
- Standard production mode

**Debug mode** (`StartSingleThread()`):
- Runs HTTP server on the calling goroutine
- Blocks until shutdown signal
- Eliminates e5s's own concurrency

### When to Use Debug Mode

Use `StartSingleThread()` when:

1. **Step debugging** - You want to use a debugger without goroutine switching
2. **Race detection** - Running with `-race` and `GOMAXPROCS=1`
3. **Isolating concurrency issues** - Determining if a bug is concurrency-related
4. **Simplifying stack traces** - Easier to read without goroutine overhead

### When NOT to Use Debug Mode

Don't use `StartSingleThread()` for:
- Production deployments
- Performance testing
- Testing actual concurrent behavior
- Any scenario where you need goroutines

### Limitation

Debug mode does NOT eliminate ALL concurrency. The go-spiffe SDK still runs background goroutines for:
- Certificate rotation
- Workload API watching
- TLS handshake processing

Debug mode only eliminates e5s's own HTTP server goroutine.

---

## Debugging Concurrency Issues

### Workflow: Is This a Concurrency Bug?

Follow this systematic approach:

#### Step 1: Reproduce in Normal Mode

```bash
./scripts/run-example-server.sh
```

Try to trigger the issue. If you can't reproduce it reliably, it may be a concurrency bug.

#### Step 2: Try Debug Mode

```bash
./scripts/run-example-server.sh --debug
```

Try to trigger the same issue

**If the issue disappears in debug mode:**
- Likely a concurrency bug in your handler code
- Check for data races, shared state, missing locks

**If the issue persists in debug mode:**
- Not related to e5s's HTTP server goroutine
- May be in SDK goroutines or your handler logic
- Continue to Step 3

#### Step 3: Run with Race Detector

Debug mode + race detector

```bash
E5S_DEBUG_SINGLE_THREAD=1 go run -race ./cmd/example-server

# Or normal mode + race detector
go run -race ./cmd/example-server
```

The race detector will report data races with stack traces.

#### Step 4: Reduce to Single CPU

```bash
# Debug mode + single CPU + race detector
GOMAXPROCS=1 E5S_DEBUG_SINGLE_THREAD=1 go run -race ./cmd/example-server
```

This maximizes reproducibility of race conditions.

---

## Common Issues and Solutions

### Issue: "workload_socket must be set"

**Cause:** Missing or empty `spire.workload_socket` in config

**Fix:**
```yaml
spire:
  workload_socket: "unix:///tmp/spire-agent/public/api.sock"
```

### Issue: "failed to create X509Source"

**Causes:**
- SPIRE agent not running
- Socket path incorrect
- Workload not registered with SPIRE

**Debug steps:**
1. Check SPIRE agent is running:
   ```bash
   ps aux | grep spire-agent
   ```

2. Check socket exists:
   ```bash
   ls -la /tmp/spire-agent/public/api.sock
   ```

3. Test workload API directly:
   ```bash
   SPIFFE_ENDPOINT_SOCKET=unix:///tmp/spire-agent/public/api.sock \
     go run ./cmd/example-server
   ```

### Issue: "tls: failed to verify certificate"

**Causes:**
- Client/server mismatch in trust domains
- Certificate expired
- SPIRE federation misconfigured

**Debug steps:**
1. Check trust domains match:
   ```bash
   # In server config
   allowed_client_trust_domain: "example.org"

   # In client config
   expected_server_trust_domain: "example.org"
   ```

2. Check SPIFFE IDs:
   ```bash
   # Add logging in your handler
   peer, ok := e5s.PeerInfo(r)
   log.Printf("Peer ID: %s", peer.ID.String())
   log.Printf("Peer expires: %s", peer.ExpiresAt)
   ```

### Issue: "context deadline exceeded"

**Cause:** `initial_fetch_timeout` too short

**Fix:**
```yaml
spire:
  initial_fetch_timeout: "60s"  # Increase from default 30s
```

### Issue: Data Race Detected

**Cause:** Shared state accessed from multiple goroutines without synchronization

**Example race:**
```go
// BAD: Concurrent access without lock
var counter int
http.HandleFunc("/count", func(w http.ResponseWriter, r *http.Request) {
    counter++  // RACE!
    fmt.Fprintf(w, "Count: %d", counter)
})
```

**Fix with mutex:**
```go
var (
    counter int
    mu      sync.Mutex
)

http.HandleFunc("/count", func(w http.ResponseWriter, r *http.Request) {
    mu.Lock()
    counter++
    count := counter
    mu.Unlock()

    fmt.Fprintf(w, "Count: %d", count)
})
```

**Fix with atomic:**
```go
var counter atomic.Int64

http.HandleFunc("/count", func(w http.ResponseWriter, r *http.Request) {
    count := counter.Add(1)
    fmt.Fprintf(w, "Count: %d", count)
})
```

---

## Advanced Debugging Techniques

### Enable Verbose Logging

```bash
# Set Go's HTTP server debug logging
export GODEBUG=http2debug=2

# Run with verbose output
./scripts/run-example-server.sh
```

### Profile CPU Usage

```go
import _ "net/http/pprof"

// In main():
go func() {
    log.Println(http.ListenAndServe("localhost:6060", nil))
}()
```

Then access:
```bash
go tool pprof http://localhost:6060/debug/pprof/profile
```

### Check TLS Handshake

```go
tlsCfg, _ := spiffehttp.NewServerTLSConfig(...)

// Add callback to debug handshakes
tlsCfg.GetCertificate = func(hello *tls.ClientHelloInfo) (*tls.Certificate, error) {
    log.Printf("TLS handshake from: %s", hello.ServerName)
    // Call original logic...
}
```

### Inspect SPIRE SVIDs

```bash
# Get workload SVID
SPIFFE_ENDPOINT_SOCKET=unix:///tmp/spire-agent/public/api.sock \
  spire-agent api fetch -socketPath /tmp/spire-agent/public/api.sock
```

---

## Testing in Kubernetes

### Check Pod SPIFFE ID

```bash
# Method 1: Using e5s CLI
go build -o /tmp/e5s ./cmd/e5s
kubectl exec my-pod -- /tmp/e5s discover pod my-pod

# Method 2: Check workload registration
kubectl exec -n spire spire-server-0 -- \
  spire-server entry show -spiffeID spiffe://example.org/ns/default/sa/default
```

### Debug Pod-to-Pod mTLS

```bash
# In server pod
kubectl logs my-server-pod

# In client pod
kubectl exec my-client-pod -- curl https://my-server:8443/time
```

### Common Kubernetes Issues

**Issue:** "no such host"

**Fix:** Use Kubernetes service DNS:
```yaml
client:
  server_url: "https://my-server-service:8443/api"  # Not pod IP!
```

**Issue:** "connection refused"

**Fix:** Check service is exposing the right port:
```yaml
spec:
  ports:
  - port: 8443
    targetPort: 8443  # Must match server listen_addr
```

---

## Getting Help

If you're still stuck:

1. **Check logs** - e5s logs errors with context
2. **Try debug mode** - Eliminate concurrency as a factor
3. **Use race detector** - Find data races automatically
4. **Simplify** - Remove complexity until it works, then add back
5. **Compare with examples** - Check cmd/example-server for working code

For bugs or feature requests:
- GitHub Issues: https://github.com/sufield/e5s/issues
- Include: Go version, e5s version, config file, error message, steps to reproduce

---

## Related Documentation

- [../how-to/debug-mtls.md](./../how-to/debug-mtls.md) - Detailed explanation of StartSingleThread()
- [../explanation/architecture.md](./../explanation/architecture.md) - e5s internal layering
- [CONFIG.md](config.md) - Configuration reference
- [API.md](api.md) - API documentation
