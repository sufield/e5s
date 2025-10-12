package spire

import (
	"context"
	"crypto/x509"
	"fmt"
	"sync"

	"github.com/pocket/hexagon/spire/internal/domain"
	"github.com/pocket/hexagon/spire/internal/ports"
)

// Server implements the ports.IdentityServer interface for SPIRE production adapter.
//
// Important: This adapter wraps the SPIRE Workload API, which has specific constraints:
// - The Workload API only returns SVIDs for the calling process (this workload)
// - It cannot issue SVIDs for arbitrary identities
// - Bundle management handles certificate rotation and federated trust domains
//
// In production SPIRE deployments, the actual SPIRE Server runs externally and manages
// the CA, certificate issuance, and attestation. This adapter fetches identities via
// the Workload API socket.
type Server struct {
	client            *SPIREClient
	trustDomain       *domain.TrustDomain
	trustDomainParser ports.TrustDomainParser

	mu             sync.RWMutex
	caCertificates []*x509.Certificate
}

// NewServer creates a new SPIRE-backed server adapter.
//
// Validates that the configured trust domain matches the client's trust domain
// to prevent silent misconfigurations.
func NewServer(
	ctx context.Context,
	client *SPIREClient,
	trustDomainStr string,
	trustDomainParser ports.TrustDomainParser,
) (*Server, error) {
	if client == nil {
		return nil, fmt.Errorf("SPIRE client cannot be nil")
	}
	if trustDomainParser == nil {
		return nil, fmt.Errorf("trust domain parser cannot be nil")
	}

	// Parse and validate trust domain
	trustDomain, err := trustDomainParser.FromString(ctx, trustDomainStr)
	if err != nil {
		return nil, fmt.Errorf("invalid trust domain: %w", err)
	}

	// Ensure trust domain matches client's configured trust domain
	if clientTD := client.GetTrustDomain(); clientTD != "" && clientTD != trustDomain.String() {
		return nil, fmt.Errorf("%w: server trust domain %q does not match client trust domain %q",
			domain.ErrInvalidTrustDomain, trustDomain.String(), clientTD)
	}

	// Fetch CA certificates (trust bundle) from SPIRE
	caCerts, err := client.FetchX509Bundle(ctx)
	if err != nil {
		return nil, fmt.Errorf("fetch CA bundle from SPIRE: %w", err)
	}

	if len(caCerts) == 0 {
		return nil, domain.ErrCANotInitialized
	}

	return &Server{
		client:            client,
		trustDomain:       trustDomain,
		trustDomainParser: trustDomainParser,
		caCertificates:    caCerts,
	}, nil
}

// IssueIdentity returns the SVID for THIS process and verifies it matches the requested credential.
//
// Important limitations:
// - The SPIRE Workload API only returns the identity for the calling process
// - This method cannot mint arbitrary identities; it can only fetch THIS workload's identity
// - If the requested identity doesn't match this process's identity, it returns an error
//
// This design is inherent to the SPIRE Workload API security model: workloads can only
// obtain their own credentials, not credentials for other workloads.
//
// For a true "issue any identity" capability, you would need to use the SPIRE Server API
// (which requires admin privileges) instead of the Workload API.
func (s *Server) IssueIdentity(ctx context.Context, identityCredential *domain.IdentityCredential) (*domain.IdentityDocument, error) {
	if identityCredential == nil {
		return nil, fmt.Errorf("%w: identity credential cannot be nil", domain.ErrIdentityDocumentInvalid)
	}

	// Verify the identity credential belongs to this trust domain
	if !identityCredential.IsInTrustDomain(s.trustDomain) {
		return nil, fmt.Errorf("%w: identity %s not in trust domain %s",
			domain.ErrIdentityDocumentInvalid, identityCredential.String(), s.trustDomain.String())
	}

	// Fetch X.509 SVID from SPIRE Workload API
	// This returns the SVID for THIS process only
	doc, err := s.client.FetchX509SVID(ctx)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", domain.ErrServerUnavailable, err)
	}

	// Verify the issued document matches the requested identity
	// This will fail if the caller requests a different identity than this process
	if !doc.IdentityCredential().Equals(identityCredential) {
		return nil, fmt.Errorf("%w: SPIRE Workload API returns SVID for this process (%s), not requested identity %s",
			domain.ErrIdentityDocumentMismatch,
			doc.IdentityCredential().String(),
			identityCredential.String())
	}

	return doc, nil
}

// GetTrustDomain returns the trust domain this server manages
func (s *Server) GetTrustDomain() *domain.TrustDomain {
	return s.trustDomain
}

// GetCA returns the first CA certificate from the trust bundle.
//
// Note: Trust bundles may contain multiple root certificates (during rotation or
// for federated trust domains). This method returns only the first certificate.
// For full bundle access, use GetCABundle() instead.
//
// This method is safe for concurrent use.
func (s *Server) GetCA() *x509.Certificate {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if len(s.caCertificates) == 0 {
		return nil
	}
	return s.caCertificates[0]
}

// GetCABundle returns a copy of the full CA certificate bundle.
//
// Trust bundles may contain multiple root certificates:
// - During CA rotation (old and new roots)
// - For federated trust domains
// - For cross-signing scenarios
//
// Returns a defensive copy to prevent external mutations.
// This method is safe for concurrent use.
func (s *Server) GetCABundle() []*x509.Certificate {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if len(s.caCertificates) == 0 {
		return nil
	}

	// Defensive copy
	bundle := make([]*x509.Certificate, len(s.caCertificates))
	copy(bundle, s.caCertificates)
	return bundle
}

// RefreshCA refreshes the CA certificates from SPIRE.
//
// This can be called periodically to update the trust bundle, ensuring
// the server has the latest root certificates (important during rotation).
//
// This method is safe for concurrent use.
func (s *Server) RefreshCA(ctx context.Context) error {
	caCerts, err := s.client.FetchX509Bundle(ctx)
	if err != nil {
		return fmt.Errorf("refresh CA certificates: %w", err)
	}

	if len(caCerts) == 0 {
		return domain.ErrCANotInitialized
	}

	s.mu.Lock()
	s.caCertificates = caCerts
	s.mu.Unlock()

	return nil
}

var _ ports.IdentityServer = (*Server)(nil)
