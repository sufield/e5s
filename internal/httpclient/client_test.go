package httpclient

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	// Verify WorkloadAPI defaults
	assert.Equal(t, "unix:///tmp/spire-agent/public/api.sock", cfg.WorkloadAPI.SocketPath)

	// Verify SPIFFE defaults (empty = allow any from trust domain)
	assert.Empty(t, cfg.SPIFFE.ExpectedServerID)
	assert.Empty(t, cfg.SPIFFE.ExpectedTrustDomain)

	// Verify HTTP defaults
	assert.Equal(t, 30*time.Second, cfg.HTTP.Timeout)
	assert.Equal(t, 100, cfg.HTTP.MaxIdleConns)
	assert.Equal(t, 10, cfg.HTTP.MaxIdleConnsPerHost)
	assert.Equal(t, 90*time.Second, cfg.HTTP.IdleConnTimeout)
}

func TestConfig_CustomValues(t *testing.T) {
	cfg := Config{
		WorkloadAPI: WorkloadAPIConfig{
			SocketPath: "unix:///custom/path/api.sock",
		},
		SPIFFE: SPIFFEConfig{
			ExpectedServerID:    "spiffe://example.org/server",
			ExpectedTrustDomain: "example.org",
		},
		HTTP: HTTPClientConfig{
			Timeout:             10 * time.Second,
			MaxIdleConns:        50,
			MaxIdleConnsPerHost: 5,
			IdleConnTimeout:     30 * time.Second,
		},
	}

	// Verify custom values are preserved
	assert.Equal(t, "unix:///custom/path/api.sock", cfg.WorkloadAPI.SocketPath)
	assert.Equal(t, "spiffe://example.org/server", cfg.SPIFFE.ExpectedServerID)
	assert.Equal(t, "example.org", cfg.SPIFFE.ExpectedTrustDomain)
	assert.Equal(t, 10*time.Second, cfg.HTTP.Timeout)
	assert.Equal(t, 50, cfg.HTTP.MaxIdleConns)
}

func TestValidateConfig_Valid(t *testing.T) {
	cfg := Config{
		WorkloadAPI: WorkloadAPIConfig{
			SocketPath: "unix:///tmp/spire-agent/public/api.sock",
		},
		HTTP: HTTPClientConfig{
			Timeout: 30 * time.Second,
		},
	}

	err := validateConfig(cfg)
	assert.NoError(t, err)
}

func TestValidateConfig_MissingSocketPath(t *testing.T) {
	cfg := Config{
		WorkloadAPI: WorkloadAPIConfig{
			SocketPath: "", // Missing
		},
		HTTP: HTTPClientConfig{
			Timeout: 30 * time.Second,
		},
	}

	err := validateConfig(cfg)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "SocketPath is required")
}

func TestValidateConfig_MissingTimeout(t *testing.T) {
	cfg := Config{
		WorkloadAPI: WorkloadAPIConfig{
			SocketPath: "unix:///tmp/spire-agent/public/api.sock",
		},
		HTTP: HTTPClientConfig{
			Timeout: 0, // Missing
		},
	}

	err := validateConfig(cfg)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "Timeout must be > 0")
}

func TestCreateAuthorizer_SpecificServerID(t *testing.T) {
	cfg := SPIFFEConfig{
		ExpectedServerID: "spiffe://example.org/server",
	}

	authorizer, err := createAuthorizer(cfg)
	require.NoError(t, err)
	assert.NotNil(t, authorizer)
}

func TestCreateAuthorizer_TrustDomain(t *testing.T) {
	cfg := SPIFFEConfig{
		ExpectedTrustDomain: "example.org",
	}

	authorizer, err := createAuthorizer(cfg)
	require.NoError(t, err)
	assert.NotNil(t, authorizer)
}

func TestCreateAuthorizer_Any(t *testing.T) {
	cfg := SPIFFEConfig{
		// Empty = allow any from trust domain
	}

	authorizer, err := createAuthorizer(cfg)
	require.NoError(t, err)
	assert.NotNil(t, authorizer)
}

func TestCreateAuthorizer_InvalidSPIFFEID(t *testing.T) {
	cfg := SPIFFEConfig{
		ExpectedServerID: "not-a-spiffe-id", // Invalid format
	}

	authorizer, err := createAuthorizer(cfg)
	require.Error(t, err)
	assert.Nil(t, authorizer)
	assert.Contains(t, err.Error(), "invalid ExpectedServerID")
}

func TestCreateAuthorizer_InvalidTrustDomain(t *testing.T) {
	cfg := SPIFFEConfig{
		ExpectedTrustDomain: "invalid domain with spaces", // Invalid format (should be valid domain)
	}

	authorizer, err := createAuthorizer(cfg)
	require.Error(t, err)
	assert.Nil(t, authorizer)
	assert.Contains(t, err.Error(), "invalid ExpectedTrustDomain")
}
