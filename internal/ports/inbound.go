package ports

import (
	"context"
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
}

// Service represents application use cases (business logic)
// This is for demonstration purposes - showing identity-based operations
type Service interface {
	// ExchangeMessage performs an authenticated message exchange
	ExchangeMessage(ctx context.Context, from Identity, to Identity, content string) (*Message, error)
}
