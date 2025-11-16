# Success Path

**This document maps YOUR path from where you are NOW to where you want to BE.**

Choose your starting point below, then follow the steps in order.

---

## Path 1: "I want to add mTLS to my HTTP service"

**Where you are NOW:** You have an HTTP service in Go. You want to secure it with mutual TLS using SPIFFE/SPIRE.

**Where you want to BE:** Your service authenticates peers using cryptographic identities instead of API keys.

**Your success path:**

### Step 1: Understand what you're getting (2 minutes)

**What is mTLS?**
- Both client and server prove their identity with certificates
- No API keys, no passwords, no secrets to leak

**What is SPIFFE/SPIRE?**
- SPIFFE = standard for service identities (like `spiffe://prod.company.com/weather-service`)
- SPIRE = the system that issues and rotates certificates automatically
- Your service gets a certificate that proves its identity

**What is e5s?**
- A Go library that connects your HTTP service to SPIRE
- Handles all the complexity of TLS configuration and certificate rotation
- Works with standard `net/http` - no framework lock-in

**✅ Checkpoint:** You understand that e5s connects your Go HTTP service to SPIRE for automatic mTLS.

### Step 2: Install prerequisites (5 minutes)

**You need:**
1. **Go 1.25+** installed
   ```bash
   go version  # Should show go1.25 or higher
   ```

2. **SPIRE running** (for local development)
   ```bash
   # Quick start - we'll auto-start SPIRE in examples
   # OR install SPIRE manually: https://spiffe.io/downloads
   ```

3. **e5s library**
   ```bash
   go get github.com/sufield/e5s@latest
   ```

**✅ Checkpoint:** You have Go 1.25+ and the e5s library installed.

### Step 3: Try the working example (10 minutes)

**Don't write code yet.** First, see it working:

```bash
# Clone and run the example
cd /tmp
git clone https://github.com/sufield/e5s
cd e5s/examples/highlevel

# Auto-start SPIRE and run both server and client
make start-stack
make run-server   # Terminal 1
make run-client   # Terminal 2 - should successfully connect
```

**What you should see:**
- Server starts on port 8443
- Client makes request to server
- Request succeeds with "Authenticated request from spiffe://example.org/client"

**✅ Checkpoint:** You saw a working mTLS client/server exchange using e5s.

### Step 4: Understand the server code (5 minutes)

Open `examples/highlevel/server/main.go` and read this:

```go
package main

import (
    "encoding/json"
    "net/http"
    "github.com/sufield/e5s"
    "github.com/sufield/e5s/spiffehttp"
)

func handler(w http.ResponseWriter, r *http.Request) {
    // Extract authenticated peer's SPIFFE ID
    peer, ok := spiffehttp.PeerFromRequest(r)
    if !ok {
        http.Error(w, "Unauthorized", 401)
        return
    }

    // Business logic - peer is authenticated
    response := map[string]string{
        "message": "Hello from server",
        "peer_id": peer.ID.String(),
    }
    json.NewEncoder(w).Encode(response)
}

func main() {
    // Start mTLS server - reads config from config.yaml
    shutdown, err := e5s.Start("config.yaml", http.HandlerFunc(handler))
    if err != nil {
        panic(err)
    }
    defer shutdown()

    select {} // Keep running
}
```

Only 3 things different from regular HTTP:

1. Call `e5s.Start()` instead of creating `http.Server` manually
2. Use `spiffehttp.PeerFromRequest()` to get authenticated peer
3. Use a config file instead of hardcoding ports/IDs

**✅ Checkpoint:** You understand the basic server structure.

### Step 5: Understand the config file (3 minutes)

Open `examples/highlevel/server-config.yaml`:

```yaml
version: 1
spire:
  workload_socket: "unix:///tmp/spire-agent/public/api.sock"  # Where SPIRE agent lives
server:
  listen_addr: ":8443"  # Your server port
  allowed_client_trust_domain: "example.org"  # Who can connect
```

**That's it.** Three settings:
1. Where to find SPIRE agent
2. What port to listen on
3. Who's allowed to connect

**✅ Checkpoint:** You understand the config file structure.

### Step 6: Adapt to your service (15 minutes)

**Now adapt the example to your existing service:**

1. **Copy your existing HTTP handler:**
   ```go
   func myHandler(w http.ResponseWriter, r *http.Request) {
       // Your existing business logic
   }
   ```

2. **Wrap it with peer authentication:**
   ```go
   func myHandler(w http.ResponseWriter, r *http.Request) {
       // NEW: Check authentication
       peer, ok := spiffehttp.PeerFromRequest(r)
       if !ok {
           http.Error(w, "Unauthorized", 401)
           return
       }

       // NEW: Optional - check specific peer identity
       if peer.ID.String() != "spiffe://prod.company.com/client" {
           http.Error(w, "Forbidden", 403)
           return
       }

       // Your existing business logic unchanged
       // ...
   }
   ```

3. **Replace your server startup:**
   ```go
   // OLD:
   // http.ListenAndServe(":8080", handler)

   // NEW:
   shutdown, err := e5s.Start("config.yaml", handler)
   if err != nil {
       log.Fatal(err)
   }
   defer shutdown()
   select {}
   ```

4. **Create your config.yaml:**
   ```yaml
   version: 1
   spire:
     workload_socket: "unix:///tmp/spire-agent/public/api.sock"
   server:
     listen_addr: ":8443"
     allowed_client_trust_domain: "your-domain.com"
   ```

**✅ Checkpoint:** Your service now uses e5s for mTLS.

### Step 7: Test locally (10 minutes)

```bash
# Start SPIRE (one-time setup)
cd examples/highlevel
make start-stack

# Register your service with SPIRE
kubectl exec -n spire-system spire-server-0 -- \
  spire-server entry create \
  -spiffeID spiffe://example.org/my-service \
  -parentID spiffe://example.org/spire-agent \
  -selector k8s:pod-label:app:my-service

# Run your service
go run main.go
```

**✅ Checkpoint:** Your service runs with mTLS locally.

### Step 8: Deploy to production

**Next step:** See [docs/how-to/deploy-helm.md](docs/how-to/deploy-helm.md) for Kubernetes deployment.

**🎉 SUCCESS:** You've added mTLS to your HTTP service using SPIFFE/SPIRE!

---

## Path 2: "I want to migrate from go-spiffe SDK to e5s"

**Where you are NOW:** You're using go-spiffe SDK directly. Too much boilerplate code.

**Where you want to BE:** Simpler code using e5s config files instead of hardcoded values.

**Your success path:**

### Step 1: Identify what you're replacing (5 minutes)

**Find this pattern in your code:**

```go
// OLD: Using go-spiffe SDK directly
import (
    "github.com/spiffe/go-spiffe/v2/workloadapi"
    "github.com/spiffe/go-spiffe/v2/spiffetls/tlsconfig"
)

func main() {
    ctx := context.Background()

    // Create X.509 source
    source, err := workloadapi.NewX509Source(ctx,
        workloadapi.WithClientOptions(
            workloadapi.WithAddr("unix:///tmp/spire-agent/public/api.sock"),
        ),
    )
    if err != nil {
        log.Fatal(err)
    }
    defer source.Close()

    // Configure TLS
    tlsConfig := tlsconfig.MTLSServerConfig(
        source,
        source,
        tlsconfig.AuthorizeAny(),
    )

    // Create HTTP server
    server := &http.Server{
        Addr:      ":8443",
        Handler:   handler,
        TLSConfig: tlsConfig,
    }

    // Manual graceful shutdown setup
    // ... 20 more lines of shutdown logic
}
```

**Count the lines:** Typically 60-80 lines of TLS/SPIRE setup code.

**✅ Checkpoint:** You found the go-spiffe setup code in your service.

### Step 2: Extract hardcoded values to YAML (10 minutes)

**Create `config.yaml` from your hardcoded values:**

```yaml
version: 1
spire:
  # FROM: workloadapi.WithAddr("unix:///tmp/spire-agent/public/api.sock")
  workload_socket: "unix:///tmp/spire-agent/public/api.sock"

server:
  # FROM: Addr: ":8443"
  listen_addr: ":8443"

  # FROM: Your authorization logic (choose ONE)
  allowed_client_trust_domain: "example.org"  # If you accept any client in domain
  # OR
  # allowed_client_spiffe_id: "spiffe://example.org/specific-client"  # If specific client only
```

**✅ Checkpoint:** You created a config file from your hardcoded values.

### Step 3: Replace source creation (2 minutes)

**Before:**
```go
source, err := workloadapi.NewX509Source(ctx, ...)
defer source.Close()
tlsConfig := tlsconfig.MTLSServerConfig(source, source, ...)
server := &http.Server{...}
```

**After:**
```go
shutdown, err := e5s.Start("config.yaml", handler)
defer shutdown()
```

**Delete:** ~60-80 lines of SPIRE setup code
**Add:** 2 lines

**✅ Checkpoint:** You replaced source creation with `e5s.Start()`.

### Step 4: Update imports (1 minute)

**Remove:**
```go
"github.com/spiffe/go-spiffe/v2/workloadapi"
"github.com/spiffe/go-spiffe/v2/spiffetls/tlsconfig"
```

**Add:**
```go
"github.com/sufield/e5s"
"github.com/sufield/e5s/spiffehttp"  // Only if you use PeerFromRequest
```

**✅ Checkpoint:** Imports updated.

### Step 5: Keep your handlers unchanged (0 minutes)

**Good news:** Your HTTP handlers work exactly the same.

```go
func handler(w http.ResponseWriter, r *http.Request) {
    // If you were using spiffeid.FromContext before:
    // OLD: peerID, ok := spiffeid.FromContext(r.Context())

    // NEW:
    peer, ok := spiffehttp.PeerFromRequest(r)
    peerID := peer.ID  // Same spiffeid.ID type

    // Rest of your handler unchanged
}
```

**✅ Checkpoint:** Handlers migrated.

### Step 6: Test the migration (10 minutes)

```bash
# Build and run
go mod tidy
go run main.go

# Should see:
# "Server listening on :8443"
# "Workload API connection established"
```

**Test with curl:**
```bash
# This should fail (no mTLS cert)
curl https://localhost:8443/

# Test with your mTLS client
go run your-client/main.go
```

**✅ Checkpoint:** Migration complete and tested.

### Step 7: Measure the improvement

**Before migration:**
- Lines of SPIRE setup code: ~60-80
- Configuration method: Hardcoded in Go
- Shutdown handling: Manual (~20 lines)

**After migration:**
- Lines of SPIRE setup code: ~2
- Configuration method: YAML file
- Shutdown handling: Automatic

**Code reduction: ~80%**

**🎉 SUCCESS:** You've migrated from go-spiffe SDK to e5s!

---

## Path 3: "I need to debug mTLS connection failures"

**Where you are NOW:** Your e5s service isn't working. Clients can't connect or get TLS errors.

**Where you want to BE:** Connection working, or you understand exactly what's broken.

**Your success path:**

### Step 1: Enable debug mode (2 minutes)

**Add one line to your code:**

```go
// OLD:
shutdown, err := e5s.Start("config.yaml", handler)

// NEW:
shutdown, err := e5s.StartDebug("config.yaml", handler)
```

**Run again:**
```bash
go run main.go
```

**You'll now see detailed output:**
```
[e5s DEBUG] Loading config from: config.yaml
[e5s DEBUG] Workload socket: unix:///tmp/spire-agent/public/api.sock
[e5s DEBUG] Connecting to SPIRE Workload API...
[e5s DEBUG] Successfully connected to SPIRE
[e5s DEBUG] Received X.509 SVID: spiffe://example.org/server
[e5s DEBUG] Trust bundle contains 1 root CAs
[e5s DEBUG] TLS Config created with TLS 1.3 minimum
[e5s DEBUG] HTTP server starting on :8443
[e5s DEBUG] Server ready to accept connections
```

**✅ Checkpoint:** Debug mode enabled, you can see what's happening.

### Step 2: Identify which step fails (5 minutes)

**Read the debug output. Where does it stop?**

**Scenario A: Stops at "Connecting to SPIRE Workload API..."**
→ **Problem:** Can't reach SPIRE agent
→ **Next step:** Go to Step 3A

**Scenario B: Stops at "Received X.509 SVID..."**
→ **Problem:** SPIRE isn't issuing certificates
→ **Next step:** Go to Step 3B

**Scenario C: Server starts, but clients get TLS errors**
→ **Problem:** Certificate verification failing
→ **Next step:** Go to Step 3C

**Scenario D: Everything looks good in debug output**
→ **Problem:** Application-level issue, not e5s
→ **Next step:** Go to Step 3D

### Step 3A: Fix SPIRE agent connection (10 minutes)

**Problem:** Can't connect to SPIRE agent socket.

**Fix 1: Check if SPIRE agent is running**
```bash
# Check if socket exists
ls -la /tmp/spire-agent/public/api.sock

# Should show:
# srwxrwxrwx 1 user user 0 Nov 14 10:00 /tmp/spire-agent/public/api.sock
```

**If socket missing:**
```bash
# Start SPIRE agent
cd examples/highlevel
make start-stack

# Verify
ls -la /tmp/spire-agent/public/api.sock
```

**Fix 2: Check socket path in config**
```yaml
spire:
  workload_socket: "unix:///tmp/spire-agent/public/api.sock"  # ← Must match actual socket location
```

**Fix 3: In Kubernetes, mount the socket**
```yaml
volumeMounts:
- name: spire-agent-socket
  mountPath: /tmp/spire-agent/public
  readOnly: true
```

**✅ Checkpoint:** SPIRE agent connection working.

### Step 3B: Fix certificate issuance (15 minutes)

**Problem:** SPIRE agent connected, but no certificate issued.

**Check registration:**
```bash
# List SPIRE entries
kubectl exec -n spire-system spire-server-0 -- \
  spire-server entry show

# Look for your service's SPIFFE ID
# Should see: spiffe://example.org/your-service
```

**If your service not registered:**
```bash
# Register it
kubectl exec -n spire-system spire-server-0 -- \
  spire-server entry create \
  -spiffeID spiffe://example.org/your-service \
  -parentID spiffe://example.org/spire-agent \
  -selector k8s:pod-label:app:your-service

# Verify
kubectl exec -n spire-system spire-server-0 -- \
  spire-server entry show | grep your-service
```

**Restart your service:**
```bash
go run main.go
# Should now see: "Received X.509 SVID: spiffe://example.org/your-service"
```

**✅ Checkpoint:** Certificate issued successfully.

### Step 3C: Fix certificate verification (10 minutes)

**Problem:** Server starts, client connects, but TLS handshake fails.

**Check error message:**
```
Client error: tls: bad certificate
```
→ Client not registered with SPIRE (go to Step 3B for client)

```
Client error: tls: failed to verify certificate: x509: certificate signed by unknown authority
```
→ Different trust domains (server and client in different SPIRE deployments)

**Fix: Ensure same trust domain**

**Server config:**
```yaml
server:
  allowed_client_trust_domain: "example.org"
```

**Client must have SPIFFE ID starting with:** `spiffe://example.org/...`

**Verify:**
```bash
# Check client's SPIFFE ID
kubectl exec -n spire-system spire-server-0 -- \
  spire-server entry show | grep client

# Should show: spiffe://example.org/client (same domain as server expects)
```

**✅ Checkpoint:** Certificate verification working.

### Step 3D: Debug application logic (5 minutes)

**Problem:** mTLS working, but application behavior wrong.

**Add logging to your handler:**
```go
func handler(w http.ResponseWriter, r *http.Request) {
    peer, ok := spiffehttp.PeerFromRequest(r)
    if !ok {
        log.Println("ERROR: No peer certificate found")
        http.Error(w, "Unauthorized", 401)
        return
    }

    log.Printf("Authenticated request from: %s", peer.ID.String())
    log.Printf("Request path: %s", r.URL.Path)
    log.Printf("Request method: %s", r.Method)

    // Your business logic
}
```

**Run and check logs:**
```bash
go run main.go
# Make request
# Check what gets logged
```

**✅ Checkpoint:** You can see what your application is doing.

### Step 4: Turn off debug mode for production (1 minute)

**Once working:**
```go
// Change back from:
shutdown, err := e5s.StartDebug("config.yaml", handler)

// To:
shutdown, err := e5s.Start("config.yaml", handler)
```

**🎉 SUCCESS:** You debugged and fixed your mTLS connection!

---

## Path 4: "I want to deploy to Kubernetes production"

**Where you are NOW:** Working locally with e5s. Ready for production deployment.

**Where you want to BE:** Service running in Kubernetes with mTLS using SPIRE.

**Your success path:**

See [docs/how-to/deploy-helm.md](docs/how-to/deploy-helm.md) for complete production deployment guide.

**Quick overview of steps:**
1. Install SPIRE in your cluster (Helm chart)
2. Register your workloads
3. Deploy your service with socket mount
4. Configure network policies
5. Monitor and verify

---

## Need Help?

**If you're stuck on any step:**

1. ✅ First, check if you completed all previous steps in YOUR path
2. ✅ Re-read the step that's not working - did you miss something?
3. ✅ Use debug mode (Path 3) to see what's happening
4. ❌ Don't jump to other documentation yet

**Still stuck?**
- Open an issue: https://github.com/sufield/e5s/issues
- Include: Which path, which step, what error message

---

## Success Metrics

**You know you're successful when:**

✅ **Path 1 (Add mTLS):** Your service accepts requests with mTLS, rejects requests without certificates
✅ **Path 2 (Migration):** Your code is 80% shorter, uses config files, still works exactly the same
✅ **Path 3 (Debug):** You found the problem, fixed it, connection works
✅ **Path 4 (Production):** Service running in Kubernetes, automatic certificate rotation working
