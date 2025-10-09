package domain

// IdentityCredential represents a unique, URI-formatted identifier for a workload or agent
// This is a minimal domain type that holds parsed identity data.
// Parsing logic is delegated to IdentityCredentialParser port (implemented in adapters).
//
// Format: <scheme>://<trust-domain>/<path>
// Example: spiffe://example.org/host (SPIFFE-specific in adapters)
//
// Design Note: We avoid duplicating go-spiffe SDK's spiffeid.ID parsing/validation
// logic by moving that to an adapter. The domain only models the concept of an
// identity credential as a value object with trust domain and path components.
//
// Naming: "IdentityCredential" emphasizes the structured, formatted nature of the identity
// (scheme + domain + path) representing what is commonly known as a SPIFFE ID in SPIFFE terminology.
type IdentityCredential struct {
	trustDomain *TrustDomain
	path        string
	uri         string // Cached string representation
}

// NewIdentityCredentialFromComponents creates an IdentityCredential from already-parsed components.
// This is used by the IdentityCredentialParser adapter after validation.
// TrustDomain must not be nil, path defaults to "/" if empty.
func NewIdentityCredentialFromComponents(trustDomain *TrustDomain, path string) *IdentityCredential {
	if path == "" {
		path = "/"
	}
	// SPIFFE scheme hardcoded for this context; generalize in adapters if needed
	uri := "spiffe://" + trustDomain.String() + path
	return &IdentityCredential{
		trustDomain: trustDomain,
		path:        path,
		uri:         uri,
	}
}

// String returns the IdentityCredential as a URI string
func (i *IdentityCredential) String() string {
	return i.uri
}

// TrustDomain returns the trust domain component
func (i *IdentityCredential) TrustDomain() *TrustDomain {
	return i.trustDomain
}

// Path returns the path component
func (i *IdentityCredential) Path() string {
	return i.path
}

// Equals checks if two IdentityCredentials are equal by comparing their URI strings
func (i *IdentityCredential) Equals(other *IdentityCredential) bool {
	if other == nil {
		return false
	}
	return i.uri == other.uri
}

// IsInTrustDomain checks if this IdentityCredential belongs to the given trust domain
func (i *IdentityCredential) IsInTrustDomain(td *TrustDomain) bool {
	return i.trustDomain.Equals(td)
}
