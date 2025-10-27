//go:build !debug

package debug

import "context"

// Server is a stub implementation of the debug server used in non-debug builds.
// In production builds Start() always returns nil, so Application.Close()
// will see a nil debugServer and skip shutdown logic.
type Server struct{}

// Stop is a no-op stub in production builds.
func (s *Server) Stop(ctx context.Context) error {
	return nil
}

// Start is a no-op in production builds.
// The introspector parameter is accepted to match the debug build signature,
// but is never used since no debug endpoints are exposed in production.
// Returns nil to indicate no server was started.
func Start(Introspector) *Server {
	// Debug server disabled in production build
	return nil
}
