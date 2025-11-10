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

// validateSPIRESection validates and parses common SPIRE configuration from a SPIRESection.
func validateSPIRESection(spire SPIRESection) (SPIREConfig, error) {
	if strings.TrimSpace(spire.WorkloadSocket) == "" {
		return SPIREConfig{}, errors.New("spire.workload_socket must be set")
	}
	timeout := DefaultInitialFetchTimeout
	timeoutStr := strings.TrimSpace(spire.InitialFetchTimeout)
	if timeoutStr != "" {
		var err error
		if timeout, err = time.ParseDuration(timeoutStr); err != nil {
			return SPIREConfig{}, fmt.Errorf("invalid spire.initial_fetch_timeout %q: %w", timeoutStr, err)
		}
		if timeout <= 0 {
			return SPIREConfig{}, fmt.Errorf("spire.initial_fetch_timeout must be positive, got %q", timeoutStr)
		}
	}
	return SPIREConfig{InitialFetchTimeout: timeout}, nil
}

// validateAuthz parses and validates a SPIFFE ID or trust domain policy.
// Ensures exactly one is set, trims whitespace, and uses SDK for validation.
func validateAuthz(idStr, tdStr, prefix string) (spiffeid.ID, spiffeid.TrustDomain, error) {
	idStr = strings.TrimSpace(idStr)
	tdStr = strings.TrimSpace(tdStr)
	hasID := idStr != ""
	hasTD := tdStr != ""
	if !hasID && !hasTD {
		return spiffeid.ID{}, spiffeid.TrustDomain{}, fmt.Errorf("must set exactly one of %s_spiffe_id or %s_trust_domain", prefix, prefix)
	}
	if hasID && hasTD {
		return spiffeid.ID{}, spiffeid.TrustDomain{}, fmt.Errorf("cannot set both %s_spiffe_id and %s_trust_domain", prefix, prefix)
	}
	if hasID {
		id, err := spiffeid.FromString(idStr)
		if err != nil {
			return spiffeid.ID{}, spiffeid.TrustDomain{}, fmt.Errorf("invalid %s_spiffe_id %q: %w", prefix, idStr, err)
		}
		return id, spiffeid.TrustDomain{}, nil
	}
	td, err := spiffeid.TrustDomainFromString(tdStr)
	if err != nil {
		return spiffeid.ID{}, spiffeid.TrustDomain{}, fmt.Errorf("invalid %s_trust_domain %q: %w", prefix, tdStr, err)
	}
	return spiffeid.ID{}, td, nil
}

// ValidateServerConfig validates server configuration and returns parsed authorization policy.
func ValidateServerConfig(cfg *ServerFileConfig) (SPIREConfig, ServerAuthz, error) {
	spireConfig, err := validateSPIRESection(cfg.SPIRE)
	if err != nil {
		return SPIREConfig{}, ServerAuthz{}, err
	}
	if strings.TrimSpace(cfg.Server.ListenAddr) == "" {
		return SPIREConfig{}, ServerAuthz{}, errors.New("server.listen_addr must be set")
	}
	id, td, err := validateAuthz(cfg.Server.AllowedClientSPIFFEID, cfg.Server.AllowedClientTrustDomain, "server.allowed_client")
	if err != nil {
		return SPIREConfig{}, ServerAuthz{}, err
	}
	return spireConfig, ServerAuthz{ID: id, TrustDomain: td}, nil
}

// ValidateClientConfig validates client configuration and returns parsed verification policy.
func ValidateClientConfig(cfg *ClientFileConfig) (SPIREConfig, ClientAuthz, error) {
	spireConfig, err := validateSPIRESection(cfg.SPIRE)
	if err != nil {
		return SPIREConfig{}, ClientAuthz{}, err
	}
	id, td, err := validateAuthz(cfg.Client.ExpectedServerSPIFFEID, cfg.Client.ExpectedServerTrustDomain, "client.expected_server")
	if err != nil {
		return SPIREConfig{}, ClientAuthz{}, err
	}
	return spireConfig, ClientAuthz{ID: id, TrustDomain: td}, nil
}
