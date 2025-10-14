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

// IdentityIssuer is the server-side service for issuing identity credentials to workloads.
//
// This represents the core use case of identity issuance: given an authenticated workload,
// determine what identity it should receive and issue the appropriate credential.
//
// Security Critical: The workload's process credentials MUST be extracted from
// a trusted source (e.g., OS kernel via system calls) by the adapter layer.
// Never accept workload identity claims from the network or user input.
//
// Process Flow:
// 1. Adapter extracts caller's process credentials from trusted source
// 2. Adapter calls this service with the verified credentials
// 3. Service authenticates workload against known registrations
// 4. Service determines authorized identity for the workload
// 5. Service issues identity credential if authorized
//
// Error Contract: Implementations wrap errors with domain sentinels:
//   - ErrWorkloadAttestationFailed: Workload authentication failed
//   - ErrIdentityNotFound: No identity registered for workload
//   - ErrWorkloadInvalid: Invalid workload credentials
type IdentityIssuer interface {
	// IssueIdentity creates an identity credential for an authenticated workload.
	//
	// The workload parameter contains process credentials that were extracted
	// from a trusted source by the adapter. This is never user-provided data.
	//
	// The service:
	// 1. Validates the workload credentials
	// 2. Authenticates the workload against known registrations
	// 3. Determines the authorized identity for this workload
	// 4. Issues the identity credential if authorized
	//
	// Parameters:
	//   - ctx: Request context for timeout and cancellation
	//   - workload: Process credentials from trusted source
	//
	// Returns:
	//   - *Identity: The issued identity credential for the workload
	//   - error: Domain error if authentication or authorization fails
	IssueIdentity(ctx context.Context, workload ProcessIdentity) (*Identity, error)
}
