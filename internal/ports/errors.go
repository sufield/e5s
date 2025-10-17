package ports

import "errors"

// Infrastructure errors for adapter layer.
//
// These errors represent infrastructure/adapter concerns and are separate from
// domain errors which represent business/semantic failures.
//
// Usage:
//   - Adapters return these errors when infrastructure operations fail
//   - Domain layer never imports or uses these errors directly
//   - Application layer may catch these and map to domain errors if appropriate
//
// Moved from domain/errors.go per architecture review (docs/5.md):
// Infrastructure errors belong at the adapter boundary, not in the domain.

// ErrServerUnavailable indicates the SPIRE server or identity provider is unavailable.
// This is an infrastructure concern - the server/CA being down is not a domain concept.
//
// Used by:
//   - Server adapters when unable to connect to SPIRE Server
//   - IdentityDocumentProvider adapters when CA is unavailable
var ErrServerUnavailable = errors.New("server unavailable")

// ErrAgentUnavailable indicates the SPIRE agent or workload API is unavailable.
// This is an infrastructure concern - agent connectivity is not a domain concept.
//
// Used by:
//   - Agent adapters when unable to connect to SPIRE Agent
//   - Workload API clients when socket is unavailable
var ErrAgentUnavailable = errors.New("agent unavailable")

// ErrCANotInitialized indicates the CA certificate is not initialized.
// This is an infrastructure concern - CA initialization is an adapter responsibility.
//
// Used by:
//   - Server adapters during startup before CA is loaded
//   - IdentityDocumentProvider adapters when CA is not ready
var ErrCANotInitialized = errors.New("CA not initialized")

// Compile-time check that errors implement error interface
var (
	_ error = ErrServerUnavailable
	_ error = ErrAgentUnavailable
	_ error = ErrCANotInitialized
)
