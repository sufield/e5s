package zerotrustclient

import (
	"fmt"
	"os"
	"time"

	"github.com/sufield/e5s/internal/ports"
)

// buildConfig constructs an MTLSConfig from the user's Config, applying defaults.
func buildConfig(cfg *Config) (*ports.MTLSConfig, error) {
	// Auto-detect socket path if not provided
	socketPath := cfg.SocketPath
	if socketPath == "" {
		socketPath = selectSocket()
	}

	// Validate ServerID and ServerTrustDomain are mutually exclusive
	if cfg.ServerID != "" && cfg.ServerTrustDomain != "" {
		return nil, fmt.Errorf("ServerID and ServerTrustDomain are mutually exclusive; specify only one")
	}

	// Build SPIFFE configuration
	spiffeCfg := ports.SPIFFEConfig{}
	if cfg.ServerID != "" {
		spiffeCfg.AllowedPeerID = cfg.ServerID
	} else if cfg.ServerTrustDomain != "" {
		spiffeCfg.AllowedTrustDomain = cfg.ServerTrustDomain
	}
	// If both are empty, leave spiffeCfg empty - this will be handled by validation
	// Actually, the adapter requires one to be set, so we need to fail here
	if cfg.ServerID == "" && cfg.ServerTrustDomain == "" {
		return nil, fmt.Errorf("either ServerID or ServerTrustDomain must be specified")
	}

	return &ports.MTLSConfig{
		WorkloadAPI: ports.WorkloadAPIConfig{
			SocketPath: socketPath,
		},
		SPIFFE: spiffeCfg,
		HTTP: ports.HTTPConfig{
			ReadTimeout:  10 * time.Second,
			WriteTimeout: 10 * time.Second,
			IdleTimeout:  120 * time.Second,
		},
	}, nil
}

// selectSocket auto-detects the SPIRE agent socket path.
// It checks environment variables and common paths in order of preference.
func selectSocket() string {
	// Preferred (SPIFFE-standard) and common fallbacks
	candidates := []string{
		os.Getenv("SPIFFE_ENDPOINT_SOCKET"),
		os.Getenv("SPIRE_AGENT_SOCKET"),
		"unix:///tmp/spire-agent/public/api.sock",  // Minikube/local development
		"unix:///var/run/spire/sockets/agent.sock", // Kubernetes daemonset default
	}
	for _, c := range candidates {
		if c != "" {
			return c
		}
	}
	// Last resort: standard Kubernetes path
	return "unix:///var/run/spire/sockets/agent.sock"
}
