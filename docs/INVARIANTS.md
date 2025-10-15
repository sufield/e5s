# Code Invariants

## Overview

This document identifies and describes **invariants**—properties or conditions that must **always hold true** at specific points in the codebase. Invariants clarify design intent, catch bugs early, and guide testing and refactoring.

**What are Invariants?**
- **State Assumptions**: Conditions that must be true after an operation completes
- **Pre/Post Conditions**: Entry and exit requirements for methods
- **Business Rules**: Domain-specific constraints that must never be violated
- **Data Integrity**: Consistency guarantees across layers and components

**Benefits of Identifying Invariants:**
- 🎯 **Clarity**: Makes implicit assumptions explicit in code and documentation
- 🐛 **Bug Prevention**: Catches violations early through assertions and tests
- 🧪 **Testing**: Provides clear properties to validate in test cases
- 🔧 **Refactoring**: Ensures correctness is maintained during code changes
- 📚 **Onboarding**: Helps new developers understand critical constraints

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
// Location: NewTrustDomainFromName (line 19)
func NewTrustDomainFromName(name string) *TrustDomain
```
- **Pre**: `name` must not be empty (validated by TrustDomainParser adapter before calling)
- **Post**: `td.name != ""` always holds
- **Rationale**: Empty trust domain is invalid per SPIFFE spec

```go
// Invariant: Equals() returns false for nil input
// Location: Equals (line 30)
func (td *TrustDomain) Equals(other *TrustDomain) bool
```
- **Post**: Returns `false` if `other == nil`, never panics
- **Rationale**: Defensive nil handling prevents runtime crashes

```go
// Invariant: String() never returns empty string for valid TrustDomain
// Location: String (line 24)
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
// Location: NewIdentityCredentialFromComponents (line 25)
func NewIdentityCredentialFromComponents(trustDomain *TrustDomain, path string) *IdentityCredential
```
- **Pre**: `trustDomain != nil` (enforced by caller, see line 24 comment)
- **Post**: `i.trustDomain != nil` always holds
- **Rationale**: IdentityCredential without trust domain is meaningless

```go
// Invariant: path defaults to "/" if empty, never stored as empty string
// Location: NewIdentityCredentialFromComponents (line 26-28)
func NewIdentityCredentialFromComponents(trustDomain *TrustDomain, path string) *IdentityCredential
```
- **Post**: `i.path != ""` (always "/" or user-provided non-empty path)
- **Rationale**: SPIFFE IDs require path component, "/" is valid root

```go
// Invariant: uri is always formatted as "spiffe://<trustDomain><path>"
// Location: NewIdentityCredentialFromComponents (line 30)
func NewIdentityCredentialFromComponents(trustDomain *TrustDomain, path string) *IdentityCredential
```
- **Post**: `i.uri` starts with `"spiffe://"` and matches `trustDomain + path`
- **Rationale**: Cached representation must match components

```go
// Invariant: Equals() is reflexive, symmetric, transitive
// Location: Equals (line 54)
func (i *IdentityCredential) Equals(other *IdentityCredential) bool
```
- **Post**: `i.Equals(i) == true` (reflexive)
- **Post**: `i.Equals(j) == j.Equals(i)` (symmetric)
- **Post**: `i.Equals(j) && j.Equals(k) => i.Equals(k)` (transitive)
- **Post**: Returns `false` for `nil` input, never panics
- **Rationale**: Proper equivalence relation for value objects

```go
// Invariant: IsInTrustDomain(td) iff i.trustDomain.Equals(td)
// Location: IsInTrustDomain (line 62)
func (i *IdentityCredential) IsInTrustDomain(td *TrustDomain) bool
```
- **Post**: Result matches `i.trustDomain.Equals(td)`
- **Rationale**: Trust domain membership is exact match only

---

### 3. `domain.Selector`

**File**: `internal/domain/selector.go`

#### Invariants:

```go
// Invariant: key and value are never empty after construction
// Location: NewSelector (line 27)
func NewSelector(selectorType SelectorType, key, value string) (*Selector, error)
```
- **Pre**: `key != ""` and `value != ""` (validated, returns error otherwise)
- **Post**: If `err == nil`, then `s.key != ""` and `s.value != ""` always hold
- **Rationale**: Empty key/value makes selector meaningless

```go
// Invariant: formatted matches "type:key:value" pattern
// Location: NewSelector (line 35)
func NewSelector(selectorType SelectorType, key, value string) (*Selector, error)
```
- **Post**: `s.formatted == fmt.Sprintf("%s:%s:%s", type, key, value)`
- **Rationale**: Cached string must match components

```go
// Invariant: ParseSelectorFromString requires at least 3 parts (type:key:value)
// Location: ParseSelectorFromString (line 76)
func ParseSelectorFromString(s string) (*Selector, error)
```
- **Pre**: Input format is "type:key:value[:more...]"
- **Post**: If `err == nil`, selector has non-empty `selectorType`, `key`, `value`
- **Post**: Values with colons are preserved (e.g., "unix:uid:1000:extra" → value="1000:extra")
- **Rationale**: Selector format is strictly defined

```go
// Invariant: Equals() is reflexive, symmetric, transitive
// Location: Equals (line 112)
func (s *Selector) Equals(other *Selector) bool
```
- **Post**: `s.Equals(s) == true` (reflexive)
- **Post**: `s.Equals(t) == t.Equals(s)` (symmetric)
- **Post**: `s.Equals(t) && t.Equals(u) => s.Equals(u)` (transitive)
- **Post**: Returns `false` for `nil` input, never panics
- **Rationale**: Proper equivalence relation for selector comparison

---

### 4. `domain.SelectorSet`

**File**: `internal/domain/selector.go`

#### Invariants:

```go
// Invariant: Set contains no duplicate selectors (uniqueness)
// Location: Add (line 152)
func (ss *SelectorSet) Add(selector *Selector)
```
- **Pre**: Any selector can be added
- **Post**: After `Add(s)`, `ss.Contains(s) == true`
- **Post**: If selector already exists, set size unchanged
- **Post**: No two selectors `s1, s2` where `s1.Equals(s2)` exist in set
- **Rationale**: Set semantics require uniqueness

```go
// Invariant: Contains() never modifies the set
// Location: Contains (line 159)
func (ss *SelectorSet) Contains(selector *Selector) bool
```
- **Post**: Calling `Contains()` never changes `ss.selectors` slice
- **Rationale**: Query operation must be side-effect free

```go
// Invariant: All() returns defensive copy to prevent external mutation
// Location: All (line 169)
func (ss *SelectorSet) All() []*Selector
```
- **Post**: Modifying returned slice does not affect `ss.selectors`
- **Rationale**: Immutability protection (DDD pattern)

---

### 5. `domain.IdentityDocument`

**File**: `internal/domain/identity_document.go`

#### Invariants:

```go
// Invariant: identityCredential is never nil for valid document
// Location: NewIdentityDocumentFromComponents (line 43)
func NewIdentityDocumentFromComponents(...) *IdentityDocument
```
- **Pre**: `identityCredential != nil` (enforced by caller)
- **Post**: `id.identityCredential != nil` always holds
- **Rationale**: Document without identity credential is meaningless

```go
// Invariant: For X.509 documents, cert/privateKey/chain are non-nil
// Location: NewIdentityDocumentFromComponents (line 43)
func NewIdentityDocumentFromComponents(...) *IdentityDocument
```
- **Pre**: If `identityDocumentType == IdentityDocumentTypeX509`, then `cert != nil`, `privateKey != nil`, `chain != nil`
- **Post**: X.509 document guarantees non-nil crypto material
- **Rationale**: X.509 documents require certificate and key

```go
// Invariant: For JWT documents, cert/privateKey/chain are nil
// Location: NewIdentityDocumentFromComponents (line 43)
func NewIdentityDocumentFromComponents(...) *IdentityDocument
```
- **Pre**: If `identityDocumentType == IdentityDocumentTypeJWT`, then `cert == nil`, `privateKey == nil`, `chain == nil`
- **Rationale**: JWT documents don't use X.509 certificates

```go
// Invariant: IsExpired() iff time.Now().After(expiresAt)
// Location: IsExpired (line 92)
func (id *IdentityDocument) IsExpired() bool
```
- **Post**: Returns `true` when current time > `expiresAt`, `false` otherwise
- **Rationale**: Simple time-based expiration check

```go
// Invariant: IsValid() == !IsExpired() for current implementation
// Location: IsValid (line 98)
func (id *IdentityDocument) IsValid() bool
```
- **Post**: `IsValid() == !IsExpired()` always holds
- **Post**: Simple time check, no chain-of-trust validation
- **Rationale**: Full validation delegated to IdentityDocumentValidator port (future)

---

### 6. `domain.IdentityMapper`

**File**: `internal/domain/identity_mapper.go`

#### Invariants:

```go
// Invariant: identityCredential is never nil after construction
// Location: NewIdentityMapper (line 15)
func NewIdentityMapper(identityCredential *IdentityCredential, selectors *SelectorSet) (*IdentityMapper, error)
```
- **Pre**: `identityCredential != nil` (validated, returns `ErrInvalidIdentityCredential` otherwise)
- **Post**: If `err == nil`, then `im.identityCredential != nil` always holds
- **Rationale**: Mapper without identity credential is meaningless

```go
// Invariant: selectors is never nil or empty after construction
// Location: NewIdentityMapper (line 15)
func NewIdentityMapper(identityCredential *IdentityCredential, selectors *SelectorSet) (*IdentityMapper, error)
```
- **Pre**: `selectors != nil && len(selectors.All()) > 0` (validated, returns `ErrInvalidSelectors` otherwise)
- **Post**: If `err == nil`, then `im.selectors != nil && len(im.selectors.All()) > 0` always hold
- **Rationale**: Mapper without selectors cannot match any workload

```go
// Invariant: MatchesSelectors() uses AND logic (ALL mapper selectors must be present)
// Location: MatchesSelectors (line 53)
func (im *IdentityMapper) MatchesSelectors(selectors *SelectorSet) bool
```
- **Post**: Returns `true` iff ALL selectors in `im.selectors` are contained in input `selectors`
- **Post**: Returns `false` if ANY required selector is missing
- **Rationale**: Workload must satisfy all required conditions to qualify for identity

---

## Application Layer Invariants

### 7. `app.IdentityService`

**File**: `internal/app/service.go`

#### Invariants:

```go
// Invariant: ExchangeMessage requires non-nil identity credentials
// Location: ExchangeMessage (line 28)
func (s *IdentityService) ExchangeMessage(ctx context.Context, from dto.Identity, to dto.Identity, content string) (*dto.Message, error)
```
- **Pre**: `from.IdentityCredential != nil` and `to.IdentityCredential != nil`
- **Post**: If `err != nil`, then error message indicates which identity credential is nil
- **Rationale**: Cannot exchange messages without knowing sender/receiver identities

```go
// Invariant: ExchangeMessage requires valid (non-expired) identity documents
// Location: ExchangeMessage (line 28)
func (s *IdentityService) ExchangeMessage(ctx context.Context, from dto.Identity, to dto.Identity, content string) (*dto.Message, error)
```
- **Pre**: `from.IdentityDocument != nil && from.IdentityDocument.IsValid()`
- **Pre**: `to.IdentityDocument != nil && to.IdentityDocument.IsValid()`
- **Post**: If documents are nil or invalid, returns error (never creates message)
- **Rationale**: Cannot authenticate parties without valid identity documents

```go
// Invariant: ExchangeMessage never returns msg != nil when err != nil
// Location: ExchangeMessage (line 28)
func (s *IdentityService) ExchangeMessage(ctx context.Context, from dto.Identity, to dto.Identity, content string) (*dto.Message, error)
```
- **Post**: If `err != nil`, then `msg == nil` always holds
- **Post**: If `err == nil`, then `msg != nil` and `msg.From/To` match inputs
- **Rationale**: Error implies failure, no partial result returned

```go
// Invariant: Created message preserves input identities and content
// Location: ExchangeMessage (line 46)
func (s *IdentityService) ExchangeMessage(ctx context.Context, from dto.Identity, to dto.Identity, content string) (*dto.Message, error)
```
- **Post**: If `err == nil`, then:
  - `msg.From.IdentityCredential == from.IdentityCredential`
  - `msg.To.IdentityCredential == to.IdentityCredential`
  - `msg.Content == content`
- **Rationale**: Message must accurately represent exchange parameters

---

## Adapter Layer Invariants

### 8. `inmemory.InMemoryRegistry`

**File**: `internal/adapters/outbound/inmemory/registry.go`

#### Invariants:

```go
// Invariant: Registry is immutable after sealing
// Location: Seal (line 52)
func (r *InMemoryRegistry) Seal()
```
- **Post**: Once `r.sealed == true`, `Seed()` always returns `ErrRegistrySealed`
- **Post**: After sealing, `r.mappers` map is never modified
- **Rationale**: Prevents runtime mutations to control plane configuration

```go
// Invariant: Seed() rejects duplicates by identity credential
// Location: Seed (line 33)
func (r *InMemoryRegistry) Seed(ctx context.Context, mapper *domain.IdentityMapper) error
```
- **Pre**: Registry is not sealed (`r.sealed == false`)
- **Post**: If `err == nil`, mapper is added and `r.mappers[mapper.IdentityCredential().String()] == mapper`
- **Post**: If mapper already exists, returns error and registry unchanged
- **Rationale**: Each identity credential maps to exactly one set of selectors

```go
// Invariant: FindBySelectors() is read-only (never modifies registry)
// Location: FindBySelectors (line 61)
func (r *InMemoryRegistry) FindBySelectors(ctx context.Context, selectors *domain.SelectorSet) (*domain.IdentityMapper, error)
```
- **Post**: Calling `FindBySelectors()` never changes `r.mappers` or `r.sealed`
- **Post**: Uses read lock only (`r.mu.RLock()`)
- **Rationale**: Runtime queries must not mutate control plane state

```go
// Invariant: FindBySelectors() validates input before search
// Location: FindBySelectors (line 66)
func (r *InMemoryRegistry) FindBySelectors(ctx context.Context, selectors *domain.SelectorSet) (*domain.IdentityMapper, error)
```
- **Pre**: `selectors != nil && len(selectors.All()) > 0`
- **Post**: If input invalid, returns `ErrInvalidSelectors` before searching
- **Rationale**: Prevents wasteful search with invalid input

```go
// Invariant: FindBySelectors() returns first match using AND logic
// Location: FindBySelectors (line 71)
func (r *InMemoryRegistry) FindBySelectors(ctx context.Context, selectors *domain.SelectorSet) (*domain.IdentityMapper, error)
```
- **Post**: Returns mapper where `mapper.MatchesSelectors(selectors) == true`
- **Post**: If no match found, returns `ErrNoMatchingMapper`
- **Rationale**: Workload selectors must satisfy mapper's required selectors

```go
// Invariant: ListAll() never returns nil slice when mappers exist
// Location: ListAll (line 81)
func (r *InMemoryRegistry) ListAll(ctx context.Context) ([]*domain.IdentityMapper, error)
```
- **Post**: If `len(r.mappers) > 0`, returns non-nil slice
- **Post**: If `len(r.mappers) == 0`, returns `ErrRegistryEmpty`
- **Rationale**: Clear distinction between empty registry and error

---

### 9. `inmemory.InMemoryServer`

**File**: `internal/adapters/outbound/inmemory/server.go`

#### Invariants:

```go
// Invariant: trustDomain is never nil after construction
// Location: NewInMemoryServer (line 28)
func NewInMemoryServer(ctx context.Context, trustDomainStr string, trustDomainParser ports.TrustDomainParser, certProvider ports.IdentityDocumentProvider) (*InMemoryServer, error)
```
- **Pre**: `trustDomainStr` is valid (validated by `trustDomainParser`)
- **Post**: If `err == nil`, then `s.trustDomain != nil` always holds
- **Rationale**: Server must operate within a trust domain

```go
// Invariant: CA certificate and key are never nil after construction
// Location: NewInMemoryServer (line 36)
func NewInMemoryServer(ctx context.Context, trustDomainStr string, trustDomainParser ports.TrustDomainParser, certProvider ports.IdentityDocumentProvider) (*InMemoryServer, error)
```
- **Post**: If `err == nil`, then `s.caCert != nil && s.caKey != nil` always hold
- **Rationale**: Server must have CA to issue identity documents

```go
// Invariant: IssueIdentity() validates inputs before issuing
// Location: IssueIdentity (line 51)
func (s *InMemoryServer) IssueIdentity(ctx context.Context, identityCredential *domain.IdentityCredential) (*domain.IdentityDocument, error)
```
- **Pre**: `identityCredential != nil`
- **Pre**: `s.caCert != nil && s.caKey != nil`
- **Post**: If validation fails, returns error before calling provider
- **Rationale**: Prevents issuing documents with invalid inputs

```go
// Invariant: IssueIdentity() delegates document creation to provider
// Location: IssueIdentity (line 65)
func (s *InMemoryServer) IssueIdentity(ctx context.Context, identityCredential *domain.IdentityCredential) (*domain.IdentityDocument, error)
```
- **Post**: If `err == nil`, returned document is created by `certificateProvider`
- **Post**: Document's identity credential matches input `identityCredential`
- **Rationale**: Document creation logic is in provider port

```go
// Invariant: GetTrustDomain() and GetCA() are read-only
// Location: GetTrustDomain (line 74), GetCA (line 79)
func (s *InMemoryServer) GetTrustDomain() *domain.TrustDomain
func (s *InMemoryServer) GetCA() *x509.Certificate
```
- **Post**: Calling these methods never modifies server state
- **Rationale**: Accessors must be side-effect free

---

### 10. `inmemory.InMemoryAgent`

**File**: `internal/adapters/outbound/inmemory/agent.go`

#### Invariants:

```go
// Invariant: identityCredential is never nil after construction
// Location: NewInMemoryAgent (line 24)
func NewInMemoryAgent(...) (*InMemoryAgent, error)
```
- **Pre**: `agentSpiffeIDStr` is valid (validated by `parser`)
- **Post**: If `err == nil`, then `a.identityCredential != nil` always holds
- **Rationale**: Agent must have its own identity credential

```go
// Invariant: agentIdentity is initialized before agent is returned
// Location: NewInMemoryAgent (line 50)
func NewInMemoryAgent(...) (*InMemoryAgent, error)
```
- **Post**: If `err == nil`, then `a.agentIdentity != nil` and `a.agentIdentity.IdentityDocument.IsValid() == true`
- **Rationale**: Agent needs valid identity document to operate

```go
// Invariant: GetIdentity() never returns nil identity for initialized agent
// Location: GetIdentity (line 78)
func (a *InMemoryAgent) GetIdentity(ctx context.Context) (*domain.IdentityDocument, error)
```
- **Post**: If agent is initialized, `doc != nil && err == nil`
- **Post**: If agent not initialized, `doc == nil && err != nil`
- **Rationale**: GetIdentity should always succeed for valid agent

```go
// Invariant: FetchIdentityDocument() follows strict flow: Attest → Match → Issue
// Location: FetchIdentityDocument (line 87)
func (a *InMemoryAgent) FetchIdentityDocument(ctx context.Context, workload *domain.Workload) (*domain.IdentityDocument, error)
```
- **Post**: If `err == nil`, then:
  1. Workload was attested (selectors obtained)
  2. Selectors matched registry (mapper found)
  3. Document issued by server
  4. Returned document has non-nil identity credential and is valid
- **Rationale**: Identity issuance requires all steps to succeed

```go
// Invariant: FetchIdentityDocument() validates attestation result
// Location: FetchIdentityDocument (line 94)
func (a *InMemoryAgent) FetchIdentityDocument(ctx context.Context, workload *domain.Workload) (*domain.IdentityDocument, error)
```
- **Post**: If attestation returns empty selectors, returns error (never proceeds to match)
- **Rationale**: Cannot match workload without selectors

```go
// Invariant: FetchIdentityDocument() returns identity document with non-nil credential
// Location: FetchIdentityDocument (line 121)
func (a *InMemoryAgent) FetchIdentityDocument(ctx context.Context, workload *domain.Workload) (*domain.IdentityDocument, error)
```
- **Post**: If `err == nil`, then:
  - `doc != nil`
  - `doc.IdentityCredential() != nil`
  - `doc.IsValid() == true` (freshly issued)
- **Rationale**: Returned identity document must be complete and valid

---

## Cross-Layer Invariants

### 11. Identity Document Lifecycle

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
// Applies to: app.ExchangeMessage, inmemory.Agent.FetchIdentityDocument
```
- **Pre**: Before use in authentication, `doc != nil && doc.IsValid() == true`
- **Rationale**: Business logic requires valid credentials

---

### 12. Registry Lifecycle

**Files**: `inmemory/registry.go`, `app/application.go`

#### Invariants:

```
// Invariant: Registry transitions: Unsealed (mutable) → Sealed (immutable)
// Applies to: InMemoryRegistry
```
- **Post**: State transition is one-way: `sealed == false` → `sealed == true`, never reversed
- **Rationale**: Control plane is configured once at startup, immutable at runtime

```
// Invariant: Registry is sealed before any runtime operations
// Applies to: Bootstrap flow in application.go
```
- **Post**: After `Bootstrap()` completes, `registry.sealed == true`
- **Post**: Runtime code (services, agents) only calls `FindBySelectors()` and `ListAll()` (read-only)
- **Rationale**: Prevents runtime mutations to registration database

---

### 13. Hexagonal Architecture Boundaries

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
   func NewIdentityMapper(id *IdentityCredential, sel *SelectorSet) (*IdentityMapper, error) {
       if id == nil {
           return nil, ErrInvalidIdentityCredential // Enforce invariant
       }
       // ...
   }
   ```

2. **Compile-Time Checks**:
   ```go
   var _ ports.Agent = (*InMemoryAgent)(nil) // Enforces interface invariant
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
   _, err := NewIdentityMapper(nil, selectors)
   assert.Error(t, err) // Should reject nil namespace
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
2. **Test Invariants Systematically**: Use unit tests, table-driven tests, and assertions
3. **Enforce at Multiple Levels**: Constructors, compile-time checks, runtime validation
4. **Hexagonal Boundaries Are Invariants**: Domain never imports app/adapters, ports are pure interfaces
5. **Lifecycle Invariants Matter**: Registry sealing, document immutability, expiration monotonicity

**Current Status:**
- ✅ 13 major invariant categories documented across domain/app/adapter layers
- ✅ 50+ specific invariants identified with pre/post conditions
- ✅ Test coverage validates core invariants (51+ test cases)
- ⏳ Property-based testing (future enhancement)
- ⏳ Runtime monitoring of invariant violations (future enhancement)

---

## References

- [Design by Contract](https://en.wikipedia.org/wiki/Design_by_contract) - Formal invariant specification
- [Go Best Practices: Effective Go](https://go.dev/doc/effective_go) - Constructor validation patterns
- [Hexagonal Architecture](https://alistair.cockburn.us/hexagonal-architecture/) - Port invariants
- [SPIFFE Specification](https://github.com/spiffe/spiffe/blob/main/standards/SPIFFE.md) - Trust domain and SPIFFE ID invariants
- [TESTING.md](TESTING.md) - Test strategies for validating invariants
- [CONTROL_PLANE.md](CONTROL_PLANE.md) - Registry sealing and control plane invariants

