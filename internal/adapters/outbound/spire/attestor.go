package spire

import (
	"context"
	"fmt"

	"github.com/pocket/hexagon/spire/internal/domain"
)

// Attest performs workload attestation for the given process ID.
// Note: SPIRE handles attestation internally via the Workload API.
// This method provides a simplified interface that returns selectors
// based on the current workload's identity.
func (c *SPIREClient) Attest(ctx context.Context, pid int32) (*domain.SelectorSet, error) {
	// Use client timeout if no deadline set
	if _, ok := ctx.Deadline(); !ok {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, c.timeout)
		defer cancel()
	}

	// Fetch X.509 context which includes workload information
	x509Ctx, err := c.client.FetchX509Context(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch X.509 context for attestation: %w", err)
	}

	// SPIRE Workload API doesn't expose PID-based attestation directly
	// Instead, it attests the calling workload automatically
	// We return selectors from the current SVID
	if len(x509Ctx.SVIDs) == 0 {
		return nil, fmt.Errorf("no SVIDs available - workload not attested")
	}

	svid := x509Ctx.SVIDs[0]

	// Build selectors from the SVID
	selectors := []*domain.Selector{}

	spiffeIDSel, err := domain.NewSelector(domain.SelectorTypeWorkload, "spiffe_id", svid.ID.String())
	if err != nil {
		return nil, fmt.Errorf("failed to create spiffe_id selector: %w", err)
	}
	selectors = append(selectors, spiffeIDSel)

	trustDomainSel, err := domain.NewSelector(domain.SelectorTypeWorkload, "trust_domain", svid.ID.TrustDomain().String())
	if err != nil {
		return nil, fmt.Errorf("failed to create trust_domain selector: %w", err)
	}
	selectors = append(selectors, trustDomainSel)

	// Add PID selector if provided
	if pid > 0 {
		pidSel, err := domain.NewSelector(domain.SelectorTypeWorkload, "pid", fmt.Sprintf("%d", pid))
		if err != nil {
			return nil, fmt.Errorf("failed to create pid selector: %w", err)
		}
		selectors = append(selectors, pidSel)
	}

	return domain.NewSelectorSet(selectors...), nil
}
