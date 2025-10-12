package spire

import (
	"context"
	"crypto/x509"
	"fmt"

	"github.com/pocket/hexagon/spire/internal/domain"
	"github.com/spiffe/go-spiffe/v2/spiffeid"
)

// FetchX509Bundle returns the X.509 trust bundle for the chosen trust domain.
//
// Trust Domain Resolution (in priority order):
//  1. Uses c.trustDomain if configured
//  2. Falls back to default SVID's trust domain if present
//  3. If exactly one bundle is present, uses that trust domain
//  4. Otherwise returns error
//
// Performance Note: Prefers cached X509Source when available to avoid RPC overhead.
// Falls back to direct Workload API fetch if source is unavailable.
//
// Parameters:
//   - ctx: Context for timeout/cancellation (client timeout applied if no deadline set)
//
// Returns:
//   - Defensive copy of CA certificates (slice isolation)
//   - Error if bundle fetch fails or trust domain cannot be determined
func (c *SPIREClient) FetchX509Bundle(ctx context.Context) ([]*x509.Certificate, error) {
	// Apply client timeout only if no deadline exists and timeout is valid
	if _, ok := ctx.Deadline(); !ok && c.timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, c.timeout)
		defer cancel()
	}

	// Fetch X.509 context (cached via source if available, otherwise RPC)
	x509Ctx, err := c.client.FetchX509Context(ctx)
	if err != nil {
		return nil, fmt.Errorf("fetch X.509 context: %w", err)
	}

	// Determine target trust domain deterministically
	var td spiffeid.TrustDomain
	switch {
	case c.trustDomain != "":
		// Priority 1: Use configured trust domain
		td, err = spiffeid.TrustDomainFromString(c.trustDomain)
		if err != nil {
			return nil, fmt.Errorf("invalid configured trust domain %q: %w", c.trustDomain, err)
		}
	case x509Ctx.DefaultSVID() != nil:
		// Priority 2: Use default SVID's trust domain
		td = x509Ctx.DefaultSVID().ID.TrustDomain()
	case len(x509Ctx.Bundles.Bundles()) == 1:
		// Priority 3: If exactly one bundle present, use that trust domain
		td = x509Ctx.Bundles.Bundles()[0].TrustDomain()
	default:
		// No way to determine trust domain
		bundleCount := len(x509Ctx.Bundles.Bundles())
		return nil, fmt.Errorf("%w: unable to determine trust domain (no default SVID, %d bundles)", domain.ErrTrustBundleNotFound, bundleCount)
	}

	// Get bundle for determined trust domain
	bundle, err := x509Ctx.Bundles.GetX509BundleForTrustDomain(td)
	if err != nil {
		return nil, fmt.Errorf("get X.509 bundle for trust domain %q: %w", td.String(), err)
	}
	if bundle == nil {
		return nil, fmt.Errorf("%w: %s", domain.ErrTrustBundleNotFound, td.String())
	}

	// Return defensive copy to prevent external mutation
	orig := bundle.X509Authorities()
	certs := make([]*x509.Certificate, len(orig))
	copy(certs, orig)
	return certs, nil
}
