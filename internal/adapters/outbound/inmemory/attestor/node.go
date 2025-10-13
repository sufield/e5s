//go:build !production
// +build !production

package attestor

import (
	"context"
	"fmt"

	"github.com/pocket/hexagon/spire/internal/domain"
	"github.com/pocket/hexagon/spire/internal/ports"
)

// InMemoryNodeAttestor is a trivial dev-only attestor.
// - No concurrency guards (dev only).
// - No platform calls.
// - No defaults: selectors must be registered explicitly.
type InMemoryNodeAttestor struct {
	trustDomain   string
	nodeSelectors map[string][]*domain.Selector // keyed by identityCredential.String()
}

func NewInMemoryNodeAttestor(trustDomain string) *InMemoryNodeAttestor {
	return &InMemoryNodeAttestor{
		trustDomain:   trustDomain,
		nodeSelectors: make(map[string][]*domain.Selector),
	}
}

// RegisterNodeSelectors sets selectors for the given node SPIFFE ID string.
// Overwrites any existing entry. Dev-only; no validation beyond non-empty ID.
func (a *InMemoryNodeAttestor) RegisterNodeSelectors(spiffeID string, selectors []*domain.Selector) {
	if spiffeID == "" {
		return
	}
	a.nodeSelectors[spiffeID] = selectors
}

// AttestNode returns a Node marked attested with pre-registered selectors.
// Fails if credential is nil, trust domain mismatches, or no selectors registered.
func (a *InMemoryNodeAttestor) AttestNode(ctx context.Context, cred *domain.IdentityCredential) (*domain.Node, error) {
	if cred == nil {
		return nil, domain.ErrInvalidIdentityCredential
	}
	if cred.TrustDomain().String() != a.trustDomain {
		return nil, fmt.Errorf("node trust domain mismatch: expected %s, got %s",
			a.trustDomain, cred.TrustDomain().String())
	}

	sels := a.nodeSelectors[cred.String()]
	if len(sels) == 0 {
		return nil, fmt.Errorf("no selectors registered for %s", cred.String())
	}

	node := domain.NewNode(cred)

	set := domain.NewSelectorSet()
	for _, s := range sels {
		set.Add(s)
	}
	node.SetSelectors(set)
	node.MarkAttested()

	return node, nil
}

var _ ports.NodeAttestor = (*InMemoryNodeAttestor)(nil)
