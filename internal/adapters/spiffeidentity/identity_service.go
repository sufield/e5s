package spiffeidentity

import (
	"context"
	"fmt"
	"strings"

	"github.com/spiffe/go-spiffe/v2/spiffeid"
	"github.com/spiffe/go-spiffe/v2/workloadapi"

	"github.com/sufield/e5s/internal/ports"
)

// IdentityServiceSPIFFE implements ports.IdentityService using real SPIRE.
//
// This adapter works with ANY SPIRE Agent configuration:
//   - Unix workload attestor (dev/laptop/VM/bare metal)
//   - Kubernetes workload attestor (production k8s)
//   - AWS workload attestor (production AWS EC2/ECS/Lambda)
//   - Azure workload attestor (production Azure VMs)
//   - GCP workload attestor (production GCP GCE)
//   - Docker attestor
//   - Custom attestors
//
// The attestation method is configured in the SPIRE Agent, not in this code.
// This makes the same binary work across all environments.
type IdentityServiceSPIFFE struct {
	client *workloadapi.Client
}

// NewIdentityServiceSPIFFE creates a new SPIFFE-based identity service.
//
// The socketPath should point to the SPIRE Agent's Workload API socket.
//
// Examples:
//   - Dev (unix attestor): "unix:///tmp/spire-agent/public/api.sock"
//   - Prod k8s: "unix:///spiffe-workload-api/spire-agent.sock"
//   - Prod AWS: "unix:///run/spire/sockets/agent.sock"
//
// The agent socket path is environment-specific but the code is not.
func NewIdentityServiceSPIFFE(ctx context.Context, socketPath string) (*IdentityServiceSPIFFE, error) {
	// Create workload API client
	// This client works regardless of attestation method
	client, err := workloadapi.New(ctx, workloadapi.WithAddr(socketPath))
	if err != nil {
		return nil, fmt.Errorf("failed to create workload API client: %w", err)
	}

	return &IdentityServiceSPIFFE{
		client: client,
	}, nil
}

// NewIdentityServiceFromConfig creates a new SPIFFE-based identity service from configuration.
//
// This is the recommended way to create an identity service as it supports
// trust domain validation and follows the runtime configuration pattern.
//
// Example dev usage:
//
//	cfg := spiffeidentity.Config{
//	    WorkloadAPISocket: "unix:///tmp/spire-agent/public/api.sock",
//	    ExpectedTrustDomain: "dev.local",
//	}
//	svc, err := spiffeidentity.NewIdentityServiceFromConfig(ctx, cfg)
//
// Example production Kubernetes usage:
//
//	cfg := spiffeidentity.Config{
//	    WorkloadAPISocket: "unix:///spiffe-workload-api/spire-agent.sock",
//	    ExpectedTrustDomain: "prod.example.com",
//	}
//	svc, err := spiffeidentity.NewIdentityServiceFromConfig(ctx, cfg)
//
// Example loading from environment:
//
//	cfg, err := spiffeidentity.LoadFromEnv()
//	if err != nil {
//	    return err
//	}
//	svc, err := spiffeidentity.NewIdentityServiceFromConfig(ctx, cfg)
func NewIdentityServiceFromConfig(ctx context.Context, cfg Config) (*IdentityServiceSPIFFE, error) {
	// Validate configuration
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	// Create service
	svc, err := NewIdentityServiceSPIFFE(ctx, cfg.WorkloadAPISocket)
	if err != nil {
		return nil, err
	}

	// If trust domain is specified, validate it immediately
	if cfg.ExpectedTrustDomain != "" {
		if err := svc.ValidateTrustDomain(ctx, cfg.ExpectedTrustDomain); err != nil {
			// Close the client before returning error
			_ = svc.Close()
			return nil, fmt.Errorf("trust domain validation failed: %w", err)
		}
	}

	return svc, nil
}

// Current returns the identity of the current process/workload.
//
// This fetches the X.509 SVID from the SPIRE Agent and extracts the SPIFFE ID.
// The SPIRE Agent handles attestation using whatever method it's configured with:
//   - unix attestor: validates UID/GID/PID/executable
//   - k8s attestor: validates pod namespace/service account/labels
//   - aws attestor: validates EC2 instance metadata
//   - etc.
//
// The application code doesn't know or care which method was used.
func (s *IdentityServiceSPIFFE) Current(ctx context.Context) (ports.Identity, error) {
	// Fetch X.509 SVID from the SPIRE Agent
	// This works regardless of the attestation method the agent uses
	svid, err := s.client.FetchX509SVID(ctx)
	if err != nil {
		return ports.Identity{}, fmt.Errorf("failed to fetch X.509 SVID: %w", err)
	}

	// Extract SPIFFE ID
	spiffeID := svid.ID

	// Parse components
	return s.parseIdentity(spiffeID)
}

// Close closes the workload API client and releases resources.
func (s *IdentityServiceSPIFFE) Close() error {
	if s.client != nil {
		return s.client.Close()
	}
	return nil
}

// parseIdentity converts a spiffeid.ID into ports.Identity
func (s *IdentityServiceSPIFFE) parseIdentity(id spiffeid.ID) (ports.Identity, error) {
	// Get full SPIFFE ID string
	// Example: "spiffe://example.org/my-service"
	fullID := id.String()

	// Get trust domain
	// Example: "example.org"
	trustDomain := id.TrustDomain().String()

	// Get path component
	// Example: "/my-service"
	path := id.Path()
	if path == "" {
		path = "/"
	}

	return ports.Identity{
		SPIFFEID:    fullID,
		TrustDomain: trustDomain,
		Path:        path,
	}, nil
}

// ValidateTrustDomain checks if the fetched identity matches the expected trust domain.
//
// This is useful for additional validation to ensure you're getting identities
// from the expected SPIRE deployment.
func (s *IdentityServiceSPIFFE) ValidateTrustDomain(ctx context.Context, expectedTrustDomain string) error {
	identity, err := s.Current(ctx)
	if err != nil {
		return fmt.Errorf("failed to get current identity: %w", err)
	}

	// Normalize trust domains for comparison
	expected := strings.TrimPrefix(expectedTrustDomain, "spiffe://")
	actual := identity.TrustDomain

	if actual != expected {
		return fmt.Errorf("trust domain mismatch: expected %q, got %q", expected, actual)
	}

	return nil
}

// Verify that IdentityServiceSPIFFE implements ports.IdentityService
var _ ports.IdentityService = (*IdentityServiceSPIFFE)(nil)
