package identityserver

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateConfig_Valid(t *testing.T) {
	cfg := Config{
		WorkloadAPI: WorkloadAPIConfig{
			SocketPath: "unix:///tmp/spire-agent/public/api.sock",
		},
		HTTP: HTTPConfig{
			Address:           ":8443",
			ReadHeaderTimeout: 10 * time.Second,
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
		HTTP: HTTPConfig{
			Address:           ":8443",
			ReadHeaderTimeout: 10 * time.Second,
		},
	}

	err := validateConfig(cfg)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "SocketPath is required")
}

func TestValidateConfig_MissingAddress(t *testing.T) {
	cfg := Config{
		WorkloadAPI: WorkloadAPIConfig{
			SocketPath: "unix:///tmp/spire-agent/public/api.sock",
		},
		HTTP: HTTPConfig{
			Address:           "", // Missing
			ReadHeaderTimeout: 10 * time.Second,
		},
	}

	err := validateConfig(cfg)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "Address is required")
}

func TestValidateConfig_MissingReadHeaderTimeout(t *testing.T) {
	cfg := Config{
		WorkloadAPI: WorkloadAPIConfig{
			SocketPath: "unix:///tmp/spire-agent/public/api.sock",
		},
		HTTP: HTTPConfig{
			Address:           ":8443",
			ReadHeaderTimeout: 0, // Missing
		},
	}

	err := validateConfig(cfg)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "ReadHeaderTimeout must be > 0")
}

func TestCreateAuthorizer_SpecificID(t *testing.T) {
	cfg := SPIFFEConfig{
		AllowedClientID: "spiffe://example.org/client",
	}

	authorizer, err := createAuthorizer(cfg)
	require.NoError(t, err)
	assert.NotNil(t, authorizer)
}

func TestCreateAuthorizer_TrustDomain(t *testing.T) {
	cfg := SPIFFEConfig{
		AllowedTrustDomain: "example.org",
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
		AllowedClientID: "not-a-spiffe-id", // Invalid format
	}

	authorizer, err := createAuthorizer(cfg)
	require.Error(t, err)
	assert.Nil(t, authorizer)
	assert.Contains(t, err.Error(), "invalid AllowedClientID")
}

func TestCreateAuthorizer_InvalidTrustDomain(t *testing.T) {
	cfg := SPIFFEConfig{
		AllowedTrustDomain: "invalid domain with spaces", // Invalid format (should be valid domain)
	}

	authorizer, err := createAuthorizer(cfg)
	require.Error(t, err)
	assert.Nil(t, authorizer)
	assert.Contains(t, err.Error(), "invalid AllowedTrustDomain")
}
