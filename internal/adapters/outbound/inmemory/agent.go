package inmemory

import (
	"context"
	"fmt"

	"github.com/pocket/hexagon/spire/internal/app/ports"
	"github.com/pocket/hexagon/spire/internal/domain"
)

// InMemoryAgent is an in-memory implementation of SPIRE agent
type InMemoryAgent struct {
	identityNamespace      *domain.IdentityNamespace
	trustDomain         *domain.TrustDomain
	server              *InMemoryServer
	store               *InMemoryStore
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
	store *InMemoryStore,
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
		identityNamespace:      identityNamespace,
		trustDomain:         server.GetTrustDomain(),
		server:              server,
		store:               store,
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
		IdentityNamespace:   a.identityNamespace,
		Name:             "agent",
		IdentityDocument: agentDoc,
	}

	return nil
}

// GetIdentity returns the agent's own identity
func (a *InMemoryAgent) GetIdentity(ctx context.Context) (*ports.Identity, error) {
	return a.agentIdentity, nil
}

// FetchIdentityDocument attests a workload and fetches its identity document from the server
func (a *InMemoryAgent) FetchIdentityDocument(ctx context.Context, workload ports.ProcessIdentity) (*ports.Identity, error) {
	// Step 1: Attest the workload using the attestor
	selectors, err := a.attestor.Attest(ctx, workload)
	if err != nil {
		return nil, fmt.Errorf("workload attestation failed: %w", err)
	}

	if len(selectors) == 0 {
		return nil, fmt.Errorf("no selectors returned for workload")
	}

	// Step 2: Look up the identity from the store using the selector
	identity, err := a.store.GetIdentityBySelector(ctx, selectors[0])
	if err != nil {
		return nil, fmt.Errorf("failed to find identity for selector %s: %w", selectors[0], err)
	}

	// Step 3: Request identity document from server using domain identity namespace
	doc, err := a.server.IssueIdentity(ctx, identity.IdentityNamespace)
	if err != nil {
		return nil, fmt.Errorf("failed to issue identity document: %w", err)
	}

	identity.IdentityDocument = doc
	return identity, nil
}
