//go:build dev

package inmemory

import (
	"context"
	"fmt"

	"github.com/pocket/hexagon/spire/internal/domain"
	"github.com/pocket/hexagon/spire/internal/ports"
)

// InMemoryAgent is an in-memory implementation of SPIRE agent
type InMemoryAgent struct {
	identityCredential  *domain.IdentityCredential
	trustDomain         *domain.TrustDomain
	server              *InMemoryServer
	registry            ports.IdentityMapperRegistry
	attestor            ports.WorkloadAttestor
	parser              ports.IdentityCredentialParser
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
	parser ports.IdentityCredentialParser,
	certProvider ports.IdentityDocumentProvider,
) (*InMemoryAgent, error) {
	// Use parser port instead of domain constructor
	identityCredential, err := parser.ParseFromString(ctx, agentSpiffeIDStr)
	if err != nil {
		return nil, fmt.Errorf("inmemory: invalid agent identity credential: %w", err)
	}

	agent := &InMemoryAgent{
		identityCredential:  identityCredential,
		trustDomain:         server.GetTrustDomain(),
		server:              server,
		registry:            registry,
		attestor:            attestor,
		parser:              parser,
		certificateProvider: certProvider,
	}

	// Initialize agent's own identity
	if err := agent.initializeAgentIdentity(ctx); err != nil {
		return nil, fmt.Errorf("inmemory: failed to initialize agent identity: %w", err)
	}

	return agent, nil
}

// initializeAgentIdentity creates the agent's identity document
func (a *InMemoryAgent) initializeAgentIdentity(ctx context.Context) error {
	// Use IdentityDocumentProvider port to create agent identity document (delegates document generation)
	caCert := a.server.GetCA()
	caKey := a.server.caKey

	agentDoc, err := a.certificateProvider.CreateX509IdentityDocument(ctx, a.identityCredential, caCert, caKey)
	if err != nil {
		return fmt.Errorf("inmemory: failed to create agent identity document: %w", err)
	}

	a.agentIdentity = &ports.Identity{
		IdentityCredential: a.identityCredential,
		Name:               "agent",
		IdentityDocument:   agentDoc,
	}

	return nil
}

// GetIdentity returns the agent's own identity
func (a *InMemoryAgent) GetIdentity(ctx context.Context) (*ports.Identity, error) {
	if a.agentIdentity == nil {
		return nil, fmt.Errorf("inmemory: %w: agent identity not initialized", domain.ErrAgentUnavailable)
	}
	return a.agentIdentity, nil
}

// FetchIdentityDocument attests a workload and fetches its identity document from the server
// Runtime flow: Attest → Match (FindBySelectors) → Issue → Return
func (a *InMemoryAgent) FetchIdentityDocument(ctx context.Context, workload ports.ProcessIdentity) (*ports.Identity, error) {
	// Step 1: Attest the workload to get selectors
	selectorStrings, err := a.attestor.Attest(ctx, workload)
	if err != nil {
		return nil, fmt.Errorf("inmemory: workload attestation failed: %w", err)
	}

	if len(selectorStrings) == 0 {
		return nil, domain.ErrNoAttestationData
	}

	// Step 2: Convert selector strings to SelectorSet
	selectorSet := domain.NewSelectorSet()
	for _, selStr := range selectorStrings {
		selector, err := domain.ParseSelectorFromString(selStr)
		if err != nil {
			return nil, fmt.Errorf("inmemory: invalid selector %s: %w", selStr, err)
		}
		selectorSet.Add(selector)
	}

	// Step 3: Match selectors against registry (READ-ONLY operation)
	mapper, err := a.registry.FindBySelectors(ctx, selectorSet)
	if err != nil {
		return nil, fmt.Errorf("inmemory: no identity mapper found for selectors: %w", err)
	}

	// Step 4: Issue identity document from server
	doc, err := a.server.IssueIdentity(ctx, mapper.IdentityCredential())
	if err != nil {
		return nil, fmt.Errorf("inmemory: failed to issue identity document: %w", err)
	}

	// Step 5: Return identity with document
	return &ports.Identity{
		IdentityCredential: mapper.IdentityCredential(),
		Name:               extractNameFromIdentityCredential(mapper.IdentityCredential()),
		IdentityDocument:   doc,
	}, nil
}

// extractNameFromIdentityCredential extracts a human-readable name from identity credential
func extractNameFromIdentityCredential(id *domain.IdentityCredential) string {
	// Extract last path segment as name
	path := id.Path()
	if path == "/" || path == "" {
		return id.TrustDomain().String()
	}
	// Remove leading slash and return
	return path[1:]
}

var _ ports.Agent = (*InMemoryAgent)(nil)
