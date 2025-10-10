package config

import "github.com/pocket/hexagon/spire/internal/ports"

// ToServerConfig converts MTLSConfig to ports.MTLSConfig for server use
func (c *MTLSConfig) ToServerConfig() ports.MTLSConfig {
	return ports.MTLSConfig{
		WorkloadAPI: ports.WorkloadAPIConfig{
			SocketPath: c.SPIRE.SocketPath,
		},
		SPIFFE: ports.SPIFFEConfig{
			AllowedPeerID: c.HTTP.Auth.AllowedID,
		},
		HTTP: ports.HTTPConfig{
			Address:           c.HTTP.Address,
			ReadHeaderTimeout: c.HTTP.ReadHeaderTimeout,
			WriteTimeout:      c.HTTP.WriteTimeout,
			IdleTimeout:       c.HTTP.IdleTimeout,
		},
	}
}

// ToClientConfig converts MTLSConfig to ports.MTLSConfig for client use
func (c *MTLSConfig) ToClientConfig() ports.MTLSConfig {
	return ports.MTLSConfig{
		WorkloadAPI: ports.WorkloadAPIConfig{
			SocketPath: c.SPIRE.SocketPath,
		},
		SPIFFE: ports.SPIFFEConfig{
			AllowedPeerID: c.HTTP.Auth.AllowedID,
		},
		HTTP: ports.HTTPConfig{
			Timeout: c.HTTP.Timeout,
		},
	}
}
