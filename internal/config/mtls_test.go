package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadFromFile(t *testing.T) {
	// Create a temporary config file
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	configContent := `
http:
  enabled: true
  port: 9443
  timeout: 60s
  authentication:
    policy: trust-domain
    trust_domain: example.org

spire:
  socket_path: unix:///custom/path/api.sock
  trust_domain: example.org
`

	err := os.WriteFile(configPath, []byte(configContent), 0644)
	require.NoError(t, err)

	cfg, err := LoadFromFile(configPath)
	require.NoError(t, err)
	require.NotNil(t, cfg)

	assert.True(t, cfg.HTTP.Enabled)
	assert.Equal(t, 9443, cfg.HTTP.Port)
	assert.Equal(t, 60*time.Second, cfg.HTTP.Timeout)
	assert.Equal(t, "trust-domain", cfg.HTTP.Auth.Policy)
	assert.Equal(t, "example.org", cfg.HTTP.Auth.TrustDomain)
	assert.Equal(t, "unix:///custom/path/api.sock", cfg.SPIRE.SocketPath)
	assert.Equal(t, "example.org", cfg.SPIRE.TrustDomain)
}

func TestLoadFromFile_NonexistentFile(t *testing.T) {
	cfg, err := LoadFromFile("/nonexistent/path/config.yaml")
	assert.Error(t, err)
	assert.Nil(t, cfg)
}

func TestLoadFromFile_InvalidYAML(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "invalid.yaml")

	invalidContent := `
http:
  enabled: true
  port: "not a number"
    bad indentation
`

	err := os.WriteFile(configPath, []byte(invalidContent), 0644)
	require.NoError(t, err)

	cfg, err := LoadFromFile(configPath)
	assert.Error(t, err)
	assert.Nil(t, cfg)
}

func TestLoadFromEnv(t *testing.T) {
	// Save original env vars
	origSocket := os.Getenv("SPIRE_AGENT_SOCKET")
	origTrust := os.Getenv("SPIRE_TRUST_DOMAIN")
	origAddress := os.Getenv("HTTP_ADDRESS")
	origPolicy := os.Getenv("AUTH_POLICY")
	origAllowedID := os.Getenv("ALLOWED_CLIENT_ID")

	// Clean up env vars after test
	defer func() {
		os.Setenv("SPIRE_AGENT_SOCKET", origSocket)
		os.Setenv("SPIRE_TRUST_DOMAIN", origTrust)
		os.Setenv("HTTP_ADDRESS", origAddress)
		os.Setenv("AUTH_POLICY", origPolicy)
		os.Setenv("ALLOWED_CLIENT_ID", origAllowedID)
	}()

	// Set test env vars
	os.Setenv("SPIRE_AGENT_SOCKET", "unix:///test/socket")
	os.Setenv("SPIRE_TRUST_DOMAIN", "test.org")
	os.Setenv("HTTP_ADDRESS", ":9999")
	os.Setenv("AUTH_POLICY", "specific-id")
	os.Setenv("ALLOWED_CLIENT_ID", "spiffe://test.org/client")

	cfg := LoadFromEnv()
	require.NotNil(t, cfg)

	assert.Equal(t, "unix:///test/socket", cfg.SPIRE.SocketPath)
	assert.Equal(t, "test.org", cfg.SPIRE.TrustDomain)
	assert.Equal(t, ":9999", cfg.HTTP.Address)
	assert.Equal(t, "specific-id", cfg.HTTP.Auth.Policy)
	assert.Equal(t, "spiffe://test.org/client", cfg.HTTP.Auth.AllowedID)
}

func TestEnvOverrides(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	configContent := `
http:
  address: ":8443"
  authentication:
    policy: any

spire:
  socket_path: unix:///default/api.sock
  trust_domain: default.org
`

	err := os.WriteFile(configPath, []byte(configContent), 0644)
	require.NoError(t, err)

	// Save and set env var
	origSocket := os.Getenv("SPIRE_AGENT_SOCKET")
	defer os.Setenv("SPIRE_AGENT_SOCKET", origSocket)
	os.Setenv("SPIRE_AGENT_SOCKET", "unix:///override/socket")

	cfg, err := LoadFromFile(configPath)
	require.NoError(t, err)

	// Env var should override file value
	assert.Equal(t, "unix:///override/socket", cfg.SPIRE.SocketPath)
	// File value should remain for non-overridden fields
	assert.Equal(t, "default.org", cfg.SPIRE.TrustDomain)
}

func TestApplyDefaults(t *testing.T) {
	cfg := &MTLSConfig{}
	applyDefaults(cfg)

	assert.Equal(t, "unix:///tmp/spire-agent/public/api.sock", cfg.SPIRE.SocketPath)
	assert.Equal(t, "example.org", cfg.SPIRE.TrustDomain)
	assert.Equal(t, ":8443", cfg.HTTP.Address)
	assert.Equal(t, 30*time.Second, cfg.HTTP.Timeout)
	assert.Equal(t, "any", cfg.HTTP.Auth.Policy)
}

func TestApplyDefaults_PortToAddress(t *testing.T) {
	cfg := &MTLSConfig{
		HTTP: HTTPConfig{
			Port: 9443,
		},
	}
	applyDefaults(cfg)

	assert.Equal(t, ":9443", cfg.HTTP.Address)
}

func TestValidate_Valid(t *testing.T) {
	tests := []struct {
		name string
		cfg  *MTLSConfig
	}{
		{
			name: "any policy",
			cfg: &MTLSConfig{
				SPIRE: SPIREConfig{
					SocketPath:  "unix:///test/socket",
					TrustDomain: "test.org",
				},
				HTTP: HTTPConfig{
					Auth: AuthConfig{
						Policy: "any",
					},
				},
			},
		},
		{
			name: "trust-domain policy",
			cfg: &MTLSConfig{
				SPIRE: SPIREConfig{
					SocketPath:  "unix:///test/socket",
					TrustDomain: "test.org",
				},
				HTTP: HTTPConfig{
					Auth: AuthConfig{
						Policy:      "trust-domain",
						TrustDomain: "test.org",
					},
				},
			},
		},
		{
			name: "specific-id policy",
			cfg: &MTLSConfig{
				SPIRE: SPIREConfig{
					SocketPath:  "unix:///test/socket",
					TrustDomain: "test.org",
				},
				HTTP: HTTPConfig{
					Auth: AuthConfig{
						Policy:    "specific-id",
						AllowedID: "spiffe://test.org/client",
					},
				},
			},
		},
		{
			name: "one-of policy",
			cfg: &MTLSConfig{
				SPIRE: SPIREConfig{
					SocketPath:  "unix:///test/socket",
					TrustDomain: "test.org",
				},
				HTTP: HTTPConfig{
					Auth: AuthConfig{
						Policy:     "one-of",
						AllowedIDs: []string{"spiffe://test.org/client1", "spiffe://test.org/client2"},
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cfg.Validate()
			assert.NoError(t, err)
		})
	}
}

func TestValidate_Invalid(t *testing.T) {
	tests := []struct {
		name        string
		cfg         *MTLSConfig
		expectedErr string
	}{
		{
			name: "missing socket path",
			cfg: &MTLSConfig{
				SPIRE: SPIREConfig{
					TrustDomain: "test.org",
				},
			},
			expectedErr: "socket_path is required",
		},
		{
			name: "missing trust domain",
			cfg: &MTLSConfig{
				SPIRE: SPIREConfig{
					SocketPath: "unix:///test/socket",
				},
			},
			expectedErr: "trust_domain is required",
		},
		{
			name: "invalid policy",
			cfg: &MTLSConfig{
				SPIRE: SPIREConfig{
					SocketPath:  "unix:///test/socket",
					TrustDomain: "test.org",
				},
				HTTP: HTTPConfig{
					Auth: AuthConfig{
						Policy: "invalid-policy",
					},
				},
			},
			expectedErr: "invalid auth policy",
		},
		{
			name: "specific-id without allowed_id",
			cfg: &MTLSConfig{
				SPIRE: SPIREConfig{
					SocketPath:  "unix:///test/socket",
					TrustDomain: "test.org",
				},
				HTTP: HTTPConfig{
					Auth: AuthConfig{
						Policy: "specific-id",
					},
				},
			},
			expectedErr: "requires allowed_id",
		},
		{
			name: "one-of without allowed_ids",
			cfg: &MTLSConfig{
				SPIRE: SPIREConfig{
					SocketPath:  "unix:///test/socket",
					TrustDomain: "test.org",
				},
				HTTP: HTTPConfig{
					Auth: AuthConfig{
						Policy: "one-of",
					},
				},
			},
			expectedErr: "requires allowed_ids",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cfg.Validate()
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.expectedErr)
		})
	}
}

func TestHTTPEnabledEnvVar(t *testing.T) {
	tests := []struct {
		name     string
		value    string
		expected bool
	}{
		{"true string", "true", true},
		{"1 string", "1", true},
		{"false string", "false", false},
		{"0 string", "0", false},
		{"empty string", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			orig := os.Getenv("HTTP_ENABLED")
			defer os.Setenv("HTTP_ENABLED", orig)

			os.Setenv("HTTP_ENABLED", tt.value)

			cfg := LoadFromEnv()
			assert.Equal(t, tt.expected, cfg.HTTP.Enabled)
		})
	}
}
