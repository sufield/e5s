package config

import (
	"errors"
	"fmt"
	"strings"

	"github.com/spiffe/go-spiffe/v2/spiffeid"
)

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

// ValidateServer validates server configuration and returns parsed authorization policy.
//
// Ensures:
//   - ListenAddr is non-empty
//   - WorkloadSocket is non-empty
//   - Exactly one of AllowedClientSPIFFEID or AllowedClientTrustDomain is set
//   - SPIFFE ID / trust domain strings are syntactically valid (using SDK validation)
//
// Returns parsed authorization policy to avoid reparsing downstream.
func ValidateServer(cfg FileConfig) (ServerAuthz, error) {
	// Validate SPIRE config
	if strings.TrimSpace(cfg.SPIRE.WorkloadSocket) == "" {
		return ServerAuthz{}, errors.New("spire.workload_socket must be set")
	}

	// Validate server config
	if strings.TrimSpace(cfg.Server.ListenAddr) == "" {
		return ServerAuthz{}, errors.New("server.listen_addr must be set")
	}

	// Trim input to defend against accidental whitespace
	clientID := strings.TrimSpace(cfg.Server.AllowedClientSPIFFEID)
	trustDomain := strings.TrimSpace(cfg.Server.AllowedClientTrustDomain)

	hasClientID := clientID != ""
	hasTrustDomain := trustDomain != ""

	if !hasClientID && !hasTrustDomain {
		return ServerAuthz{}, errors.New("must set exactly one of server.allowed_client_spiffe_id or server.allowed_client_trust_domain")
	}
	if hasClientID && hasTrustDomain {
		return ServerAuthz{}, errors.New("cannot set both server.allowed_client_spiffe_id and server.allowed_client_trust_domain")
	}

	var authz ServerAuthz

	// Validate and parse using SDK
	if hasClientID {
		id, err := spiffeid.FromString(clientID)
		if err != nil {
			return ServerAuthz{}, fmt.Errorf("invalid server.allowed_client_spiffe_id %q: %w", clientID, err)
		}
		authz.ID = id
	}
	if hasTrustDomain {
		td, err := spiffeid.TrustDomainFromString(trustDomain)
		if err != nil {
			return ServerAuthz{}, fmt.Errorf("invalid server.allowed_client_trust_domain %q: %w", trustDomain, err)
		}
		authz.TrustDomain = td
	}

	return authz, nil
}

// ValidateClient validates client configuration and returns parsed verification policy.
//
// Ensures:
//   - WorkloadSocket is non-empty
//   - Exactly one of ExpectedServerSPIFFEID or ExpectedServerTrustDomain is set
//   - SPIFFE ID / trust domain strings are syntactically valid (using SDK validation)
//
// Returns parsed verification policy to avoid reparsing downstream.
func ValidateClient(cfg FileConfig) (ClientAuthz, error) {
	// Validate SPIRE config
	if strings.TrimSpace(cfg.SPIRE.WorkloadSocket) == "" {
		return ClientAuthz{}, errors.New("spire.workload_socket must be set")
	}

	// Trim input to defend against accidental whitespace
	serverID := strings.TrimSpace(cfg.Client.ExpectedServerSPIFFEID)
	trustDomain := strings.TrimSpace(cfg.Client.ExpectedServerTrustDomain)

	hasServerID := serverID != ""
	hasTrustDomain := trustDomain != ""

	if !hasServerID && !hasTrustDomain {
		return ClientAuthz{}, errors.New("must set exactly one of client.expected_server_spiffe_id or client.expected_server_trust_domain")
	}
	if hasServerID && hasTrustDomain {
		return ClientAuthz{}, errors.New("cannot set both client.expected_server_spiffe_id and client.expected_server_trust_domain")
	}

	var authz ClientAuthz

	// Validate and parse using SDK
	if hasServerID {
		id, err := spiffeid.FromString(serverID)
		if err != nil {
			return ClientAuthz{}, fmt.Errorf("invalid client.expected_server_spiffe_id %q: %w", serverID, err)
		}
		authz.ID = id
	}
	if hasTrustDomain {
		td, err := spiffeid.TrustDomainFromString(trustDomain)
		if err != nil {
			return ClientAuthz{}, fmt.Errorf("invalid client.expected_server_trust_domain %q: %w", trustDomain, err)
		}
		authz.TrustDomain = td
	}

	return authz, nil
}
