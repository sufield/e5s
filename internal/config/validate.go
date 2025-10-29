package config

import (
	"errors"
	"fmt"

	"github.com/spiffe/go-spiffe/v2/spiffeid"
)

// ValidateServer validates server configuration.
//
// Ensures:
//   - ListenAddr is non-empty
//   - WorkloadSocket is non-empty
//   - Exactly one of AllowedClientSPIFFEID or AllowedClientTrustDomain is set
//   - If provided, SPIFFE ID / trust domain strings are syntactically valid (using SDK validation)
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

	// Validate formats using SDK
	if hasClientID {
		if _, err := spiffeid.FromString(cfg.Server.AllowedClientSPIFFEID); err != nil {
			return fmt.Errorf("invalid server.allowed_client_spiffe_id %q: %w", cfg.Server.AllowedClientSPIFFEID, err)
		}
	}
	if hasTrustDomain {
		if _, err := spiffeid.TrustDomainFromString(cfg.Server.AllowedClientTrustDomain); err != nil {
			return fmt.Errorf("invalid server.allowed_client_trust_domain %q: %w", cfg.Server.AllowedClientTrustDomain, err)
		}
	}

	return nil
}

// ValidateClient validates client configuration.
//
// Ensures:
//   - WorkloadSocket is non-empty
//   - Exactly one of ExpectedServerSPIFFEID or ExpectedServerTrustDomain is set
//   - If provided, SPIFFE ID / trust domain strings are syntactically valid (using SDK validation)
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
		return errors.New("cannot set both client.expected_server_spiffe_id and client.expected_server_trust_domain")
	}

	// Validate formats using SDK
	if hasServerID {
		if _, err := spiffeid.FromString(cfg.Client.ExpectedServerSPIFFEID); err != nil {
			return fmt.Errorf("invalid client.expected_server_spiffe_id %q: %w", cfg.Client.ExpectedServerSPIFFEID, err)
		}
	}
	if hasTrustDomain {
		if _, err := spiffeid.TrustDomainFromString(cfg.Client.ExpectedServerTrustDomain); err != nil {
			return fmt.Errorf("invalid client.expected_server_trust_domain %q: %w", cfg.Client.ExpectedServerTrustDomain, err)
		}
	}

	return nil
}
