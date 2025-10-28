package app

import (
	"context"
	"fmt"
	"net/http"
	"time"

	// debug is imported in all builds because Application holds a *debug.Server.
	// In debug builds this is the real HTTP debug server; in non-debug builds
	// it's a stub type (see internal/debug/server_stub.go). The shared field +
	// method (`SetDebugServer`, `Close`) must compile in both cases so shutdown
	// behavior stays consistent across build tags.
	"github.com/sufield/e5s/internal/debug"

	"github.com/sufield/e5s/internal/dto"
	"github.com/sufield/e5s/internal/ports"
)

// Application wires application dependencies.
//
// Identity Operations:
//   - Use IdentityService for SPIRE-agnostic identity operations (preferred)
//   - Agent provides lower-level SPIRE operations (use only when necessary)
//
// The IdentityService abstracts SPIRE implementation details and returns
// ports.Identity, making application code independent of SPIRE-specific types.
type Application struct {
	cfg             *dto.Config
	agent           ports.Agent
	identityService ports.IdentityService
	debugServer     *debug.Server // nil in production builds or if debug server not started
}

// New constructs an Application and validates required deps.
func New(cfg *dto.Config, agent ports.Agent, identityService ports.IdentityService) (*Application, error) {
	if cfg == nil {
		return nil, fmt.Errorf("config is nil")
	}
	if agent == nil {
		return nil, fmt.Errorf("agent is nil")
	}
	if identityService == nil {
		return nil, fmt.Errorf("identity service is nil")
	}
	return &Application{
		cfg:             cfg,
		agent:           agent,
		identityService: identityService,
	}, nil
}

// Close releases resources owned by the application (idempotent).
// Shutdown order:
//  1. Stop debug server (graceful, 5s timeout) so no new requests race with teardown.
//  2. Close agent.
// Safe to call multiple times.
func (a *Application) Close() error {
	if a == nil {
		return nil
	}

	var firstErr error

	// Stop debug server first (graceful shutdown with timeout)
	if a.debugServer != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := a.debugServer.Stop(ctx); err != nil && err != http.ErrServerClosed {
			// http.ErrServerClosed is expected on repeated calls - treat as non-fatal
			// We can't rely on debug.GetLogger() in !debug builds (no stub),
			// so capture the first error and surface it to the caller instead.
			if firstErr == nil {
				firstErr = fmt.Errorf("error stopping debug server: %w", err)
			}
		}
		a.debugServer = nil // Clear to ensure idempotence
	}

	// Then close agent
	if a.agent != nil {
		if err := a.agent.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
		a.agent = nil // Clear to ensure idempotence
	}

	return firstErr
}

// SetDebugServer sets the debug server instance for graceful shutdown.
// This is called by BootstrapWithDebug in debug builds.
//
// In non-debug builds, debug.Server is a stub (see internal/debug/server_stub.go)
// and contains no active listener. Passing that stub here is harmless and will
// still satisfy Close(), which will see a nil or inert server and no-op.
func (a *Application) SetDebugServer(srv *debug.Server) {
	if a != nil {
		a.debugServer = srv
	}
}

// Accessors (add only what you need)

// Config returns the immutable runtime configuration.
func (a *Application) Config() *dto.Config { return a.cfg }

// Agent returns the underlying SPIRE agent adapter.
// Prefer IdentityService() unless you need low-level SPIRE operations.
func (a *Application) Agent() ports.Agent { return a.agent }

// IdentityService returns the high-level identity service abstraction
// used by application code for identity operations.
func (a *Application) IdentityService() ports.IdentityService { return a.identityService }
