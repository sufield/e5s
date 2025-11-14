# Debug Mode: Single-Threaded Server Execution

## Overview

e5s provides a special execution mode designed for debugging concurrency issues: **`StartSingleThread()`**

This mode runs the HTTP server synchronously in the current goroutine instead of spawning background threads, making it easier to:
- Step through code with a debugger
- Isolate whether bugs are related to e5s's threading or other components
- Understand the library's internal behavior
- Reproduce issues in a controlled environment

**What StartSingleThread() DOES eliminate:**
- e5s's HTTP server goroutine (the one that runs `http.Server.ListenAndServeTLS()`)
- Makes the HTTP server run in your calling goroutine (blocking)
- **Result**: e5s's own concurrency is eliminated

**What StartSingleThread() does NOT eliminate:**
- **go-spiffe SDK goroutines**: The SDK runs background goroutines for:
  - Automatic certificate rotation (watching for cert expiry, fetching new certs)
  - Trust bundle updates (keeping CA certificates current)
  - Workload API connection management (maintaining gRPC connection to SPIRE agent)
- **net/http request handling goroutines**: Go's net/http package spawns a goroutine per request, so your handlers are still called concurrently
- **Your application's own goroutines**: Any background workers you've created

## Why Debug Mode

### The Problem: Hard-to-Reproduce Concurrency Bugs

In production mTLS services, you might encounter bugs that:
- Only appear under specific timing conditions
- Can't be reproduced in local development
- Involve race conditions or goroutine interactions
- Make debugging difficult due to unpredictable execution order

When these bugs appear, you need to answer: **"Is this an e5s threading issue, a go-spiffe issue, or my application code?"**

### The Solution: A Binary Switch for e5s's Server Goroutine

Debug mode gives you a simple A/B test:

1. **Start() - Background server**: e5s runs the HTTP server in a background goroutine
2. **StartSingleThread() - Foreground server**: e5s runs the HTTP server in the current goroutine (blocking)

**Important**: StartSingleThread() eliminates e5s's concurrency but NOT go-spiffe's concurrency.

Both modes still have go-spiffe SDK goroutines running:
- Certificate rotation goroutines (watching cert expiry, auto-renewing)
- Trust bundle update goroutines (keeping CA certs current)
- Workload API connection management

If the bug only appears with Start(), you know e5s's server goroutine is involved. If it appears with both, the issue is in go-spiffe SDK, SPIRE, net/http handlers, or your application code.

## Quick Start

### Production Mode (Background Server)

```go
func main() {
    handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        id, ok := e5s.PeerID(r)
        if !ok {
            http.Error(w, "unauthorized", http.StatusUnauthorized)
            return
        }
        fmt.Fprintf(w, "Hello %s\n", id)
    })

    // Background server mode: server runs in background goroutine
    shutdown, err := e5s.Start("e5s.yaml", handler)
    if err != nil {
        log.Fatal(err)
    }
    defer shutdown()

    // Your application continues running...
    sigChan := make(chan os.Signal, 1)
    signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
    <-sigChan
}
```

### Debug Mode (Foreground Server)

```go
func main() {
    handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        id, ok := e5s.PeerID(r)
        if !ok {
            http.Error(w, "unauthorized", http.StatusUnauthorized)
            return
        }
        fmt.Fprintf(w, "Hello %s\n", id)
    })

    // Foreground server mode: server blocks here (no background goroutine)
    if err := e5s.StartSingleThread("e5s.yaml", handler); err != nil {
        log.Fatal(err)
    }
    // This line only runs when server exits
}
```

### Environment Variable Toggle

For easy switching without code changes:

```go
func main() {
    handler := myHandler()

    if os.Getenv("E5S_DEBUG_SINGLE_THREAD") == "1" {
        // Foreground server (blocking, no server goroutine)
        if err := e5s.StartSingleThread("e5s.yaml", handler); err != nil {
            log.Fatal(err)
        }
    } else {
        // Background server (spawns goroutine)
        shutdown, err := e5s.Start("e5s.yaml", handler)
        if err != nil {
            log.Fatal(err)
        }
        defer shutdown()

        sigChan := make(chan os.Signal, 1)
        signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
        <-sigChan
    }
}
```

Run with: `E5S_DEBUG_SINGLE_THREAD=1 ./your-server`

## What Changes in Debug Mode

### Same Behavior

- ✅ Configuration loading (same `e5s.yaml`)
- ✅ SPIRE connection and identity fetching
- ✅ TLS configuration and certificate rotation
- ✅ mTLS handshake and peer verification
- ✅ Request handling and `PeerID()` / `PeerInfo()` extraction
- ✅ go-spiffe SDK behavior (unchanged)

### Different Behavior

| Aspect | `Start()` - Background | `StartSingleThread()` - Foreground |
|--------|------------------------|-----------------------------------|
| HTTP server goroutine | Spawned in background | Runs in current goroutine (blocks) |
| Function return | Returns immediately | Blocks until server exits |
| Shutdown function | Returns `func() error` | No shutdown function |
| Error channel | Uses `errCh` for startup errors | Directly returns errors |
| go-spiffe SDK goroutines | Running (cert rotation, trust bundle updates, Workload API) | Running (cert rotation, trust bundle updates, Workload API) |
| e5s's own goroutines | +1 HTTP server goroutine | 0 (e5s runs in your goroutine) |

**Distinction**:
- e5s concurrency: **Eliminated** with StartSingleThread()
- go-spiffe concurrency: **Unchanged** (SDK goroutines run in both modes)

## Troubleshooting Workflow

### When You Have a Concurrency Bug

Follow this systematic approach to isolate the issue:

#### Step 1: Reproduce with Start() (Background Server)

```bash
# Run your service normally
go run ./cmd/your-server

# Or with race detector
go run -race ./cmd/your-server
```

Observe the symptom:
- Race detector reports
- Crashes or panics
- Weird behavior (wrong peer IDs, failed handshakes, etc.)
- Hangs or deadlocks

#### Step 2: Switch to StartSingleThread() (Foreground Server)

Change your code or use environment variable:

```bash
E5S_DEBUG_SINGLE_THREAD=1 go run ./cmd/your-server
```

Run the same test scenario.

**What you've eliminated**: e5s's HTTP server goroutine (e5s's own concurrency is now gone)

**What's still concurrent**:
- go-spiffe SDK goroutines (certificate rotation, trust bundle updates, Workload API connection)
- net/http handler goroutines (your handlers still process requests concurrently)
- Your application's own goroutines

#### Step 3: Interpret the Results

**Case A: Bug only appears with Start() (disappears with StartSingleThread())**

→ The bug involves e5s's HTTP server goroutine specifically.

Investigate:
- Interaction between HTTP server goroutine and your main goroutine
- Timing assumptions about server startup (Start() returns immediately, StartSingleThread() blocks)
- Shutdown sequence and cleanup ordering
- Shared state accessed by both server goroutine and main goroutine

Example issues:
- Race accessing shared config/state during startup
- Shutdown called before server fully initialized
- Order-dependent initialization between server and main code

**Case B: Bug appears with both Start() and StartSingleThread()**

→ The bug is NOT in e5s's server goroutine. It's in:
- go-spiffe SDK goroutines (certificate rotation, trust bundle updates)
- SPIRE agent/server behavior
- Your application's own goroutines
- Handler-level concurrency (multiple requests processed concurrently by net/http)

Investigate:
- go-spiffe SDK behavior (check COMPATIBILITY.md for known issues)
- SPIRE configuration and logs
- Your application's background workers
- Handler state management (note: handlers are STILL called concurrently even with StartSingleThread())

Example issues:
- Certificate rotation race in go-spiffe
- SPIRE agent connectivity problems
- Your own background goroutines
- Handler accessing shared state without locks

**Critical**: StartSingleThread() eliminates **e5s's concurrency** but not all concurrency:
- ✅ Eliminated: e5s's HTTP server goroutine
- ❌ Still present: go-spiffe SDK goroutines, net/http handler goroutines, your goroutines

Even with StartSingleThread(), your handlers are still called concurrently by net/http, and go-spiffe is still rotating certificates in the background.

**Case C: Bug disappears with both modes (or becomes intermittent)**

→ The bug is highly timing-sensitive or environment-dependent.

Possible causes:
- Removing the server goroutine changed timing enough to hide the race
- Load-dependent (only appears under high request concurrency)
- Environment-specific (network latency, SPIRE agent timing, etc.)

Next steps:
- Use `GOMAXPROCS=1` to reduce scheduler variability
- Run with `-race` flag to detect data races
- Load test to reproduce under concurrent requests
- Check environment differences (DNS, SPIRE config, network)

### Advanced Debugging with GOMAXPROCS

Further reduce scheduling non-determinism:

```bash
# Force single OS thread execution
GOMAXPROCS=1 E5S_DEBUG_SINGLE_THREAD=1 go run ./cmd/your-server
```

This makes execution even more predictable:
- All goroutines (yours, e5s, go-spiffe) run on one thread
- Interleaving becomes more sequential
- Race conditions become easier to spot

If a race still occurs with `GOMAXPROCS=1`, it's a real data race (shared memory without synchronization), not just a scheduling issue.

## IDE Debugging

Debug mode shines when using IDE debuggers (VS Code, GoLand, Delve):

### Setting Breakpoints

**Start() challenges:**
```
main() → Start() → [spawns server goroutine] → HTTP server listens
         ↓                                       ↓
         returns immediately               handler execution
```
Stepping through this is difficult because execution splits: main continues while server runs in background.

**StartSingleThread() simplicity:**
```
main() → StartSingleThread() → config load → SPIRE setup → TLS config → ListenAndServeTLS() → handler
```
Linear execution path that you can step through line-by-line.

### VS Code Debug Configuration

```json
{
    "version": "0.2.0",
    "configurations": [
        {
            "name": "Debug e5s Server (Single-Thread)",
            "type": "go",
            "request": "launch",
            "mode": "debug",
            "program": "${workspaceFolder}/cmd/your-server",
            "env": {
                "E5S_DEBUG_SINGLE_THREAD": "1",
                "E5S_CONFIG": "${workspaceFolder}/e5s.yaml"
            },
            "args": []
        }
    ]
}
```

Set breakpoints in:
- `e5s.buildServer()` - See configuration loading
- Your handler functions - Examine peer extraction
- `spiffehttp.NewServerTLSConfig()` - Inspect TLS setup

Step through the entire server initialization without jumping between goroutines.

## Example: Troubleshooting a Non-Reproducible Bug

### Scenario

A user reports:
> "Our service sometimes rejects valid clients with 'unauthorized' errors. We can't reproduce it locally, but it happens in production under load."

### Investigation Steps

#### 1. Get Context

```bash
# User provides:
# - e5s version: v0.2.0
# - go-spiffe version: v2.1.0
# - SPIRE server: 1.6.3
# - Config: e5s.yaml (attached)
# - Load: 100 req/s
```

#### 2. Reproduce with Official Example

```bash
# Use minimal e5s example
cd examples/highlevel
go build -o /tmp/e5s-server ./examples/basic-server
go build -o /tmp/e5s-client ./examples/basic-client

# Run against user's SPIRE
export SPIRE_SOCKET=/path/to/their/spire/agent.sock
/tmp/e5s-server &
SERVER_PID=$!

# Try to reproduce
for i in {1..1000}; do
    /tmp/e5s-client || echo "FAIL: iteration $i"
done

kill $SERVER_PID
```

If it reproduces → likely e5s or go-spiffe issue
If it doesn't → likely user's application code

#### 3. Apply Debug Mode Toggle

Add to example server:

```go
func main() {
    handler := myHandler()

    if os.Getenv("E5S_DEBUG_SINGLE_THREAD") == "1" {
        log.Println("Running in SINGLE-THREAD debug mode")
        if err := e5s.StartSingleThread("e5s.yaml", handler); err != nil {
            log.Fatal(err)
        }
    } else {
        log.Println("Running with background server (Start)")
        shutdown, err := e5s.Start("e5s.yaml", handler)
        if err != nil {
            log.Fatal(err)
        }
        defer shutdown()
        // ... signal handling
    }
}
```

#### 4. Run Both Modes

```bash
# With Start() (background server)
./e5s-server &
./run-load-test.sh
# Observe: intermittent failures

# With StartSingleThread() (foreground server)
E5S_DEBUG_SINGLE_THREAD=1 ./e5s-server &
./run-load-test.sh
# Observe: ?
```

#### 5. Interpret Results

**If failures disappear with StartSingleThread():**

The issue involves e5s's HTTP server goroutine specifically.

Check:
- Interaction between server goroutine and main goroutine
- Startup timing (Start() returns immediately, StartSingleThread() blocks)
- Shutdown sequence and cleanup ordering

**If failures persist with StartSingleThread():**

The issue is NOT in e5s's server goroutine. It's in go-spiffe, SPIRE, your handlers, or your application's goroutines.

Check:
- Certificate rotation timing (watch logs during rotation)
- SPIRE agent availability
- Handler-level bugs (use `go test -race`)
- Application's own goroutines

#### 6. Narrow Further with GOMAXPROCS

```bash
# Ultra-predictable mode
GOMAXPROCS=1 E5S_DEBUG_SINGLE_THREAD=1 ./e5s-server &
./run-load-test.sh
```

This eliminates almost all concurrency. If the bug still appears, it's likely:
- Not concurrency-related
- A logic error
- External system behavior (SPIRE, network)

### Resolution

This workflow turned a vague "sometimes breaks" into:
- **Reproducible**: minimal example that fails
- **Isolated**: with server goroutine (Start) vs without server goroutine (StartSingleThread)
- **Classified**: e5s server threading vs go-spiffe/SPIRE/application layers

Now you can fix the root cause with confidence.

## Real-World Use Cases

### Use Case 1: Learning the Library

**Goal:** Understand how e5s sets up mTLS

```go
// Set breakpoints and step through:
func main() {
    handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        peer, _ := e5s.PeerInfo(r)
        log.Printf("Authenticated: %s", peer.ID)
        w.WriteHeader(http.StatusOK)
    })

    // Step through every line
    if err := e5s.StartSingleThread("e5s.yaml", handler); err != nil {
        log.Fatal(err)
    }
}
```

You can see:
1. Config file parsing
2. SPIRE Workload API connection
3. X.509 source creation
4. TLS config construction
5. Server startup
6. First request handling

No jumping between goroutines.

### Use Case 2: Race Detector Investigation

**Start() with race detector:**

```bash
go run -race ./cmd/server
# Output: 50 race warnings (includes races with server goroutine)
```

**StartSingleThread() with race detector:**

```bash
E5S_DEBUG_SINGLE_THREAD=1 go run -race ./cmd/server
# Output: Fewer warnings (server goroutine races eliminated)
```

Now you can focus on the remaining races, which are in your code or go-spiffe.

### Use Case 3: Testing Specific Scenarios

**Test certificate rotation behavior:**

```go
func TestCertRotation(t *testing.T) {
    // Use debug mode for predictable timing
    handler := recordingHandler()

    go func() {
        if err := e5s.StartSingleThread("e5s.yaml", handler); err != nil {
            t.Errorf("Server error: %v", err)
        }
    }()

    // Wait for server to start
    time.Sleep(100 * time.Millisecond)

    // Make requests before rotation
    client, shutdown, _ := e5s.Client("e5s.yaml")
    defer shutdown()

    resp1, _ := client.Get("https://localhost:8443/test")
    // Verify cert1 details

    // Trigger rotation (simulate SPIRE rotation)
    triggerSPIRERotation()

    // Make requests after rotation
    resp2, _ := client.Get("https://localhost:8443/test")
    // Verify cert2 details

    // Assertions about rotation behavior
}
```

Debug mode makes this test deterministic.

## Comparison with go-spiffe SDK Debugging

### What Debug Mode Does NOT Change

e5s debug mode does **not** affect go-spiffe internals:

```
                          ┌─────────────────────────────────────┐
                          │         Your Application            │
                          │  - main()                           │
                          │  - handlers                         │
                          │  - your own goroutines              │
                          └──────────────┬──────────────────────┘
                                         │
                          ┌──────────────▼──────────────────────┐
                          │             e5s                     │
                          │  • Start() / StartSingleThread()    │
                          │  • buildServer() ◄── DEBUG MODE     │
                          │  • PeerInfo() / PeerID()            │
                          └──────────────┬──────────────────────┘
                                         │
                          ┌──────────────▼──────────────────────┐
                          │          go-spiffe SDK              │
                          │  • X509Source (rotation goroutines) │
                          │  • TLS handshake                    │
                          │  • Trust bundle updates             │
                          └──────────────┬──────────────────────┘
                                         │
                          ┌──────────────▼──────────────────────┐
                          │         SPIRE Agent                 │
                          │  • Workload API                     │
                          │  • Certificate issuance             │
                          │  • Attestation                      │
                          └─────────────────────────────────────┘
```

**Debug mode only affects the e5s layer.** It does not:
- Change how go-spiffe rotates certificates
- Modify SPIRE Workload API behavior
- Alter TLS handshake mechanics
- Remove go-spiffe's internal goroutines

It only removes the goroutine that e5s spawns for `http.Server.ListenAndServeTLS()`.

### When to Debug go-spiffe Instead

If you suspect issues in go-spiffe itself:

1. Enable go-spiffe debug logging:
   ```go
   // In your code
   import "github.com/spiffe/go-spiffe/v2/logger"

   logger.Set(logger.Std) // Enable SDK logging
   ```

2. Use go-spiffe's examples directly (without e5s):
   ```bash
   cd $GOPATH/src/github.com/spiffe/go-spiffe/v2/examples
   go run server/main.go
   ```

3. Check go-spiffe issues: https://github.com/spiffe/go-spiffe/issues

Debug mode eliminates **e5s's own concurrency** to help isolate whether bugs are in e5s's threading. It does NOT eliminate go-spiffe SDK concurrency, so it won't help debug SDK-internal threading issues.

## Technical Details

### Internal Changes

When you call `StartSingleThread`:

```go
func StartSingleThread(configPath string, handler http.Handler) error {
    // 1. Build server (same as Start)
    srv, identityShutdown, err := buildServer(configPath, handler)
    if err != nil {
        return err
    }
    defer identityShutdown() // Cleanup on exit

    // 2. Run server in current goroutine (DIFFERENCE)
    err = srv.ListenAndServeTLS("", "")
    if err != nil && err != http.ErrServerClosed {
        return fmt.Errorf("server exited with error: %w", err)
    }

    return nil
}
```

Contrast with `Start`:

```go
func Start(configPath string, handler http.Handler) (shutdown func() error, err error) {
    // 1. Build server (same as StartSingleThread)
    srv, identityShutdown, err := buildServer(configPath, handler)
    if err != nil {
        return nil, err
    }

    errCh := make(chan error, 1)

    // 2. Run server in background goroutine (DIFFERENCE)
    go func() {
        err := srv.ListenAndServeTLS("", "")
        if err != nil && err != http.ErrServerClosed {
            errCh <- err
        }
    }()

    // 3. Wait for startup or failure
    select {
    case err := <-errCh:
        identityShutdown()
        return nil, err
    case <-time.After(100 * time.Millisecond):
        // Success
    }

    // 4. Return shutdown function
    shutdownFunc := func() error {
        // ... shutdown logic
    }

    return shutdownFunc, nil
}
```

The difference: **one `go func()` statement.**

Everything else is identical, which is why this mode is so useful for debugging.

## Best Practices

### When to Use Debug Mode

✅ **DO use debug mode when:**
- Stepping through code with a debugger
- Investigating race conditions
- Learning how e5s works
- Isolating whether a bug is in e5s's threading
- Writing deterministic tests
- Reproducing timing-sensitive bugs

❌ **DON'T use debug mode for:**
- Production deployments
- Performance testing
- Load testing
- High-concurrency scenarios
- Final integration testing

### Combining with Other Tools

**With race detector:**
```bash
go run -race -E5S_DEBUG_SINGLE_THREAD=1 ./cmd/server
```

**With GOMAXPROCS:**
```bash
GOMAXPROCS=1 E5S_DEBUG_SINGLE_THREAD=1 go run ./cmd/server
```

**With profiling:**
```bash
go run -cpuprofile=cpu.prof -E5S_DEBUG_SINGLE_THREAD=1 ./cmd/server
# Simpler profile without goroutine noise
```

**With delve:**
```bash
dlv debug ./cmd/server -- -E5S_DEBUG_SINGLE_THREAD=1
(dlv) break e5s.StartSingleThread
(dlv) continue
(dlv) step  # Step through initialization
```

### Reporting Bugs with Debug Mode

When reporting an e5s bug, include:

```
### Bug Report

**e5s version:** v0.2.0
**go-spiffe version:** v2.1.0
**Go version:** 1.21.0

**Reproduction:**
1. Run: `go run ./cmd/server`
2. Make request: `curl https://localhost:8443/test`
3. Observe: [symptom]

**Debug Mode Test:**
- With Start() (background server): [does bug appear? yes/no]
- With StartSingleThread() (foreground server): [does bug appear? yes/no]

**Race Detector:**
[paste race detector output if applicable]

**Additional context:**
- SPIRE version: 1.6.3
- Kubernetes version: 1.25
- Config: [attach e5s.yaml]
```

This helps maintainers quickly classify the issue.

## Limitations

### What Debug Mode Cannot Fix

- **go-spiffe issues**: SDK behavior is unchanged
- **SPIRE configuration problems**: Agent/server issues persist
- **Network issues**: TLS handshake failures remain
- **Load-dependent bugs**: Low request rate may hide issues
- **Multi-service interactions**: Debug mode is server-side only

### Performance Impact

Debug mode is **not suitable for production** because:
- Single-threaded HTTP server (poor concurrency)
- Blocked main goroutine (no graceful shutdown)
- No startup/shutdown control
- Synchronous execution (slow request handling)

This is intentional - debug mode prioritizes predictability over performance.

## Summary

Debug mode (`StartSingleThread`) is a troubleshooting tool that:

✅ **Eliminates e5s's own concurrency** by removing the HTTP server goroutine
✅ Keeps all other behavior identical (config, SPIRE, TLS)
✅ Makes IDE debugging straightforward (e5s code runs in your main goroutine)
✅ Helps isolate whether bugs involve e5s's server goroutine specifically
✅ Works as an A/B switch: with vs without e5s's concurrency

**What remains concurrent:**
- go-spiffe SDK goroutines (certificate rotation, trust bundle updates, Workload API connection)
- net/http handler goroutines (your handlers are called concurrently per request)
- Your application's own goroutines

Use it to answer: **"Is this bug related to e5s's HTTP server goroutine?"**

**Remember**:
- StartSingleThread() **DOES** eliminate e5s's own concurrency (the HTTP server goroutine)
- StartSingleThread() **does NOT** eliminate go-spiffe SDK concurrency (cert rotation, trust bundle updates) or net/http handler concurrency

For production, always use `Start()` or `Serve()`.

## Additional Resources

- [API Documentation](../reference/api.md) - Full e5s API reference
- [FAQ](../explanation/faq.md) - Common questions and answers
- [go-spiffe Issues](https://github.com/spiffe/go-spiffe/issues) - SDK-level debugging

## Getting Help

If you're still stuck after trying debug mode:

1. Check if it's documented: [docs/](.)
2. Search existing issues: [GitHub Issues](https://github.com/sufield/e5s/issues)
3. Ask with context:
   - e5s/go-spiffe/SPIRE versions
   - Debug mode results (does it change behavior?)
   - Race detector output
   - Minimal reproduction

Debug mode is a diagnostic tool. It helps you ask better questions by narrowing down where the problem lies.
