# Integration Testing with SPIRE

This document explains how integration tests work with real SPIRE infrastructure in GitHub CI.

## Overview

The e5s library includes two types of tests:

1. **Unit Tests** (`*_test.go` without build tags) - Fast, run without external dependencies
2. **Integration Tests** (`*_test.go` with `//go:build integration`) - Require running SPIRE server and agent

Integration tests verify:
- SPIFFE identity issuance and fetching
- Certificate rotation mechanisms
- mTLS server/client connectivity
- Trust bundle management
- Real-world SPIRE integration scenarios

## Running Integration Tests in CI

Integration tests run automatically in GitHub Actions on every push and pull request via the `.github/workflows/integration.yml` workflow.

### How CI Works

The GitHub Actions workflow:

1. **Sets up Go** environment (version from go.mod)
2. **Downloads SPIRE binaries** from GitHub releases
3. **Sets environment variables** (`SPIRE_SERVER` and `SPIRE_AGENT` paths)
4. **Runs integration tests** with `go test -tags=integration`
5. **Uploads coverage** to Codecov

The test helpers in the codebase automatically start and stop SPIRE server/agent processes using the binaries from the environment variables.

### Workflow Configuration

```yaml
# .github/workflows/integration.yml
- name: Download and setup SPIRE binaries
  run: |
    wget https://github.com/spiffe/spire/releases/download/v1.13.0/spire-1.13.0-linux-amd64-musl.tar.gz
    tar xzf spire-1.13.0-linux-amd64-musl.tar.gz
    echo "SPIRE_SERVER=$PWD/spire-1.13.0/bin/spire-server" >> $GITHUB_ENV
    echo "SPIRE_AGENT=$PWD/spire-1.13.0/bin/spire-agent" >> $GITHUB_ENV

- name: Run Integration Tests
  run: go test -v -tags=integration -timeout=5m -coverprofile=coverage-integration.txt ./...
```

## Test Structure

### Integration Test Files

```
spire/integration_test.go              # SPIRE identity source tests
spiffehttp/integration_test.go         # mTLS TLS config tests
integration_test.go                    # High-level API tests
```

### Build Tags

All integration tests use the `integration` build tag:

```go
//go:build integration
// +build integration

package spire_test

import (
    "testing"
    // ...
)

func TestIntegration_NewIdentitySource_RealSPIRE(t *testing.T) {
    // Test code that requires real SPIRE...
}
```

This ensures:
- Integration tests don't run during normal `go test ./...`
- They only run when explicitly requested with `-tags=integration`
- Fast feedback loop for unit tests, comprehensive validation via integration tests

### Test Setup

Most integration tests follow this structure:

```go
func TestIntegration_Feature(t *testing.T) {
    // 1. Get SPIRE socket from environment
    socket := os.Getenv("SPIFFE_ENDPOINT_SOCKET")
    if socket == "" {
        socket = "unix:///tmp/spire-agent/public/api.sock"
    }

    // 2. Create identity source
    ctx := context.Background()
    source, err := spire.NewIdentitySource(ctx, spire.Config{
        WorkloadSocket: socket,
    })
    if err != nil {
        t.Fatalf("Failed to create identity source: %v", err)
    }
    defer source.Close()

    // 3. Test your feature
    svid, err := source.X509Source().GetX509SVID()
    require.NoError(t, err)
    require.NotNil(t, svid)
}
```

### Skipping Tests

Skip tests when infrastructure isn't available:

```go
func TestIntegration_Feature(t *testing.T) {
    if os.Getenv("SKIP_INTEGRATION_TESTS") != "" {
        t.Skip("SKIP_INTEGRATION_TESTS is set")
    }

    // Or check for SPIRE availability
    if _, err := os.Stat("/tmp/spire-agent/public/api.sock"); os.IsNotExist(err) {
        t.Skip("SPIRE agent socket not found")
    }

    // Test code...
}
```

### Test Isolation

Each test should be independent:

```go
func TestIntegration_Feature(t *testing.T) {
    // Create fresh identity source
    source, err := spire.NewIdentitySource(...)
    require.NoError(t, err)
    defer source.Close()  // Always clean up

    // Test doesn't depend on other tests
    // Test doesn't modify shared state
}
```

## Troubleshooting CI Failures

### Tests Hang or Timeout

**Check:**
- Test timeout is sufficient (currently 5m in workflow)
- SPIRE binaries downloaded correctly
- Test helpers can find SPIRE_SERVER and SPIRE_AGENT env vars

**Solutions:**
- Increase timeout in `.github/workflows/integration.yml`
- Check GitHub Actions logs for SPIRE startup errors

### Connection Refused

**Check:**
- SPIRE agent socket path is correct
- Test helpers successfully started SPIRE processes
- Timing issues (test started before SPIRE ready)

**Solutions:**
```go
// Add startup wait in test helpers
time.Sleep(2 * time.Second)  // Or better: poll for readiness
```

### Certificate Expiry Errors

**Check:**
- SPIRE configuration has appropriate TTLs
- System clock is correct

## Best Practices

### 1. Use Build Tags

Always use `//go:build integration` to separate integration tests from unit tests.

### 2. Test Real Scenarios

Integration tests should verify real-world usage:
- Certificate rotation
- Trust bundle updates
- mTLS handshakes
- Multi-workload scenarios

### 3. Keep Tests Fast

- Use `testing.Short()` for expensive tests
- Run in parallel when possible (`t.Parallel()`)
- Clean up resources promptly

### 4. Make Tests Deterministic

- Don't rely on specific certificate values
- Use retries for timing-sensitive operations
- Avoid hardcoded sleeps (poll for readiness instead)

### 5. Document Requirements

```go
// TestIntegration_Feature tests X with a real SPIRE agent.
//
// Requirements:
//   - SPIRE agent running and healthy
//   - SPIFFE_ENDPOINT_SOCKET env var set
//   - Workload registered with selector unix:uid:$(id -u)
func TestIntegration_Feature(t *testing.T) {
    // ...
}
```

## Coverage

Integration test coverage is tracked separately from unit test coverage:

```bash
# Unit test coverage (local)
go test -coverprofile=coverage-unit.txt ./...

# Integration test coverage (CI)
go test -tags=integration -coverprofile=coverage-integration.txt ./...
```

CI automatically uploads integration coverage reports to Codecov with the `integration` flag.
