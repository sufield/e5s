package spire

import (
	"context"
	"fmt"

	"github.com/spiffe/go-spiffe/v2/spiffeid"
	"github.com/spiffe/go-spiffe/v2/svid/x509svid"
	"github.com/spiffe/go-spiffe/v2/workloadapi"

	"github.com/pocket/hexagon/spire/internal/domain"
	"github.com/pocket/hexagon/spire/internal/ports"
)

// FetchX509SVID fetches an X.509 SVID and converts it to a domain IdentityDocument.
//
// Fast Path (preferred):
//   - Uses cached X509Source if available (no RPC overhead)
//   - Source provides auto-rotating SVIDs with background refresh
//   - If trust domain is configured and default SVID doesn't match, falls back to RPC selection
//
// SVID Selection Policy (deterministic, stable):
//  1. If trust domain configured → select SVID matching that TD
//     - If multiple matches → pick lexicographically smallest ID (stable)
//  2. Else if DefaultSVID available → use that
//  3. Else → pick lexicographically smallest ID from all SVIDs (fallback)
//
// Error handling:
//   - Returns ports.ErrAgentUnavailable for nil/uninitialized client
//   - Returns domain.ErrNoAttestationData when Workload API returns no SVIDs
//   - Wraps all SDK errors with %w for errors.Is/As compatibility
//
// Concurrency: Safe for concurrent use. Both X509Source and Workload API client
// are safe for concurrent access.
func (c *Client) FetchX509SVID(ctx context.Context) (*domain.IdentityDocument, error) {
	// Guard: ensure client is initialized
	if c == nil || c.client == nil {
		return nil, fmt.Errorf("%w: SPIRE client is not initialized", ports.ErrAgentUnavailable)
	}

	// Defensive: handle nil context
	if ctx == nil {
		ctx = context.Background()
	}

	// Apply timeout if needed
	if _, ok := ctx.Deadline(); !ok && c.timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, c.timeout)
		defer cancel()
	}

	// Fast path: cached source (default SVID)
	if c.source != nil {
		if svid, err := c.source.GetX509SVID(); err == nil && svid != nil {
			// If trust domain is configured and default SVID's TD doesn't match,
			// fall back to RPC so we can select the correct one
			if c.trustDomain.String() == "" || svid.ID.TrustDomain() == c.trustDomain {
				return TranslateX509SVIDToIdentityDocument(svid)
			}
			// else: continue to RPC selection below
		}
	}

	// Slow path: RPC context fetch for full selection
	x509Ctx, err := c.client.FetchX509Context(ctx)
	if err != nil {
		return nil, fmt.Errorf("fetch X.509 context: %w", err)
	}
	if x509Ctx == nil || len(x509Ctx.SVIDs) == 0 {
		return nil, fmt.Errorf("%w: no SVIDs returned by Workload API", domain.ErrNoAttestationData)
	}

	svid := selectSVID(x509Ctx, c.trustDomain)
	return TranslateX509SVIDToIdentityDocument(svid)
}

// selectSVID chooses one SVID deterministically:
//  1. If tdConfigured is non-zero, choose an SVID in that TD
//     - If multiple, choose lexicographically smallest by ID string (single pass, no allocation)
//  2. Else, use DefaultSVID if present
//  3. Else, choose lexicographically smallest across all SVIDs (single pass, no allocation)
//
// Optimization: Avoids allocations and sorting by finding the smallest in a single pass.
//
// Parameters:
//   - x509Ctx: X.509 context from Workload API (must be non-nil with len(SVIDs) > 0)
//   - tdConfigured: Configured trust domain (value type, safe to pass zero value)
//
// Returns: Selected SVID (never nil given valid input preconditions).
func selectSVID(x509Ctx *workloadapi.X509Context, tdConfigured spiffeid.TrustDomain) *x509svid.SVID {
	svids := x509Ctx.SVIDs

	// Fast path: single SVID
	if len(svids) == 1 {
		return svids[0]
	}

	// Policy 1: trust-domain match (no allocations)
	// Type-safe comparison: spiffeid.TrustDomain is a value type
	if tdConfigured.String() != "" {
		var best *x509svid.SVID
		var bestStr string
		for _, s := range svids {
			if s.ID.TrustDomain() == tdConfigured {
				idStr := s.ID.String()
				if best == nil || idStr < bestStr {
					best, bestStr = s, idStr
				}
			}
		}
		if best != nil {
			return best
		}
		// No matches; fall through to default/fallback
	}

	// Policy 2: default SVID, if available
	if def := x509Ctx.DefaultSVID(); def != nil {
		return def
	}

	// Policy 3: lexicographically smallest across all (single pass, no allocation)
	var best *x509svid.SVID
	var bestStr string
	for _, s := range svids {
		idStr := s.ID.String()
		if best == nil || idStr < bestStr {
			best, bestStr = s, idStr
		}
	}
	return best
}
