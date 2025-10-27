# Code Invariants

## Overview

This document identifies and describes **invariants**—properties or conditions that must **always hold true** at specific points in the codebase. Invariants clarify design intent, catch bugs early, and guide testing and refactoring.

**What are Invariants?**
- **State Assumptions**: Conditions that must be true after an operation completes
- **Pre/Post Conditions**: Entry and exit requirements for methods
- **Business Rules**: Domain-specific constraints that must never be violated
- **Data Integrity**: Consistency guarantees across layers and components

**Benefits of Identifying Invariants:**
- **Clarity**: Makes implicit assumptions explicit in code and documentation
- **Bug Prevention**: Catches violations early through assertions and tests
- **Testing**: Provides clear properties to validate in test cases
- **Refactoring**: Ensures correctness is maintained during code changes
- **Onboarding**: Helps new developers understand critical constraints

---

## How to Identify Invariants

### Systematic Scan Techniques

1. **State Assumptions**: For each method, ask "What must be true after this completes?"
2. **Error-Prone Spots**: Focus on mutable state, concurrent access, and critical paths
3. **Business Rules**: Document domain-specific constraints from requirements
4. **Data Integrity**: Trace data flows to find consistency requirements

### Practical Methods

- **Code Walkthrough**: Review each public method and trace execution flow
- **"What if X flips?"**: Consider edge cases (nil values, expired data, empty collections)
- **Static Analysis**: Use `go vet`, `staticcheck` for automated nil/bounds checks
- **Review Sessions**: Pair-program to identify hidden assumptions
- **Production Monitoring**: Log/metric violations to catch runtime issues

---

## Invariants by Layer

## Domain Layer Invariants

### 1. `domain.TrustDomain`

**File**: `internal/domain/trust_domain.go`

#### Invariants:

```go
// Invariant: name is never empty after construction
// Location: NewTrustDomainFromName
func NewTrustDomainFromName(name string) *TrustDomain
```
- **Pre**: `name` must not be empty (validated by TrustDomainParser adapter before calling)
- **Post**: `td.name != ""` always holds
- **Rationale**: Empty trust domain is invalid per SPIFFE spec

```go
// Invariant: Equals() returns false for nil input
// Location: Equals
func (td *TrustDomain) Equals(other *TrustDomain) bool
```
- **Post**: Returns `false` if `other == nil`, never panics
- **Rationale**: Defensive nil handling prevents runtime crashes

```go
// Invariant: String() never returns empty string for valid TrustDomain
// Location: String
func (td *TrustDomain) String() string
```
- **Post**: `len(td.String()) > 0` for all valid trust domains
- **Rationale**: Follows from `name != ""` invariant

---

### 2. `domain.IdentityCredential`

**File**: `internal/domain/identity_credential.go`

#### Invariants:

```go
// Invariant: trustDomain is never nil after construction
// Location: NewIdentityCredentialFromComponents
func NewIdentityCredentialFromComponents(trustDomain *TrustDomain, path string) *IdentityCredential
```
- **Pre**: `trustDomain != nil` (enforced by caller)
- **Post**: `i.trustDomain != nil` always holds
- **Rationale**: IdentityCredential without trust domain is meaningless

```go
// Invariant: path is normalized and validated on construction
// Location: NewIdentityCredentialFromComponents, normalizePath
func NewIdentityCredentialFromComponents(trustDomain *TrustDomain, path string) *IdentityCredential
```
- **Post**: `i.path != ""` (always "/" or normalized non-empty path)
- **Post**: Path never contains `.` or `..` segments - SPIFFE forbids traversal (panics if violated)
- **Post**: Path has leading slash, no repeated slashes, no trailing slash (except root "/")
- **Post**: Colons are allowed in path segments per RFC 3986 and SPIFFE spec
- **Rationale**: SPIFFE IDs require normalized path; dot segments are security risks

```go
// Invariant: uri is always formatted as "spiffe://<trustDomain><path>"
// Location: NewIdentityCredentialFromComponents
func NewIdentityCredentialFromComponents(trustDomain *TrustDomain, path string) *IdentityCredential
```
- **Post**: `i.uri` starts with `"spiffe://"` and matches `trustDomain + path`
- **Rationale**: Cached representation must match components

```go
// Invariant: Equals() is reflexive, symmetric, transitive
// Location: Equals
func (i *IdentityCredential) Equals(other *IdentityCredential) bool
```
- **Post**: `i.Equals(i) == true` (reflexive)
- **Post**: `i.Equals(j) == j.Equals(i)` (symmetric)
- **Post**: `i.Equals(j) && j.Equals(k) => i.Equals(k)` (transitive)
- **Post**: Returns `false` for `nil` input, never panics
- **Rationale**: Proper equivalence relation for value objects

```go
// Invariant: IsInTrustDomain(td) iff i.trustDomain.Equals(td)
// Location: IsInTrustDomain
func (i *IdentityCredential) IsInTrustDomain(td *TrustDomain) bool
```
- **Post**: Result matches `i.trustDomain.Equals(td)`
- **Rationale**: Trust domain membership is exact match only

#### normalizePath() Algebraic Properties

**File**: `internal/domain/identity_credential.go` (private function)
**Verified By**: Property-based tests in `identity_credential_pbt_test.go`

The following mathematical properties of `normalizePath()` have been verified through property-based testing (10,000 test cases per property):

```go
// Property 1: Idempotency - normalize(normalize(p)) == normalize(p)
// Location: normalizePath
func normalizePath(path string) string
```
- **Property**: For all valid paths `p`, `normalizePath(normalizePath(p)) == normalizePath(p)`
- **Verified**: ✅ TestNormalizePath_Properties/idempotency (10,000 cases)
- **Rationale**: Normalized output is already in canonical form

```go
// Property 2: Canonical Form - consistent structure
// Location: normalizePath
func normalizePath(path string) string
```
- **Property**: For all valid paths `p`, `normalizePath(p)` has:
  - Starts with "/" (leading slash)
  - No trailing slash (except root "/")
- **Verified**: ✅ TestNormalizePath_Properties/canonical_form (10,000 cases)
- **Rationale**: SPIFFE spec requires consistent path format

```go
// Property 3: Length Bound - minimal transformation
// Location: normalizePath
func normalizePath(path string) string
```
- **Property**: For all valid paths `p`:
  - If `p` starts with "/": `len(normalizePath(p)) == len(p)`
  - Otherwise: `len(normalizePath(p)) == len(p) + 1`
- **Verified**: ✅ TestNormalizePath_Properties/exact_length (10,000 cases)
- **Rationale**: Only adds leading slash when needed, no expansion

```go
// Property 4: No Consecutive Slashes
// Location: normalizePath
func normalizePath(path string) string
```
- **Property**: For all valid paths `p`, `normalizePath(p)` contains no "//"
- **Verified**: ✅ TestNormalizePath_Properties/no_consecutive_slashes (10,000 cases)
- **Rationale**: Normalized paths have single slashes between segments

```go
// Property 5: No Whitespace
// Location: normalizePath
func normalizePath(path string) string
```
- **Property**: For all valid paths `p`, `normalizePath(p)` contains no whitespace
- **Verified**: ✅ TestNormalizePath_Properties/no_whitespace (10,000 cases)
- **Rationale**: RFC 3986 compliance - URIs forbid whitespace

```go
// Property 6: No Traversal Segments
// Location: normalizePath
func normalizePath(path string) string
```
- **Property**: For all valid paths `p`, `normalizePath(p)` has no "." or ".." segments
- **Verified**: ✅ TestNormalizePath_Properties/no_traversal_segments (10,000 cases)
- **Rationale**: SPIFFE spec forbids path traversal (security)

**Testing Approach**: These properties complement fuzz testing by verifying mathematical invariants rather than just crash safety. See `docs/engineering/pbt.md` for property-based testing methodology.

---

### 3. `domain.IdentityDocument`

**File**: `internal/domain/identity_document.go`

#### Invariants:

```go
// Invariant: identityCredential is never nil for valid document
// Location: NewIdentityDocumentFromComponents 
func NewIdentityDocumentFromComponents(...) *IdentityDocument
```
- **Pre**: `identityCredential != nil` (enforced by caller)
- **Post**: `id.identityCredential != nil` always holds
- **Rationale**: Document without identity credential is meaningless

```go
// Invariant: For X.509 documents, cert and chain are non-nil
// Location: NewIdentityDocumentFromComponents
func NewIdentityDocumentFromComponents(...) *IdentityDocument
```
- **Pre**: If `identityDocumentType == IdentityDocumentTypeX509`, then `cert != nil` and `chain != nil`
- **Post**: X.509 document guarantees non-nil certificate and chain
- **Rationale**: X.509 documents require certificate; private keys are managed by adapters/DTO layer
- **Note**: Private keys are NOT stored in domain model - managed by SDK's X509SVID or dto.Identity

```go
// Invariant: JWT documents (if implemented) would not use X.509 certificates
// Location: NewIdentityDocumentFromComponents
func NewIdentityDocumentFromComponents(...) *IdentityDocument
```
- **Note**: JWT support is not currently implemented in the domain model
- **Rationale**: The system currently supports X.509-only for simplicity

```go
// Invariant: IsExpired() delegates to IsExpiredAt(time.Now())
// Location: IsExpired 
func (id *IdentityDocument) IsExpired() bool
```
- **Post**: Returns same result as `IsExpiredAt(time.Now())`
- **Post**: Returns `true` when current time > `cert.NotAfter`, `false` otherwise
- **Rationale**: Convenience method that uses IsExpiredAt for clock injection

```go
// Invariant: IsExpiredAt(t) checks expiration at given time (clock injection)
// Location: IsExpiredAt 
func (id *IdentityDocument) IsExpiredAt(t time.Time) bool
```
- **Post**: Returns `true` when `t.After(cert.NotAfter)`, `false` otherwise
- **Post**: Does NOT call time.Now() - pure function for testability
- **Rationale**: Allows injecting time for testing; avoids time.Now() dependency in tests

```go
// Invariant: IsValid() == !IsExpired() for current implementation
// Location: IsValid 
func (id *IdentityDocument) IsValid() bool
```
- **Post**: `IsValid() == !IsExpired()` always holds
- **Post**: Simple time check, no chain-of-trust validation
- **Rationale**: Full validation delegated to IdentityDocumentValidator port (future)

---

### 4. `domain.Workload`

**File**: `internal/domain/workload.go`

#### Invariants:

```go
// Invariant: Workload contains process information for attestation
// Location: Workload struct
type Workload struct {
    PID  int32
    UID  int32
    GID  int32
    Path string
}
```
- **Post**: All fields are accessible for attestation purposes
- **Rationale**: SPIRE agents use process information to attest workload identity

---

## Application Layer Invariants

### 5. `app.IdentityService`

**File**: `internal/app/service.go`

#### Invariants:

```go
// Invariant: ExchangeMessage requires non-nil identity credentials
// Location: ExchangeMessage 
func (s *IdentityService) ExchangeMessage(ctx context.Context, from dto.Identity, to dto.Identity, content string) (*dto.Message, error)
```
- **Pre**: `from.IdentityCredential != nil` and `to.IdentityCredential != nil`
- **Post**: If `err != nil`, then error message indicates which identity credential is nil
- **Rationale**: Cannot exchange messages without knowing sender/receiver identities

```go
// Invariant: ExchangeMessage requires valid (non-expired) identity documents
// Location: ExchangeMessage 
func (s *IdentityService) ExchangeMessage(ctx context.Context, from dto.Identity, to dto.Identity, content string) (*dto.Message, error)
```
- **Pre**: `from.IdentityDocument != nil && from.IdentityDocument.IsValid()`
- **Pre**: `to.IdentityDocument != nil && to.IdentityDocument.IsValid()`
- **Post**: If documents are nil or invalid, returns error (never creates message)
- **Rationale**: Cannot authenticate parties without valid identity documents

```go
// Invariant: ExchangeMessage never returns msg != nil when err != nil
// Location: ExchangeMessage 
func (s *IdentityService) ExchangeMessage(ctx context.Context, from dto.Identity, to dto.Identity, content string) (*dto.Message, error)
```
- **Post**: If `err != nil`, then `msg == nil` always holds
- **Post**: If `err == nil`, then `msg != nil` and `msg.From/To` match inputs
- **Rationale**: Error implies failure, no partial result returned

```go
// Invariant: Created message preserves input identities and content
// Location: ExchangeMessage 
func (s *IdentityService) ExchangeMessage(ctx context.Context, from dto.Identity, to dto.Identity, content string) (*dto.Message, error)
```
- **Post**: If `err == nil`, then:
  - `msg.From.IdentityCredential == from.IdentityCredential`
  - `msg.To.IdentityCredential == to.IdentityCredential`
  - `msg.Content == content`
- **Rationale**: Message must accurately represent exchange parameters

---

## Configuration Layer Invariants

### 6. `config.splitCleanDedup()`

**File**: `internal/config/mtls_env.go` (private function)
**Verified By**: Property-based tests in `mtls_env_pbt_test.go`

The following set-theoretic properties of `splitCleanDedup()` have been verified through property-based testing (10,000 test cases per property):

```go
// Property 1: No Duplicates - set semantics
// Location: splitCleanDedup
func splitCleanDedup(s string, sep string) []string
```
- **Property**: For all inputs `s`, result contains no duplicate elements
- **Verified**: ✅ TestSplitCleanDedup_Properties/no_duplicates (10,000 cases)
- **Rationale**: Core correctness property - function must deduplicate

```go
// Property 2: Idempotency - stable output
// Location: splitCleanDedup
func splitCleanDedup(s string, sep string) []string
```
- **Property**: For all inputs `s`, `splitCleanDedup(join(splitCleanDedup(s))) == splitCleanDedup(s)`
- **Verified**: ✅ TestSplitCleanDedup_Properties/idempotency (10,000 cases)
- **Rationale**: Processing twice produces same result (canonical form)

```go
// Property 3: Subset Preservation - no invented elements
// Location: splitCleanDedup
func splitCleanDedup(s string, sep string) []string
```
- **Property**: For all inputs `s`, every element in result was in original (after cleaning)
- **Verified**: ✅ TestSplitCleanDedup_Properties/subset_preservation (10,000 cases)
- **Rationale**: Function only removes/deduplicates, never adds elements

```go
// Property 4: No Invalid Elements - cleaning guarantees
// Location: splitCleanDedup
func splitCleanDedup(s string, sep string) []string
```
- **Property**: For all inputs `s`, result contains no empty or whitespace-only strings
- **Verified**: ✅ TestSplitCleanDedup_Properties/no_invalid_elements (10,000 cases)
- **Rationale**: All elements are properly trimmed and non-empty

```go
// Property 5: Order Preservation - first occurrence wins
// Location: splitCleanDedup
func splitCleanDedup(s string, sep string) []string
```
- **Property**: For all inputs `s`, result preserves order of first occurrences
- **Verified**: ✅ TestSplitCleanDedup_Properties/order_preservation (10,000 cases)
- **Rationale**: Deterministic output order for reproducible behavior

**Testing Approach**: These properties verify set semantics, cleaning behavior, and determinism. See `docs/engineering/pbt.md` for methodology.

---

### 7. `config.parseDurationInto()`

**File**: `internal/config/mtls_env.go` (private function)
**Verified By**: Property-based tests in `mtls_env_pbt_test.go`

The following properties of duration parsing have been verified through property-based testing (10,000 test cases per property):

```go
// Property 1: Roundtrip - parse/format consistency
// Location: parseDurationInto
func parseDurationInto(envVar string, target *time.Duration) error
```
- **Property**: For valid durations `d`, `parse(format(d)) == d` (equivalent representation)
- **Verified**: ✅ TestParseDurationInto_Properties/roundtrip (10,000 cases)
- **Rationale**: Parse and format should be inverses

```go
// Property 2: Parse Equivalence - determinism
// Location: parseDurationInto
func parseDurationInto(envVar string, target *time.Duration) error
```
- **Property**: For all inputs, calling twice produces same result
- **Verified**: ✅ TestParseDurationInto_Properties/parse_equivalence (10,000 cases)
- **Rationale**: Parsing is deterministic, no hidden state

```go
// Property 3: Non-Negative for Positive - sign preservation
// Location: parseDurationInto
func parseDurationInto(envVar string, target *time.Duration) error
```
- **Property**: Parsing positive duration strings produces non-negative durations
- **Verified**: ✅ TestParseDurationInto_Properties/non_negative_for_positive_durations (10,000 cases)
- **Rationale**: Ensures configuration values have expected sign

**Testing Approach**: These properties verify parse/format consistency and determinism. See `docs/engineering/pbt.md` for methodology.

---

## Cross-Layer Invariants

### 8. Identity Document Lifecycle

**Files**: Multiple (domain, app, adapters)

#### Invariants:

```
// Invariant: Identity documents are immutable after creation
// Applies to: All IdentityDocument instances
```
- **Post**: Once created via `NewIdentityDocumentFromComponents()`, no fields are modified
- **Rationale**: Documents represent signed credentials that shouldn't change

```
// Invariant: Identity documents expire monotonically (never extended)
// Applies to: All IdentityDocument instances
```
- **Post**: `IsExpired()` transitions from `false` → `true`, never `true` → `false`
- **Rationale**: Time moves forward, expiration is irreversible without renewal

```
// Invariant: Valid documents are always non-nil and non-expired
// Applies to: app.ExchangeMessage, SPIRE agent operations
```
- **Pre**: Before use in authentication, `doc != nil && doc.IsValid() == true`
- **Rationale**: Business logic requires valid credentials

---

### 9. Hexagonal Architecture Boundaries

**Files**: All layers (domain, app, adapters, ports)

#### Invariants:

```
// Invariant: Domain layer never imports from app or adapters
// Applies to: All files in internal/domain/
```
- **Post**: Domain imports only standard library and other domain packages
- **Rationale**: Domain is innermost layer, no outward dependencies

```
// Invariant: Application layer depends only on domain and ports
// Applies to: All files in internal/app/
```
- **Post**: App imports only `domain` and `ports` packages, never adapters
- **Rationale**: App layer is independent of implementation details

```
// Invariant: Adapters implement port interfaces via compile-time checks
// Applies to: All adapter implementations
```
- **Post**: Each adapter file contains `var _ ports.Interface = (*Impl)(nil)`
- **Rationale**: Enforces interface satisfaction at build time

```
// Invariant: Ports define contracts, never contain logic
// Applies to: All files in internal/ports/
```
- **Post**: Ports package contains only interface definitions and types, no implementations
- **Rationale**: Ports are pure contracts for dependency inversion

---

## Testing Invariants

### How to Test Invariants

1. **Unit Tests**: Assert invariants in success and failure cases
   ```go
   // Test: ExchangeMessage invariant
   msg, err := service.ExchangeMessage(ctx, from, to, content)
   require.NoError(t, err)
   assert.NotNil(t, msg)
   assert.Equal(t, from.IdentityCredential, msg.From.IdentityCredential) // Invariant: preserves identities
   ```

2. **Table-Driven Tests**: Cover edge cases that might violate invariants
   ```go
   tests := []struct {
       name        string
       identity    *dto.Identity
       expectError bool
   }{
       {"nil identity credential", &dto.Identity{IdentityCredential: nil}, true}, // Invariant violation
       {"valid identity", createValidIdentity(t), false},
   }
   ```

3. **Assertion Helpers**: Create reusable validation functions
   ```go
   func assertValidIdentity(t *testing.T, identity *dto.Identity) {
       t.Helper()
       require.NotNil(t, identity)
       require.NotNil(t, identity.IdentityCredential) // Invariant
       require.NotNil(t, identity.IdentityDocument)  // Invariant
       assert.True(t, identity.IdentityDocument.IsValid()) // Invariant
   }
   ```

4. **Property-Based Testing** (future): Use libraries like `gopter` to generate random inputs and verify invariants hold

---

## Enforcing Invariants

### In Code

1. **Constructor Validation**:
   ```go
   func NewIdentityDocumentFromComponents(
       identityCredential *IdentityCredential,
       cert []byte,
       privateKey interface{},
       chain [][]byte,
   ) (*IdentityDocument, error) {
       if identityCredential == nil {
           return nil, ErrInvalidIdentityCredential // Enforce invariant
       }
       // ...
   }
   ```

2. **Compile-Time Checks**:
   ```go
   var _ ports.Agent = (*spire.Agent)(nil) // Enforces interface invariant
   ```

3. **Documentation Comments**:
   ```go
   // Invariant: trustDomain is never nil after construction
   func NewTrustDomainFromName(name string) *TrustDomain { /*...*/ }
   ```

4. **Defensive Programming**:
   ```go
   func (td *TrustDomain) Equals(other *TrustDomain) bool {
       if other == nil {
           return false // Enforce nil invariant
       }
       return td.name == other.name
   }
   ```

### In Tests

5. **Explicit Invariant Checks**:
   ```go
   require.NotNil(t, result.IdentityCredential) // Test invariant explicitly
   ```

6. **Negative Test Cases**:
   ```go
   // Test that invariant violation is prevented
   _, err := NewIdentityCredentialFromComponents(nil, "/path")
   assert.Error(t, err) // Should reject nil trust domain
   ```

### In Production

7. **Monitoring and Alerts**:
   - Log violations: `if doc == nil { log.Error("Invariant violated: nil document") }`
   - Metrics: Track expired document usage attempts
   - Alerts: Trigger on invariant violation patterns

8. **Runtime Assertions** (dev/staging only):
   ```go
   if buildMode == "debug" && identity.IdentityCredential == nil {
       panic("Invariant violated: nil identity credential")
   }
   ```

---

## Maintenance

### When Adding New Code

1. **Identify New Invariants**: For each new method/type, ask "What must always be true?"
2. **Document in Code**: Add `// Invariant:` comments above relevant functions
3. **Update This Document**: Add invariants to appropriate section
4. **Write Tests**: Create tests that verify new invariants

### When Modifying Existing Code

1. **Check Existing Invariants**: Ensure changes don't break documented invariants
2. **Update Tests**: Modify tests if invariants change
3. **Update Documentation**: Revise this document if invariants evolve

### Review Checklist

- [ ] All public methods have documented pre/post conditions
- [ ] Invariants are tested in unit tests
- [ ] Edge cases (nil, empty, expired) are covered
- [ ] Compile-time checks exist for port implementations
- [ ] Critical invariants are enforced in constructors
- [ ] This document is up-to-date with code changes

---

## Summary

1. **Invariants Make Assumptions Explicit**: Document what must always hold true
2. **Test Invariants Systematically**: Use unit tests, property-based tests, and assertions
3. **Enforce at Multiple Levels**: Constructors, compile-time checks, runtime validation
4. **Hexagonal Boundaries Are Invariants**: Domain never imports app/adapters, ports are pure interfaces
5. **Lifecycle Invariants Matter**: Document immutability, expiration monotonicity

**Current Status:**
- ✅ 9 major invariant categories documented across domain/config/app layers
- ✅ 30+ specific invariants identified with pre/post conditions
- ✅ Test coverage validates core invariants (unit + integration tests)
- ✅ Property-based testing implemented (14 properties verified with 10,000 cases each)
  - normalizePath: 6 algebraic properties
  - splitCleanDedup: 5 set-theoretic properties
  - parseDurationInto: 3 consistency properties
- ⏳ Runtime monitoring of invariant violations (future enhancement)

---

## References

- [Go Best Practices: Effective Go](https://go.dev/doc/effective_go) - Constructor validation patterns
- [Hexagonal Architecture](https://alistair.cockburn.us/hexagonal-architecture/) - Port invariants
- [SPIFFE Specification](https://github.com/spiffe/spiffe/blob/main/standards/SPIFFE.md) - Trust domain and SPIFFE ID invariants
- Internal: [TESTING.md](../engineering/TESTING.md) - Test strategies for validating invariants
- Internal: [pbt.md](../engineering/pbt.md) - Property-based testing methodology for verifying algebraic invariants

