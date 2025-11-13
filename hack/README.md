# hack/

This directory contains legacy shell scripts for manual testing and development workflows.

## Recommendation

**For automated testing, prefer:**
- **Testcontainers** (in-process SPIRE containers via `internal/testhelpers/containers.go`)
- **e5s CLI tool** (production deployment via `e5s deploy` commands)

These approaches provide better:
- Reliability (explicit timeouts, health checks)
- Debuggability (clear stack traces, logs in Go)
- Portability (same behavior in CI and local)
- Integration (works with `go test`)

## Scripts Overview

### Development Scripts

- **`install-tools-ubuntu-24.04.sh`** - Install development dependencies on Ubuntu
- **`env-versions.sh`** - Display versions of tools in environment
- **`fix-broken-links.sh`** - Automatically fix common broken link patterns

### Example/Demo Scripts

- **`run-example-server.sh`** - Start example server
- **`run-example-client.sh`** - Start example client

### Integration Testing Scripts

- **`test-prerelease.sh`** - Full integration test with Minikube + SPIRE + Helm
- **`rebuild-and-test.sh`** - Quick rebuild and test cycle
- **`cleanup-prerelease.sh`** - Clean up test resources
- **`run-ci-locally.sh`** - Run CI checks locally

**Note:** These scripts are for quick manual testing. For automated testing and production deployments, use the approaches recommended above (Testcontainers or e5s CLI).

## Migration Path

### Old Approach (Shell Scripts)

```bash
# Start minikube
minikube start

# Install SPIRE via Helm
helm install spire-crds...
helm install spire...

# Deploy app
kubectl apply -f deployment.yaml

# Wait for ready
sleep 60

# Test (manual)
kubectl logs...
```

**Problems:**
- Platform-dependent (bash, kubectl, helm, minikube)
- Hard to debug (buried in shell output)
- Unclear timeouts (arbitrary sleeps)
- Race conditions (server not ready)

### New Approach (Testcontainers)

```go
func TestE5SIntegration(t *testing.T) {
    spire, cleanup := testhelpers.SetupSPIREContainers(t)
    defer cleanup()

    // Test with real SPIRE mTLS
    // Explicit timeouts via context.WithTimeout
    // Clear stack traces on failure
}
```

**Benefits:**
- Cross-platform (only needs Docker)
- Debuggable (Go stack traces)
- Explicit timeouts (context)
- Reliable startup (wait strategies)

### New Approach (e5s CLI)

```bash
# Create test cluster
e5s deploy cluster create --wait 60s

# Install SPIRE
e5s deploy spire install --trust-domain demo.e5s.io

# Deploy app
e5s deploy app install

# Run integration tests
e5s deploy test run

# Clean up
e5s deploy cluster delete
```

**Benefits:**
- Single binary (no shell dependencies)
- Clear error messages
- Progress indicators
- Automatic cleanup

## When to Use Scripts

Shell scripts in this directory are useful for:

1. **Manual exploration** - Quick one-off testing during development
2. **CI pipeline setup** - Installing tools on CI runners
3. **Documentation** - Examples of manual setup procedures

Do NOT use for:
- ❌ Automated integration tests (use testcontainers)
- ❌ Production deployment (use e5s CLI)
- ❌ CI/CD workflows (use Go tests or e5s CLI)

## Example: Converting a Test

### Before (Shell Script)

```bash
#!/bin/bash
set -e

# Start cluster
minikube start
sleep 10

# Install SPIRE
helm install spire-crds...
sleep 30

# Deploy app
kubectl apply -f app.yaml
sleep 20

# Test
kubectl logs pod/app-server | grep "Hello"
if [ $? -ne 0 ]; then
    echo "Test failed"
    exit 1
fi
```

### After (Testcontainer)

```go
func TestAppWithSPIRE(t *testing.T) {
    spire, cleanup := testhelpers.SetupSPIREContainers(t)
    defer cleanup()

    // Start app with real SPIRE
    shutdown, err := e5s.Start("/path/to/config.yaml", handler)
    if err != nil {
        t.Fatal(err)
    }
    defer shutdown()

    // Make request
    resp, err := client.Get("https://localhost:8443/hello")
    if err != nil {
        t.Fatalf("Request failed: %v", err)
    }

    // Assert response
    if resp.StatusCode != 200 {
        t.Errorf("Expected 200, got %d", resp.StatusCode)
    }
}
```

## Testing Strategies

| Approach | Use Case | Speed | Reliability | CI-Friendly |
|----------|----------|-------|-------------|-------------|
| Unit tests | Library functions | ⚡ Very fast | ✅ High | ✅ Yes |
| Testhelpers (binaries) | Quick SPIRE tests | ⚡ Fast | ⚠️ Needs binaries | ⚠️ Requires setup |
| Testcontainers | Full integration | 🐢 Moderate | ✅ High | ✅ Yes |
| e5s CLI | Manual testing | 🐢 Moderate | ✅ High | ✅ Yes |
| Shell scripts | Ad-hoc tasks | 🐢 Slow | ❌ Low | ❌ No |

## References

- Testcontainers documentation: https://golang.testcontainers.org/
- e5s CLI documentation: `e5s deploy --help`
- SPIRE documentation: https://spiffe.io/docs/latest/spire/
