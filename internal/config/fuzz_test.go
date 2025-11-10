package config

import (
	"testing"
)

// FuzzValidateServer tests server config validation with random inputs
func FuzzValidateServer(f *testing.F) {
	// Seed with valid configs
	f.Add("unix:///tmp/agent.sock", ":8443", "spiffe://example.org/client", "", "30s")
	f.Add("/tmp/agent.sock", "0.0.0.0:443", "", "example.org", "1m")

	f.Fuzz(func(t *testing.T, socket, addr, clientID, trustDomain, timeout string) {
		cfg := &ServerFileConfig{
			SPIRE: SPIRESection{
				WorkloadSocket:      socket,
				InitialFetchTimeout: timeout,
			},
			Server: ServerSection{
				ListenAddr:               addr,
				AllowedClientSPIFFEID:    clientID,
				AllowedClientTrustDomain: trustDomain,
			},
		}

		// Should never panic, even with malformed input
		_, _, _ = ValidateServerConfig(cfg)
	})
}

// FuzzValidateClient tests client config validation with random inputs
func FuzzValidateClient(f *testing.F) {
	// Seed with valid configs
	f.Add("unix:///tmp/agent.sock", "spiffe://example.org/server", "", "30s")
	f.Add("/tmp/agent.sock", "", "example.org", "1m")

	f.Fuzz(func(t *testing.T, socket, serverID, trustDomain, timeout string) {
		cfg := &ClientFileConfig{
			SPIRE: SPIRESection{
				WorkloadSocket:      socket,
				InitialFetchTimeout: timeout,
			},
			Client: ClientSection{
				ExpectedServerSPIFFEID:    serverID,
				ExpectedServerTrustDomain: trustDomain,
			},
		}

		// Should never panic, even with malformed input
		_, _, _ = ValidateClientConfig(cfg)
	})
}
