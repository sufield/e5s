package spire

import (
	"context"
	"fmt"

	"github.com/pocket/hexagon/spire/internal/domain"
	"github.com/pocket/hexagon/spire/internal/ports"
)

// Agent implements the ports.Agent interface by delegating to external SPIRE
// This agent does NOT do local selector matching or attestation.
// It fully delegates to SPIRE Server's registration entries and workload attestation.
type Agent struct {
	client              *SPIREClient
	credentialParser    ports.IdentityCredentialParser
	agentIdentity       *ports.Identity
	agentSpiffeID       string
}

// NewAgent creates a new SPIRE agent that fully delegates to external SPIRE
func NewAgent(
	ctx context.Context,
	client *SPIREClient,
	agentSpiffeID string,
	parser ports.IdentityCredentialParser,
) (*Agent, error) {
	if client == nil {
		return nil, fmt.Errorf("SPIRE client cannot be nil")
	}
	if parser == nil {
		return nil, fmt.Errorf("parser cannot be nil")
	}
	if agentSpiffeID == "" {
		return nil, fmt.Errorf("agent SPIFFE ID cannot be empty")
	}

	// Parse agent identity credential
	agentIdentityCredential, err := parser.ParseFromString(ctx, agentSpiffeID)
	if err != nil {
		return nil, fmt.Errorf("failed to parse agent SPIFFE ID: %w", err)
	}

	// Fetch agent's own X.509 SVID from SPIRE
	agentDoc, err := client.FetchX509SVID(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch agent SVID: %w", err)
	}

	return &Agent{
		client:           client,
		credentialParser: parser,
		agentIdentity: &ports.Identity{
			IdentityCredential: agentIdentityCredential,
			IdentityDocument:   agentDoc,
			Name:              "spire-agent",
		},
		agentSpiffeID: agentSpiffeID,
	}, nil
}

// GetIdentity returns the agent's own identity
func (a *Agent) GetIdentity(ctx context.Context) (*ports.Identity, error) {
	// Refresh identity document if expired
	if a.agentIdentity.IdentityDocument.IsExpired() {
		freshDoc, err := a.client.FetchX509SVID(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to refresh agent identity: %w", err)
		}
		a.agentIdentity.IdentityDocument = freshDoc
	}

	return a.agentIdentity, nil
}

// FetchIdentityDocument fetches an identity document for a workload
// PRODUCTION MODE: Fully delegates to SPIRE Agent/Server
//
// IMPORTANT: This does NOT do local attestation or selector matching.
// SPIRE Server performs:
//   1. Workload attestation (via SPIRE Agent)
//   2. Selector matching against registration entries (in SPIRE Server)
//   3. SVID issuance for the matched identity
//
// This agent simply requests the SVID from SPIRE Workload API, which handles everything.
func (a *Agent) FetchIdentityDocument(ctx context.Context, workload ports.ProcessIdentity) (*ports.Identity, error) {
	// In production, we delegate EVERYTHING to SPIRE:
	// - SPIRE Agent performs workload attestation (extracts selectors from the calling process)
	// - SPIRE Server matches selectors against registration entries
	// - SPIRE Server issues the appropriate SVID
	// - SPIRE Agent returns it via Workload API

	// Fetch X.509 SVID from SPIRE Workload API
	// The Workload API will:
	//   1. Authenticate the calling process (via Unix domain socket peer credentials)
	//   2. Request attestation from SPIRE Server
	//   3. Get the appropriate SVID based on SPIRE Server's registration entries
	//   4. Return the SVID to us
	doc, err := a.client.FetchX509SVID(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch workload SVID from SPIRE: %w", err)
	}

	// Extract the identity credential from the document
	identityCredential := doc.IdentityCredential()

	// Create identity with the SPIRE-issued document
	identity := &ports.Identity{
		IdentityCredential: identityCredential,
		IdentityDocument:   doc,
		Name:              extractNameFromCredential(identityCredential),
	}

	return identity, nil
}

// extractNameFromCredential extracts a human-readable name from identity credential
func extractNameFromCredential(credential *domain.IdentityCredential) string {
	if credential == nil {
		return "unknown"
	}

	// Use the path component as the name (e.g., "/workload" → "workload")
	path := credential.Path()
	if len(path) > 1 && path[0] == '/' {
		return path[1:] // Remove leading slash
	}
	return path
}

// Verify Agent implements ports.Agent
var _ ports.Agent = (*Agent)(nil)
