package spire

import (
	"context"
	"crypto/x509"
	"fmt"

	"github.com/spiffe/go-spiffe/v2/spiffeid"
	"github.com/spiffe/go-spiffe/v2/workloadapi"

	"github.com/sufield/e5s/internal/domain"
)

// FetchX509Bundle returns the X.509 trust bundle for the chosen trust domain.
//
// Trust Domain Resolution (in priority order):
//  1. Uses configured trust domain if set
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
func (c *Client) FetchX509Bundle(ctx context.Context) ([]*x509.Certificate, error) {
	// Apply client timeout only if no deadline exists and timeout is valid
	if _, hasDeadline := ctx.Deadline(); !hasDeadline && c.timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, c.timeout)
		defer cancel()
	}

	// Fetch X.509 context (cached via source if available, otherwise RPC)
	x509Ctx, err := c.client.FetchX509Context(ctx)
	if err != nil {
		return nil, fmt.Errorf("fetch X.509 context: %w", err)
	}
	if x509Ctx == nil || x509Ctx.Bundles == nil {
		return nil, fmt.Errorf("%w: no bundles in X.509 context", domain.ErrTrustBundleNotFound)
	}

	// Determine which trust domain to use
	td, err := c.resolveTrustDomain(x509Ctx)
	if err != nil {
		return nil, err
	}

	// Get bundle for determined trust domain using SDK helper
	bundle, err := x509Ctx.Bundles.GetX509BundleForTrustDomain(td)
	if err != nil || bundle == nil {
		return nil, fmt.Errorf("%w: %s", domain.ErrTrustBundleNotFound, td.String())
	}

	// Return defensive copy to prevent external mutation
	orig := bundle.X509Authorities()
	certs := make([]*x509.Certificate, len(orig))
	copy(certs, orig)
	return certs, nil
}

// resolveTrustDomain determines which trust domain to use for bundle selection.
//
// Resolution Priority (see FetchX509Bundle docstring):
//  1. Explicit configuration (c.trustDomain)
//  2. Default SVID's trust domain
//  3. Single bundle present → use that bundle's trust domain
//  4. Cannot determine → error
//
// This helper is extracted for testability and clarity of selection logic.
//
// Parameters:
//   - x509Ctx: X.509 context from Workload API (must be non-nil with non-nil Bundles)
//
// Returns:
//   - Resolved trust domain
//   - Error if trust domain cannot be determined or configured TD is invalid
func (c *Client) resolveTrustDomain(x509Ctx *workloadapi.X509Context) (spiffeid.TrustDomain, error) {
	// Priority 1: Use configured trust domain if present
	if c.trustDomain.String() != "" {
		// Already normalized during construction in NewClient
		// But validate it's still well-formed (defensive)
		td, err := spiffeid.TrustDomainFromString(c.trustDomain.String())
		if err != nil {
			return spiffeid.TrustDomain{}, fmt.Errorf("%w: invalid configured trust domain %q: %v",
				domain.ErrInvalidTrustDomain, c.trustDomain.String(), err)
		}
		return td, nil
	}

	// Priority 2: Use default SVID's trust domain
	if svid := x509Ctx.DefaultSVID(); svid != nil {
		return svid.ID.TrustDomain(), nil
	}

	// Priority 3: If exactly one bundle available, use that trust domain
	set := x509Ctx.Bundles
	if set.Len() == 1 {
		bundles := set.Bundles()
		if len(bundles) > 0 && bundles[0] != nil {
			return bundles[0].TrustDomain(), nil
		}
	}

	// Priority 4: Cannot determine trust domain
	return spiffeid.TrustDomain{}, fmt.Errorf("%w: unable to determine trust domain (no default SVID, %d bundles)",
		domain.ErrTrustBundleNotFound, x509Ctx.Bundles.Len())
}
