package spire

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/pocket/hexagon/spire/internal/domain"
	"github.com/spiffe/go-spiffe/v2/spiffeid"
	"github.com/spiffe/go-spiffe/v2/svid/x509svid"
	"github.com/spiffe/go-spiffe/v2/workloadapi"
)

// FetchX509SVID fetches an X.509 SVID from SPIRE and converts it to an IdentityDocument.
//
// Fast Path (preferred):
//   - Uses cached X509Source if available (no RPC overhead)
//   - Source provides auto-rotating SVIDs with background refresh
//   - Falls back to RPC only if source is unavailable or fails
//
// SVID Selection Policy (deterministic, stable):
//  1. If trust domain configured → select SVID matching that TD
//     - If multiple matches → pick lexicographically smallest ID (stable)
//  2. Else if DefaultSVID available → use that
//  3. Else → pick lexicographically smallest ID from all SVIDs (fallback)
//
// Selection is deterministic to prevent non-repeatable behavior across calls.
//
// Error handling:
//   - Returns domain.ErrAgentUnavailable for nil/uninitialized client
//   - Returns domain.ErrNoAttestationData when Workload API returns no SVIDs
//   - Wraps all SDK errors with %w for errors.Is/As compatibility
//
// Validation:
//   - Delegates certificate/key validation to TranslateX509SVIDToIdentityDocument
//   - That function enforces: non-nil certs, crypto.Signer key, key-cert matching
//
// Concurrency: Safe for concurrent use. Both X509Source and Workload API client
// are safe for concurrent access.
func (c *SPIREClient) FetchX509SVID(ctx context.Context) (*domain.IdentityDocument, error) {
	// Guard clause: ensure client is initialized
	if c == nil || c.client == nil {
		return nil, fmt.Errorf("%w: SPIRE client is not initialized", domain.ErrAgentUnavailable)
	}

	// Apply client timeout only if no deadline exists and timeout is valid
	if _, ok := ctx.Deadline(); !ok && c.timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, c.timeout)
		defer cancel()
	}

	// Fast path: prefer cached source (no RPC, auto-rotating)
	// Source maintains fresh SVIDs in background; this is near-instant
	if c.source != nil {
		svid, err := c.source.GetX509SVID()
		if err == nil && svid != nil {
			// Source served a valid SVID; translate and return
			return TranslateX509SVIDToIdentityDocument(svid)
		}
		// Source failed or unavailable; fall through to RPC
		// This can happen during initial startup before first rotation
	}

	// Slow path: fetch from Workload API via RPC
	x509Ctx, err := c.client.FetchX509Context(ctx)
	if err != nil {
		return nil, fmt.Errorf("fetch X.509 context: %w", err)
	}

	// Validate context is non-nil and contains SVIDs
	if x509Ctx == nil || len(x509Ctx.SVIDs) == 0 {
		return nil, fmt.Errorf("%w: no SVIDs returned by Workload API", domain.ErrNoAttestationData)
	}

	// Select SVID using deterministic policy
	svid := selectSVID(x509Ctx, c.td)

	// Convert SVID to domain IdentityDocument
	// TranslateX509SVIDToIdentityDocument handles all validation:
	// - Non-nil certificates and leaf
	// - crypto.Signer private key
	// - Key-certificate matching
	// - Defensive slice copying
	return TranslateX509SVIDToIdentityDocument(svid)
}

// selectSVID implements deterministic SVID selection from X.509 context.
//
// Selection Policy (in priority order):
//  1. Trust domain match: If tdConfigured is non-empty, select SVID(s) with matching TD
//     - Case-insensitive comparison (per SPIFFE spec, TDs are DNS names)
//     - If multiple matches → pick lexicographically smallest ID (stable)
//     - If no matches → fall through to next policy
//  2. Default SVID: If X509Context.DefaultSVID() is present, use that
//     - Workload API may designate one SVID as default for the workload
//  3. Fallback: Pick lexicographically smallest ID from all SVIDs
//     - Ensures deterministic, repeatable selection
//
// Fast path: If only one SVID exists, return immediately (no comparison needed).
//
// Stability guarantee: For a given set of SVIDs and trust domain configuration,
// this function always returns the same SVID. This prevents flapping behavior
// in distributed systems where selection order matters.
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

	// Policy 1: Trust domain match
	if tdConfigured.String() != "" {
		var matches []*x509svid.SVID
		for _, s := range svids {
			// Case-insensitive comparison (trust domains are DNS names)
			if strings.EqualFold(s.ID.TrustDomain().String(), tdConfigured.String()) {
				matches = append(matches, s)
			}
		}

		// If exactly one match, return it
		if len(matches) == 1 {
			return matches[0]
		}

		// If multiple matches, pick lexicographically smallest (stable)
		if len(matches) > 1 {
			sort.Slice(matches, func(i, j int) bool {
				return matches[i].ID.String() < matches[j].ID.String()
			})
			return matches[0]
		}

		// No matches; fall through to default/fallback
	}

	// Policy 2: Default SVID if available
	// Workload API may designate one SVID as default for the workload
	if defaultSVID := x509Ctx.DefaultSVID(); defaultSVID != nil {
		return defaultSVID
	}

	// Policy 3: Deterministic fallback - lexicographically smallest ID
	// This ensures stable selection when no policy matches
	sorted := make([]*x509svid.SVID, len(svids))
	copy(sorted, svids)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].ID.String() < sorted[j].ID.String()
	})
	return sorted[0]
}
