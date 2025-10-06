# Testing Strategy

This document explains the different testing layers and what each one verifies.

## TL;DR

```bash
# Unit tests (fast, in-memory, no SPIRE needed)
make test              # or: go test ./...

# Full verification (includes coverage, race detection)
make verify

# Integration tests (requires live SPIRE in Minikube)
make minikube-up
make register-test-workload    # Register test pod in SPIRE
make test-integration          # All 8 tests pass ✅
```

## Testing Layers

### 1. Unit Tests (In-Memory) ✅

**Command**: `go test ./...` or `make test`

**What it tests**:
- Domain layer business logic
- In-memory adapter implementations
- Application layer with mock backends
- No external dependencies

**What it does NOT test**:
- ❌ SPIRE production adapters
- ❌ Live SPIRE connectivity
- ❌ Actual SVID fetching from SPIRE

**Output**:
```
ok    github.com/pocket/hexagon/spire/internal/domain           (cached)
ok    github.com/pocket/hexagon/spire/internal/adapters/outbound/inmemory  1.001s
?     github.com/pocket/hexagon/spire/internal/adapters/outbound/spire  [no test files]
```

Note the `[no test files]` for the SPIRE adapter - this is expected! Unit tests use in-memory implementations.

**Coverage**: ~45.8% (domain + in-memory adapters)

---

### 2. Integration Tests (Live SPIRE) 🚀

**Command**:
```bash
make minikube-up              # Start SPIRE
make register-test-workload   # Register test pod
make test-integration         # Run tests
```

**What it tests**:
- ✅ **Actual SPIRE production adapters**
- ✅ Live connectivity to SPIRE Agent
- ✅ Real X.509/JWT SVID fetching
- ✅ Trust bundle retrieval
- ✅ Workload attestation
- ✅ Token validation

**Requirements**:
- SPIRE running in Minikube (`make minikube-up`)
- Test workload registered (`make register-test-workload`)

**Test file**: `internal/adapters/outbound/spire/integration_test.go`

**Tests included** (all passing ✅):
- `TestSPIREClientConnection` - Basic connectivity
- `TestFetchX509SVID` - Fetch X.509 identity document
- `TestFetchX509Bundle` - Fetch CA certificates
- `TestFetchJWTSVID` - Fetch JWT tokens
- `TestValidateJWTSVID` - Validate JWT tokens
- `TestAttestation` - Workload attestation
- `TestSPIREClientReconnect` - Connection resilience
- `TestSPIREClientTimeout` - Timeout handling

**Actual output**:
```bash
$ make test-integration

=== RUN   TestSPIREClientConnection
--- PASS: TestSPIREClientConnection (0.00s)
=== RUN   TestFetchX509SVID
    integration_test.go:69: Fetched SVID for identity: spiffe://example.org/test/integration-test
    integration_test.go:70: Certificate expires: 2025-10-06T21:48:38Z
--- PASS: TestFetchX509SVID (0.01s)
=== RUN   TestFetchX509Bundle
    integration_test.go:90: CA 1: example.org (expires: 2025-10-07T18:40:19Z)
--- PASS: TestFetchX509Bundle (0.00s)
=== RUN   TestFetchJWTSVID
    integration_test.go:109: Fetched JWT SVID (length: 658 bytes)
--- PASS: TestFetchJWTSVID (0.00s)
=== RUN   TestValidateJWTSVID
--- PASS: TestValidateJWTSVID (0.01s)
=== RUN   TestAttestation
    integration_test.go:154: Attestation selectors for PID 32351:
    integration_test.go:156:   - workload:spiffe_id:spiffe://example.org/test/integration-test
--- PASS: TestAttestation (0.00s)
=== RUN   TestSPIREClientReconnect
--- PASS: TestSPIREClientReconnect (0.00s)
=== RUN   TestSPIREClientTimeout
--- PASS: TestSPIREClientTimeout (0.00s)
PASS
ok  	github.com/pocket/hexagon/spire/internal/adapters/outbound/spire	0.033s

✅ Integration tests passed!
```

---

### 3. Comprehensive Verification

**Command**: `make verify`

**What it does**:
- Builds production and dev binaries
- Verifies binary separation (prod excludes dev code)
- Runs unit tests with coverage
- Runs race detector
- Checks code formatting
- Runs go vet
- Verifies dependencies
- Checks file structure

**Output**: See [VERIFICATION.md](docs/VERIFICATION.md) for details

---

## Test Matrix

| Test Type | Command | SPIRE Required? | Tests What | Speed | Coverage |
|-----------|---------|-----------------|------------|-------|----------|
| **Unit** | `go test ./...` | No | In-memory adapters | Fast (~2s) | 45.8% |
| **Integration** | `make test-integration` | Yes | SPIRE adapters | Medium (~0.5s) | SPIRE adapters |
| **Verification** | `make verify` | No | Everything + quality | Slow (~30s) | Full |

---

## When to Run Each Test

### During Development
```bash
# Quick feedback loop
go test ./internal/domain/...           # Test specific package
go test -run TestSpecificTest ./...     # Test specific function
```

### Before Committing
```bash
make verify                              # Full verification
```

### Before Deploying
```bash
make verify                              # Verify everything
make minikube-up                         # Start SPIRE
make test-integration                    # Test against live SPIRE
```

### In CI/CD
```bash
# Unit tests (always)
go test -race -cover ./...

# Integration tests (if SPIRE available)
if [ "$SPIRE_AVAILABLE" = "true" ]; then
  make test-integration
fi
```

---

## Understanding Test Output

### Unit Tests (In-Memory)
```
?     github.com/pocket/hexagon/spire/internal/adapters/outbound/spire  [no test files]
```
**This is EXPECTED!** The SPIRE adapter has no unit tests because it requires live SPIRE.

### Integration Tests (Live SPIRE)
```
--- PASS: TestFetchX509SVID (0.12s)
    integration_test.go:68: Fetched SVID for identity: spiffe://example.org/agent
```
**This confirms** the SPIRE adapter works with real SPIRE infrastructure!

---

## Troubleshooting

### "no test files" for SPIRE adapter
**This is normal** for unit tests. The SPIRE adapter only has integration tests (with `-tags=integration`).

### Integration tests fail with "connection refused"
```bash
# Ensure SPIRE is running
kubectl get pods -n spire-system

# Check agent socket exists
kubectl exec -n spire-system spire-agent-xxx -- ls -la /tmp/spire-agent/public/api.sock
```

### "Failed to fetch X.509 SVID"
```bash
# Check agent logs
kubectl logs -n spire-system spire-agent-xxx

# Verify workload registration
kubectl exec -n spire-system spire-server-0 -- \
  spire-server entry show
```

---

## Adding New Tests

### Unit Test (In-Memory)
```go
// internal/domain/example_test.go
func TestExample(t *testing.T) {
    // Test domain logic with no external dependencies
}
```

### Integration Test (Live SPIRE)
```go
//go:build integration

// internal/adapters/outbound/spire/example_test.go
func TestExample(t *testing.T) {
    client, err := spire.NewSPIREClient(ctx, config)
    require.NoError(t, err)
    // Test against real SPIRE
}
```

---

## Summary

- **`go test ./...`** = Fast unit tests with in-memory adapters (NO live SPIRE)
- **`make test-integration`** = Integration tests with live SPIRE (YES live SPIRE)
- **`make verify`** = Comprehensive verification (builds, tests, quality checks)

Always run `make verify` before committing!
Run `make test-integration` before deploying!
