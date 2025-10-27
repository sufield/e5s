# Design by Contract in SPIRE Identity Library

This document describes how Design by Contract principles are applied throughout the SPIRE library to ensure correctness, security, and maintainability.

## Overview

Design by Contract uses three types of assertions to ensure code correctness:

1. **Preconditions** - Requirements the caller must meet
2. **Postconditions** - Guarantees the function makes on success
3. **Invariants** - Properties that must always hold

For security-critical identity infrastructure like SPIRE, these contracts are essential to prevent:
- Invalid SPIFFE IDs from entering the system
- Expired or malformed certificates being used
- Trust domain violations
- Half-validated identity credentials

## Contract Types in SPIRE

### 1. Preconditions: Input Validation

**When to use:** Validating external input (user data, network requests, configuration)

**How to implement:** Explicit checks with descriptive error returns

**SPIRE-specific rules:**
- SPIFFE IDs must parse as valid URIs (`spiffe://trust-domain/path`)
- Certificates must be well-formed X.509 and not expired
- Trust domains must match between ID and expected domain
- SVIDs must contain SPIFFE URI SANs
- Trust bundles must contain at least one valid CA

**Example: Identity Credential Constructor**

```go
package identity

import (
    "crypto/x509"
    "errors"
    "fmt"
    "time"

    "github.com/spiffe/go-spiffe/v2/spiffeid"
)

var (
    ErrInvalidSPIFFEID = errors.New("invalid SPIFFE ID")
    ErrExpiredCert     = errors.New("certificate expired")
    ErrTrustDomain     = errors.New("trust domain mismatch")
)

type IdentityCredential struct {
    spiffeID spiffeid.ID
    cert     *x509.Certificate
    certPEM  []byte
}

// NewIdentityCredential creates a validated identity credential.
//
// Preconditions:
// - idStr must be a valid SPIFFE ID URI
// - certPEM must be a valid X.509 certificate
// - Certificate must not be expired
// - SPIFFE ID trust domain must match td parameter
// - Certificate must contain SPIFFE URI SAN
func NewIdentityCredential(idStr string, certPEM []byte, td spiffeid.TrustDomain) (*IdentityCredential, error) {
    // Precondition 1: Valid SPIFFE ID
    parsedID, err := spiffeid.FromString(idStr)
    if err != nil {
        return nil, fmt.Errorf("%w: %v", ErrInvalidSPIFFEID, err)
    }

    // Precondition 2: Trust domain match
    if parsedID.TrustDomain() != td {
        return nil, fmt.Errorf("%w: got %s, want %s",
            ErrTrustDomain, parsedID.TrustDomain(), td)
    }

    // Precondition 3: Valid certificate
    cert, err := x509.ParseCertificate(certPEM)
    if err != nil {
        return nil, fmt.Errorf("invalid certificate: %w", err)
    }

    // Precondition 4: Not expired
    if time.Now().After(cert.NotAfter) {
        return nil, fmt.Errorf("%w: NotAfter=%s", ErrExpiredCert, cert.NotAfter)
    }

    // Precondition 5: Contains SPIFFE URI SAN
    if !cert.IsCA && len(cert.URIs) == 0 {
        return nil, errors.New("certificate lacks SPIFFE URI SAN")
    }

    // All preconditions satisfied - construct
    return &IdentityCredential{
        spiffeID: parsedID,
        cert:     cert,
        certPEM:  certPEM,
    }, nil
}
```

**Why explicit error returns, not panics:**
- Callers can handle validation failures gracefully
- Clear error messages help operators debug misconfigurations
- Follows Go idioms for API boundaries

---

### 2. Postconditions: Output Verification

**When to use:** Verifying that your function produced the correct result

**How to implement:**
- Internal sanity checks before returning
- Test assertions
- Debug-only assertions for performance-critical paths

**Example: Constructor Postconditions**

```go
func NewIdentityCredential(idStr string, certPEM []byte, td spiffeid.TrustDomain) (*IdentityCredential, error) {
    // ... preconditions ...

    ic := &IdentityCredential{
        spiffeID: parsedID,
        cert:     cert,
        certPEM:  certPEM,
    }

    // Postcondition 1: SPIFFE ID is canonical
    assert.Postcondition(ic.spiffeID.String() != "",
        "SPIFFE ID must not be empty after construction")

    // Postcondition 2: Certificate is stored
    assert.Postcondition(ic.cert != nil && len(ic.certPEM) > 0,
        "certificate data must be present")

    // Postcondition 3: Trust domain preserved
    assert.Postcondition(ic.spiffeID.TrustDomain() == td,
        "constructed credential must preserve trust domain")

    return ic, nil
}
```

**Testing Postconditions:**

```go
func TestNewIdentityCredential_Postconditions(t *testing.T) {
    td := spiffeid.RequireTrustDomainFromString("example.org")

    ic, err := NewIdentityCredential(
        "spiffe://example.org/workload",
        validCertPEM,
        td,
    )
    require.NoError(t, err)

    // Postcondition: ID is canonical
    assert.Equal(t, "spiffe://example.org/workload", ic.SPIFFEID().String())

    // Postcondition: Certificate is valid
    assert.NotNil(t, ic.Cert())
    assert.True(t, ic.Cert().NotAfter.After(time.Now()))

    // Postcondition: Trust domain matches
    assert.Equal(t, td, ic.SPIFFEID().TrustDomain())
}
```

---

### 3. Invariants: System State Protection

**When to use:** Ensuring critical properties are never violated

**How to implement:**
- Unexported fields + controlled mutation
- Checks after every state change
- Debug assertions for internal consistency

**Example: Trust Bundle**

```go
type TrustBundle struct {
    domain spiffeid.TrustDomain
    cas    []*x509.Certificate // Unexported - controlled access only
}

// NewTrustBundle creates a trust bundle.
//
// Preconditions:
// - domain must not be zero
// - caPEMs must contain at least one valid CA certificate
//
// Invariants:
// - Trust bundle always has at least one CA
// - All CAs are valid X.509 certificates with IsCA=true
func NewTrustBundle(domain spiffeid.TrustDomain, caPEMs [][]byte) (*TrustBundle, error) {
    // Precondition: valid domain
    if domain.IsZero() {
        return nil, errors.New("trust domain required")
    }

    // Precondition: parse and validate all CAs
    var cas []*x509.Certificate
    for i, pem := range caPEMs {
        ca, err := x509.ParseCertificate(pem)
        if err != nil {
            return nil, fmt.Errorf("invalid CA at index %d: %w", i, err)
        }
        if !ca.IsCA {
            return nil, fmt.Errorf("certificate at index %d is not a CA", i)
        }
        cas = append(cas, ca)
    }

    // Precondition: at least one CA
    if len(cas) == 0 {
        return nil, errors.New("at least one CA required")
    }

    tb := &TrustBundle{
        domain: domain,
        cas:    cas,
    }

    // Invariant check
    assert.Invariant(len(tb.cas) > 0, "trust bundle must not be empty")
    assert.Invariant(!tb.domain.IsZero(), "trust bundle domain must be set")

    return tb, nil
}

// CAs returns a copy of the CA certificates (read-only).
func (tb *TrustBundle) CAs() []*x509.Certificate {
    // Return defensive copy to prevent external mutation
    return append([]*x509.Certificate(nil), tb.cas...)
}

// AddCA adds a CA certificate to the bundle.
//
// Precondition: caPEM must be a valid CA certificate
// Invariant: bundle remains non-empty after addition
func (tb *TrustBundle) AddCA(caPEM []byte) error {
    // Precondition: valid CA
    ca, err := x509.ParseCertificate(caPEM)
    if err != nil {
        return fmt.Errorf("invalid CA: %w", err)
    }
    if !ca.IsCA {
        return errors.New("certificate is not a CA")
    }

    tb.cas = append(tb.cas, ca)

    // Invariant: still non-empty (should always be true after append)
    assert.Invariant(len(tb.cas) > 0, "trust bundle must not be empty after add")

    return nil
}
```

**Why this design prevents bugs:**
- ✅ Cannot create empty trust bundle
- ✅ Cannot mutate CAs array directly (unexported)
- ✅ Cannot add non-CA certificates
- ✅ Invariants checked after mutations

---

## Debug vs. Production Behavior

### Using Build Tags for Assertions

**Production builds (default):**
```bash
go build ./cmd
```
- Assertions are no-ops (zero overhead)
- Only preconditions are checked (via explicit `if` statements)
- Postconditions and invariants stripped

**Debug builds:**
```bash
go build -tags=debug ./cmd
```
- All assertions active
- Violations panic immediately
- Helps catch bugs during development

### Implementation

See `internal/assert/` package:

```go
//go:build debug
package assert

func Invariant(ok bool, msg string) {
    if !ok {
        panic("INVARIANT VIOLATION: " + msg)
    }
}
```

```go
//go:build !debug
package assert

func Invariant(ok bool, msg string) {
    // No-op in production
}
```

---

## SPIRE-Specific Contract Examples

### SVID Rotation

```go
// RotateSVID replaces the certificate with a new one.
//
// Preconditions:
// - newCertPEM must be a valid X.509 certificate
// - New certificate must not be expired
// - New certificate SPIFFE ID must match current ID
// - New certificate must be from same trust domain
//
// Postcondition: Certificate is updated, SPIFFE ID unchanged
func (ic *IdentityCredential) RotateSVID(newCertPEM []byte) error {
    // Precondition: parse new cert
    newCert, err := x509.ParseCertificate(newCertPEM)
    if err != nil {
        return fmt.Errorf("invalid certificate: %w", err)
    }

    // Precondition: not expired
    if time.Now().After(newCert.NotAfter) {
        return fmt.Errorf("%w: new certificate expired", ErrExpiredCert)
    }

    // Precondition: SPIFFE ID unchanged
    if len(newCert.URIs) == 0 {
        return errors.New("new certificate lacks SPIFFE URI")
    }
    newID, err := spiffeid.FromURI(newCert.URIs[0])
    if err != nil || newID != ic.spiffeID {
        return fmt.Errorf("%w: ID mismatch", ErrInvalidSPIFFEID)
    }

    // Update state
    oldID := ic.spiffeID
    ic.cert = newCert
    ic.certPEM = newCertPEM

    // Postcondition: SPIFFE ID unchanged
    assert.Postcondition(ic.spiffeID == oldID,
        "SPIFFE ID must not change during rotation")

    return nil
}
```

### Bundle Verification

```go
// VerifySVIDAgainstBundle verifies an SVID against this trust bundle.
//
// Preconditions:
// - svid must be non-nil
// - svid's trust domain must match bundle's domain
//
// Postcondition: if returns nil, SVID chain is valid
func (tb *TrustBundle) VerifySVIDAgainstBundle(svid *IdentityCredential) error {
    // Precondition: non-nil SVID
    if svid == nil {
        return errors.New("svid required")
    }

    // Precondition: trust domain match
    if svid.SPIFFEID().TrustDomain() != tb.domain {
        return fmt.Errorf("%w: SVID from %s, bundle for %s",
            ErrTrustDomain, svid.SPIFFEID().TrustDomain(), tb.domain)
    }

    // Build cert pool from CAs
    roots := x509.NewCertPool()
    for _, ca := range tb.cas {
        roots.AddCert(ca)
    }

    // Verify chain
    opts := x509.VerifyOptions{
        Roots:     roots,
        KeyUsages: []x509.ExtKeyUsage{x509.ExtKeyUsageAny},
    }
    chains, err := svid.Cert().Verify(opts)
    if err != nil {
        return fmt.Errorf("chain verification failed: %w", err)
    }

    // Postcondition: at least one valid chain
    assert.Postcondition(len(chains) > 0,
        "successful verification must produce at least one chain")

    return nil
}
```

---

## Testing Contracts

### Unit Tests Should Verify Postconditions

```go
func TestTrustBundle_AddCA_Postconditions(t *testing.T) {
    tb, err := NewTrustBundle(testDomain, [][]byte{validCAPEM})
    require.NoError(t, err)

    err = tb.AddCA(anotherValidCAPEM)
    require.NoError(t, err)

    // Postcondition: bundle now has 2 CAs
    assert.Len(t, tb.CAs(), 2)

    // Invariant: all are valid CAs
    for _, ca := range tb.CAs() {
        assert.True(t, ca.IsCA)
    }
}
```

### Integration Tests Should Verify Preconditions

```go
func TestIdentityCredential_Preconditions(t *testing.T) {
    tests := []struct {
        name    string
        id      string
        certPEM []byte
        domain  spiffeid.TrustDomain
        wantErr error
    }{
        {
            name:    "invalid SPIFFE ID",
            id:      "not-a-spiffe-id",
            certPEM: validCertPEM,
            domain:  testDomain,
            wantErr: ErrInvalidSPIFFEID,
        },
        {
            name:    "trust domain mismatch",
            id:      "spiffe://other.org/workload",
            certPEM: validCertPEM,
            domain:  testDomain,
            wantErr: ErrTrustDomain,
        },
        {
            name:    "expired certificate",
            id:      "spiffe://example.org/workload",
            certPEM: expiredCertPEM,
            domain:  testDomain,
            wantErr: ErrExpiredCert,
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            _, err := NewIdentityCredential(tt.id, tt.certPEM, tt.domain)
            assert.ErrorIs(t, err, tt.wantErr)
        })
    }
}
```

---

## Best Practices for SPIRE

### 1. Preconditions Are Always Active

```go
// ✅ Good: Always validate input
func ProcessSVID(svid *IdentityCredential) error {
    if svid == nil {
        return errors.New("svid required")
    }
    // ...
}

// ❌ Bad: Using assertions for input validation
func ProcessSVID(svid *IdentityCredential) error {
    assert.Invariant(svid != nil, "svid required") // Wrong!
    // ...
}
```

### 2. Use Sentinel Errors for Domain Violations

```go
var (
    ErrInvalidSPIFFEID = errors.New("invalid SPIFFE ID")
    ErrTrustDomain     = errors.New("trust domain mismatch")
    ErrExpiredCert     = errors.New("certificate expired")
)

// Allows callers to handle specific errors
if errors.Is(err, ErrTrustDomain) {
    // Handle trust domain issues
}
```

### 3. Protect Invariants with Type Design

```go
// ✅ Good: Unexported fields + controlled mutation
type TrustBundle struct {
    domain spiffeid.TrustDomain
    cas    []*x509.Certificate // Cannot be mutated directly
}

// ❌ Bad: Public fields allow invalid states
type TrustBundle struct {
    Domain spiffeid.TrustDomain
    CAs    []*x509.Certificate // Caller can set to nil or empty!
}
```

### 4. Document Contracts in Godoc

```go
// NewIdentityCredential creates a validated identity credential.
//
// Preconditions:
// - idStr must be a valid SPIFFE ID URI
// - certPEM must be a valid X.509 certificate
// - Certificate must not be expired
//
// Postconditions:
// - Returned credential has canonical SPIFFE ID
// - Certificate is parsed and stored
```

---

## Summary

| Contract Type | When | How | Example |
|--------------|------|-----|---------|
| **Precondition** | Validate input | `if` + `return error` | Check SPIFFE ID format |
| **Postcondition** | Verify output | `assert.Postcondition()` | Verify cert was parsed |
| **Invariant** | Protect state | `assert.Invariant()` | Bundle never empty |

**Key Principles:**
1. Preconditions reject invalid input early and explicitly
2. Postconditions verify your logic is correct
3. Invariants make illegal states unrepresentable
4. Build tags enable debug assertions without production overhead
5. Tests verify contracts are maintained

This approach brings Eiffel-style Design by Contract to Go in a way that's production-safe and security-appropriate for identity infrastructure.
