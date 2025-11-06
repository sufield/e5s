# Integration Testing with SPIRE

This document explains how to run end-to-end integration tests for the e5s library with real SPIRE infrastructure.

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

## Quick Start

### Local Testing with Docker

The easiest way to run integration tests locally is using Docker Compose:

```bash
# Start SPIRE infrastructure
docker-compose -f docker-compose.test.yml up -d

# Wait for SPIRE to be ready (about 10 seconds)
sleep 10

# Run integration tests
go test -tags=integration -v ./...

# Clean up
docker-compose -f docker-compose.test.yml down
```

### Running in CI

Integration tests run automatically in GitHub Actions on every push and pull request. See `.github/workflows/integration.yml` for the CI configuration.

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

## Running Tests

### All Integration Tests

```bash
go test -tags=integration -v ./...
```

### Specific Package

```bash
go test -tags=integration -v ./spire
```

### Single Test

```bash
go test -tags=integration -v ./spire -run TestIntegration_NewIdentitySource_RealSPIRE
```

### With Coverage

```bash
go test -tags=integration -v -coverprofile=coverage-integration.txt ./...
```

### Skip Long-Running Tests

```bash
go test -tags=integration -v -short ./...
```

## Local Setup Options

### Option 1: Docker Compose (Recommended)

**Pros:**
- Isolated, reproducible environment
- Automatic cleanup
- Matches CI environment

**Cons:**
- Requires Docker
- Slightly slower startup

```bash
docker-compose -f docker-compose.test.yml up -d
export SPIFFE_ENDPOINT_SOCKET=unix:///var/run/spire/agent.sock
go test -tags=integration -v ./...
docker-compose -f docker-compose.test.yml down
```

### Option 2: Local SPIRE Binaries

**Pros:**
- Faster iteration
- Direct access to SPIRE logs

**Cons:**
- Manual setup
- Requires SPIRE binaries in PATH

```bash
# Download SPIRE release
wget https://github.com/spiffe/spire/releases/download/v1.13.0/spire-1.13.0-linux-amd64-musl.tar.gz
tar xzf spire-1.13.0-linux-amd64-musl.tar.gz
cd spire-1.13.0

# Start server
bin/spire-server run -config conf/server/server.conf &

# Start agent
bin/spire-agent run -config conf/agent/agent.conf &

# Run tests
export SPIFFE_ENDPOINT_SOCKET=unix:///tmp/spire-agent/public/api.sock
go test -tags=integration -v ../e5s/...

# Cleanup
killall spire-server spire-agent
```

### Option 3: Kubernetes (Advanced)

For testing in a Kubernetes environment:

```bash
# Deploy SPIRE to kind cluster
kind create cluster
kubectl apply -f https://raw.githubusercontent.com/spiffe/spire/main/support/k8s/k8s-workload-registrar/mode-crd/config/spire.yaml

# Port-forward agent socket
kubectl port-forward -n spire spire-agent-xxxxx 8081:8081

# Run tests
export SPIFFE_ENDPOINT_SOCKET=unix:///tmp/spire-agent/public/api.sock
go test -tags=integration -v ./...
```

## CI/CD Integration

### GitHub Actions

The project includes a complete GitHub Actions workflow at `.github/workflows/integration.yml` that:

1. Starts SPIRE server as a service container
2. Downloads and starts SPIRE agent
3. Runs integration tests with coverage
4. Uploads coverage reports

**Running on Pull Requests:**
```yaml
on:
  pull_request:
    branches: [main]
```

**Manual Trigger:**
```yaml
on:
  workflow_dispatch:
```

## Test Patterns

### Test Setup

Most integration tests follow this pattern:

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

## Troubleshooting

### Tests Hang or Timeout

**Problem:** Tests hang waiting for SPIRE agent

**Solutions:**
```bash
# Verify agent socket exists
ls -la /tmp/spire-agent/public/api.sock

# Check agent logs
docker-compose -f docker-compose.test.yml logs spire-agent

# Increase test timeout
go test -tags=integration -timeout=10m -v ./...
```

### Connection Refused

**Problem:** `connection refused` when connecting to SPIRE

**Solutions:**
```bash
# Check SPIRE services are running
docker-compose -f docker-compose.test.yml ps

# Verify network connectivity
docker-compose -f docker-compose.test.yml exec spire-agent /opt/spire/bin/spire-agent healthcheck

# Check socket path
export SPIFFE_ENDPOINT_SOCKET=unix:///tmp/spire-agent/public/api.sock
```

### Certificate Expiry Errors

**Problem:** Tests fail with expired certificates

**Solutions:**
```bash
# Restart SPIRE to get fresh certificates
docker-compose -f docker-compose.test.yml restart

# Or use shorter certificate TTLs in SPIRE config for testing
```

### Tests Pass Locally but Fail in CI

**Problem:** Tests work locally but not in CI

**Check:**
- Socket path differences (Docker volume mounts)
- Timing issues (add retries/waits)
- Network connectivity (service containers vs localhost)
- Environment variables

**Solutions:**
```go
// Make socket path configurable
socket := os.Getenv("SPIFFE_ENDPOINT_SOCKET")
if socket == "" {
    socket = "unix:///tmp/spire-agent/public/api.sock"  // CI default
}

// Add startup wait
time.Sleep(2 * time.Second)  // Or better: poll for readiness
```

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
- Run in parallel when possible
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
# Unit test coverage
go test -coverprofile=coverage-unit.txt ./...

# Integration test coverage
go test -tags=integration -coverprofile=coverage-integration.txt ./...

# View combined coverage
go tool cover -html=coverage-integration.txt
```

CI automatically uploads both coverage reports to Codecov with different flags.

## Contributing

When adding new features:

1. Write unit tests first (fast feedback)
2. Add integration tests for E2E scenarios
3. Ensure CI passes before merging
4. Update this document if adding new test patterns

See [CONTRIBUTING.md](../CONTRIBUTING.md) for general contribution guidelines.
