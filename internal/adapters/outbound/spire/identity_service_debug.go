//go:build debug

package spire

import (
	"context"
	"time"

	"github.com/pocket/hexagon/spire/internal/debug"
)

// Ensure IdentityServiceSPIRE implements debug.Introspector in debug builds.
// This compile-time assertion verifies the interface is satisfied.
var _ debug.Introspector = (*IdentityServiceSPIRE)(nil)

// SnapshotData returns a sanitized view of the current SPIRE identity state.
//
// This method is only available in debug builds (via //go:build debug tag).
// It provides a safe view of identity information for debugging without
// exposing secrets like private keys or raw certificate data.
//
// WARNING: This endpoint should NEVER be exposed in production builds.
// The build tag ensures it's compiled out of production binaries.
//
// Implementation notes:
//   - Fetches current X.509 SVID from SPIRE Agent
//   - Calculates certificate expiration time (not the raw cert)
//   - Returns only public identity information (SPIFFE IDs, expiration times)
//   - Does NOT include private keys, raw certificates, or sensitive data
//
// Concurrency: Safe for concurrent use (delegates to thread-safe Client).
func (s *IdentityServiceSPIRE) SnapshotData(ctx context.Context) debug.Snapshot {
	snapshot := debug.Snapshot{
		Mode:            "debug",        // Indicates this is a debug build
		Adapter:         "spire",        // Using real SPIRE (not inmemory)
		Certs:           []debug.CertView{},
		RecentDecisions: []debug.AuthDecision{}, // SPIRE identity service doesn't track auth decisions
	}

	// Attempt to fetch current identity document
	doc, err := s.client.FetchX509SVID(ctx)
	if err != nil {
		// If we can't fetch the SVID, return partial snapshot with error info
		// Don't panic - debug endpoints should be resilient
		snapshot.TrustDomain = "error: " + err.Error()
		return snapshot
	}

	// Extract trust domain if we got a valid document
	if doc != nil {
		cred := doc.IdentityCredential()
		if cred != nil {
			snapshot.TrustDomain = cred.TrustDomainString()

			// Add certificate view with expiration info
			// Calculate time until expiration (negative if already expired)
			expiresIn := time.Until(doc.ExpiresAt()).Seconds()

			snapshot.Certs = append(snapshot.Certs, debug.CertView{
				SpiffeID:         cred.SPIFFEID(),
				ExpiresInSeconds: int64(expiresIn),
				RotationPending:  false, // SPIRE handles rotation transparently
			})
		}
	}

	return snapshot
}
