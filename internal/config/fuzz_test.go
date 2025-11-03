package config

import (
	"testing"
)

// FuzzLoad tests the config loading function with random inputs
// to find panics, crashes, or unexpected behavior
func FuzzLoad(f *testing.F) {
	// Seed corpus with valid YAML examples
	f.Add([]byte(`
spire:
  workload_socket: unix:///tmp/spire-agent/public/api.sock
  initial_fetch_timeout: 30s
server:
  listen_addr: :8443
  allowed_client_spiffe_id: spiffe://example.org/myservice
`))

	f.Add([]byte(`
spire:
  workload_socket: /tmp/agent.sock
client:
  expected_server_spiffe_id: spiffe://example.org/server
`))

	// Fuzz with random YAML-like data
	f.Fuzz(func(t *testing.T, data []byte) {
		// Write to temp file
		tmpfile := t.TempDir() + "/fuzz_config.yaml"
		if err := WriteForTest(tmpfile, data); err != nil {
			t.Skip()
		}

		// Try to load - should never panic
		_, _ = Load(tmpfile)
	})
}

// FuzzValidateServer tests server config validation with random inputs
func FuzzValidateServer(f *testing.F) {
	// Seed with valid configs
	f.Add("unix:///tmp/agent.sock", ":8443", "spiffe://example.org/client", "", "30s")
	f.Add("/tmp/agent.sock", "0.0.0.0:443", "", "example.org", "1m")

	f.Fuzz(func(t *testing.T, socket, addr, clientID, trustDomain, timeout string) {
		cfg := &FileConfig{
			SPIRE: SPIRESection{
				WorkloadSocket:      socket,
				InitialFetchTimeout: timeout,
			},
			Server: ServerSection{
				ListenAddr:                 addr,
				AllowedClientSPIFFEID:      clientID,
				AllowedClientTrustDomain:   trustDomain,
			},
		}

		// Should never panic, even with malformed input
		_, _, _ = ValidateServer(cfg)
	})
}

// FuzzValidateClient tests client config validation with random inputs
func FuzzValidateClient(f *testing.F) {
	// Seed with valid configs
	f.Add("unix:///tmp/agent.sock", "spiffe://example.org/server", "", "30s")
	f.Add("/tmp/agent.sock", "", "example.org", "1m")

	f.Fuzz(func(t *testing.T, socket, serverID, trustDomain, timeout string) {
		cfg := &FileConfig{
			SPIRE: SPIRESection{
				WorkloadSocket:      socket,
				InitialFetchTimeout: timeout,
			},
			Client: ClientSection{
				ExpectedServerSPIFFEID:      serverID,
				ExpectedServerTrustDomain:   trustDomain,
			},
		}

		// Should never panic, even with malformed input
		_, _, _ = ValidateClient(cfg)
	})
}
