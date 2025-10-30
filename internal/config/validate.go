package config

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/spiffe/go-spiffe/v2/spiffeid"
)

const (
	// DefaultInitialFetchTimeout is the default timeout for fetching the first SVID
	// from the SPIRE Workload API if not specified in config.
	DefaultInitialFetchTimeout = 30 * time.Second
)

// SPIREConfig contains parsed SPIRE Workload API configuration.
type SPIREConfig struct {
	// InitialFetchTimeout is the parsed timeout for initial SVID fetch
	InitialFetchTimeout time.Duration
}

// ServerAuthz contains the parsed authorization policy for a server.
// Exactly one of ID or TrustDomain will be set (never both).
type ServerAuthz struct {
	// ID is the specific client SPIFFE ID to allow (if using ID-based authz)
	ID spiffeid.ID
	// TrustDomain is the trust domain to allow (if using trust-domain-based authz)
	TrustDomain spiffeid.TrustDomain
}

// ClientAuthz contains the parsed verification policy for a client.
// Exactly one of ID or TrustDomain will be set (never both).
type ClientAuthz struct {
	// ID is the expected server SPIFFE ID (if using ID-based verification)
	ID spiffeid.ID
	// TrustDomain is the expected server trust domain (if using trust-domain-based verification)
	TrustDomain spiffeid.TrustDomain
}

// validateSPIREConfig validates and parses common SPIRE configuration.
func validateSPIREConfig(cfg FileConfig) (SPIREConfig, error) {
	// Validate workload socket
	if strings.TrimSpace(cfg.SPIRE.WorkloadSocket) == "" {
		return SPIREConfig{}, errors.New("spire.workload_socket must be set")
	}

	// Parse and validate initial fetch timeout
	timeoutStr := strings.TrimSpace(cfg.SPIRE.InitialFetchTimeout)
	var timeout time.Duration
	if timeoutStr == "" {
		// Use default if not specified
		timeout = DefaultInitialFetchTimeout
	} else {
		var err error
		timeout, err = time.ParseDuration(timeoutStr)
		if err != nil {
			return SPIREConfig{}, fmt.Errorf("invalid spire.initial_fetch_timeout %q: %w", timeoutStr, err)
		}
		if timeout <= 0 {
			return SPIREConfig{}, fmt.Errorf("spire.initial_fetch_timeout must be positive, got %q", timeoutStr)
		}
	}

	return SPIREConfig{
		InitialFetchTimeout: timeout,
	}, nil
}

// ValidateServer validates server configuration and returns parsed authorization policy.
//
// Ensures:
//   - ListenAddr is non-empty
//   - WorkloadSocket is non-empty
//   - InitialFetchTimeout is valid (or uses default)
//   - Exactly one of AllowedClientSPIFFEID or AllowedClientTrustDomain is set
//   - SPIFFE ID / trust domain strings are syntactically valid (using SDK validation)
//
// Returns parsed SPIRE config and authorization policy to avoid reparsing downstream.
func ValidateServer(cfg FileConfig) (SPIREConfig, ServerAuthz, error) {
	// Validate SPIRE config
	spireConfig, err := validateSPIREConfig(cfg)
	if err != nil {
		return SPIREConfig{}, ServerAuthz{}, err
	}

	// Validate server config
	if strings.TrimSpace(cfg.Server.ListenAddr) == "" {
		return SPIREConfig{}, ServerAuthz{}, errors.New("server.listen_addr must be set")
	}

	// Trim input to defend against accidental whitespace
	clientID := strings.TrimSpace(cfg.Server.AllowedClientSPIFFEID)
	trustDomain := strings.TrimSpace(cfg.Server.AllowedClientTrustDomain)

	hasClientID := clientID != ""
	hasTrustDomain := trustDomain != ""

	if !hasClientID && !hasTrustDomain {
		return SPIREConfig{}, ServerAuthz{}, errors.New("must set exactly one of server.allowed_client_spiffe_id or server.allowed_client_trust_domain")
	}
	if hasClientID && hasTrustDomain {
		return SPIREConfig{}, ServerAuthz{}, errors.New("cannot set both server.allowed_client_spiffe_id and server.allowed_client_trust_domain")
	}

	var authz ServerAuthz

	// Validate and parse using SDK
	if hasClientID {
		id, err := spiffeid.FromString(clientID)
		if err != nil {
			return SPIREConfig{}, ServerAuthz{}, fmt.Errorf("invalid server.allowed_client_spiffe_id %q: %w", clientID, err)
		}
		authz.ID = id
	}
	if hasTrustDomain {
		td, err := spiffeid.TrustDomainFromString(trustDomain)
		if err != nil {
			return SPIREConfig{}, ServerAuthz{}, fmt.Errorf("invalid server.allowed_client_trust_domain %q: %w", trustDomain, err)
		}
		authz.TrustDomain = td
	}

	return spireConfig, authz, nil
}

// ValidateClient validates client configuration and returns parsed verification policy.
//
// Ensures:
//   - WorkloadSocket is non-empty
//   - InitialFetchTimeout is valid (or uses default)
//   - Exactly one of ExpectedServerSPIFFEID or ExpectedServerTrustDomain is set
//   - SPIFFE ID / trust domain strings are syntactically valid (using SDK validation)
//
// Returns parsed SPIRE config and verification policy to avoid reparsing downstream.
func ValidateClient(cfg FileConfig) (SPIREConfig, ClientAuthz, error) {
	// Validate SPIRE config
	spireConfig, err := validateSPIREConfig(cfg)
	if err != nil {
		return SPIREConfig{}, ClientAuthz{}, err
	}

	// Trim input to defend against accidental whitespace
	serverID := strings.TrimSpace(cfg.Client.ExpectedServerSPIFFEID)
	trustDomain := strings.TrimSpace(cfg.Client.ExpectedServerTrustDomain)

	hasServerID := serverID != ""
	hasTrustDomain := trustDomain != ""

	if !hasServerID && !hasTrustDomain {
		return SPIREConfig{}, ClientAuthz{}, errors.New("must set exactly one of client.expected_server_spiffe_id or client.expected_server_trust_domain")
	}
	if hasServerID && hasTrustDomain {
		return SPIREConfig{}, ClientAuthz{}, errors.New("cannot set both client.expected_server_spiffe_id and client.expected_server_trust_domain")
	}

	var authz ClientAuthz

	// Validate and parse using SDK
	if hasServerID {
		id, err := spiffeid.FromString(serverID)
		if err != nil {
			return SPIREConfig{}, ClientAuthz{}, fmt.Errorf("invalid client.expected_server_spiffe_id %q: %w", serverID, err)
		}
		authz.ID = id
	}
	if hasTrustDomain {
		td, err := spiffeid.TrustDomainFromString(trustDomain)
		if err != nil {
			return SPIREConfig{}, ClientAuthz{}, fmt.Errorf("invalid client.expected_server_trust_domain %q: %w", trustDomain, err)
		}
		authz.TrustDomain = td
	}

	return spireConfig, authz, nil
}
