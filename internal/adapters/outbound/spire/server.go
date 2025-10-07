package spire

import (
	"context"
	"crypto/x509"
	"fmt"

	"github.com/pocket/hexagon/spire/internal/domain"
	"github.com/pocket/hexagon/spire/internal/ports"
)

// Server implements the ports.Server interface for SPIRE production adapter
// Note: In production SPIRE deployment, the actual SPIRE Server runs externally
// This adapter provides a minimal implementation that delegates to the external SPIRE infrastructure
type Server struct {
	client            *SPIREClient
	trustDomain       *domain.TrustDomain
	trustDomainParser ports.TrustDomainParser
	caCertificates    []*x509.Certificate
}

// NewServer creates a new SPIRE-backed server adapter
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

	// Fetch CA certificates (trust bundle) from SPIRE
	caCerts, err := client.FetchX509Bundle(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch CA certificates from SPIRE: %w", err)
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

// IssueIdentity issues an identity document for an identity namespace
// In production SPIRE, identity issuance is handled by the external SPIRE Server
// This method fetches the identity document from SPIRE via the Workload API
func (s *Server) IssueIdentity(ctx context.Context, identityNamespace *domain.IdentityNamespace) (*domain.IdentityDocument, error) {
	if identityNamespace == nil {
		return nil, fmt.Errorf("%w: identity namespace cannot be nil", domain.ErrIdentityDocumentInvalid)
	}

	// Verify the identity namespace belongs to this trust domain
	if !identityNamespace.IsInTrustDomain(s.trustDomain) {
		return nil, fmt.Errorf("%w: identity namespace %s does not belong to trust domain %s",
			domain.ErrIdentityDocumentInvalid, identityNamespace.String(), s.trustDomain.String())
	}

	// Fetch X.509 SVID from SPIRE
	// Note: The actual SPIRE Server manages the CA and signs certificates
	// We fetch the issued certificate via the Workload API
	doc, err := s.client.FetchX509SVID(ctx)
	if err != nil {
		return nil, fmt.Errorf("%w: failed to fetch identity from SPIRE: %v", domain.ErrServerUnavailable, err)
	}

	// Verify the issued document matches the requested identity
	if !doc.IdentityNamespace().Equals(identityNamespace) {
		return nil, fmt.Errorf("identity mismatch: requested %s, SPIRE issued %s",
			identityNamespace.String(), doc.IdentityNamespace().String())
	}

	return doc, nil
}

// GetTrustDomain returns the trust domain this server manages
func (s *Server) GetTrustDomain() *domain.TrustDomain {
	return s.trustDomain
}

// GetCA returns the CA certificate (root of trust)
// Returns the first CA certificate from the trust bundle
func (s *Server) GetCA() *x509.Certificate {
	if len(s.caCertificates) == 0 {
		return nil
	}
	return s.caCertificates[0]
}

// RefreshCA refreshes the CA certificates from SPIRE
// This can be called periodically to update the trust bundle
func (s *Server) RefreshCA(ctx context.Context) error {
	caCerts, err := s.client.FetchX509Bundle(ctx)
	if err != nil {
		return fmt.Errorf("failed to refresh CA certificates: %w", err)
	}

	if len(caCerts) == 0 {
		return domain.ErrCANotInitialized
	}

	s.caCertificates = caCerts
	return nil
}

var _ ports.IdentityServer = (*Server)(nil)
