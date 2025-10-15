//go:build dev
// +build dev

package attestor

import (
	"context"
	"fmt"

	"github.com/pocket/hexagon/spire/internal/domain"
	"github.com/pocket/hexagon/spire/internal/ports"
)

// UnixWorkloadAttestor provides fake Unix attestation for walking skeleton / dev mode only.
// It returns pre-configured selectors based on UID without any real OS-level verification.
type UnixWorkloadAttestor struct {
	// Map of UID to selector
	uidSelectors map[int]string
}

// NewUnixWorkloadAttestor creates a new Unix workload attestor
func NewUnixWorkloadAttestor() *UnixWorkloadAttestor {
	return &UnixWorkloadAttestor{
		uidSelectors: make(map[int]string),
	}
}

// RegisterUID registers a UID with a selector pattern
func (a *UnixWorkloadAttestor) RegisterUID(uid int, selector string) {
	a.uidSelectors[uid] = selector
}

// Attest returns fake selectors for dev mode only - no real verification.
func (a *UnixWorkloadAttestor) Attest(ctx context.Context, workload *domain.Workload) ([]string, error) {
	// Validate workload
	if workload == nil {
		return nil, fmt.Errorf("%w: nil workload", domain.ErrInvalidProcessIdentity)
	}
	if workload.UID() < 0 {
		return nil, fmt.Errorf("%w: invalid UID %d", domain.ErrInvalidProcessIdentity, workload.UID())
	}

	selector, exists := a.uidSelectors[workload.UID()]
	if !exists {
		return nil, fmt.Errorf("%w: no attestation data for UID %d", domain.ErrWorkloadAttestationFailed, workload.UID())
	}

	// Return Unix-style selectors
	selectors := []string{
		selector,
		fmt.Sprintf("unix:uid:%d", workload.UID()),
		fmt.Sprintf("unix:gid:%d", workload.GID()),
	}

	return selectors, nil
}

// Verify that UnixWorkloadAttestor implements ports.WorkloadAttestor
var _ ports.WorkloadAttestor = (*UnixWorkloadAttestor)(nil)
