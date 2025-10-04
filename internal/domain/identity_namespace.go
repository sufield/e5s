package domain

// IdentityNamespace represents a unique, URI-formatted identifier for a workload or agent
// This is a minimal domain type that holds parsed identity data.
// Parsing logic is delegated to IdentityNamespaceParser port (implemented in adapters).
//
// Format: <scheme>://<trust-domain>/<path>
// Example: spiffe://example.org/host (SPIFFE-specific in adapters)
//
// Design Note: We avoid duplicating go-spiffe SDK's spiffeid.ID parsing/validation
// logic by moving that to an adapter. The domain only models the concept of an
// identity namespace as a value object with trust domain and path components.
//
// Naming: "IdentityNamespace" emphasizes the structured, formatted nature of the identity
// (scheme + domain + path) representing what is commonly known as a SPIFFE ID in SPIFFE terminology.
type IdentityNamespace struct {
	trustDomain *TrustDomain
	path        string
	uri         string // Cached string representation
}

// NewIdentityNamespaceFromComponents creates an IdentityNamespace from already-parsed components.
// This is used by the IdentityNamespaceParser adapter after validation.
// TrustDomain must not be nil, path defaults to "/" if empty.
func NewIdentityNamespaceFromComponents(trustDomain *TrustDomain, path string) *IdentityNamespace {
	if path == "" {
		path = "/"
	}
	// SPIFFE scheme hardcoded for this context; generalize in adapters if needed
	uri := "spiffe://" + trustDomain.String() + path
	return &IdentityNamespace{
		trustDomain: trustDomain,
		path:        path,
		uri:         uri,
	}
}

// String returns the IdentityNamespace as a URI string
func (i *IdentityNamespace) String() string {
	return i.uri
}

// TrustDomain returns the trust domain component
func (i *IdentityNamespace) TrustDomain() *TrustDomain {
	return i.trustDomain
}

// Path returns the path component
func (i *IdentityNamespace) Path() string {
	return i.path
}

// Equals checks if two IdentityNamespaces are equal by comparing their URI strings
func (i *IdentityNamespace) Equals(other *IdentityNamespace) bool {
	if other == nil {
		return false
	}
	return i.uri == other.uri
}

// IsInTrustDomain checks if this IdentityNamespace belongs to the given trust domain
func (i *IdentityNamespace) IsInTrustDomain(td *TrustDomain) bool {
	return i.trustDomain.Equals(td)
}
