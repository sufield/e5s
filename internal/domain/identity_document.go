package domain

import (
	"crypto/x509"
	"fmt"
	"time"

	"github.com/pocket/hexagon/spire/internal/assert"
)

// IdentityDocument represents an immutable X.509-based verifiable identity document (SVID).
//
// Components:
//   - Identity credential: The SPIFFE ID for this workload/agent
//   - Certificate: X.509 leaf certificate
//   - Chain: Certificate chain (leaf-first)
//
// Design Philosophy:
//   - Immutable: All fields validated once at construction, never modified
//   - Defensive: Chain is copied on read/write to prevent aliasing bugs
//   - Clock skew aware: Provides IsCurrentlyValid(skew) for production time handling
//   - Key management: Private keys are managed by adapters, not stored in domain model
//
// Certificate Validity:
//   - ExpiresAt() derived from cert.NotAfter (single source of truth, no drift)
//   - NotBefore() derived from cert.NotBefore
//   - IsCurrentlyValid(skew) handles clock skew between systems
//
// Creation and validation delegated to IdentityDocumentProvider port (implemented in adapters).
// The domain only models the concept; adapters handle crypto/parsing using go-spiffe SDK.
//
// Note: This implementation is X.509-only. JWT SVIDs are not supported.
//
// Concurrency: Safe for concurrent use (immutable value object).
type IdentityDocument struct {
	identityCredential *IdentityCredential
	cert               *x509.Certificate
	chain              []*x509.Certificate // Leaf-first (cert), then intermediates
}

// NewIdentityDocumentFromComponents creates a validated, immutable identity document.
//
// This is typically called by IdentityDocumentProvider adapters after SDK validation.
// The constructor performs final validation and ensures immutable, correct construction.
//
// Validation:
//   - identityCredential must be non-nil
//   - cert must be non-nil (leaf certificate)
//   - chain is normalized to be leaf-first (cert, then intermediates)
//   - Defensive copy of chain to prevent aliasing
//
// Chain Normalization:
//   - If chain is empty or doesn't start with cert, rebuilt as [cert, ...intermediates]
//   - If chain starts with cert, defensive copy is made
//   - Ensures leaf-first invariant for SDK compatibility
//
// Parameters:
//   - identityCredential: The SPIFFE ID for this identity
//   - cert: X.509 leaf certificate
//   - chain: Certificate chain (may or may not include leaf)
//
// Returns:
//   - IdentityDocument instance on success
//   - ErrIdentityDocumentInvalid (wrapped with %w) if validation fails
//
// Examples:
//
//	// Chain includes leaf (common from SDK)
//	doc, err := NewIdentityDocumentFromComponents(id, leaf, []*x509.Certificate{leaf, ca})
//
//	// Chain doesn't include leaf (will be added)
//	doc, err := NewIdentityDocumentFromComponents(id, leaf, []*x509.Certificate{ca})
//
//	// No intermediates (leaf-only chain)
//	doc, err := NewIdentityDocumentFromComponents(id, leaf, nil)
//
// Concurrency: Safe for concurrent use (pure function, no shared state).
//
// Note: The expiresAt parameter has been removed (breaking change).
// Use cert.NotAfter instead to avoid drift. Adapters should update their calls.
func NewIdentityDocumentFromComponents(
	identityCredential *IdentityCredential,
	cert *x509.Certificate,
	chain []*x509.Certificate,
) (*IdentityDocument, error) {
	// Step 1: Validate required components
	if identityCredential == nil {
		return nil, fmt.Errorf("%w: identity credential cannot be nil", ErrIdentityDocumentInvalid)
	}
	if cert == nil {
		return nil, fmt.Errorf("%w: certificate cannot be nil", ErrIdentityDocumentInvalid)
	}

	// Step 2: Normalize chain to be leaf-first
	// Ensures invariant: chain[0] == cert (leaf)
	// Performs defensive copy to prevent aliasing bugs
	var normalizedChain []*x509.Certificate

	if len(chain) == 0 || chain[0] != cert {
		// Chain missing leaf or incorrect order
		// Rebuild as [leaf, ...intermediates]
		normalizedChain = make([]*x509.Certificate, 1, 1+len(chain))
		normalizedChain[0] = cert
		normalizedChain = append(normalizedChain, chain...)
	} else {
		// Chain already leaf-first; defensive copy
		normalizedChain = make([]*x509.Certificate, len(chain))
		copy(normalizedChain, chain)
	}

	doc := &IdentityDocument{
		identityCredential: identityCredential,
		cert:               cert,
		chain:              normalizedChain,
	}

	// Invariant: Verify chain normalization logic (leaf-first invariant)
	// This catches bugs in the normalization logic above, not caller errors
	assert.Invariant(len(doc.chain) >= 1 && doc.chain[0] == cert,
		"chain must be non-empty with leaf certificate at position 0 (normalization bug)")

	return doc, nil
}

// IdentityCredential returns the SPIFFE ID for this identity.
//
// The identity credential is treated as immutable per the domain's immutability contract.
//
// Example:
//
//	id.IdentityCredential().String() // "spiffe://example.org/workload"
func (id *IdentityDocument) IdentityCredential() *IdentityCredential {
	return id.identityCredential
}

// Certificate returns the leaf X.509 certificate.
//
// The leaf certificate contains the SPIFFE ID in its URI SAN extension
// and is the certificate presented during TLS handshakes.
//
// Example:
//
//	cert := id.Certificate()
//	fmt.Println(cert.Subject.CommonName)
func (id *IdentityDocument) Certificate() *x509.Certificate {
	return id.cert
}

// Chain returns a defensive copy of the certificate chain (leaf-first).
//
// The chain includes the leaf certificate at position 0, followed by intermediates.
// A defensive copy is returned to prevent callers from modifying the internal slice.
//
// Immutability contract:
//   - Returned slice is a copy (callers can modify without affecting document)
//   - Certificate pointers remain shared (x509.Certificate is immutable)
//
// Example:
//
//	chain := id.Chain()
//	leaf := chain[0]          // Always the leaf certificate
//	intermediates := chain[1:] // CA certificates
func (id *IdentityDocument) Chain() []*x509.Certificate {
	out := make([]*x509.Certificate, len(id.chain))
	copy(out, id.chain)
	return out
}

// LeafAndChain returns the leaf certificate and a defensive copy of the intermediates.
//
// This is a convenience method that separates the leaf from the chain for
// operations that need them separately (e.g., TLS Config where leaf and
// intermediates are set via different fields).
//
// Returns:
//   - leaf: The X.509 leaf certificate (same as Certificate())
//   - intermediates: Defensive copy of intermediate CA certificates (may be empty)
//
// Example:
//
//	leaf, intermediates := id.LeafAndChain()
//	tlsConfig.Certificates = []tls.Certificate{{
//	    Certificate: append([][]byte{leaf.Raw}, intermediateDERs...),
//	    PrivateKey:  privateKey,  // Managed by adapter
//	}}
func (id *IdentityDocument) LeafAndChain() (*x509.Certificate, []*x509.Certificate) {
	if len(id.chain) <= 1 {
		return id.cert, nil
	}
	rest := make([]*x509.Certificate, len(id.chain)-1)
	copy(rest, id.chain[1:])
	return id.cert, rest
}

// ExpiresAt returns the expiration time from the leaf certificate's NotAfter field.
//
// This is the single source of truth for expiration. No separate expiresAt field
// is stored to avoid drift between certificate and cached timestamp.
//
// Example:
//
//	expires := id.ExpiresAt()
//	fmt.Printf("Expires: %s\n", expires.Format(time.RFC3339))
func (id *IdentityDocument) ExpiresAt() time.Time {
	return id.cert.NotAfter
}

// NotBefore returns the validity start time from the leaf certificate.
//
// The certificate is not valid before this time. Use IsValidAt() or
// IsCurrentlyValid() for proper time-based validity checks.
//
// Example:
//
//	notBefore := id.NotBefore()
//	if time.Now().Before(notBefore) {
//	    // Certificate not yet valid
//	}
func (id *IdentityDocument) NotBefore() time.Time {
	return id.cert.NotBefore
}

// Remaining returns the time duration until expiration from now.
//
// Returns negative duration if already expired.
//
// Example:
//
//	remaining := id.Remaining()
//	if remaining < 5*time.Minute {
//	    // Rotate soon
//	}
func (id *IdentityDocument) Remaining() time.Duration {
	return time.Until(id.ExpiresAt())
}

// IsExpired reports whether the document is past its NotAfter time.
//
// This is a simple time comparison without clock skew handling.
// For production use with clock skew tolerance, use IsCurrentlyValid(skew).
//
// Note: This method calls time.Now() internally. For testability,
// use IsExpiredAt(t) which accepts an explicit time parameter.
//
// Example:
//
//	if id.IsExpired() {
//	    // Need to rotate certificate
//	}
func (id *IdentityDocument) IsExpired() bool {
	return id.IsExpiredAt(time.Now())
}

// IsExpiredAt reports whether the document is past its NotAfter time at a given time.
//
// This method allows injecting the clock for testing and avoids time.Now()
// dependency in domain logic. Use this in tests to control time.
//
// Parameters:
//   - t: The time to check expiration against
//
// Returns:
//   - true if t is after the document's expiration time
//   - false if t is before or equal to the expiration time
//
// Example:
//
//	// In tests
//	testTime := time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)
//	if id.IsExpiredAt(testTime) {
//	    // Document expired at test time
//	}
func (id *IdentityDocument) IsExpiredAt(t time.Time) bool {
	return t.After(id.ExpiresAt())
}

// IsValid checks if the identity document is currently valid (not expired).
//
// This is a convenience method equivalent to !IsExpired().
// For production use with clock skew tolerance, use IsCurrentlyValid(skew).
//
// Example:
//
//	if id.IsValid() {
//	    // Certificate is valid
//	}
func (id *IdentityDocument) IsValid() bool {
	return !id.IsExpired()
}

// IsValidAt checks if the document is valid at a specific time.
//
// Returns true if t is within the certificate's NotBefore/NotAfter window.
// This does NOT account for clock skew; use IsCurrentlyValid(skew) for that.
//
// Validity window: NotBefore <= t < NotAfter
//
// Example:
//
//	// Check if valid 1 hour from now
//	future := time.Now().Add(1 * time.Hour)
//	if id.IsValidAt(future) {
//	    // Still valid in 1 hour
//	}
func (id *IdentityDocument) IsValidAt(t time.Time) bool {
	return !t.Before(id.NotBefore()) && t.Before(id.ExpiresAt())
}

// IsCurrentlyValid checks validity with clock skew allowance.
//
// This is the production-recommended validity check as it handles clock drift
// between systems (common in distributed environments).
//
// Clock Skew Handling:
//   - NotBefore: Allow documents valid up to skew duration in the future
//   - NotAfter: Allow documents expired up to skew duration ago
//
// Example with 5-minute skew:
//
//	NotBefore: 10:00, NotAfter: 11:00, skew: 5min
//	- 09:55: Valid (within skew of NotBefore)
//	- 10:30: Valid (within validity window)
//	- 11:04: Valid (within skew of NotAfter)
//	- 11:06: Invalid (beyond skew tolerance)
//
// Recommended skew values:
//   - Development: 1 * time.Minute
//   - Production: 5 * time.Minute
//   - High-latency networks: 10 * time.Minute
//
// Example:
//
//	// Production with 5-minute clock skew tolerance
//	if id.IsCurrentlyValid(5 * time.Minute) {
//	    // Certificate is valid accounting for clock drift
//	}
func (id *IdentityDocument) IsCurrentlyValid(skew time.Duration) bool {
	now := time.Now()
	return !now.Before(id.NotBefore().Add(-skew)) && now.Before(id.ExpiresAt().Add(skew))
}

// IsZero reports whether this document is uninitialized or invalid.
//
// Returns true if:
//   - Document is nil
//   - Identity credential is nil
//   - Certificate is nil
//
// Use this to detect zero-value instances or programming errors.
//
// Example:
//
//	var doc *IdentityDocument
//	if doc.IsZero() {
//	    // Need to fetch identity document
//	}
func (id *IdentityDocument) IsZero() bool {
	return id == nil || id.identityCredential == nil || id.cert == nil
}
