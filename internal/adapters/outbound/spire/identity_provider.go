package spire

import (
	"context"
	"fmt"

	"github.com/pocket/hexagon/spire/internal/domain"
	"github.com/spiffe/go-spiffe/v2/svid/jwtsvid"
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

	// Parse SPIFFE ID to get identity namespace
	// Extract trust domain and path from SPIFFE ID
	spiffeID := svid.ID
	trustDomain := domain.NewTrustDomainFromName(spiffeID.TrustDomain().String())
	path := spiffeID.Path()
	identityNamespace := domain.NewIdentityNamespaceFromComponents(trustDomain, path)

	// Create identity document
	return domain.NewIdentityDocumentFromComponents(
		identityNamespace,
		domain.IdentityDocumentTypeX509,
		svid.Certificates[0], // Leaf certificate
		svid.PrivateKey,
		svid.Certificates, // Full chain
		svid.Certificates[0].NotAfter,
	), nil
}

// FetchJWTSVID fetches a JWT SVID from SPIRE for the given audiences
func (c *SPIREClient) FetchJWTSVID(ctx context.Context, audiences []string) (string, error) {
	if len(audiences) == 0 {
		return "", fmt.Errorf("at least one audience is required")
	}

	// Use client timeout if no deadline set
	if _, ok := ctx.Deadline(); !ok {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, c.timeout)
		defer cancel()
	}

	// Prepare JWT SVID parameters
	params := jwtsvid.Params{
		Audience: audiences[0],
	}
	if len(audiences) > 1 {
		params.ExtraAudiences = audiences[1:]
	}

	// Fetch JWT SVID from SPIRE
	svid, err := c.client.FetchJWTSVID(ctx, params)
	if err != nil {
		return "", fmt.Errorf("failed to fetch JWT SVID: %w", err)
	}

	// Return the raw JWT token
	return svid.Marshal(), nil
}

// ValidateJWTSVID validates a JWT token using SPIRE bundles
func (c *SPIREClient) ValidateJWTSVID(ctx context.Context, token string, audience string) error {
	// Use client timeout if no deadline set
	if _, ok := ctx.Deadline(); !ok {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, c.timeout)
		defer cancel()
	}

	// Fetch JWT bundles for validation
	jwtBundles, err := c.client.FetchJWTBundles(ctx)
	if err != nil {
		return fmt.Errorf("failed to fetch JWT bundles for validation: %w", err)
	}

	// Parse and validate the JWT SVID using the bundle source
	_, err = jwtsvid.ParseAndValidate(token, jwtBundles, []string{audience})
	if err != nil {
		return fmt.Errorf("JWT SVID validation failed: %w", err)
	}

	return nil
}
