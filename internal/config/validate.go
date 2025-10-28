package config

import (
	"errors"
	"fmt"
)

// ValidateServer validates server configuration.
//
// Ensures:
//   - ListenAddr is non-empty
//   - WorkloadSocket is non-empty
//   - Exactly one of AllowedClientSPIFFEID or AllowedClientTrustDomain is set
func ValidateServer(cfg FileConfig) error {
	// Validate SPIRE config
	if cfg.SPIRE.WorkloadSocket == "" {
		return errors.New("spire.workload_socket must be set")
	}

	// Validate server config
	if cfg.Server.ListenAddr == "" {
		return errors.New("server.listen_addr must be set")
	}

	// Ensure exactly one authorization policy is set
	hasClientID := cfg.Server.AllowedClientSPIFFEID != ""
	hasTrustDomain := cfg.Server.AllowedClientTrustDomain != ""

	if !hasClientID && !hasTrustDomain {
		return errors.New("must set exactly one of server.allowed_client_spiffe_id or server.allowed_client_trust_domain")
	}

	if hasClientID && hasTrustDomain {
		return errors.New("cannot set both server.allowed_client_spiffe_id and server.allowed_client_trust_domain")
	}

	return nil
}

// ValidateClient validates client configuration.
//
// Ensures:
//   - WorkloadSocket is non-empty
//   - Exactly one of ExpectedServerSPIFFEID or ExpectedServerTrustDomain is set
func ValidateClient(cfg FileConfig) error {
	// Validate SPIRE config
	if cfg.SPIRE.WorkloadSocket == "" {
		return errors.New("spire.workload_socket must be set")
	}

	// Ensure exactly one server verification policy is set
	hasServerID := cfg.Client.ExpectedServerSPIFFEID != ""
	hasTrustDomain := cfg.Client.ExpectedServerTrustDomain != ""

	if !hasServerID && !hasTrustDomain {
		return errors.New("must set exactly one of client.expected_server_spiffe_id or client.expected_server_trust_domain")
	}

	if hasServerID && hasTrustDomain {
		return fmt.Errorf("cannot set both client.expected_server_spiffe_id and client.expected_server_trust_domain")
	}

	return nil
}
