//go:build dev

package inmemory

import (
	"context"
	"fmt"

	"github.com/pocket/hexagon/spire/internal/domain"
	"github.com/pocket/hexagon/spire/internal/dto"
	"github.com/pocket/hexagon/spire/internal/ports"
)

// InMemoryAgent is an in-memory implementation of SPIRE agent.
//
// NOTE: This is used ONLY for the CLI dev demo (cmd/main.go).
// It is NOT used for HTTP mTLS examples - those use production SPIRE Workload API.
//
// For "two HTTP services using mTLS with X.509 SVIDs from SPIFFE Workload API",
// use the production identityserver adapter which connects to real SPIRE.
type InMemoryAgent struct {
	identityCredential  *domain.IdentityCredential
	trustDomain         *domain.TrustDomain
	server              *InMemoryServer
	registry            ports.IdentityMapperRegistry
	attestor            ports.WorkloadAttestor
	parser              ports.IdentityCredentialParser
	certificateProvider ports.IdentityDocumentProvider
	agentIdentity       *dto.Identity
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
	// Validate required deps early (fail fast on mis-wiring in dev)
	if server == nil || registry == nil || attestor == nil || parser == nil || certProvider == nil {
		return nil, fmt.Errorf("inmemory: nil dependency (server/registry/attestor/parser/certProvider)")
	}

	identityCredential, err := parser.ParseFromString(ctx, agentSpiffeIDStr)
	if err != nil {
		return nil, fmt.Errorf("inmemory: invalid agent identity credential: %w", err)
	}

	// Enforce trust-domain parity (agent ↔ server)
	if identityCredential.TrustDomain().String() != server.GetTrustDomain().String() {
		return nil, fmt.Errorf("inmemory: trust domain mismatch: agent=%s server=%s",
			identityCredential.TrustDomain(), server.GetTrustDomain())
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

	if err := agent.initializeAgentIdentity(ctx); err != nil {
		return nil, fmt.Errorf("inmemory: failed to initialize agent identity: %w", err)
	}

	return agent, nil
}

// initializeAgentIdentity creates the agent's identity document
func (a *InMemoryAgent) initializeAgentIdentity(ctx context.Context) error {
	// Guard CA material before creating agent's document
	if a.server.GetCA() == nil || a.server.caKey == nil {
		return fmt.Errorf("inmemory: missing CA materials for agent identity")
	}

	agentDoc, err := a.certificateProvider.CreateX509IdentityDocument(
		ctx, a.identityCredential, a.server.GetCA(), a.server.caKey)
	if err != nil {
		return fmt.Errorf("inmemory: failed to create agent identity document: %w", err)
	}

	a.agentIdentity = &dto.Identity{
		IdentityCredential: a.identityCredential,
		Name:               "agent",
		IdentityDocument:   agentDoc,
	}

	return nil
}

// GetIdentity returns the agent's own identity document.
func (a *InMemoryAgent) GetIdentity(ctx context.Context) (*domain.IdentityDocument, error) {
	if a.agentIdentity == nil {
		return nil, fmt.Errorf("inmemory: %w: agent identity not initialized", ports.ErrAgentUnavailable)
	}
	return a.agentIdentity.IdentityDocument, nil
}

// FetchIdentityDocument attests a workload and fetches its identity document from the server
// Runtime flow: Attest → Match (FindBySelectors) → Issue → Return
func (a *InMemoryAgent) FetchIdentityDocument(ctx context.Context, workload *domain.Workload) (*domain.IdentityDocument, error) {
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
			return nil, fmt.Errorf("inmemory: invalid selector %q: %w", selStr, err)
		}
		selectorSet.Add(selector)
	}

	// Step 3: Match selectors against registry
	mapper, err := a.registry.FindBySelectors(ctx, selectorSet)
	if err != nil {
		return nil, fmt.Errorf("inmemory: no identity mapper found for selectors: %w", err)
	}
	// Handle impossible nil from registry lookup (defensive)
	if mapper == nil || mapper.IdentityCredential() == nil {
		return nil, fmt.Errorf("inmemory: registry returned no mapper for selectors")
	}

	// Step 4: Issue identity document from server
	doc, err := a.server.IssueIdentity(ctx, mapper.IdentityCredential())
	if err != nil {
		return nil, fmt.Errorf("inmemory: failed to issue identity document: %w", err)
	}

	// Step 5: Return identity document
	return doc, nil
}

// Close releases resources held by the agent
// For in-memory implementation, this is a no-op but required by ports.Agent interface
func (a *InMemoryAgent) Close() error {
	// In-memory agent has no resources to release (no sockets, watchers, etc.)
	return nil
}

// extractNameFromIdentityCredential extracts a human-readable name from identity credential
func extractNameFromIdentityCredential(id *domain.IdentityCredential) string {
	p := id.Path()
	if len(p) == 0 || p == "/" {
		return id.TrustDomain().String()
	}
	if p[0] == '/' && len(p) > 1 {
		return p[1:]
	}
	return p
}

var _ ports.Agent = (*InMemoryAgent)(nil)
