package ports

import (
	"context"
	"crypto/tls"
)

// IdentityClient is the main entrypoint for workloads to fetch their SVID
// This interface matches go-spiffe SDK's workloadapi.Client for seamless transition
//
// Implementation note: The server-side adapter extracts calling process credentials
// from the connection (Unix socket peer credentials) - not passed as parameter
//
// SDK compatibility: Signature matches github.com/spiffe/go-spiffe/v2/workloadapi.Client
// When migrating to real SDK, inject *workloadapi.Client wrapped to implement this interface
type IdentityClient interface {
	// FetchX509SVID fetches an X.509 SVID for the calling workload
	// Returns the workload's identity document (SVID) after attestation
	// Matches: (*workloadapi.Client).FetchX509SVID(ctx) (*x509svid.SVID, error)
	FetchX509SVID(ctx context.Context) (*Identity, error)

	// FetchX509SVIDWithConfig fetches an X.509 SVID with custom TLS configuration
	// Enables mTLS authentication when connecting to the Workload API server
	// The tlsConfig parameter allows specifying client certificates for mutual authentication
	// If tlsConfig is nil, returns an error (tlsConfig is required for this method)
	FetchX509SVIDWithConfig(ctx context.Context, tlsConfig *tls.Config) (*Identity, error)
}

// CLI is the inbound port for CLI orchestration.
// Adapters implement this to drive use cases via command-line.
type CLI interface {
	Run(ctx context.Context) error
}

// IdentityClientService is the server-side service for issuing SVIDs
// This is called by the Workload API server adapter after extracting caller credentials
type IdentityClientService interface {
	// FetchX509SVIDForCaller issues an SVID for a caller after credential extraction
	FetchX509SVIDForCaller(ctx context.Context, callerIdentity ProcessIdentity) (*Identity, error)
}

// X509SVIDResponse is the response format for X.509 SVID requests from the Workload API
type X509SVIDResponse interface {
	GetSPIFFEID() string
	GetX509SVID() string
	GetExpiresAt() int64
}

// WorkloadAPIClient is the client interface for workloads to fetch their SVID
// This matches the outbound workloadapi.Client adapter
type WorkloadAPIClient interface {
	// FetchX509SVID fetches an X.509 SVID for the calling workload
	FetchX509SVID(ctx context.Context) (X509SVIDResponse, error)

	// FetchX509SVIDWithConfig fetches an X.509 SVID with custom TLS configuration
	FetchX509SVIDWithConfig(ctx context.Context, tlsConfig *tls.Config) (X509SVIDResponse, error)
}

// WorkloadAPIServer is the server-side interface for the Workload API
// This is implemented by the inbound workloadapi server adapter
type WorkloadAPIServer interface {
	// Start starts the Workload API server
	Start(ctx context.Context) error

	// Stop stops the Workload API server
	Stop(ctx context.Context) error
}

// Service represents application use cases (business logic)
// This is for demonstration purposes - showing identity-based operations
type Service interface {
	// ExchangeMessage performs an authenticated message exchange
	ExchangeMessage(ctx context.Context, from Identity, to Identity, content string) (*Message, error)
}
