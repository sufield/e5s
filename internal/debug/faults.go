package debug

import (
	"fmt"
	"sync"
)

// FaultProfile defines faults that can be injected for testing.
// All faults are one-shot (consumed after check) to ensure predictable,
// isolated test behavior and prevent accidental long-term system corruption.
type FaultProfile struct {
	mu sync.RWMutex

	// DropNextHandshake causes the next mTLS handshake to fail (one-shot)
	DropNextHandshake bool

	// CorruptNextSPIFFEID returns a malformed SPIFFE ID (one-shot)
	CorruptNextSPIFFEID bool

	// DelayNextIssueSeconds delays next identity issuance (one-shot, must be >= 0)
	DelayNextIssueSeconds int

	// ForceTrustDomainMismatch returns wrong trust domain (one-shot)
	ForceTrustDomainMismatch bool

	// ForceExpiredCert returns an expired certificate (one-shot)
	ForceExpiredCert bool

	// RejectNextWorkloadLookup makes next workload lookup fail (one-shot)
	RejectNextWorkloadLookup bool
}

// Faults is the global fault profile
var Faults = &FaultProfile{}

// SetDropNextHandshake enables/disables handshake dropping
func (f *FaultProfile) SetDropNextHandshake(enabled bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.DropNextHandshake = enabled
}

// ShouldDropHandshake checks and consumes the drop handshake flag
func (f *FaultProfile) ShouldDropHandshake() bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.DropNextHandshake {
		f.DropNextHandshake = false // One-shot
		return true
	}
	return false
}

// SetCorruptNextSPIFFEID enables/disables SPIFFE ID corruption
func (f *FaultProfile) SetCorruptNextSPIFFEID(enabled bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.CorruptNextSPIFFEID = enabled
}

// ShouldCorruptSPIFFEID checks and consumes the corrupt SPIFFE ID flag
func (f *FaultProfile) ShouldCorruptSPIFFEID() bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.CorruptNextSPIFFEID {
		f.CorruptNextSPIFFEID = false // One-shot
		return true
	}
	return false
}

// SetDelayNextIssue sets delay for next identity issuance.
// Returns an error if seconds is negative.
func (f *FaultProfile) SetDelayNextIssue(seconds int) error {
	if seconds < 0 {
		return fmt.Errorf("delay must be non-negative, got %d", seconds)
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.DelayNextIssueSeconds = seconds
	return nil
}

// GetAndClearDelay gets and clears the delay setting
func (f *FaultProfile) GetAndClearDelay() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	delay := f.DelayNextIssueSeconds
	f.DelayNextIssueSeconds = 0
	return delay
}

// SetForceTrustDomainMismatch enables/disables trust domain mismatch
func (f *FaultProfile) SetForceTrustDomainMismatch(enabled bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.ForceTrustDomainMismatch = enabled
}

// ShouldForceTrustDomainMismatch checks and consumes the trust domain mismatch flag
func (f *FaultProfile) ShouldForceTrustDomainMismatch() bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.ForceTrustDomainMismatch {
		f.ForceTrustDomainMismatch = false // One-shot
		return true
	}
	return false
}

// SetForceExpiredCert enables/disables expired certificate injection
func (f *FaultProfile) SetForceExpiredCert(enabled bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.ForceExpiredCert = enabled
}

// ShouldForceExpiredCert checks and consumes the expired cert flag
func (f *FaultProfile) ShouldForceExpiredCert() bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.ForceExpiredCert {
		f.ForceExpiredCert = false // One-shot
		return true
	}
	return false
}

// SetRejectNextWorkloadLookup enables/disables workload lookup rejection
func (f *FaultProfile) SetRejectNextWorkloadLookup(enabled bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.RejectNextWorkloadLookup = enabled
}

// ShouldRejectWorkloadLookup checks and consumes the reject lookup flag
func (f *FaultProfile) ShouldRejectWorkloadLookup() bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.RejectNextWorkloadLookup {
		f.RejectNextWorkloadLookup = false // One-shot
		return true
	}
	return false
}

// Reset clears all fault flags
func (f *FaultProfile) Reset() {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.DropNextHandshake = false
	f.CorruptNextSPIFFEID = false
	f.DelayNextIssueSeconds = 0
	f.ForceTrustDomainMismatch = false
	f.ForceExpiredCert = false
	f.RejectNextWorkloadLookup = false
}

// Snapshot returns the current state of all faults as a map.
// This is useful for debugging, logging, or exposing via HTTP endpoints.
// The snapshot is a point-in-time view and won't reflect subsequent changes.
func (f *FaultProfile) Snapshot() map[string]any {
	f.mu.RLock()
	defer f.mu.RUnlock()
	return map[string]any{
		"drop_next_handshake":         f.DropNextHandshake,
		"corrupt_next_spiffe_id":      f.CorruptNextSPIFFEID,
		"delay_next_issue_seconds":    f.DelayNextIssueSeconds,
		"force_trust_domain_mismatch": f.ForceTrustDomainMismatch,
		"force_expired_cert":          f.ForceExpiredCert,
		"reject_next_workload_lookup": f.RejectNextWorkloadLookup,
	}
}
