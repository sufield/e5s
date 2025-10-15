package ports

import (
	"context"
)

// IdentityProvider is the inbound port for workloads to obtain their identity credentials.
// This represents the core capability: a workload requests its identity from the system.
//
// In domain terms, this is about authentication and identity issuance, not about
// specific protocols or transport mechanisms. Adapters translate between this
// domain-centric interface and specific implementations (local agent, remote service, etc.).
//
// Security: The identity provider (via its adapter) MUST authenticate the calling
// process through secure means (e.g., OS-level process credentials) before issuing
// identity credentials. The caller never provides its own identity - it's always
// extracted from a trusted source by the adapter.
//
// Error Contract: Implementations wrap errors with domain sentinels for error handling:
//   - ErrIdentityServerUnavailable: Identity server unreachable
//   - ErrWorkloadAttestationFailed: Workload authentication failed
//   - ErrIdentityNotFound: No identity registered for workload
//
// Example:
//
//	provider := NewIdentityProvider(...) // adapter implementation
//	identity, err := provider.FetchIdentity(ctx)
//	if errors.Is(err, domain.ErrIdentityServerUnavailable) {
//	    // handle server unavailable
//	}
type IdentityProvider interface {
	// FetchIdentity retrieves the identity credential for the calling workload.
	//
	// The workload is authenticated based on its runtime properties (process ID,
	// user ID, executable path, etc.), and if authorized, receives its identity
	// credential (certificate-based proof of identity).
	//
	// Process:
	// 1. Adapter extracts workload's runtime credentials from OS/transport
	// 2. Workload is authenticated against known workload registrations
	// 3. Identity credential is issued if workload is authorized
	//
	// Returns:
	//   - *Identity: The workload's identity credential with valid proof
	//   - error: Domain error if authentication or authorization fails
	FetchIdentity(ctx context.Context) (*Identity, error)

	// Close releases any resources held by the provider.
	// This method is idempotent and safe to call multiple times.
	//
	// Usage:
	//
	//	provider, err := adapter.NewIdentityProvider(...)
	//	if err != nil { return err }
	//	defer provider.Close()
	Close() error
}

// CLI is the inbound port for CLI orchestration.
// Adapters implement this to drive use cases via command-line interface.
//
// Example:
//
//	cli := NewCLIAdapter(app)
//	if err := cli.Run(ctx); err != nil {
//	    log.Fatal(err)
//	}
type CLI interface {
	// Run executes the CLI command with the given context.
	// Returns an error if command execution fails.
	Run(ctx context.Context) error
}
