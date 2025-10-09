# SPIRE Domain Model

This directory contains the domain entities and value objects representing SPIRE/SPIFFE concepts. These are pure domain model concepts with no infrastructure dependencies.

## Domain Entities

### TrustDomain (`trust_domain.go`)

The scope of identities issued by a SPIRE Server, defining the namespace for SPIFFE IDs.

**Example**: `example.org`

**Responsibilities**:
- Validates trust domain format
- Provides equality comparison
- Immutable value object

### IdentityCredential (`identity_credential.go`)

A unique, URI-formatted identifier for a workload or agent, serving as the core identity in the system.

**Format**: `spiffe://<trust-domain>/<path>`

**Examples**:
- `spiffe://example.org/host` (agent)
- `spiffe://example.org/server-workload` (workload)

**Responsibilities**:
- Parses and validates SPIFFE ID URIs
- Extracts trust domain and path components
- Checks trust domain membership
- Immutable value object

### Selector (`selector.go`)

A key-value pair used to match workload or node attributes during attestation.

**Types**:
- **Node selectors**: Match node/host attributes
- **Workload selectors**: Match workload process attributes

**Examples**:
- `unix:uid:1001`
- `unix:user:server-workload`
- `k8s:namespace:default`

**Responsibilities**:
- Parses selector strings
- Categorizes by type (node vs workload)
- Provides selector set operations
- Immutable value object

### Workload (`workload.go`)

A software process or service requesting an identity; the primary entity that undergoes attestation and receives an IdentityDocument.

**Attributes**:
- PID (Process ID)
- UID (User ID)
- GID (Group ID)
- Path (executable path)

**Responsibilities**:
- Encapsulates workload process information
- Provides attribute access

### RegistrationEntry (`registration_entry.go`)

A mapping that associates a SPIFFE ID with a set of selectors, defining the conditions under which a workload qualifies for that identity.

**Components**:
- IdentityCredential (identity to issue)
- Selectors (matching criteria)
- Parent ID (agent IdentityCredential)

**Responsibilities**:
- Maps selectors to identities
- Matches workload selectors to determine eligibility
- Entity with identity

### IdentityDocument (`identity_document.go`)

SPIFFE Verifiable Identity Document - the issued identity artifact provided to workloads for authentication and authorization.

**Formats**:
- X.509 (certificate-based)
- JWT (token-based)

**Components** (X.509):
- SPIFFE ID
- X.509 certificate
- Private key
- Certificate chain
- Expiration time

**Responsibilities**:
- Encapsulates identity credentials
- Validates expiration
- Provides certificate access
- Entity with lifecycle

### Node (`node.go`)

The host machine or environment where the agent and workloads run; its identity is verified via node attestation.

**Attributes**:
- SPIFFE ID (node identity)
- Selectors (node attributes)
- Attestation status

**Responsibilities**:
- Represents the compute node
- Tracks attestation state
- Stores node selectors
- Entity with state

## Domain Services

### AttestationService (`attestation.go`)

Encapsulates domain logic for attestation processes.

**Responsibilities**:
- **MatchWorkloadToEntry**: Finds registration entries matching workload selectors
- **ValidateSVID**: Verifies IdentityDocument validity and identity match

**Value Objects**:
- `NodeAttestationResult`: Result of node attestation
- `WorkloadAttestationResult`: Result of workload attestation

## Principles

### 1. **Ubiquitous Language**
All domain concepts use SPIRE/SPIFFE terminology:
- TrustDomain, IdentityCredential, IdentityDocument, Selector, etc.
- Names match the official SPIRE specification

### 2. **Value Objects vs Entities**
- **Value Objects** (immutable, compared by value):
  - TrustDomain
  - IdentityCredential
  - Selector
  - SelectorSet

- **Entities** (identity, lifecycle):
  - IdentityDocument
  - RegistrationEntry
  - Node
  - Workload

### 3. **Domain Invariants**
- IdentityCredential must follow URI format
- Trust domains cannot contain schemes or paths
- Selectors must have key and value
- IdentityDocument validate expiration

### 4. **No Infrastructure Dependencies**
- Pure Go domain model
- No database, HTTP, or external dependencies
- Only `crypto/x509` for certificate types (standard library)

### 5. **Rich Domain Model**
- Behavior and data together
- Domain logic in domain objects
- Validation at construction time

## Usage Example

```go
// Create trust domain
td, _ := domain.NewTrustDomain("example.org")

// Create SPIFFE ID
identityCredential, _ := domain.NewIdentityCredentialFromParts(td, "/server-workload")

// Create selectors
selector1, _ := domain.NewSelector(domain.SelectorTypeWorkload, "unix:uid", "1001")
selector2, _ := domain.ParseSelector(domain.SelectorTypeWorkload, "unix:user:server-workload")
selectors := domain.NewSelectorSet(selector1, selector2)

// Create registration entry
entry, _ := domain.NewRegistrationEntry(identityCredential, selectors)

// Create workload
workload := domain.NewWorkload(12345, 1001, 1001, "/usr/bin/server")

// Attest workload (via service)
attestationSvc := domain.NewAttestationService()
matchedEntry, _ := attestationSvc.MatchWorkloadToEntry(selectors, []*domain.RegistrationEntry{entry})

// Create IdentityDocument
svid, _ := domain.NewX509SVID(identityCredential, cert, privateKey, chain)

// Validate IdentityDocument
err := attestationSvc.ValidateSVID(svid, identityCredential)
```

## Relationship to Ports

The domain model is used by ports (interfaces in `internal/app/ports.go`):
- Ports define operations using domain types
- Adapters translate between domain and infrastructure
- Core business logic operates on domain entities

## Benefits

1. **Clear Semantics**: SPIRE concepts are explicit types
2. **Type Safety**: Compile-time validation of domain operations
3. **Testability**: Pure domain logic easily unit tested
4. **Documentation**: Domain model serves as living documentation
5. **Refactoring Safety**: Changes to domain propagate through type system
