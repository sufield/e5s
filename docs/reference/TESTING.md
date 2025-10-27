---
type: reference
audience: intermediate
---

# Testing Strategy

This document explains the different testing layers and what each one verifies.

## TL;DR

```bash
# Unit tests (fast, no SPIRE needed)
make test              # or: go test ./...

# Property-based tests (verify algebraic properties, 10K cases each)
go test -run Properties ./...

# Full verification (includes coverage, race detection, PBT)
make verify

# Integration tests (requires live SPIRE in Minikube)
make minikube-up
make register-test-workload    # Register test pod in SPIRE
make test-integration          # All 8 tests pass ✅
```

## Testing Layers

### 1. Unit Tests ✅

**Command**: `go test ./...` or `make test`

**What it tests**:
- Domain layer business logic (identity credentials, trust domains, workload information)
- Configuration parsing and validation
- HTTP middleware and handlers
- Certificate management logic
- No external dependencies (no SPIRE infrastructure required)

**What it does NOT test**:
- ❌ Live SPIRE connectivity
- ❌ Actual SVID fetching from SPIRE Agent
- ❌ Real certificate issuance

**Example Output**:
```
ok    github.com/pocket/hexagon/spire/internal/domain           0.287s
ok    github.com/pocket/hexagon/spire/internal/config           0.156s
ok    github.com/pocket/hexagon/spire/internal/adapters/inbound/httpapi  0.234s
```

**Coverage**: Tests verify domain logic, configuration handling, and HTTP layer without requiring SPIRE infrastructure.

---

### 2. Integration Tests (Live SPIRE)

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

### 3. Property-Based Tests (PBT)

**Command**: `go test -v -run Properties ./...`

**What it tests**:
- ✅ **Algebraic properties** of domain functions
- ✅ Mathematical invariants (idempotency, commutativity, etc.)
- ✅ Set-theoretic properties (uniqueness, ordering)
- ✅ Parse/format consistency
- ✅ Automated testing with thousands of generated inputs

**Requirements**:
- None - runs as part of unit tests
- Uses Go's built-in `testing/quick` package
- Configurable via `PBT_MAX_COUNT` env var (default: 10,000 cases)

**Test files**:
- `internal/domain/identity_credential_pbt_test.go` - Path normalization properties
- `internal/config/mtls_env_pbt_test.go` - Configuration parsing properties
- `internal/debug/config_pbt_test.go` - Debug config properties

**Properties tested**:

#### normalizePath() - 6 Properties
- **Idempotency**: `normalize(normalize(p)) == normalize(p)`
- **Canonical Form**: Starts with "/", no trailing slash (except root)
- **Length Bound**: Only adds leading slash when needed
- **No Consecutive Slashes**: Result contains no "//"
- **No Whitespace**: Result contains no whitespace
- **No Traversal**: Result has no "." or ".." segments

#### splitCleanDedup() - 5 Properties
- **No Duplicates**: Result contains unique elements only
- **Idempotency**: Processing twice produces same result
- **Subset Preservation**: No elements added, only removed
- **No Invalid Elements**: No empty or whitespace-only strings
- **Order Preservation**: First occurrence order maintained

#### parseDurationInto() - 3 Properties
- **Roundtrip**: `parse(format(d))` produces equivalent duration
- **Parse Equivalence**: Deterministic parsing (same input → same output)
- **Non-Negative**: Positive duration strings → non-negative results

**Example output**:
```bash
$ go test -v -run TestNormalizePath_Properties ./internal/domain

=== RUN   TestNormalizePath_Properties
=== RUN   TestNormalizePath_Properties/idempotency
--- PASS: TestNormalizePath_Properties/idempotency (0.05s)
=== RUN   TestNormalizePath_Properties/canonical_form
--- PASS: TestNormalizePath_Properties/canonical_form (0.04s)
=== RUN   TestNormalizePath_Properties/exact_length
--- PASS: TestNormalizePath_Properties/exact_length (0.04s)
=== RUN   TestNormalizePath_Properties/no_consecutive_slashes
--- PASS: TestNormalizePath_Properties/no_consecutive_slashes (0.04s)
=== RUN   TestNormalizePath_Properties/no_whitespace
--- PASS: TestNormalizePath_Properties/no_whitespace (0.05s)
=== RUN   TestNormalizePath_Properties/no_traversal_segments
--- PASS: TestNormalizePath_Properties/no_traversal_segments (0.05s)
PASS
ok      github.com/pocket/hexagon/spire/internal/domain    0.287s
```

**Adjusting test count**:
```bash
# Fast local runs (100 cases per property)
PBT_MAX_COUNT=100 go test -v -run Properties ./...

# High confidence (100,000 cases per property)
PBT_MAX_COUNT=100000 go test -v -run Properties ./...
```

**Why PBT?**
- Complements fuzz testing by verifying mathematical properties vs crash safety
- Finds edge cases through structured generation (not just random mutation)
- Provides minimal counterexamples through automatic shrinking
- Documents invariants as executable tests

**See also**: [`docs/reference/pbt.md`](pbt.md) for complete PBT guide and methodology.

---

### 4. Comprehensive Verification

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
| **Unit** | `go test ./...` | No | Domain logic, config, HTTP | Fast (~2s) | Core logic |
| **Property-Based** | `go test -run Properties ./...` | No | Algebraic properties | Fast (~0.3s) | Domain invariants |
| **Integration** | `make test-integration` | Yes | Live SPIRE connectivity | Medium (~0.5s) | SPIRE integration |
| **Verification** | `make verify` | No | Everything + quality checks | Slow (~30s) | Full |

---

## When to Run Each Test

### During Development
```bash
# Quick feedback loop
go test ./internal/domain/...           # Test specific package
go test -run TestSpecificTest ./...     # Test specific function
go test -run Properties ./...           # Run property-based tests
```

### Before Committing
```bash
make verify                              # Full verification (includes PBT)
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

### Unit Tests
```
ok    github.com/pocket/hexagon/spire/internal/domain    0.287s
ok    github.com/pocket/hexagon/spire/internal/config    0.156s
```
Unit tests verify domain logic, configuration, and other components without external dependencies.

### Property-Based Tests
```
=== RUN   TestNormalizePath_Properties
=== RUN   TestNormalizePath_Properties/idempotency
--- PASS: TestNormalizePath_Properties/idempotency (0.05s)
```
Each property is tested with thousands of generated inputs (default: 10,000 cases).

### Integration Tests (Live SPIRE)
```
=== RUN   TestFetchX509SVID
    integration_test.go:69: Fetched SVID for identity: spiffe://example.org/test/integration-test
    integration_test.go:70: Certificate expires: 2025-10-06T21:48:38Z
--- PASS: TestFetchX509SVID (0.01s)
```
Integration tests confirm connectivity to real SPIRE infrastructure and certificate fetching.

---

## Troubleshooting

See [`docs/how-to-guides/TROUBLESHOOTING.md`](../how-to-guides/TROUBLESHOOTING.md) for detailed troubleshooting, including:

- Connection refused errors
- SPIRE socket access issues
- Registration and permission problems
- Integration testing specific issues

---

## Adding New Tests

### Unit Test
```go
// internal/domain/identity_credential_test.go
func TestNewIdentityCredentialFromComponents(t *testing.T) {
    // Test domain logic with no external dependencies
    trustDomain := NewTrustDomainFromName("example.org")
    ic := NewIdentityCredentialFromComponents(trustDomain, "/webapp")

    assert.NotNil(t, ic)
    assert.Equal(t, "spiffe://example.org/webapp", ic.String())
}
```

### Property-Based Test
```go
// internal/domain/identity_credential_pbt_test.go
func TestNormalizePath_Properties(t *testing.T) {
    t.Run("idempotency", func(t *testing.T) {
        // Verify normalize(normalize(p)) == normalize(p)
        property := func(p string) bool {
            normalized := normalizePath(p)
            renormalized := normalizePath(normalized)
            return normalized == renormalized
        }

        if err := quick.Check(property, &quick.Config{MaxCount: 10000}); err != nil {
            t.Error(err)
        }
    })
}
```

### Integration Test (Live SPIRE)
```go
// internal/adapters/outbound/spire/integration_test.go
func TestFetchX509SVID(t *testing.T) {
    // Test against real SPIRE infrastructure
    // Note: No build tags needed - integration tests are in separate package
    client, err := NewSPIREClient(ctx, config)
    require.NoError(t, err)

    svid, err := client.FetchX509SVID(ctx)
    require.NoError(t, err)
    assert.NotNil(t, svid)
}
```

---

## Summary

- **`go test ./...`** = Fast unit tests for domain logic and configuration (NO SPIRE infrastructure needed)
- **`go test -run Properties ./...`** = Property-based tests verifying algebraic invariants (10K cases each)
- **`make test-integration`** = Integration tests with live SPIRE (YES live SPIRE infrastructure required)
- **`make verify`** = Comprehensive verification (builds, tests, quality checks, PBT)

**Best Practices:**
- Run `make verify` before committing (includes unit + PBT + quality checks)
- Run `make test-integration` before deploying (tests against real SPIRE)
- Run PBT tests when modifying domain logic or configuration parsing
- Unit tests should be fast and have no external dependencies

**Test Coverage:**
- **Unit tests**: Example-based verification of domain logic, configuration, HTTP handlers
- **Property-based tests**: Mathematical invariants (14 properties, 140,000 total test cases)
- **Integration tests**: Real SPIRE connectivity and certificate fetching
- **Fuzz tests**: Crash safety and edge cases (path normalization, etc.)
