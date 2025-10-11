// Package compose provides AdapterFactory implementations that compose outbound
// adapters for different deployment modes.
//
// This package contains two factory implementations:
//
//  1. SPIREAdapterFactory (production):
//     - Connects to external SPIRE infrastructure (SPIRE Agent, SPIRE Server)
//     - Uses go-spiffe SDK for parsing and validation
//     - Delegates identity operations to SPIRE (attestation, issuance, rotation)
//     - Minimal in-memory components (only lightweight validators)
//     - Implements ports.CoreAdapterFactory interface
//
//  2. InMemoryAdapterFactory (development, build tag: dev):
//     - Provides in-memory implementations for local development
//     - Used by walking skeleton and examples
//     - No external SPIRE dependencies
//     - Implements full ports.AdapterFactory composite interface
//
// Factory Selection:
//
// Production builds automatically use SPIREAdapterFactory. Development builds
// can choose between factories based on IDP_MODE environment variable:
//
//	IDP_MODE=inmem  -> InMemoryAdapterFactory (local development)
//	IDP_MODE=spire  -> SPIREAdapterFactory (SPIRE integration testing)
//
// Design Rationale:
//
// The SPIREAdapterFactory is intentionally hybrid - it composes SPIRE
// infrastructure for heavy operations with SDK-based parsers for validation.
// This design:
//   - Keeps the factory focused on production SPIRE integration
//   - Uses battle-tested go-spiffe SDK for parsing/validation
//   - Minimizes custom code (security, maintainability)
//   - Enables easy migration to full SDK validation (x509svid.Verify)
//
// The small inmemory dependency (IdentityDocumentProvider) is for lightweight
// validation only. Certificate creation happens on SPIRE Server in production.
//
// Usage Example:
//
//	// Production: Connect to SPIRE
//	factory, err := compose.NewSPIREAdapterFactory(ctx, &spire.Config{
//	    SocketPath: "/tmp/spire-agent/public/api.sock",
//	})
//	if err != nil {
//	    return fmt.Errorf("failed to create factory: %w", err)
//	}
//	defer factory.Close()
//
//	// Development: In-memory mode
//	factory := compose.NewInMemoryAdapterFactory()
//
// See also:
//   - internal/adapters/outbound/spire: SPIRE infrastructure adapters
//   - internal/adapters/outbound/inmemory: In-memory implementations
//   - internal/ports: Adapter interfaces (CoreAdapterFactory, AdapterFactory)
package compose
