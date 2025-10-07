package spire

import (
	"context"
	"crypto/x509"
	"fmt"

	"github.com/pocket/hexagon/spire/internal/domain"
)

// FetchX509Bundle fetches the X.509 trust bundle certificates for the trust domain
func (c *SPIREClient) FetchX509Bundle(ctx context.Context) ([]*x509.Certificate, error) {
	// Use client timeout if no deadline set
	if _, ok := ctx.Deadline(); !ok {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, c.timeout)
		defer cancel()
	}

	// Fetch X.509 context which includes bundles
	x509Ctx, err := c.client.FetchX509Context(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch X.509 context: %w", err)
	}

	// Get bundle for our trust domain
	bundle, err := x509Ctx.Bundles.GetX509BundleForTrustDomain(x509Ctx.DefaultSVID().ID.TrustDomain())
	if err != nil {
		return nil, fmt.Errorf("failed to get X.509 bundle for trust domain %s: %w", c.trustDomain, err)
	}
	if bundle == nil {
		return nil, fmt.Errorf("%w: %s", domain.ErrTrustBundleNotFound, c.trustDomain)
	}

	// Return the CA certificates
	return bundle.X509Authorities(), nil
}
