package spire

import (
	"context"
	"fmt"

	"github.com/pocket/hexagon/spire/internal/ports"
)

// IdentityServiceSPIRE is the production identity service implementation.
// It satisfies ports.IdentityService using SPIRE's Workload API.
//
// This adapter wraps the lower-level SPIRE Client and translates SPIRE-specific
// types (X.509 SVIDs, domain.IdentityDocument) into the SPIRE-agnostic
// ports.Identity type used by the application layer.
//
// Design Philosophy:
//   - Application layer depends only on ports.IdentityService interface
//   - SPIRE-specific terminology (SVID, X.509) stays in adapter layer
//   - Clean separation: adapter handles messy reality, ports expose clean abstraction
//   - Testability: application code can mock ports.IdentityService without SPIRE
//
// Hexagonal Architecture:
//   - This is an OUTBOUND ADAPTER (driven adapter)
//   - Implements: ports.IdentityService
//   - Depends on: SPIRE Workload API (via Client)
//   - Used by: internal/app layer (via interface)
//
// SPIRE Context:
//   - Calls FetchX509SVID on underlying Client
//   - Handles multiple SVIDs by selecting the default (first) identity
//   - Extracts SPIFFE ID components for ports.Identity
//
// Concurrency: Safe for concurrent use (wraps thread-safe Client).
type IdentityServiceSPIRE struct {
	client *Client // SPIRE client with FetchX509SVID method
}

// NewIdentityServiceSPIRE constructs an IdentityServiceSPIRE adapter.
//
// Parameters:
//   - client: SPIRE client for fetching X.509 SVIDs (must not be nil)
//
// Returns error if client is nil (programming error).
//
// This constructor is typically called by the adapter factory
// (compose.SPIREAdapterFactory) during application bootstrap.
//
// Example:
//
//	client, err := spire.NewClient(ctx, cfg)
//	identitySvc, err := spire.NewIdentityServiceSPIRE(client)
func NewIdentityServiceSPIRE(client *Client) (*IdentityServiceSPIRE, error) {
	if client == nil {
		return nil, fmt.Errorf("spire identity service: client is nil")
	}
	return &IdentityServiceSPIRE{
		client: client,
	}, nil
}

// Current returns the caller's identity in port-friendly form.
//
// This method hides SPIRE implementation details from the application layer:
//   - Application sees ports.Identity (SPIRE-agnostic)
//   - Application never sees domain.IdentityDocument or X.509 certificates
//   - SPIRE-specific error types are passed through (ports.ErrAgentUnavailable)
//
// Identity Selection:
//   - SPIRE can return multiple SVIDs for a workload
//   - This implementation returns the default (first) identity
//   - For multi-identity workloads, extend this interface or use FetchX509SVID directly
//
// Error Handling:
//   - Returns ports.ErrAgentUnavailable if SPIRE Agent is unreachable
//   - Returns wrapped errors for validation or parsing failures
//   - Returns error if identity document is unexpectedly empty (defensive check)
//
// Example:
//
//	identity, err := identitySvc.Current(ctx)
//	if err != nil {
//	    return fmt.Errorf("fetch identity: %w", err)
//	}
//	fmt.Printf("My identity: %s\n", identity.SPIFFEID)
//
// Concurrency: Safe for concurrent use (delegates to thread-safe Client).
func (s *IdentityServiceSPIRE) Current(ctx context.Context) (ports.Identity, error) {
	// Fetch the current X.509 SVID from SPIRE Agent
	// This calls the Workload API and returns domain.IdentityDocument
	doc, err := s.client.FetchX509SVID(ctx)
	if err != nil {
		// Pass through SPIRE-specific errors (ErrAgentUnavailable, etc.)
		// Application layer can handle these without knowing SPIRE internals
		return ports.Identity{}, err
	}

	// Defensive check: ensure we got a valid identity document
	// This should never happen if FetchX509SVID succeeded, but be safe
	if doc == nil {
		return ports.Identity{}, fmt.Errorf("spire identity service: empty identity document")
	}

	// Extract the identity credential from the document
	cred := doc.IdentityCredential()
	if cred == nil {
		return ports.Identity{}, fmt.Errorf("spire identity service: identity document has no credential")
	}

	// Translate domain.IdentityCredential → ports.Identity
	// This is the anti-corruption layer: adapter translates between layers
	return ports.Identity{
		SPIFFEID:    cred.SPIFFEID(),           // Full URI: spiffe://example.org/foo
		TrustDomain: cred.TrustDomainString(),  // Just the domain: example.org
		Path:        cred.Path(),               // Just the path: /foo
	}, nil
}
