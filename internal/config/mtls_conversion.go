package config

import (
	"fmt"
	"strings"

	"github.com/pocket/hexagon/spire/internal/ports"
)

// ToServerConfig converts MTLSConfig to ports.MTLSConfig for server use.
//
// Mapping rules (lossless only):
//   - any            -> no SPIFFE restriction (both fields empty)
//   - trust-domain   -> SPIFFE.AllowedTrustDomain = Auth.TrustDomain (required)
//   - specific-id    -> SPIFFE.AllowedPeerID = Auth.AllowedIDs[0] (exactly one)
//   - one-of         -> NOT SUPPORTED by server adapter (returns error)
//
// Returns error if the selected policy cannot be losslessly mapped.
func (c *MTLSConfig) ToServerConfig() (ports.MTLSConfig, error) {
	var spiffe ports.SPIFFEConfig

	switch strings.ToLower(c.HTTP.Auth.PeerVerification) {
	case "", "any":
		// no restriction
	case "trust-domain":
		if strings.TrimSpace(c.HTTP.Auth.TrustDomain) == "" {
			return ports.MTLSConfig{}, fmt.Errorf("peer_verification=trust-domain requires authentication.trust_domain")
		}
		spiffe.AllowedTrustDomain = c.HTTP.Auth.TrustDomain
	case "specific-id":
		if len(c.HTTP.Auth.AllowedIDs) != 1 {
			return ports.MTLSConfig{}, fmt.Errorf("peer_verification=specific-id requires exactly one authentication.allowed_ids entry")
		}
		spiffe.AllowedPeerID = c.HTTP.Auth.AllowedIDs[0]
	case "one-of":
		// identityserver only supports single ID or trust-domain today.
		return ports.MTLSConfig{}, fmt.Errorf("peer_verification=one-of is not supported by the server adapter; use specific-id or trust-domain")
	default:
		return ports.MTLSConfig{}, fmt.Errorf("invalid peer_verification %q (expected: any, trust-domain, specific-id, one-of)", c.HTTP.Auth.PeerVerification)
	}

	return ports.MTLSConfig{
		WorkloadAPI: ports.WorkloadAPIConfig{
			SocketPath: c.SPIRE.SocketPath,
		},
		SPIFFE: spiffe,
		HTTP: ports.HTTPConfig{
			Address:           c.HTTP.Address,
			ReadHeaderTimeout: c.HTTP.ReadHeaderTimeout,
			WriteTimeout:      c.HTTP.WriteTimeout,
			IdleTimeout:       c.HTTP.IdleTimeout,
		},
	}, nil
}

// ToClientConfig converts MTLSConfig to ports.MTLSConfig for client use.
//
// The client side typically doesn't enforce peer constraints (server does),
// but we mirror the same mapping in case the client adapter uses it.
//
// Mapping rules (lossless only):
//   - any            -> no SPIFFE restriction (both fields empty)
//   - trust-domain   -> SPIFFE.AllowedTrustDomain = Auth.TrustDomain (required)
//   - specific-id    -> SPIFFE.AllowedPeerID = Auth.AllowedIDs[0] (exactly one)
//   - one-of         -> NOT SUPPORTED by client adapter (returns error)
//
// Returns error if the selected policy cannot be losslessly mapped.
func (c *MTLSConfig) ToClientConfig() (ports.MTLSConfig, error) {
	var spiffe ports.SPIFFEConfig

	switch strings.ToLower(c.HTTP.Auth.PeerVerification) {
	case "", "any":
		// no restriction
	case "trust-domain":
		if strings.TrimSpace(c.HTTP.Auth.TrustDomain) == "" {
			return ports.MTLSConfig{}, fmt.Errorf("peer_verification=trust-domain requires authentication.trust_domain")
		}
		spiffe.AllowedTrustDomain = c.HTTP.Auth.TrustDomain
	case "specific-id":
		if len(c.HTTP.Auth.AllowedIDs) != 1 {
			return ports.MTLSConfig{}, fmt.Errorf("peer_verification=specific-id requires exactly one authentication.allowed_ids entry")
		}
		spiffe.AllowedPeerID = c.HTTP.Auth.AllowedIDs[0]
	case "one-of":
		// If a future client adapter supports sets, change ports.SPIFFEConfig first.
		return ports.MTLSConfig{}, fmt.Errorf("peer_verification=one-of is not supported by the client adapter; use specific-id or trust-domain")
	default:
		return ports.MTLSConfig{}, fmt.Errorf("invalid peer_verification %q (expected: any, trust-domain, specific-id, one-of)", c.HTTP.Auth.PeerVerification)
	}

	return ports.MTLSConfig{
		WorkloadAPI: ports.WorkloadAPIConfig{
			SocketPath: c.SPIRE.SocketPath,
		},
		SPIFFE: spiffe,
		HTTP:    ports.HTTPConfig{
			// Client adapter will apply its own defaults
		},
	}, nil
}
