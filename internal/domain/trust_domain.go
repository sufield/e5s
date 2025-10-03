package domain

// TrustDomain represents the scope of identities issued by a SPIRE Server
// This is a minimal domain type that holds a trust domain name.
// Parsing logic is delegated to TrustDomainParser port (implemented in adapters).
//
// It defines the namespace for SPIFFE IDs (e.g., example.org)
//
// Design Note: We avoid duplicating go-spiffe SDK's spiffeid.TrustDomain validation
// logic (DNS label validation, case-insensitive equality, etc.) by moving that to an adapter.
// The domain only models the concept of a trust domain as a value object.
type TrustDomain struct {
	name string
}

// NewTrustDomainFromName creates a TrustDomain from an already-validated name.
// This is used by the TrustDomainParser adapter after validation.
// Name must not be empty.
func NewTrustDomainFromName(name string) *TrustDomain {
	return &TrustDomain{name: name}
}

// String returns the trust domain as a string
func (td *TrustDomain) String() string {
	return td.name
}

// Equals checks if two trust domains are equal (case-sensitive)
// For case-insensitive equality, use TrustDomainParser validation
func (td *TrustDomain) Equals(other *TrustDomain) bool {
	if other == nil {
		return false
	}
	return td.name == other.name
}
