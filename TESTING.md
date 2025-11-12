# Testing Guide

This document describes the testing strategy for the e5s library.

## Testing Pyramid

```
        ┌──────────────────┐
        │   End-to-End     │  e5s CLI (manual)
        │   (Manual)       │
        ├──────────────────┤
        │   Integration    │  Testcontainers
        │   (Automated)    │  (Real SPIRE)
        ├──────────────────┤
        │   Unit Tests     │  Standard Go tests
        │   (Fast)         │  (Mocked SPIRE)
        └──────────────────┘
```

## Unit Tests

**Purpose:** Test individual functions and components in isolation.

**Location:** `*_test.go` files next to implementation

**Run:** `go test ./...`

**Example:**
```go
func TestConfigValidation(t *testing.T) {
    cfg := e5s.Config{
        Mode: e5s.ModeServer,
        Server: &e5s.ServerConfig{
            ListenAddr: ":8443",
        },
    }

    if err := cfg.Validate(); err != nil {
        t.Errorf("Valid config rejected: %v", err)
    }
}
```

**Characteristics:**
- ⚡ Very fast (milliseconds)
- No external dependencies
- High coverage of edge cases
- Uses mocks/fakes for external systems

## Integration Tests (Testcontainers)

**Purpose:** Test real mTLS communication with actual SPIRE infrastructure.

**Location:** `e5s_container_test.go`, other `*_integration_test.go` files

**Run:** `go test -v -run TestE5SWithContainers`

**Skip:** `go test -short` (skips container tests)

**Example:**
```go
func TestE5SWithContainers(t *testing.T) {
    if testing.Short() {
        t.Skip("Skipping container-based integration test")
    }

    // Setup SPIRE containers (server + agent)
    spire, cleanup := testhelpers.SetupSPIREContainers(t)
    defer cleanup()

    // Create e5s server with real SPIRE
    serverCfg := e5s.Config{
        Mode: e5s.ModeServer,
        Server: &e5s.ServerConfig{
            ListenAddr: ":18443",
            TLS: e5s.TLSConfig{
                WorkloadSocket: spire.SocketPath,
            },
            AllowedTrustDomain: spire.TrustDomain,
        },
    }

    shutdown, err := e5s.StartWithConfig(serverCfg, handler)
    if err != nil {
        t.Fatal(err)
    }
    defer shutdown()

    // Create e5s client with real SPIRE
    clientCfg := e5s.Config{
        Mode: e5s.ModeClient,
        Client: &e5s.ClientConfig{
            TLS: e5s.TLSConfig{
                WorkloadSocket: spire.SocketPath,
            },
            TrustedDomain: spire.TrustDomain,
        },
    }

    // Make real mTLS request
    err = e5s.WithHTTPClientFromConfig(ctx, clientCfg, func(client *http.Client) error {
        resp, err := client.Get("https://localhost:18443/hello")
        // ... verify response ...
        return err
    })

    if err != nil {
        t.Fatal(err)
    }
}
```

**Characteristics:**
- 🐢 Moderate speed (seconds to start containers)
- Requires Docker daemon
- Tests real SPIRE behavior
- Automatic cleanup via `t.Cleanup()`
- Explicit timeouts via `context.WithTimeout()`

**Benefits vs Shell Scripts:**
- ✅ Cross-platform (only needs Docker)
- ✅ Clear error messages with Go stack traces
- ✅ Runs in CI/CD without special setup
- ✅ Integrated with `go test`
- ✅ Explicit health checks and timeouts

## Manual Testing (e5s CLI)

**Purpose:** Manual verification, production deployment testing, demos.

**Location:** `cmd/e5s/`

**Run:** See `e5s deploy --help`

**Example Workflow:**
```bash
# Create test cluster
e5s deploy cluster create --name e5s-test --wait 60s

# Install SPIRE
e5s deploy spire install --trust-domain demo.e5s.io

# Deploy application
e5s deploy app install --chart-path chart/e5s-demo

# Run integration tests
e5s deploy test run

# Verify mTLS
e5s deploy test verify

# Check status
e5s deploy spire status
e5s deploy app status

# Clean up
e5s deploy app uninstall
e5s deploy spire uninstall
e5s deploy cluster delete --name e5s-test
```

**Characteristics:**
- 🐢 Slow (minutes for full workflow)
- Requires Kubernetes cluster (Kind, Minikube, etc.)
- Production-like environment
- Good for demos and documentation

## Testing Strategy by Component

### Core Library (e5s package)

| Component | Test Type | Approach |
|-----------|-----------|----------|
| Config parsing | Unit | Mock file reading |
| Config validation | Unit | Test valid/invalid configs |
| TLS setup | Integration | Real SPIRE containers |
| mTLS handshake | Integration | Real SPIRE containers |
| SPIFFE ID extraction | Unit | Mock x509 certificates |
| Authorization logic | Unit | Test policy evaluation |

### CLI Tool (cmd/e5s)

| Component | Test Type | Approach |
|-----------|-----------|----------|
| Command parsing | Unit | Test flag parsing |
| SPIFFE ID construction | Unit | Test string formatting |
| Config file validation | Unit | Test validation logic |
| Kubernetes discovery | Integration | Real/mocked K8s API |
| Deployment workflow | Manual | e5s deploy commands |

### SPIRE Adapter (internal/spire)

| Component | Test Type | Approach |
|-----------|-----------|----------|
| Workload API client | Integration | Real SPIRE containers |
| X.509 SVID handling | Unit | Mock SPIRE responses |
| Certificate rotation | Integration | Real SPIRE containers |
| Error handling | Unit | Inject errors |

## Running Tests

### Quick Tests (Unit Only)

```bash
# Run all unit tests (skip integration)
go test -short ./...

# With coverage
go test -short -coverprofile=coverage.out ./...
go tool cover -html=coverage.out
```

### Full Tests (Unit + Integration)

```bash
# Run all tests including containers
go test -v ./...

# Run specific integration test
go test -v -run TestE5SWithContainers

# With race detector
go test -race ./...
```

### CI/CD Tests

```bash
# Run in CI environment
go test -v -race -coverprofile=coverage.out ./...

# Upload coverage
go tool cover -func=coverage.out
```

## Writing New Tests

### Unit Test Template

```go
func TestMyFeature(t *testing.T) {
    // Setup
    input := setupTestInput()

    // Execute
    result, err := MyFeature(input)

    // Assert
    if err != nil {
        t.Fatalf("Unexpected error: %v", err)
    }

    if result != expected {
        t.Errorf("Expected %v, got %v", expected, result)
    }
}
```

### Integration Test Template

```go
func TestMyFeatureWithSPIRE(t *testing.T) {
    if testing.Short() {
        t.Skip("Skipping integration test in short mode")
    }

    // Setup SPIRE
    spire, cleanup := testhelpers.SetupSPIREContainers(t)
    defer cleanup()

    // Create context with timeout
    ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
    defer cancel()

    // Test with real SPIRE
    // ...
}
```

## Debugging Tests

### Enable Verbose Logging

```bash
# See all test output
go test -v ./...

# See logs from specific test
go test -v -run TestE5SWithContainers
```

### Inspect Container Logs

```bash
# List running containers during test
docker ps

# View container logs
docker logs <container-id>

# Keep containers after test failure (modify cleanup)
```

### Debug SPIRE Issues

```go
// In your test, don't call cleanup immediately
spire, cleanup := testhelpers.SetupSPIREContainers(t)
// defer cleanup()  // Comment this out

// Inspect socket
t.Logf("Socket path: %s", spire.SocketPath)
t.Logf("Trust domain: %s", spire.TrustDomain)

// Add breakpoint here to inspect running containers
time.Sleep(5 * time.Minute)

// Cleanup manually when done
cleanup()
```

## Common Issues

### Docker Not Available

**Error:** `Cannot connect to Docker daemon`

**Solution:**
```bash
# Start Docker daemon
sudo systemctl start docker

# Or skip container tests
go test -short ./...
```

### Port Already in Use

**Error:** `bind: address already in use`

**Solution:**
```bash
# Find process using port
lsof -i :18443

# Kill process
kill -9 <pid>

# Or use random port in test
```

### Container Pull Timeout

**Error:** `Failed to pull image: context deadline exceeded`

**Solution:**
```bash
# Pre-pull images
docker pull ghcr.io/spiffe/spire-server:1.11
docker pull ghcr.io/spiffe/spire-agent:1.11

# Increase timeout in test
wait.ForLog("...").WithStartupTimeout(120 * time.Second)
```

## Best Practices

### DO

✅ Use `testing.Short()` to skip slow tests
✅ Use `t.Cleanup()` for guaranteed cleanup
✅ Use `context.WithTimeout()` for explicit timeouts
✅ Use table-driven tests for multiple cases
✅ Test error paths, not just happy paths
✅ Use meaningful test names (TestX_WhenY_ThenZ)

### DON'T

❌ Don't use `time.Sleep()` for synchronization (use wait strategies)
❌ Don't leave containers running after tests
❌ Don't test implementation details
❌ Don't write flaky tests (use deterministic inputs)
❌ Don't ignore errors in tests

## References

- Testcontainers Go: https://golang.testcontainers.org/
- SPIRE Testing: https://spiffe.io/docs/latest/spire/developing/
- Go Testing: https://go.dev/doc/tutorial/add-a-test
- e5s CLI: `e5s deploy --help`
