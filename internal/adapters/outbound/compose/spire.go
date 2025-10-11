package compose

import (
	"context"
	"fmt"

	"github.com/pocket/hexagon/spire/internal/adapters/outbound/inmemory"
	"github.com/pocket/hexagon/spire/internal/adapters/outbound/spire"
	"github.com/pocket/hexagon/spire/internal/ports"
)

// SPIREAdapterFactory provides production SPIRE implementations of adapters
// Uses external SPIRE infrastructure instead of in-memory implementations
type SPIREAdapterFactory struct {
	config *spire.Config
	client *spire.SPIREClient
}

// NewSPIREAdapterFactory creates the factory for production SPIRE adapters
func NewSPIREAdapterFactory(ctx context.Context, cfg *spire.Config) (*SPIREAdapterFactory, error) {
	if cfg == nil {
		return nil, fmt.Errorf("config cannot be nil")
	}

	// Create SPIRE Workload API client
	client, err := spire.NewSPIREClient(ctx, *cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create SPIRE client: %w", err)
	}

	return &SPIREAdapterFactory{
		config: cfg,
		client: client,
	}, nil
}

// CreateTrustDomainParser creates a trust domain parser
// Uses in-memory parser which wraps go-spiffe SDK
func (f *SPIREAdapterFactory) CreateTrustDomainParser() ports.TrustDomainParser {
	return inmemory.NewInMemoryTrustDomainParser()
}

// CreateIdentityCredentialParser creates an identity credential parser
// Uses in-memory parser which wraps go-spiffe SDK
func (f *SPIREAdapterFactory) CreateIdentityCredentialParser() ports.IdentityCredentialParser {
	return inmemory.NewInMemoryIdentityCredentialParser()
}

// CreateIdentityDocumentProvider creates an identity document provider
// For production, document creation is handled by SPIRE Server
// We use in-memory provider for validation logic
func (f *SPIREAdapterFactory) CreateIdentityDocumentProvider() ports.IdentityDocumentProvider {
	return inmemory.NewInMemoryIdentityDocumentProvider()
}

// CreateServer creates a SPIRE-backed server
func (f *SPIREAdapterFactory) CreateServer(
	ctx context.Context,
	trustDomain string,
	trustDomainParser ports.TrustDomainParser,
	docProvider ports.IdentityDocumentProvider,
) (ports.IdentityServer, error) {
	return spire.NewServer(ctx, f.client, trustDomain, trustDomainParser)
}

// CreateAgent creates a SPIRE-backed production agent
// This agent fully delegates to external SPIRE for registry and attestation
func (f *SPIREAdapterFactory) CreateAgent(
	ctx context.Context,
	spiffeID string,
	server ports.IdentityServer,
	registry ports.IdentityMapperRegistry,
	attestorInterface ports.WorkloadAttestor,
	parser ports.IdentityCredentialParser,
	docProvider ports.IdentityDocumentProvider,
) (ports.Agent, error) {
	// Use Agent which delegates everything to SPIRE
	// Registry and attestor parameters are ignored (they're nil in production)
	return spire.NewAgent(ctx, f.client, spiffeID, parser)
}

// Close closes the SPIRE client connection
func (f *SPIREAdapterFactory) Close() error {
	if f.client != nil {
		return f.client.Close()
	}
	return nil
}

// Verify SPIREAdapterFactory implements only CoreAdapterFactory (not the full composite)
var _ ports.CoreAdapterFactory = (*SPIREAdapterFactory)(nil)
