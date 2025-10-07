package identityserver

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	// Verify WorkloadAPI defaults
	assert.Equal(t, "unix:///tmp/spire-agent/public/api.sock", cfg.WorkloadAPI.SocketPath)

	// Verify SPIFFE defaults (empty = allow any from trust domain)
	assert.Empty(t, cfg.SPIFFE.AllowedClientID)
	assert.Empty(t, cfg.SPIFFE.AllowedTrustDomain)

	// Verify HTTP defaults
	assert.Equal(t, ":8443", cfg.HTTP.Address)
	assert.Equal(t, 10*time.Second, cfg.HTTP.ReadHeaderTimeout)
	assert.Equal(t, 30*time.Second, cfg.HTTP.ReadTimeout)
	assert.Equal(t, 30*time.Second, cfg.HTTP.WriteTimeout)
	assert.Equal(t, 120*time.Second, cfg.HTTP.IdleTimeout)
}

func TestConfig_CustomValues(t *testing.T) {
	cfg := Config{
		WorkloadAPI: WorkloadAPIConfig{
			SocketPath: "unix:///custom/path/api.sock",
		},
		SPIFFE: SPIFFEConfig{
			AllowedClientID:    "spiffe://example.org/client",
			AllowedTrustDomain: "example.org",
		},
		HTTP: HTTPConfig{
			Address:           ":9443",
			ReadHeaderTimeout: 5 * time.Second,
			ReadTimeout:       15 * time.Second,
			WriteTimeout:      15 * time.Second,
			IdleTimeout:       60 * time.Second,
		},
	}

	// Verify custom values are preserved
	assert.Equal(t, "unix:///custom/path/api.sock", cfg.WorkloadAPI.SocketPath)
	assert.Equal(t, "spiffe://example.org/client", cfg.SPIFFE.AllowedClientID)
	assert.Equal(t, "example.org", cfg.SPIFFE.AllowedTrustDomain)
	assert.Equal(t, ":9443", cfg.HTTP.Address)
	assert.Equal(t, 5*time.Second, cfg.HTTP.ReadHeaderTimeout)
}
