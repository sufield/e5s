//go:build debug

package spire

import (
	"context"
	"time"

	"github.com/sufield/e5s/internal/debug"
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
//   - MUST NOT include private keys, raw certificates, or sensitive data
//   - Errors are surfaced as synthetic AuthDecision entries with Decision: "ERROR"
//
// Concurrency: Safe for concurrent use (delegates to thread-safe Client).
func (s *IdentityServiceSPIRE) SnapshotData(ctx context.Context) debug.Snapshot {
	// debug.Active is initialized once at startup (debug.Init()) and then treated as read-only.
	snapshot := debug.Snapshot{
		Mode:            debug.Active.Mode, // Configured via SPIRE_DEBUG_MODE env var
		Adapter:         "spire",           // Using real SPIRE (not inmemory)
		Certs:           []debug.CertView{},
		RecentDecisions: []debug.AuthDecision{},
	}

	// Attempt to fetch current identity document
	doc, err := s.client.FetchX509SVID(ctx)
	if err != nil {
		// Don't overload TrustDomain with error messages.
		// Instead, surface error as synthetic AuthDecision with Decision: "ERROR"
		// This keeps the schema stable and allows clients to parse errors properly.
		snapshot.RecentDecisions = append(snapshot.RecentDecisions, debug.AuthDecision{
			CallerSPIFFEID: "",
			Resource:       "spire.FetchX509SVID",
			Decision:       "ERROR",
			Reason:         err.Error(),
		})
		return snapshot
	}

	// Extract trust domain and certificate info if we got a valid document
	if doc != nil {
		cred := doc.IdentityCredential()
		if cred != nil {
			snapshot.TrustDomain = cred.TrustDomainString()

			// Time until expiration in whole seconds (negative if already expired).
			expiresInSeconds := int64(time.Until(doc.ExpiresAt()).Seconds())

			snapshot.Certs = append(snapshot.Certs, debug.CertView{
				SpiffeID:         cred.SPIFFEID(),
				ExpiresInSeconds: expiresInSeconds,
				// TODO: Plumb real rotation status if available from SPIRE
				RotationPending: false, // SPIRE handles rotation transparently
			})
		}
	}

	return snapshot
}
