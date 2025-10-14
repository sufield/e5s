package compose

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/pocket/hexagon/spire/internal/adapters/outbound/spire"
	"github.com/pocket/hexagon/spire/internal/ports"
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
// The factory implements ports.AdapterFactory for SPIRE deployments.
type SPIREAdapterFactory struct {
	config *spire.Config
	client *spire.SPIREClient

	mu     sync.RWMutex // Protects client access (Close vs Create* methods)
	closed bool         // Tracks if factory has been closed
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
	if cfg.SocketPath == "" {
		return nil, fmt.Errorf("socket path is required")
	}

	// Store defensive copy with defaults applied
	// This ensures external mutation of cfg doesn't surprise the factory later
	copied := *cfg
	if copied.Timeout <= 0 {
		copied.Timeout = 30 * time.Second
	}

	// Create SPIRE Workload API client
	client, err := spire.NewSPIREClient(ctx, copied)
	if err != nil {
		return nil, fmt.Errorf("failed to create SPIRE client: %w", err)
	}

	return &SPIREAdapterFactory{
		config: &copied,
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

// CreateIdentityDocumentValidator creates an identity document validator.
//
// Design Note: In production SPIRE deployments, workloads are **client-only**.
// All certificate issuance happens in external SPIRE infrastructure:
//   - SPIRE Server issues SVIDs after validating registration entries
//   - SPIRE Agent attests workloads and delivers SVIDs via Workload API
//   - Workloads fetch their own SVIDs and verify peer certificates
//
// This validator handles **validation only** (not creation):
//   - Full chain-of-trust verification against trust bundles (x509svid.Verify)
//   - Signature validation
//   - Certificate expiration checks
//   - SPIFFE ID extraction and validation from URI SAN
//
// The bundle source (f.client) maintains up-to-date trust bundles including
// federated trust domains. SPIREClient implements x509bundle.Source, verified
// at compile-time in the spire package.
//
// Returns an SDK-based validator with full security validation for production use.
func (f *SPIREAdapterFactory) CreateIdentityDocumentValidator() ports.IdentityDocumentValidator {
	f.mu.RLock()
	defer f.mu.RUnlock()

	// SPIREClient implements x509bundle.Source (compile-time assertion in spire package)
	// No runtime check needed - any breakage is caught at build time
	return spire.NewSDKIdentityDocumentValidator(f.client)
}

// NOTE: CreateServer has been removed from production SPIRE adapter.
// Production SPIRE workloads are clients only - they fetch their own SVID via Workload API.
// Real SPIRE Server runs as external infrastructure, not embedded in workload processes.
// For development/testing with embedded server, use InMemoryAdapterFactory instead.

// CreateAgent creates a SPIRE-backed agent.
//
// The agent delegates to external SPIRE Agent/Server for all identity operations:
//   - Workload attestation (via SPIRE Agent platform attestors)
//   - Identity credential issuance (via SPIRE Server registration entries)
//   - SVID fetching (via Workload API)
//
// Parameters:
//   - ctx: Context for agent initialization (timeout, cancellation)
//   - spiffeID: Agent's own SPIFFE ID (e.g., "spiffe://example.org/agent/host")
//   - parser: Identity credential parser for ID validation
//
// Returns:
//   - ports.Agent: SPIRE-backed agent implementation
//   - error: Non-nil if SPIFFE ID is invalid or agent creation fails
func (f *SPIREAdapterFactory) CreateAgent(
	ctx context.Context,
	spiffeID string,
	parser ports.IdentityCredentialParser,
) (ports.Agent, error) {
	if spiffeID == "" {
		return nil, fmt.Errorf("SPIFFE ID cannot be empty")
	}
	if parser == nil {
		return nil, fmt.Errorf("identity credential parser cannot be nil")
	}

	// Validate via the port (hexagonal boundary - no direct SDK usage)
	if _, err := parser.ParseFromString(ctx, spiffeID); err != nil {
		return nil, fmt.Errorf("invalid SPIFFE ID: %w", err)
	}

	f.mu.RLock()
	defer f.mu.RUnlock()

	// Delegate to SPIRE Agent which handles all identity operations
	return spire.NewAgent(ctx, f.client, spiffeID, parser)
}

// Close closes the SPIRE client connection and releases resources.
//
// This method is idempotent - calling it multiple times is safe.
// After the first call, subsequent calls are no-ops.
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
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.closed || f.client == nil {
		return nil // Already closed or never initialized
	}

	err := f.client.Close()
	f.client = nil // Mark as closed to make this method idempotent
	f.closed = true

	if err != nil {
		return fmt.Errorf("failed to close SPIRE client: %w", err)
	}
	return nil
}

// Verify SPIREAdapterFactory implements the interfaces correctly
var (
	_ ports.BaseAdapterFactory = (*SPIREAdapterFactory)(nil)
	_ ports.AgentFactory       = (*SPIREAdapterFactory)(nil)
	_ ports.AdapterFactory     = (*SPIREAdapterFactory)(nil) // Composite of above
)
