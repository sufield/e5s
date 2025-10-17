package zerotrustserver

import (
	"context"
	"net/http"

	"github.com/pocket/hexagon/spire/internal/adapters/inbound/identityserver"
)

// Serve runs a Zero Trust mTLS HTTP server with intelligent defaults.
// You only provide routes. Everything else is detected or safely defaulted.
func Serve(ctx context.Context, routes map[string]http.Handler) error {
	cfg, mux, err := buildDefaults(ctx, routes)
	if err != nil {
		return err
	}

	s, err := identityserver.New(ctx, cfg)
	if err != nil {
		return err
	}
	defer s.Close()

	// Route everything through our mux.
	if err := s.Handle("/", mux); err != nil {
		return err
	}
	return s.Start(ctx)
}
