package ports

import "context"

// Identity represents an authenticated workload identity.
// This is a port-level abstraction (domain-friendly shape)
// that is adapter-agnostic.
//
// Adapters (HTTP, gRPC, CLI) translate their protocol-specific
// authentication into this common shape and inject it into the request context.
type Identity struct {
	// SPIFFEID is the full SPIFFE ID (e.g., "spiffe://example.org/client")
	SPIFFEID string

	// TrustDomain is the trust domain portion (e.g., "example.org")
	TrustDomain string

	// Path is the path portion (e.g., "/client")
	Path string
}

// identityKey is the context key for storing Identity.
// Unexported to prevent external packages from creating their own keys.
type identityKey struct{}

// WithIdentity stores an Identity in the context.
// This should be called by inbound adapters after authentication.
func WithIdentity(ctx context.Context, id Identity) context.Context {
	return context.WithValue(ctx, identityKey{}, id)
}

// IdentityFrom retrieves the Identity from the context.
// Returns (identity, true) if present, (zero, false) otherwise.
//
// This is the primary way application handlers access authenticated identity.
// Handlers depend on ports, not on specific adapters.
func IdentityFrom(ctx context.Context) (Identity, bool) {
	id, ok := ctx.Value(identityKey{}).(Identity)
	return id, ok
}
