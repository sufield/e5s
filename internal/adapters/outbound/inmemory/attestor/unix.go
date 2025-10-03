package attestor

import (
	"context"
	"fmt"

	"github.com/pocket/hexagon/spire/internal/app/ports"
)

// UnixWorkloadAttestor is an in-memory implementation of Unix workload attestation
// It attests workloads based on Unix process attributes (UID, GID, PID)
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

// Attest verifies a workload and returns its selectors
// In this in-memory implementation, we attest based on UID
func (a *UnixWorkloadAttestor) Attest(ctx context.Context, workload ports.ProcessIdentity) ([]string, error) {
	selector, exists := a.uidSelectors[workload.UID]
	if !exists {
		return nil, fmt.Errorf("no attestation selector found for UID %d", workload.UID)
	}

	// Return Unix-style selectors
	selectors := []string{
		selector,
		fmt.Sprintf("unix:uid:%d", workload.UID),
		fmt.Sprintf("unix:gid:%d", workload.GID),
	}

	return selectors, nil
}
