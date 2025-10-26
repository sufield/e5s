package spiffeidentity

import (
	"context"
	"fmt"

	"github.com/pocket/hexagon/spire/internal/ports"
)

// WireIdentityService creates and configures an identity service from environment variables.
//
// This is the recommended way to initialize the identity service in your application's
// main function or dependency injection container.
//
// This function:
//   1. Loads configuration from environment variables
//   2. Validates the configuration
//   3. Creates the identity service
//   4. Validates trust domain (if configured)
//
// Environment variables:
//   - SPIFFE_WORKLOAD_API_SOCKET (required): Path to SPIRE Agent socket
//   - SPIFFE_TRUST_DOMAIN (optional): Expected trust domain for validation
//
// Example usage in main():
//
//	func main() {
//	    ctx := context.Background()
//
//	    // Wire up identity service from environment
//	    identitySvc, err := spiffeidentity.WireIdentityService(ctx)
//	    if err != nil {
//	        log.Fatalf("failed to initialize identity service: %v", err)
//	    }
//	    defer identitySvc.Close()
//
//	    // Use identity service in your application
//	    identity, err := identitySvc.Current(ctx)
//	    if err != nil {
//	        log.Fatalf("failed to get identity: %v", err)
//	    }
//	    log.Printf("Running as: %s", identity.SPIFFEID)
//	}
//
// Development environment setup:
//
//	export SPIFFE_WORKLOAD_API_SOCKET="unix:///tmp/spire-agent/public/api.sock"
//	export SPIFFE_TRUST_DOMAIN="dev.local"
//	./server
//
// Production Kubernetes environment setup:
//
//	export SPIFFE_WORKLOAD_API_SOCKET="unix:///spiffe-workload-api/spire-agent.sock"
//	export SPIFFE_TRUST_DOMAIN="prod.example.com"
//	./server
//
// Production AWS environment setup:
//
//	export SPIFFE_WORKLOAD_API_SOCKET="unix:///run/spire/sockets/agent.sock"
//	export SPIFFE_TRUST_DOMAIN="prod.example.com"
//	./server
func WireIdentityService(ctx context.Context) (ports.IdentityService, error) {
	// Load configuration from environment
	cfg, err := LoadFromEnv()
	if err != nil {
		return nil, fmt.Errorf("failed to load identity configuration: %w", err)
	}

	// Create service from configuration
	svc, err := NewIdentityServiceFromConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create identity service: %w", err)
	}

	return svc, nil
}

// WireIdentityServiceWithConfig creates and configures an identity service from the provided configuration.
//
// This is useful when you want to provide configuration programmatically instead of
// from environment variables.
//
// Example usage:
//
//	cfg := spiffeidentity.Config{
//	    WorkloadAPISocket: "unix:///tmp/spire-agent.sock",
//	    ExpectedTrustDomain: "dev.local",
//	}
//	identitySvc, err := spiffeidentity.WireIdentityServiceWithConfig(ctx, cfg)
//	if err != nil {
//	    return err
//	}
//	defer identitySvc.Close()
func WireIdentityServiceWithConfig(ctx context.Context, cfg Config) (ports.IdentityService, error) {
	svc, err := NewIdentityServiceFromConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create identity service: %w", err)
	}

	return svc, nil
}
