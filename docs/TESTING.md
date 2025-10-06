# Testing Strategy

## Overview

This document outlines the testing strategy for the hexagonal SPIRE implementation, focusing on **unit tests** for core domain/application logic and **integration tests** for adapters at the early in-memory stage.

---

## Testing Philosophy

### Priorities at In-Memory Stage

1. **Unit Tests (80% coverage target)**: Pure domain and application logic with mocked ports
2. **Integration Tests**: Adapter + core with in-memory implementations as "production-like"
3. **Contract Tests**: Port interface satisfaction (build-time checks + runtime assertions)
4. **E2E Tests (Minimal)**: Defer heavy end-to-end testing until real SPIRE adapters

### What NOT to Test

- ❌ **Bootstrap/Control Plane**: `Bootstrap()` is infrastructure, not public API
- ❌ **Composition Root**: Treat as configuration/wiring, not business logic
- ❌ **In-Memory Fixtures**: Configuration data is not testable behavior

---

## Test Types by Layer

| Test Type          | Focus | What to Test | Implementation | Coverage Goal |
|--------------------|-------|--------------|----------------|---------------|
| **Unit Tests** | Core Domain/App | - Pure logic: Auth validation<br>- Error paths: Invalid/expired docs<br>- Domain entities: `IdentityNamespace.Equals`, `SelectorSet.Add` | Mock ports with testify/mock | 80%+ |
| **Integration Tests** | Adapters + Core | - Adapter-port contracts<br>- Flow: Attest → Match → Issue<br>- Public methods: `CLI.Run`, `Agent.FetchIdentityDocument` | Use in-memory impls | 60%+ |
| **Contract Tests** | Port Boundaries | - Interface satisfaction (build-time via `var _`)<br>- Mock impls verify core doesn't break | testify/mock for runtime checks | Build-time |
| **E2E Tests** | Full Wiring | - Happy path: Bootstrap → Auth<br>- Limit to 1-2 scenarios until real adapters | Test full application flow | Minimal |

---

## Current Test Coverage

### Unit Tests

#### Domain Layer (`internal/domain/`)

**Files**:
- `identity_namespace_test.go` - Tests for `IdentityNamespace` value object
- `selector_test.go` - Tests for `Selector` and `SelectorSet`

**Coverage**:
```
identity_namespace.go    - 100% (all methods tested)
selector.go             - 90%+ (core parsing and set operations)
```

**Test Cases**:
1. **`IdentityNamespace`**:
   - ✅ Create from components with various paths
   - ✅ Equality comparison (same/different trust domains/paths)
   - ✅ Trust domain membership check
   - ✅ String representation
   - ✅ Nil handling

2. **`Selector`**:
   - ✅ Parse from string (valid/invalid formats)
   - ✅ Type, Key, Value extraction
   - ✅ Equality comparison
   - ✅ Error cases (missing colons, empty fields)

3. **`SelectorSet`**:
   - ✅ Add selectors (uniqueness enforcement)
   - ✅ Contains check
   - ✅ Multiple selector handling
   - ✅ Duplicate prevention

#### Application Layer (`internal/app/`)

**Files**:
- `service_test.go` - Tests for `IdentityService` (core business logic)

**Coverage**:
```
service.go              - 100% (all validation paths covered)
```

**Test Cases**:
1. **`IdentityService.ExchangeMessage`**:
   - ✅ Success case with valid identities
   - ✅ Nil sender namespace
   - ✅ Nil receiver namespace
   - ✅ Expired sender document
   - ✅ Expired receiver document
   - ✅ Nil sender document
   - ✅ Nil receiver document
   - ✅ Empty content (allowed)
   - ✅ Table-driven tests for all scenarios

**Mocking**:
- `MockAgent` - Implements `ports.Agent` using testify/mock
- `MockRegistry` - Implements `ports.IdentityMapperRegistry` using testify/mock

---

## Integration Tests (TODO)

### Adapter Tests

**Target Files** (next phase):
- `internal/adapters/outbound/inmemory/agent_test.go`
- `internal/adapters/outbound/inmemory/registry_test.go`
- `internal/adapters/inbound/cli/cli_test.go`

**Test Scenarios**:
1. **`InMemoryAgent`**:
   - Seed registry, call `FetchIdentityDocument`
   - Assert returned `IdentityDocument.IsValid() == true`
   - Test attestation → matching → issuance flow

2. **`InMemoryRegistry`**:
   - Seed with mappers
   - Seal registry
   - Find by selectors (matching/non-matching)
   - Verify immutability after sealing

3. **`CLI`**:
   - Mock `Application`
   - Assert `Run()` calls `Service.ExchangeMessage` once
   - Verify output formatting

---

## Contract Tests (Already Implemented)

All port implementations have compile-time checks:

```go
// Domain layer
var _ ports.Service = (*IdentityService)(nil)

// Adapter layer
var _ ports.Agent = (*InMemoryAgent)(nil)
var _ ports.Server = (*InMemoryServer)(nil)
var _ ports.IdentityMapperRegistry = (*InMemoryRegistry)(nil)
var _ ports.AdapterFactory = (*InMemoryAdapterFactory)(nil)
var _ ports.CLI = (*CLI)(nil)
var _ ports.WorkloadAPIServer = (*Server)(nil)
var _ ports.WorkloadAPIClient = (*Client)(nil)
```

**Benefits**:
- ✅ Catch interface mismatches at build time
- ✅ Enforce hexagonal inversion of dependency
- ✅ No runtime overhead
- ✅ Refactor-safe (breaks build if port signature changes)

---

## Running Tests

### Run All Unit Tests

```bash
# Run with verbose output
go test -v ./internal/domain/... ./internal/app/...

# Check coverage
go test -cover ./internal/domain/... ./internal/app/...

# Generate coverage report
go test -coverprofile=coverage.out ./internal/domain/... ./internal/app/...
go tool cover -html=coverage.out -o coverage.html
```

### Run Specific Test

```bash
# Run domain tests only
go test -v ./internal/domain/...

# Run specific test function
go test -v ./internal/app -run TestIdentityService_ExchangeMessage_Success

# Run table-driven test subset
go test -v ./internal/domain -run TestParseSelector/valid
```

### Coverage Goals

```bash
# Aim for 80%+ on core/domain
go test -cover ./internal/domain/... | grep coverage

# Aim for 80%+ on app/services
go test -cover ./internal/app/... | grep coverage
```

---

## Testing Tools

### Dependencies

```bash
# Install testify for assertions and mocking
go get github.com/stretchr/testify
```

### Testify Features Used

1. **Assertions** (`assert`/`require`):
   ```go
   assert.Equal(t, expected, actual)
   require.NoError(t, err) // Fails fast if error
   assert.True(t, condition)
   assert.Contains(t, str, substring)
   ```

2. **Mocking** (`mock`):
   ```go
   type MockAgent struct {
       mock.Mock
   }

   func (m *MockAgent) GetIdentity(ctx context.Context) (*ports.Identity, error) {
       args := m.Called(ctx)
       return args.Get(0).(*ports.Identity), args.Error(1)
   }
   ```

3. **Table-Driven Tests**:
   ```go
   tests := []struct {
       name    string
       input   string
       wantErr bool
   }{
       {name: "valid", input: "unix:uid:1000", wantErr: false},
       {name: "invalid", input: "bad", wantErr: true},
   }

   for _, tt := range tests {
       t.Run(tt.name, func(t *testing.T) {
           // Test logic
       })
   }
   ```

---

## Test Organization

### File Naming Convention

```
internal/
├── domain/
│   ├── identity_namespace.go
│   ├── identity_namespace_test.go  ← Unit tests for IdentityNamespace
│   ├── selector.go
│   └── selector_test.go            ← Unit tests for Selector/SelectorSet
├── app/
│   ├── service.go
│   └── service_test.go             ← Unit tests for IdentityService
└── adapters/
    └── outbound/inmemory/
        ├── agent.go
        └── agent_test.go           ← Integration tests (TODO)
```

### Test Package Naming

- **Unit tests**: Use `package <pkg>_test` for black-box testing (tests external API only)
- **Integration tests**: Use `package <pkg>` for white-box testing (can access internal methods)

**Example**:
```go
// internal/domain/selector_test.go
package domain_test  // Black-box: tests public API only

import (
    "testing"
    "github.com/pocket/hexagon/spire/internal/domain"
)
```

---

## Test Patterns

### 1. Table-Driven Tests

**Use for**: Multiple scenarios with similar structure (selectors, identity namespaces)

```go
func TestParseSelector(t *testing.T) {
    tests := []struct {
        name      string
        input     string
        wantType  domain.SelectorType
        wantKey   string
        wantValue string
        wantErr   bool
    }{
        {
            name:      "valid unix uid selector",
            input:     "unix:uid:1000",
            wantType:  domain.SelectorType("unix"),
            wantKey:   "uid",
            wantValue: "1000",
            wantErr:   false,
        },
        // More test cases...
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            selector, err := domain.ParseSelectorFromString(tt.input)
            if tt.wantErr {
                assert.Error(t, err)
            } else {
                require.NoError(t, err)
                assert.Equal(t, tt.wantType, selector.Type())
            }
        })
    }
}
```

### 2. Mock-Based Unit Tests

**Use for**: Testing core logic with mocked ports

```go
func TestIdentityService_ExchangeMessage_Success(t *testing.T) {
    // Arrange
    mockAgent := new(MockAgent)
    mockRegistry := new(MockRegistry)
    service := app.NewIdentityService(mockAgent, mockRegistry)

    fromID := createValidIdentity(t, "spiffe://example.org/client", time.Now().Add(1*time.Hour))
    toID := createValidIdentity(t, "spiffe://example.org/server", time.Now().Add(1*time.Hour))

    // Act
    msg, err := service.ExchangeMessage(ctx, *fromID, *toID, "Hello")

    // Assert
    require.NoError(t, err)
    assert.Equal(t, "Hello", msg.Content)
    mockAgent.AssertExpectations(t)
}
```

### 3. Helper Functions

**Use for**: Reducing test boilerplate

```go
// Helper function to create valid identity for testing
func createValidIdentity(t *testing.T, spiffeID string, expiresAt time.Time) *ports.Identity {
    t.Helper() // Marks this as a helper for better error reporting
    // ...creation logic...
}

// Helper to parse selector or fail test
func mustParseSelector(t *testing.T, s string) *domain.Selector {
    t.Helper()
    sel, err := domain.ParseSelectorFromString(s)
    require.NoError(t, err)
    return sel
}
```

---

## Evolution Strategy

### Phase 1: In-Memory (Current)

- ✅ Unit tests for domain entities
- ✅ Unit tests for application services
- ⏳ Integration tests for in-memory adapters
- ⏳ Contract tests for port interfaces

### Phase 2: Real SPIRE Integration

- Integration tests comparing in-memory vs. real SPIRE
- E2E tests with full SPIRE deployment
- Performance/load tests
- Contract tests with go-spiffe SDK

### Phase 3: Production

- Smoke tests in CI/CD
- Canary deployment tests
- Chaos/failure injection tests
- Security/penetration tests

---

## Best Practices

### DO

- ✅ Test **public API behavior**, not internal implementation
- ✅ Use **table-driven tests** for multiple similar scenarios
- ✅ Mock **ports** in unit tests, use **real in-memory adapters** in integration tests
- ✅ Write **descriptive test names** (`TestX_Scenario_ExpectedBehavior`)
- ✅ Use **`t.Helper()`** in test utility functions
- ✅ Assert **specific error messages** when testing error paths
- ✅ Test **edge cases**: nil, empty, expired, invalid inputs

### DON'T

- ❌ Test **infrastructure code** (Bootstrap, composition root)
- ❌ Test **configuration loading** (fixtures are data, not behavior)
- ❌ Mock **domain entities** (use real instances)
- ❌ Write **brittle tests** tied to implementation details
- ❌ Skip **error cases** (they're as important as happy paths)
- ❌ Use **magic values** (define constants or use descriptive variables)
- ❌ Leave **failing tests** commented out

---

## Current Test Summary

### ✅ Implemented

#### Unit Tests (Existing)

| Package | File | Tests | Coverage |
|---------|------|-------|----------|
| `internal/domain` | `identity_namespace_test.go` | 6 test functions, 22 subtests | 100% |
| `internal/domain` | `selector_test.go` | 4 test functions, 15 subtests | 90%+ |
| `internal/app` | `service_test.go` | 10 test functions, 14 subtests | 100% |

**Subtotal**: 20 test functions, 51+ test cases

#### Invariant Tests (New)

| Package | File | Tests | Focus |
|---------|------|-------|-------|
| `internal/domain` | `trust_domain_invariants_test.go` | 6 test functions | Name non-empty, Equals properties, nil safety |
| `internal/domain` | `identity_namespace_invariants_test.go` | 8 test functions | TrustDomain non-nil, path defaults, URI format, Equals properties |
| `internal/domain` | `selector_invariants_test.go` | 11 test functions | Key/value non-empty, formatted matches, parse validation, set semantics |
| `internal/domain` | `identity_document_invariants_test.go` | 6 test functions | Namespace non-nil, expiration monotonicity, IsValid==!IsExpired, immutability |
| `internal/domain` | `identity_mapper_invariants_test.go` | 6 test functions | Namespace/selectors non-nil, AND logic matching, consistency |
| `internal/app` | `service_invariants_test.go` | 6 test functions | Pre/post conditions, no partial results, data preservation, idempotency |
| `internal/adapters/outbound/inmemory` | `registry_invariants_test.go` | 7 test functions | Immutability after sealing, no duplicates, read-only ops, AND logic |
| `internal/adapters/outbound/inmemory` | `server_invariants_test.go` | 6 test functions | TrustDomain/CA non-nil, input validation, namespace matching, read-only getters |

**Subtotal**: 56 test functions, 156+ test cases

#### Combined Coverage

```bash
go test -cover ./internal/domain/... ./internal/app/... ./internal/adapters/outbound/inmemory/...
```

- **Domain Layer**: 61.7% coverage (increased from 42.5%)
- **Application Layer**: 22.0% coverage
- **Adapter Layer**: 38.5% coverage (new coverage for registry/server)

**Grand Total**: 76 test functions, 207+ test cases across all layers

### ⏳ TODO (Next Phase)

- Integration tests for `InMemoryAgent`
- Integration tests for `CLI` adapter
- Integration tests for Workload API server
- E2E test: Bootstrap → Attest → Exchange

---

## References

- [Testify Documentation](https://github.com/stretchr/testify)
- [Go Testing Best Practices](https://golang.org/doc/tutorial/add-a-test)
- [Table-Driven Tests in Go](https://dave.cheney.net/2019/05/07/prefer-table-driven-tests)
- [Hexagonal Architecture Testing](https://alistair.cockburn.us/hexagonal-architecture/)
