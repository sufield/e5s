package spire

import (
	"context"
	"fmt"

	"github.com/pocket/hexagon/spire/internal/domain"
)

// FetchX509SVID fetches an X.509 SVID from SPIRE and converts it to an IdentityDocument
func (c *SPIREClient) FetchX509SVID(ctx context.Context) (*domain.IdentityDocument, error) {
	// Use client timeout if no deadline set
	if _, ok := ctx.Deadline(); !ok {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, c.timeout)
		defer cancel()
	}

	// Fetch X.509 SVID from SPIRE
	x509Ctx, err := c.client.FetchX509Context(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch X.509 context: %w", err)
	}

	// Get default SVID (first one)
	if len(x509Ctx.SVIDs) == 0 {
		return nil, domain.ErrNoAttestationData
	}

	svid := x509Ctx.SVIDs[0]

	// Parse SPIFFE ID to get identity credential
	// Extract trust domain and path from SPIFFE ID
	spiffeID := svid.ID
	trustDomain := domain.NewTrustDomainFromName(spiffeID.TrustDomain().String())
	path := spiffeID.Path()
	identityCredential := domain.NewIdentityCredentialFromComponents(trustDomain, path)

	// Create identity document
	return domain.NewIdentityDocumentFromComponents(
		identityCredential,
		svid.Certificates[0], // Leaf certificate
		svid.PrivateKey,
		svid.Certificates, // Full chain
		svid.Certificates[0].NotAfter,
	), nil
}
