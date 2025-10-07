package inmemory

import (
	"context"
	"fmt"

	"github.com/pocket/hexagon/spire/internal/domain"
	"github.com/pocket/hexagon/spire/internal/ports"
)

// InMemoryAgent is an in-memory implementation of SPIRE agent
type InMemoryAgent struct {
	identityNamespace   *domain.IdentityNamespace
	trustDomain         *domain.TrustDomain
	server              *InMemoryServer
	registry            ports.IdentityMapperRegistry
	attestor            ports.WorkloadAttestor
	parser              ports.IdentityNamespaceParser
	certificateProvider ports.IdentityDocumentProvider
	agentIdentity       *ports.Identity
}

// NewInMemoryAgent creates a new in-memory SPIRE agent
func NewInMemoryAgent(
	ctx context.Context,
	agentSpiffeIDStr string,
	server *InMemoryServer,
	registry ports.IdentityMapperRegistry,
	attestor ports.WorkloadAttestor,
	parser ports.IdentityNamespaceParser,
	certProvider ports.IdentityDocumentProvider,
) (*InMemoryAgent, error) {
	// Use parser port instead of domain constructor
	identityNamespace, err := parser.ParseFromString(ctx, agentSpiffeIDStr)
	if err != nil {
		return nil, fmt.Errorf("invalid agent identity namespace: %w", err)
	}

	agent := &InMemoryAgent{
		identityNamespace:   identityNamespace,
		trustDomain:         server.GetTrustDomain(),
		server:              server,
		registry:            registry,
		attestor:            attestor,
		parser:              parser,
		certificateProvider: certProvider,
	}

	// Initialize agent's own identity
	if err := agent.initializeAgentIdentity(ctx); err != nil {
		return nil, fmt.Errorf("failed to initialize agent identity: %w", err)
	}

	return agent, nil
}

// initializeAgentIdentity creates the agent's identity document
func (a *InMemoryAgent) initializeAgentIdentity(ctx context.Context) error {
	// Use IdentityDocumentProvider port to create agent identity document (delegates document generation)
	caCert := a.server.GetCA()
	caKey := a.server.caKey

	agentDoc, err := a.certificateProvider.CreateX509IdentityDocument(ctx, a.identityNamespace, caCert, caKey)
	if err != nil {
		return fmt.Errorf("failed to create agent identity document: %w", err)
	}

	a.agentIdentity = &ports.Identity{
		IdentityNamespace: a.identityNamespace,
		Name:              "agent",
		IdentityDocument:  agentDoc,
	}

	return nil
}

// GetIdentity returns the agent's own identity
func (a *InMemoryAgent) GetIdentity(ctx context.Context) (*ports.Identity, error) {
	if a.agentIdentity == nil {
		return nil, fmt.Errorf("%w: agent identity not initialized", domain.ErrAgentUnavailable)
	}
	return a.agentIdentity, nil
}

// FetchIdentityDocument attests a workload and fetches its identity document from the server
// Runtime flow: Attest → Match (FindBySelectors) → Issue → Return
func (a *InMemoryAgent) FetchIdentityDocument(ctx context.Context, workload ports.ProcessIdentity) (*ports.Identity, error) {
	// Step 1: Attest the workload to get selectors
	selectorStrings, err := a.attestor.Attest(ctx, workload)
	if err != nil {
		return nil, fmt.Errorf("workload attestation failed: %w", err)
	}

	if len(selectorStrings) == 0 {
		return nil, domain.ErrNoAttestationData
	}

	// Step 2: Convert selector strings to SelectorSet
	selectorSet := domain.NewSelectorSet()
	for _, selStr := range selectorStrings {
		selector, err := domain.ParseSelectorFromString(selStr)
		if err != nil {
			return nil, fmt.Errorf("invalid selector %s: %w", selStr, err)
		}
		selectorSet.Add(selector)
	}

	// Step 3: Match selectors against registry (READ-ONLY operation)
	mapper, err := a.registry.FindBySelectors(ctx, selectorSet)
	if err != nil {
		return nil, fmt.Errorf("no identity mapper found for selectors: %w", err)
	}

	// Step 4: Issue identity document from server
	doc, err := a.server.IssueIdentity(ctx, mapper.IdentityNamespace())
	if err != nil {
		return nil, fmt.Errorf("failed to issue identity document: %w", err)
	}

	// Step 5: Return identity with document
	return &ports.Identity{
		IdentityNamespace: mapper.IdentityNamespace(),
		Name:              extractNameFromIdentityNamespace(mapper.IdentityNamespace()),
		IdentityDocument:  doc,
	}, nil
}

// extractNameFromIdentityNamespace extracts a human-readable name from identity namespace
func extractNameFromIdentityNamespace(id *domain.IdentityNamespace) string {
	// Extract last path segment as name
	path := id.Path()
	if path == "/" || path == "" {
		return id.TrustDomain().String()
	}
	// Remove leading slash and return
	return path[1:]
}

var _ ports.Agent = (*InMemoryAgent)(nil)
