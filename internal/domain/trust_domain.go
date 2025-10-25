package domain

import (
	"encoding/json"
	"fmt"
	"strings"
)

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
// We accept the minimal duplication as the cost of architectural independence.
// If validation bugs arise or custom logic grows significantly, consider revisiting this decision.
//
// # Pointer Semantics
//
// Uses *TrustDomain (pointer) to allow nil checks in domain logic
// for uninitialized states. SDK uses value type with IsZero() instead.
//
// # Canonical Form
//
// SPIFFE treats trust domains case-insensitively per RFC 1123 DNS rules.
// We store the canonical lowercase form once to avoid subtle mismatches.
type TrustDomain struct {
	name string // canonical, lowercase
}

// NewTrustDomainFromName creates a TrustDomain from an already-validated name.
// This is used by the TrustDomainParser adapter after validation.
//
// For production safety, we apply minimal defensive checks:
// - Trim whitespace
// - Reject empty strings
// - Canonicalize to lowercase (SPIFFE trust domains are case-insensitive)
//
// Panics if name is empty after trimming, as this indicates a programming error
// where validation was bypassed.
func NewTrustDomainFromName(name string) *TrustDomain {
	name = strings.TrimSpace(name)
	if name == "" {
		// Keep domain self-protecting even if an adapter misuses it.
		panic(fmt.Errorf("%w: trust domain name cannot be empty", ErrInvalidTrustDomain))
	}

	return &TrustDomain{name: strings.ToLower(name)}
}

// String returns the trust domain as a string (e.g., "example.org").
// Safe on nil receiver (returns empty string instead of panicking).
//
// This is critical for production code where fmt.Sprintf("%s", td) must never panic.
func (td *TrustDomain) String() string {
	if td == nil {
		return ""
	}
	return td.name
}

// Equals checks if two trust domains are equal.
// Safe on nil receiver (returns false).
//
// Since we store the canonical lowercase form, this is a simple string comparison.
//
// Note: SDK's spiffeid.TrustDomain uses Compare(other) == 0 for equality.
// We provide an explicit Equals() method for clarity and consistency with
// other domain value objects (IdentityCredential, Selector).
func (td *TrustDomain) Equals(other *TrustDomain) bool {
	if td == nil || other == nil {
		return false
	}
	return td.name == other.name
}

// Compare provides a deterministic ordering for trust domains.
// Returns -1 if td < other, 0 if equal, 1 if td > other.
// Safe on nil receiver (nil sorts before non-nil).
//
// Useful for sorting trust domains in sets/maps or for stable iteration order.
func (td *TrustDomain) Compare(other *TrustDomain) int {
	switch {
	case td == nil && other == nil:
		return 0
	case td == nil:
		return -1
	case other == nil:
		return 1
	}
	if td.name < other.name {
		return -1
	}
	if td.name > other.name {
		return 1
	}
	return 0
}

// IsZero returns true if the trust domain is uninitialized or empty.
// Safe on nil receiver.
//
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

// Key returns a stable string key for use in maps/sets.
// Safe on nil receiver (returns empty string).
func (td *TrustDomain) Key() string {
	return td.String()
}

// MarshalText implements encoding.TextMarshaler for logging/config output.
// Returns error if trust domain is empty or nil.
//
// Note: We intentionally do NOT implement UnmarshalText to keep parsing
// logic in the adapter layer (anti-corruption layer pattern).
func (td *TrustDomain) MarshalText() ([]byte, error) {
	if td == nil || td.name == "" {
		return nil, fmt.Errorf("%w: empty trust domain", ErrInvalidTrustDomain)
	}
	return []byte(td.name), nil
}

// MarshalJSON implements json.Marshaler using the text form.
// Returns error if trust domain is empty or nil.
func (td *TrustDomain) MarshalJSON() ([]byte, error) {
	b, err := td.MarshalText()
	if err != nil {
		return nil, err
	}
	return json.Marshal(string(b))
}
