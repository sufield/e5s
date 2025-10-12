package compose

import (
	"context"
	"fmt"

	"github.com/pocket/hexagon/spire/internal/adapters/outbound/spire"
	"github.com/pocket/hexagon/spire/internal/ports"
	"github.com/spiffe/go-spiffe/v2/spiffeid"
)

// SPIREAdapterFactory provides production SPIRE implementations of adapters.
// This factory composes:
//   - SPIRE infrastructure components (Client, Server, Agent) for heavy operations
//   - SDK-based parsers (TrustDomainParser, IdentityCredentialParser) for validation
//   - Lightweight validator (IdentityDocumentProvider) for certificate checks
//
// Design Note: In production SPIRE deployments:
//   - Certificate creation happens on SPIRE Server (not in agent/workload)
//   - Certificate validation uses basic checks (time, identity match)
//   - Future: Implement full chain verification using x509svid.Verify from go-spiffe SDK
//
// The factory implements ports.CoreAdapterFactory for production use.
type SPIREAdapterFactory struct {
	config *spire.Config
	client *spire.SPIREClient
}

// NewSPIREAdapterFactory creates the factory for production SPIRE adapters.
//
// This factory connects to external SPIRE infrastructure (SPIRE Agent via Workload API)
// and provides adapters that delegate to SPIRE for identity operations.
//
// IMPORTANT: The provided ctx must not be already canceled. The client's internal
// sources are long-lived and will be closed via Close(). Prefer using a long-lived
// context or context.Background() if you intend the factory to outlive the calling function.
//
// Example:
//
//	factory, err := compose.NewSPIREAdapterFactory(ctx, &spire.Config{
//	    SocketPath: "/tmp/spire-agent/public/api.sock",
//	})
//	if err != nil {
//	    return fmt.Errorf("failed to create factory: %w", err)
//	}
//	defer factory.Close()
//
//	parser := factory.CreateTrustDomainParser()
//	trustDomain, err := parser.FromString(ctx, "example.org")
//
// Parameters:
//   - ctx: Context for client initialization (must not be canceled; client is long-lived)
//   - cfg: SPIRE client configuration (socket path, timeout)
//
// Returns:
//   - *SPIREAdapterFactory: The factory for creating production adapters
//   - error: Non-nil if config is invalid or SPIRE client creation fails
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
// Uses SPIRE SDK-based parser for production-grade validation
func (f *SPIREAdapterFactory) CreateTrustDomainParser() ports.TrustDomainParser {
	return spire.NewTrustDomainParser()
}

// CreateIdentityCredentialParser creates an identity credential parser
// Uses SPIRE SDK-based parser for production-grade validation
func (f *SPIREAdapterFactory) CreateIdentityCredentialParser() ports.IdentityCredentialParser {
	return spire.NewIdentityCredentialParser()
}

// CreateIdentityDocumentProvider creates an identity document provider.
//
// Design Note: In production SPIRE deployments, this provider is used only for
// validation (time checks, identity matching). Document **creation** happens on
// SPIRE Server, not in the agent/workload.
//
// This implementation uses the go-spiffe SDK's x509svid.Verify for production-grade validation:
//   - Full chain-of-trust verification against trust bundles
//   - Signature validation
//   - Certificate expiration checks
//   - SPIFFE ID extraction and validation from URI SAN
//
// The bundle source is obtained from the SPIRE Workload API client, which maintains
// up-to-date trust bundles including federated trust domains.
//
// IMPORTANT: Requires f.client to implement x509bundle.Source via GetX509BundleForTrustDomain.
//
// Returns an SDK-based provider with full security validation for production use.
func (f *SPIREAdapterFactory) CreateIdentityDocumentProvider() ports.IdentityDocumentProvider {
	// Runtime safety check: ensure the client implements the bundle source interface
	// This protects against future changes to SPIREClient that might break the contract
	if f.client != nil {
		if _, ok := any(f.client).(interface {
			GetX509BundleForTrustDomain(spiffeid.TrustDomain) (interface{}, error)
		}); !ok {
			// This should never happen in production; fail fast with clear message
			panic("SPIREClient no longer implements x509bundle.Source; update factory wiring")
		}
	}

	// Use SDK-based validator with bundle source from Workload API
	// SPIREClient implements x509bundle.Source interface via GetX509BundleForTrustDomain
	return spire.NewSDKDocumentProvider(f.client)
}

// CreateServer creates a SPIRE-backed identity server.
//
// The server delegates to external SPIRE Server for certificate issuance.
// In production, this connects to SPIRE Server via the Workload API client.
//
// Parameters:
//   - ctx: Context for server initialization
//   - trustDomain: Trust domain this server manages (e.g., "example.org")
//   - trustDomainParser: Parser for trust domain validation
//   - docProvider: Document provider (IGNORED; SPIRE Server handles issuance; server only fetches/validates)
//
// Returns:
//   - ports.IdentityServer: SPIRE-backed server implementation
//   - error: Non-nil if trust domain is invalid or server creation fails
func (f *SPIREAdapterFactory) CreateServer(
	ctx context.Context,
	trustDomain string,
	trustDomainParser ports.TrustDomainParser,
	_ ports.IdentityDocumentProvider, // unused; SPIRE issues SVIDs externally
) (ports.IdentityServer, error) {
	// Validate all required parameters
	if trustDomain == "" {
		return nil, fmt.Errorf("trust domain cannot be empty")
	}
	if trustDomainParser == nil {
		return nil, fmt.Errorf("trust domain parser cannot be nil")
	}

	// Check for trust domain mismatch between factory client and requested domain
	if clientTD := f.client.GetTrustDomain(); clientTD != "" && clientTD != trustDomain {
		return nil, fmt.Errorf("trust domain mismatch: factory/client=%s, requested=%s", clientTD, trustDomain)
	}

	return spire.NewServer(ctx, f.client, trustDomain, trustDomainParser)
}

// CreateProductionAgent creates a SPIRE-backed production agent.
//
// The agent delegates to external SPIRE Agent/Server for all identity operations:
//   - Workload attestation (via SPIRE Agent platform attestors)
//   - Identity credential issuance (via SPIRE Server registration entries)
//   - SVID fetching (via Workload API)
//
// This method implements ProductionAgentFactory with a clean signature that only
// includes parameters actually used in production. Unlike development implementations,
// SPIRE handles registry, attestation, and server operations externally.
//
// Parameters:
//   - ctx: Context for agent initialization (timeout, cancellation)
//   - spiffeID: Agent's own SPIFFE ID (e.g., "spiffe://example.org/agent/host")
//   - parser: Identity credential parser for ID validation
//
// Returns:
//   - ports.Agent: SPIRE-backed agent implementation
//   - error: Non-nil if SPIFFE ID is invalid or agent creation fails
func (f *SPIREAdapterFactory) CreateProductionAgent(
	ctx context.Context,
	spiffeID string,
	parser ports.IdentityCredentialParser,
) (ports.Agent, error) {
	// Validate all required parameters
	if spiffeID == "" {
		return nil, fmt.Errorf("SPIFFE ID cannot be empty")
	}
	if parser == nil {
		return nil, fmt.Errorf("identity credential parser cannot be nil")
	}

	// Delegate to SPIRE Agent which handles all identity operations
	return spire.NewAgent(ctx, f.client, spiffeID, parser)
}

// Close closes the SPIRE client connection and releases resources.
//
// This should be called when the factory is no longer needed, typically
// in a defer statement after factory creation:
//
//	factory, err := compose.NewSPIREAdapterFactory(ctx, cfg)
//	if err != nil {
//	    return err
//	}
//	defer factory.Close()
//
// Returns an error if closing the client fails. The error can be safely
// ignored in most cases as it only affects cleanup logging.
func (f *SPIREAdapterFactory) Close() error {
	if f.client != nil {
		if err := f.client.Close(); err != nil {
			return fmt.Errorf("failed to close SPIRE client: %w", err)
		}
	}
	return nil
}

// Verify SPIREAdapterFactory implements the segregated interfaces correctly
var (
	_ ports.BaseAdapterFactory        = (*SPIREAdapterFactory)(nil)
	_ ports.ProductionServerFactory   = (*SPIREAdapterFactory)(nil)
	_ ports.ProductionAgentFactory    = (*SPIREAdapterFactory)(nil)
	_ ports.CoreAdapterFactory        = (*SPIREAdapterFactory)(nil) // Composite of above
)
