package spire

import (
	"context"
	"fmt"

	"github.com/pocket/hexagon/spire/internal/domain"
	"github.com/pocket/hexagon/spire/internal/ports"
)

// Agent implements the ports.Agent interface using SPIRE Workload API
// It delegates to an external SPIRE agent instead of managing identities internally
type Agent struct {
	client                  *SPIREClient
	registry                ports.IdentityMapperRegistry
	attestor                ports.WorkloadAttestor
	identityNamespaceParser ports.IdentityNamespaceParser
	agentIdentity           *ports.Identity
}

// NewAgent creates a new SPIRE-backed agent
func NewAgent(
	ctx context.Context,
	client *SPIREClient,
	agentSpiffeID string,
	registry ports.IdentityMapperRegistry,
	attestor ports.WorkloadAttestor,
	parser ports.IdentityNamespaceParser,
) (*Agent, error) {
	if client == nil {
		return nil, fmt.Errorf("SPIRE client cannot be nil")
	}
	if registry == nil {
		return nil, fmt.Errorf("registry cannot be nil")
	}
	if attestor == nil {
		return nil, fmt.Errorf("attestor cannot be nil")
	}
	if parser == nil {
		return nil, fmt.Errorf("parser cannot be nil")
	}

	// Parse agent identity namespace
	agentIdentityNamespace, err := parser.ParseFromString(ctx, agentSpiffeID)
	if err != nil {
		return nil, fmt.Errorf("failed to parse agent SPIFFE ID: %w", err)
	}

	// Fetch agent's own X.509 SVID from SPIRE
	agentDoc, err := client.FetchX509SVID(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch agent identity document: %w", err)
	}

	// Verify the fetched identity matches the configured agent identity
	if !agentDoc.IdentityNamespace().Equals(agentIdentityNamespace) {
		return nil, fmt.Errorf("agent identity mismatch: expected %s, got %s",
			agentIdentityNamespace.String(), agentDoc.IdentityNamespace().String())
	}

	// Create agent identity
	agentIdentity := &ports.Identity{
		IdentityNamespace: agentIdentityNamespace,
		IdentityDocument:  agentDoc,
	}

	return &Agent{
		client:                  client,
		registry:                registry,
		attestor:                attestor,
		identityNamespaceParser: parser,
		agentIdentity:           agentIdentity,
	}, nil
}

// GetIdentity returns the agent's own identity
func (a *Agent) GetIdentity(ctx context.Context) (*ports.Identity, error) {
	if a.agentIdentity == nil {
		return nil, fmt.Errorf("%w: agent identity not initialized", domain.ErrAgentUnavailable)
	}

	// Check if identity document is expired, refresh if needed
	if a.agentIdentity.IdentityDocument.IsExpired() {
		// Fetch fresh SVID from SPIRE
		freshDoc, err := a.client.FetchX509SVID(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to refresh agent identity: %w", err)
		}
		a.agentIdentity.IdentityDocument = freshDoc
	}

	return a.agentIdentity, nil
}

// FetchIdentityDocument fetches an identity document for a workload
// Flow: Attest workload → Match selectors in registry → Fetch SVID from SPIRE
func (a *Agent) FetchIdentityDocument(ctx context.Context, workload ports.ProcessIdentity) (*ports.Identity, error) {
	// Step 1: Attest the workload to get selectors
	selectors, err := a.attestor.Attest(ctx, workload)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", domain.ErrWorkloadAttestationFailed, err)
	}

	// Step 2: Parse selectors into SelectorSet
	selectorSet := domain.NewSelectorSet()
	for _, selectorStr := range selectors {
		selector, err := domain.ParseSelectorFromString(selectorStr)
		if err != nil {
			return nil, fmt.Errorf("invalid selector %s: %w", selectorStr, err)
		}
		selectorSet.Add(selector)
	}

	// Step 3: Find matching identity mapper in registry
	mapper, err := a.registry.FindBySelectors(ctx, selectorSet)
	if err != nil {
		return nil, err // Propagate domain error (ErrNoMatchingMapper)
	}

	// Step 4: Fetch X.509 SVID from SPIRE for the calling workload
	// IMPORTANT: SPIRE Workload API automatically determines which SVID to issue based on:
	//   1. The calling process's credentials (PID/UID extracted from Unix socket)
	//   2. Workload attestation matching against SPIRE Server's registration entries
	// We cannot request a specific SPIFFE ID - SPIRE decides based on attestation.
	// This fetch gets the SVID that SPIRE determined the workload should receive.
	doc, err := a.client.FetchX509SVID(ctx)
	if err != nil {
		return nil, fmt.Errorf("spire fetch x509 svid: %w", err)
	}

	// Step 5: Verify the issued identity matches what our registry expected
	// This validates that our local registry mapping agrees with SPIRE Server's registration
	expectedIdentity := mapper.IdentityNamespace()
	if !doc.IdentityNamespace().Equals(expectedIdentity) {
		// In production SPIRE, the SPIFFE ID in the SVID should match registration
		// If not, this indicates a configuration mismatch
		return nil, fmt.Errorf("identity mismatch: registry expects %s, SPIRE issued %s",
			expectedIdentity.String(), doc.IdentityNamespace().String())
	}

	return &ports.Identity{
		IdentityNamespace: expectedIdentity,
		IdentityDocument:  doc,
	}, nil
}

// Close closes the agent and releases resources
func (a *Agent) Close() error {
	// Agent doesn't own the client, so it doesn't close it
	return nil
}

var _ ports.Agent = (*Agent)(nil)
