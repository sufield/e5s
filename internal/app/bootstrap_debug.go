//go:build debug

package app

import (
	"context"
	"fmt"

	"github.com/pocket/hexagon/spire/internal/debug"
	"github.com/pocket/hexagon/spire/internal/ports"
)

// BootstrapWithDebug extends Bootstrap to start the debug server in debug builds.
//
// This function is only available in debug builds (via //go:build debug tag).
// It bootstraps the application and starts the debug HTTP server with
// identity introspection enabled.
//
// The debug server provides these endpoints:
//   - /_debug/state - Debug configuration
//   - /_debug/identity - Current identity snapshot (SPIFFE IDs, cert expiration)
//   - /_debug/faults - Fault injection (for testing failure modes)
//   - /_debug/config - Debug server configuration
//
// WARNING: This function should NEVER be called in production builds.
// The build tag ensures it's compiled out of production binaries.
//
// Example usage:
//
//	app, err := app.BootstrapWithDebug(ctx, configLoader, factory)
//	if err != nil {
//	    log.Fatal(err)
//	}
//	defer app.Close()
//
// Concurrency: Starts debug server in a background goroutine.
func BootstrapWithDebug(ctx context.Context, configLoader ports.ConfigLoader, factory ports.AdapterFactory) (*Application, error) {
	// First, do normal bootstrap
	app, err := Bootstrap(ctx, configLoader, factory)
	if err != nil {
		return nil, fmt.Errorf("bootstrap: %w", err)
	}

	// Get identity service from application
	identitySvc := app.IdentityService()

	// Start debug server with identity introspection
	// The debug server runs in a background goroutine
	// Note: identitySvc must implement debug.Introspector (verified by compile-time assertion in identity_service_debug.go)
	if introspector, ok := identitySvc.(debug.Introspector); ok {
		debug.Start(introspector)
	} else {
		// This should never happen in debug builds where IdentityServiceSPIRE implements Introspector
		debug.Start(nil) // Start without introspection
	}

	return app, nil
}
