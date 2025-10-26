package spiffeidentity

import (
	"fmt"
	"os"
	"strings"
)

// Config holds configuration for the SPIFFE identity service.
//
// This configuration works the same way in all environments:
//   - Development: unix attestor with local agent
//   - Production Kubernetes: k8s attestor with DaemonSet agent
//   - Production AWS: aws attestor with EC2/ECS agent
//   - Production Azure: azure attestor with VM agent
//   - Production GCP: gcp attestor with GCE agent
//
// The attestation method is determined by the SPIRE Agent configuration,
// not by this configuration. This makes the same binary work everywhere.
type Config struct {
	// WorkloadAPISocket is the path to the SPIRE Agent's Workload API socket.
	//
	// Examples:
	//   - Dev (unix attestor):    "unix:///tmp/spire-agent/public/api.sock"
	//   - Prod k8s:               "unix:///spiffe-workload-api/spire-agent.sock"
	//   - Prod AWS:               "unix:///run/spire/sockets/agent.sock"
	//   - Prod Azure:             "unix:///var/run/spire/sockets/agent.sock"
	//
	// The socket path is environment-specific but the code is not.
	WorkloadAPISocket string

	// ExpectedTrustDomain is the expected SPIFFE trust domain (optional).
	//
	// If provided, the identity service will validate that fetched identities
	// belong to this trust domain. This prevents misconfiguration.
	//
	// Examples:
	//   - Dev:       "dev.local"
	//   - Prod:      "prod.example.com"
	//
	// Note: Do NOT include the "spiffe://" prefix. The trust domain is just
	// the authority component (e.g., "example.org" not "spiffe://example.org").
	ExpectedTrustDomain string
}

// LoadFromEnv loads identity service configuration from environment variables.
//
// Environment variables:
//   - SPIFFE_WORKLOAD_API_SOCKET (required): Path to agent socket
//   - SPIFFE_TRUST_DOMAIN (optional): Expected trust domain
//
// Example dev environment:
//
//	export SPIFFE_WORKLOAD_API_SOCKET="unix:///tmp/spire-agent/public/api.sock"
//	export SPIFFE_TRUST_DOMAIN="dev.local"
//	./server
//
// Example production Kubernetes environment:
//
//	export SPIFFE_WORKLOAD_API_SOCKET="unix:///spiffe-workload-api/spire-agent.sock"
//	export SPIFFE_TRUST_DOMAIN="prod.example.com"
//	./server
//
// Example production AWS environment:
//
//	export SPIFFE_WORKLOAD_API_SOCKET="unix:///run/spire/sockets/agent.sock"
//	export SPIFFE_TRUST_DOMAIN="prod.example.com"
//	./server
func LoadFromEnv() (Config, error) {
	// Trim whitespace from env values to handle accidental spaces
	socket := strings.TrimSpace(os.Getenv("SPIFFE_WORKLOAD_API_SOCKET"))
	if socket == "" {
		return Config{}, fmt.Errorf("SPIFFE_WORKLOAD_API_SOCKET environment variable is required")
	}

	// Validate socket path format
	if !strings.HasPrefix(socket, "unix://") {
		return Config{}, fmt.Errorf("invalid socket path %q: must start with 'unix://' (e.g., 'unix:///tmp/spire-agent.sock')", socket)
	}

	trustDomain := strings.TrimSpace(os.Getenv("SPIFFE_TRUST_DOMAIN"))
	if trustDomain != "" {
		// Validate trust domain format
		if strings.Contains(trustDomain, "://") {
			return Config{}, fmt.Errorf("invalid trust domain %q: must not include scheme (use 'example.org' not 'spiffe://example.org')", trustDomain)
		}
		if strings.Contains(trustDomain, "/") {
			return Config{}, fmt.Errorf("invalid trust domain %q: must not include path separators", trustDomain)
		}
	}

	return Config{
		WorkloadAPISocket:   socket,
		ExpectedTrustDomain: trustDomain,
	}, nil
}

// Validate checks if the configuration is valid.
func (c Config) Validate() error {
	if c.WorkloadAPISocket == "" {
		return fmt.Errorf("WorkloadAPISocket is required")
	}

	if !strings.HasPrefix(c.WorkloadAPISocket, "unix://") {
		return fmt.Errorf("invalid socket path %q: must start with 'unix://'", c.WorkloadAPISocket)
	}

	if c.ExpectedTrustDomain != "" {
		// Check scheme
		if strings.Contains(c.ExpectedTrustDomain, "://") {
			return fmt.Errorf("invalid trust domain %q: must not include scheme", c.ExpectedTrustDomain)
		}
		// Check path separators
		if strings.Contains(c.ExpectedTrustDomain, "/") {
			return fmt.Errorf("invalid trust domain %q: must not include path separators", c.ExpectedTrustDomain)
		}
		// Check length (DNS limit is 253 characters)
		if len(c.ExpectedTrustDomain) > 253 {
			return fmt.Errorf("trust domain %q exceeds maximum length 253", c.ExpectedTrustDomain)
		}
		// Check for invalid characters (trust domains must be valid DNS names)
		// Valid characters: a-z, 0-9, dot, hyphen
		for _, ch := range c.ExpectedTrustDomain {
			if !((ch >= 'a' && ch <= 'z') || (ch >= '0' && ch <= '9') || ch == '.' || ch == '-') {
				return fmt.Errorf("invalid trust domain %q: contains invalid character %q (must be lowercase alphanumeric with dots/hyphens)", c.ExpectedTrustDomain, string(ch))
			}
		}
	}

	return nil
}
