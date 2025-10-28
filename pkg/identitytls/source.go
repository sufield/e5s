package identitytls

import (
	"context"
	"crypto/tls"
	"crypto/x509"
)

// CertSource provides certificates and trust bundles for mTLS.
//
// This interface abstracts the identity provider (SPIRE, Vault, test fixtures, etc.)
// from the TLS configuration logic. Implementations handle certificate rotation,
// trust bundle updates, and connection lifecycle.
//
// IMPORTANT: Create one Source per process (or per trust domain) and share it
// across all TLS configs (servers and clients). Do not create per-request sources.
// The source is typically shared across multiple TLS configs and automatically
// rotates certificates before expiration.
//
// Performance contract:
//
//	GetTLSCertificate and GetRootCAs will be called on the TLS handshake path.
//	Implementations MUST return quickly (serve from in-memory state) and MUST NOT
//	perform blocking network or filesystem I/O in these methods.
//	Long-running updates (watching SPIRE Workload API, refreshing bundles, etc.)
//	should happen in the background so these calls are cheap.
//
// Context usage:
//
//	During TLS handshake we pass context.Background(). Implementations MUST NOT
//	rely on ctx deadlines for correctness in those code paths.
//
// Certificate requirements:
//
//	GetTLSCertificate MUST set Leaf (the parsed leaf x509 cert).
//	NewServerTLSConfig depends on Leaf being populated to extract the server's
//	SPIFFE ID and trust domain during startup. If Leaf is nil, NewServerTLSConfig
//	will fail fast.
//
//	Leaf is also used later for identity extraction/authorization without
//	re-parsing cert bytes. All CertSource implementations MUST populate it.
//
// Lifecycle: Call Close() when done to release resources (connections, watchers, etc.).
type CertSource interface {
	// GetTLSCertificate returns the current certificate and private key for mTLS.
	//
	// This is called during TLS handshakes to present identity. Implementations
	// must return fresh certificates if rotation has occurred since the last call.
	//
	// The returned tls.Certificate MUST have its Leaf field populated with the
	// parsed X.509 certificate.
	//
	// Returns error if certificate is unavailable or expired.
	GetTLSCertificate(ctx context.Context) (tls.Certificate, error)

	// GetRootCAs returns the trust bundle for verifying peer certificates.
	//
	// This is called during TLS handshakes to build the certificate verification chain.
	// Implementations must return fresh trust bundles if they have been updated.
	//
	// Returns error if trust bundle is unavailable.
	GetRootCAs(ctx context.Context) (*x509.CertPool, error)

	// Close releases resources held by the source.
	//
	// Close must be safe to call while handshakes are in flight. After Close()
	// returns, future GetTLSCertificate/GetRootCAs calls must fail, but in-flight
	// handshakes may still succeed.
	//
	// Idempotent: safe to call multiple times.
	Close() error
}
