package domain

// TrustDomain represents the scope of identities issued by a SPIRE Server.
// This is a minimal domain type that holds a trust domain name.
// Parsing logic is delegated to TrustDomainParser port (implemented in adapters).
//
// It defines the namespace for SPIFFE IDs (e.g., example.org).
//
// See https://github.com/spiffe/spiffe/blob/main/standards/SPIFFE-ID.md#21-trust-domain
//
// # Design Note
//
// This deliberately does NOT use spiffeid.TrustDomain from go-spiffe SDK
// to maintain hexagonal architecture purity (domain independence from infrastructure).
// The translation adapter (in adapters/outbound/spire/translation.go) converts between
// domain.TrustDomain and spiffeid.TrustDomain as needed, serving as an anti-corruption layer.
//
// This separation ensures the domain remains:
//
//   - Infrastructure-free (no third-party dependencies)
//   - Easily testable (mock-friendly)
//   - SPIFFE-spec compliant but implementation-agnostic (could support non-SPIFFE identity systems)
//
// The go-spiffe SDK's spiffeid.TrustDomain provides:
//
//   - SPIFFE spec-compliant validation (DNS label rules, RFC 1123)
//   - Compare() method for ordering
//   - Text marshaling/unmarshaling
//   - ID() and IDString() for full SPIFFE ID construction
//
// We accept the minimal duplication (36 lines) as the cost of architectural independence.
// If validation bugs arise or custom logic exceeds 50 lines, consider revisiting this decision.
//
// # Pointer Semantics
//
// Uses *TrustDomain (pointer) to allow nil checks in domain logic
// for uninitialized states. SDK uses value type with IsZero() instead.
type TrustDomain struct {
	name string
}

// NewTrustDomainFromName creates a TrustDomain from an already-validated name.
// This is used by the TrustDomainParser adapter after validation.
// Name must not be empty.
func NewTrustDomainFromName(name string) *TrustDomain {
	return &TrustDomain{name: name}
}

// String returns the trust domain as a string (e.g., "example.org")
func (td *TrustDomain) String() string {
	return td.name
}

// Equals checks if two trust domains are equal (case-sensitive string comparison)
// Returns false if other is nil.
//
// Note: SDK's spiffeid.TrustDomain uses Compare(other) == 0 for equality.
// We provide an explicit Equals() method for clarity and consistency with
// other domain value objects (IdentityCredential, Selector).
func (td *TrustDomain) Equals(other *TrustDomain) bool {
	if other == nil {
		return false
	}
	return td.name == other.name
}

// IsZero returns true if the trust domain is uninitialized or empty.
// This is useful for validation checks in domain logic.
//
// Example usage:
//
//	func ValidateIdentity(td *TrustDomain) error {
//	    if td.IsZero() {
//	        return ErrInvalidTrustDomain
//	    }
//	    // ... continue validation
//	}
//
// Note: SDK's spiffeid.TrustDomain (value type) has IsZero() built-in.
// We provide equivalent functionality for our pointer-based design.
func (td *TrustDomain) IsZero() bool {
	return td == nil || td.name == ""
}
