package attestor

import (
	"context"
	"fmt"

	"github.com/pocket/hexagon/spire/internal/domain"
	"github.com/pocket/hexagon/spire/internal/ports"
)

// InMemoryNodeAttestor is an in-memory implementation of node attestation
// In a real SPIRE deployment, this would use platform-specific attestation:
// - AWS: EC2 Instance Identity Document
// - GCP: Instance Identity Token
// - Azure: Managed Service Identity
// - TPM: TPM-based attestation
// - Join Token: Pre-shared token for initial bootstrap
//
// For the walking skeleton, this provides hardcoded attestation for demonstration
type InMemoryNodeAttestor struct {
	trustDomain string
	// nodeSelectors maps node SPIFFE IDs to their platform selectors
	nodeSelectors map[string][]*domain.Selector
}

// NewInMemoryNodeAttestor creates a new in-memory node attestor
func NewInMemoryNodeAttestor(trustDomain string) *InMemoryNodeAttestor {
	return &InMemoryNodeAttestor{
		trustDomain:   trustDomain,
		nodeSelectors: make(map[string][]*domain.Selector),
	}
}

// RegisterNodeSelectors registers platform selectors for a node
// In real SPIRE, these would be discovered during attestation (e.g., AWS region, instance tags)
// The spiffeID parameter is the string representation of the node's IdentityNamespace
func (a *InMemoryNodeAttestor) RegisterNodeSelectors(spiffeID string, selectors []*domain.Selector) {
	a.nodeSelectors[spiffeID] = selectors
}

// AttestNode performs in-memory node attestation
// Returns a domain.Node with selectors populated and marked as attested
func (a *InMemoryNodeAttestor) AttestNode(ctx context.Context, identityNamespace *domain.IdentityNamespace) (*domain.Node, error) {
	if identityNamespace == nil {
		return nil, fmt.Errorf("node identity namespace cannot be nil")
	}

	// Verify the node belongs to the correct trust domain
	if identityNamespace.TrustDomain().String() != a.trustDomain {
		return nil, fmt.Errorf("node trust domain mismatch: expected %s, got %s",
			a.trustDomain, identityNamespace.TrustDomain().String())
	}

	// Create unattested node
	node := domain.NewNode(identityNamespace)

	// In real SPIRE, this is where platform-specific attestation happens:
	// 1. Agent provides attestation data (e.g., AWS IID signature)
	// 2. Server validates attestation data with platform (e.g., AWS API)
	// 3. Server extracts platform selectors (e.g., aws:instance-id:i-1234, aws:region:us-east-1)

	// For in-memory walking skeleton, use pre-registered selectors or defaults
	selectors := a.nodeSelectors[identityNamespace.String()]
	if len(selectors) == 0 {
		// Default selectors for demonstration (node-level selectors)
		defaultSelector, _ := domain.NewSelector(
			domain.SelectorTypeNode,
			"hostname",
			"localhost",
		)
		selectors = []*domain.Selector{defaultSelector}
	}

	// Build selector set
	selectorSet := domain.NewSelectorSet()
	for _, sel := range selectors {
		selectorSet.Add(sel)
	}

	// Set selectors and mark as attested
	node.SetSelectors(selectorSet)
	node.MarkAttested()

	return node, nil
}

// Ensure InMemoryNodeAttestor implements the port
var _ ports.NodeAttestor = (*InMemoryNodeAttestor)(nil)
