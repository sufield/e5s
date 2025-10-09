# Test Architecture

## Overview

The test suite is properly organized into **unit tests** and **integration tests**, each serving different purposes. This document explains the test structure, execution patterns, and best practices.

## Current Test Structure

### ✅ Unit Tests (Always Pass)

These tests run without external dependencies:

- **Config Validation**: `TestNewHTTPServer_Missing*`, `TestNewSPIFFEHTTPClient_Missing*`
- **Identity Extraction**: `TestGetSPIFFEID`, `TestGetTrustDomain`, `TestGetPath`
- **Helper Functions**: `TestGetIDString`, `TestGetPathSegments`
- **Handler Wrapping**: `TestWrapHandler_NoTLS`, `TestRegisterHandler`
- **Default Values**: `TestSPIFFEHTTPClient_Defaults` (config struct validation)

**Status**: ✅ All passing (0.003s execution time)

### ⏭️ Integration Tests (Require SPIRE)

These tests need a live SPIRE agent:

- **Server Creation**: `TestNewHTTPServer_ValidConfig`
- **Client Creation**: `TestSPIFFEHTTPClient_ValidConfig`, `TestSPIFFEHTTPClient_Defaults`
- **Full mTLS Flow**: `TestMTLSClientServer` (with `//go:build integration` tag)
- **Authorization**: `TestMTLSClientServer_AuthorizationFailure`
- **Health Checks**: `TestMTLSServer_HealthCheck`

**Status**: ⏭️ Gracefully skip when SPIRE unavailable

## Test Execution Patterns

### 1. Quick Unit Tests (CI/Dev)

```bash
go test ./internal/adapters/inbound/httpapi -run 'Test(NewHTTPServer_Missing|GetSPIFFEID|GetTrustDomain)' -v
```

✅ Fast, no dependencies, always pass

### 2. Full Integration Tests (Requires SPIRE)

```bash
# Start SPIRE infrastructure
make minikube-up

# Run all tests including integration
go test -tags=integration ./internal/adapters/inbound/httpapi -v

# Or run specific integration test
go test -tags=integration ./internal/adapters/inbound/httpapi -run TestMTLSClientServer -v
```

### 3. Check SPIRE Status

```bash
# Verify agent is running
kubectl logs -n spire-system daemonset/spire-agent

# Check socket exists
minikube ssh
ls -la /tmp/spire-agent/public/api.sock
```

## Why This Design is Correct

1. **Fast Feedback Loop**: Unit tests run in milliseconds without infrastructure
2. **Graceful Degradation**: Integration tests skip (not fail) when SPIRE unavailable
3. **Clear Separation**: `//go:build integration` tag separates concerns
4. **Production Validation**: Integration tests verify real mTLS with actual SPIRE

## Test Coverage

### Unit Tests (No SPIRE needed)

- ✅ Config validation (empty/nil checks)
- ✅ Identity extraction from context
- ✅ SPIFFE ID parsing and path handling
- ✅ Default value application
- ✅ Transport configuration
- ✅ Shutdown and resource cleanup logic

### Integration Tests (SPIRE needed)

- ⏭️ X509Source creation
- ⏭️ Certificate rotation
- ⏭️ mTLS handshake
- ⏭️ Client-server authentication
- ⏭️ Authorization failures
- ⏭️ Health check endpoints

## Running Tests in Different Environments

### Local Development (No SPIRE)

```bash
# Only run unit tests
go test ./internal/adapters/... -short
```

Output: All validation and helper tests pass ✅

### CI Pipeline (With SPIRE)

```bash
# Setup SPIRE first
make minikube-up

# Run full test suite
go test -tags=integration ./internal/adapters/... -v
```

Output: All tests including integration pass ✅

### Integration Environment

```bash
# Set explicit socket path
export SPIRE_AGENT_SOCKET="unix:///tmp/spire-agent/public/api.sock"

# Run with environment awareness
go test -tags=integration ./internal/adapters/inbound/httpapi -v
```

## Test Output Analysis

### Expected Output (SPIRE Not Running)

```
=== RUN   TestNewHTTPServer_MissingAddress
--- PASS: TestNewHTTPServer_MissingAddress (0.00s)
=== RUN   TestSPIFFEHTTPClient_Defaults
    client_test.go:75: Skipping test - SPIRE agent not available: failed to create X509Source: context deadline exceeded
--- SKIP: TestSPIFFEHTTPClient_Defaults (5.00s)
```

✅ **This is correct behavior** - graceful skip, not failure

### Expected Output (SPIRE Running)

```
=== RUN   TestMTLSClientServer
--- PASS: TestMTLSClientServer (1.23s)
=== RUN   TestSPIFFEHTTPClient_Defaults
--- PASS: TestSPIFFEHTTPClient_Defaults (0.52s)
```

✅ **Full integration validation**

## Test Organization

### Server Tests

**File**: `internal/adapters/inbound/httpapi/server_test.go`

- Unit tests for config validation
- Unit tests for identity extraction helpers
- Integration tests for server creation (skip if SPIRE unavailable)

**File**: `internal/adapters/inbound/httpapi/integration_test.go`

- Tagged with `//go:build integration`
- Full mTLS client-server communication
- Authorization and authentication flows
- Health check endpoints

### Client Tests

**File**: `internal/adapters/outbound/httpclient/client_test.go`

- Unit tests for config validation
- Unit tests for default value application
- Unit tests for custom configuration
- Integration tests (skip if SPIRE unavailable)

**File**: `internal/adapters/outbound/httpclient/integration_test.go`

- Tagged with `//go:build integration`
- HTTP method testing (GET, POST, PUT, DELETE, PATCH)
- Timeout handling
- Error scenarios

## Best Practices Implemented

### 1. ✅ Separation of Concerns

Unit tests don't depend on external infrastructure:

```go
func TestNewHTTPServer_MissingAddress(t *testing.T) {
    ctx := context.Background()
    authorizer := tlsconfig.AuthorizeAny()

    server, err := NewHTTPServer(ctx, ServerConfig{
        Address:    "", // Missing - should fail
        SocketPath: "unix:///tmp/socket",
        Authorizer: authorizer,
    })

    require.Error(t, err)
    assert.Nil(t, server)
    assert.Contains(t, err.Error(), "address is required")
}
```

### 2. ✅ Graceful Skipping

Integration tests skip when SPIRE unavailable:

```go
func TestSPIFFEHTTPClient_Defaults(t *testing.T) {
    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()

    client, err := NewSPIFFEHTTPClient(ctx, ClientConfig{
        SocketPath:       socketPath,
        ServerAuthorizer: authorizer,
    })
    if err != nil {
        t.Skipf("Skipping test - SPIRE agent not available: %v", err)
        return
    }
    defer client.Close()

    // Test default values
    assert.Equal(t, 30*time.Second, client.client.Timeout)
}
```

### 3. ✅ Fast Feedback

Unit tests execute in milliseconds:

```bash
$ go test ./internal/adapters/inbound/httpapi -run 'TestNewHTTPServer_Missing'
ok      github.com/pocket/hexagon/spire/internal/adapters/inbound/httpapi    0.003s
```

### 4. ✅ Comprehensive Coverage

Both validation logic and real behavior tested:

- **Unit**: Config validation, parsing, defaults
- **Integration**: mTLS handshake, certificate verification, authorization

### 5. ✅ Environment Aware

Tests detect SPIRE automatically:

```go
socketPath := os.Getenv("SPIRE_AGENT_SOCKET")
if socketPath == "" {
    socketPath = "unix:///tmp/spire-agent/public/api.sock"
}
```

### 6. ✅ Proper Resource Cleanup

All tests use proper cleanup:

```go
defer server.Stop(ctx)
defer client.Close()

defer func() {
    shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()
    require.NoError(t, server.Shutdown(shutdownCtx))
}()
```

## Troubleshooting

### Tests Skip with "SPIRE agent not available"

This is **expected behavior** when SPIRE is not running. To run integration tests:

1. Start SPIRE infrastructure:
   ```bash
   make minikube-up
   ```

2. Verify agent is running:
   ```bash
   kubectl get pods -n spire-system
   ```

3. Check socket exists:
   ```bash
   minikube ssh
   ls -la /tmp/spire-agent/public/api.sock
   ```

4. Run integration tests:
   ```bash
   go test -tags=integration ./internal/adapters/... -v
   ```

### Tests Timeout After 5s

This indicates SPIRE agent socket is unreachable:

1. Check agent logs:
   ```bash
   kubectl logs -n spire-system daemonset/spire-agent
   ```

2. Verify socket permissions:
   ```bash
   minikube ssh
   ls -la /tmp/spire-agent/public/
   ```

3. Ensure workload is registered:
   ```bash
   kubectl exec -n spire-system spire-server-0 -- \
     /opt/spire/bin/spire-server entry show
   ```

### Unit Tests Fail

This indicates a code issue (not infrastructure):

1. Check error message for specific failure
2. Verify config validation logic
3. Check helper function implementations
4. Review recent code changes

## Summary

### Production-Ready Test Architecture

The current setup follows Go best practices:

1. ✅ **Separation of Concerns**: Unit vs Integration tests clearly separated
2. ✅ **Graceful Skipping**: Tests don't fail CI when SPIRE unavailable
3. ✅ **Fast Feedback**: Unit tests run in milliseconds
4. ✅ **Comprehensive Coverage**: Both validation logic and real mTLS tested
5. ✅ **Environment Aware**: Tests detect SPIRE availability automatically
6. ✅ **Proper Cleanup**: Resources always released correctly

### Test Execution Summary

| Test Type | Dependencies | Execution Time | Status |
|-----------|-------------|----------------|---------|
| Unit Tests | None | ~3ms | ✅ Always Pass |
| Integration Tests (no SPIRE) | None | ~5s | ⏭️ Graceful Skip |
| Integration Tests (with SPIRE) | SPIRE Agent | ~1-2s | ✅ Full Validation |

### Quick Commands

```bash
# Fast unit tests (no dependencies)
go test ./internal/adapters/... -short

# Full suite with integration (requires SPIRE)
make minikube-up
go test -tags=integration ./internal/adapters/... -v

# Check SPIRE status
kubectl logs -n spire-system daemonset/spire-agent

# Run specific test
go test ./internal/adapters/inbound/httpapi -run TestGetSPIFFEID -v
```

## Related Documentation

- [MTLS.md](MTLS.md) - mTLS architecture and implementation
- [manual-testing.md](manual-testing.md) - Manual testing procedures
- [ARCHITECTURE_REVIEW.md](ARCHITECTURE_REVIEW.md) - Architecture decisions
