package zerotrustserver

import (
	"context"
	"log"
	"net/http"

	"github.com/pocket/hexagon/spire/internal/adapters/inbound/identityserver"
)

// Serve runs a Zero Trust mTLS HTTP server with intelligent defaults.
// You only provide routes. Everything else is detected or safely defaulted.
func Serve(ctx context.Context, routes map[string]http.Handler) error {
	cfg, mux, err := buildDefaults(ctx, routes)
	if err != nil {
		log.Printf("❌ Failed to build configuration: %v", err)
		return err
	}

	log.Printf("✓ Configuration detected successfully")
	log.Printf("  Trust Domain: %s", cfg.SPIFFE.AllowedTrustDomain)
	log.Printf("  Socket Path: %s", cfg.WorkloadAPI.SocketPath)

	s, err := identityserver.New(ctx, &cfg)
	if err != nil {
		log.Printf("❌ Failed to create mTLS server: %v", err)
		return err
	}
	defer s.Close()

	log.Printf("✓ mTLS server created successfully")

	// Route everything through our mux.
	if err := s.Handle("/", mux); err != nil {
		log.Printf("❌ Failed to register routes: %v", err)
		return err
	}

	log.Printf("✓ Routes registered")
	log.Println("========================================")
	log.Printf("🚀 Server listening on %s", cfg.HTTP.Address)
	log.Println("========================================")
	log.Println("Waiting for connections... (Press Ctrl+C to stop)")

	if err := s.Start(ctx); err != nil {
		log.Printf("❌ Server stopped with error: %v", err)
		return err
	}

	log.Println("✓ Server stopped gracefully")
	return nil
}
