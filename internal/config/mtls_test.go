package config

// mTLS Configuration Tests
//
// These tests verify mTLS configuration loading, validation, defaults, and overrides.
// Tests cover file-based loading, environment variable overrides, validation rules,
// and peer verification policies (any, trust-domain, specific-id, one-of).
//
// Run these tests with:
//
//	go test ./internal/config/... -v
//	go test ./internal/config/... -run TestLoad -v
//	go test ./internal/config/... -cover

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoad_FromFile(t *testing.T) {
	// Create a temporary config file
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	configContent := `
http:
  address: ":9443"
  timeout: 60s
  authentication:
    peer_verification: trust-domain
    trust_domain: example.org

spire:
  socket_path: unix:///custom/path/api.sock
  trust_domain: example.org
  timeout: 45s
`

	err := os.WriteFile(configPath, []byte(configContent), 0o644)
	require.NoError(t, err)

	cfg, err := Load(configPath)
	require.NoError(t, err)
	require.NotNil(t, cfg)

	assert.Equal(t, ":9443", cfg.HTTP.Address)
	assert.Equal(t, 60*time.Second, cfg.HTTP.Timeout)
	assert.Equal(t, "trust-domain", cfg.HTTP.Auth.PeerVerification)
	assert.Equal(t, "example.org", cfg.HTTP.Auth.TrustDomain)
	assert.Equal(t, "unix:///custom/path/api.sock", cfg.SPIRE.SocketPath)
	assert.Equal(t, "example.org", cfg.SPIRE.TrustDomain)
	assert.Equal(t, 45*time.Second, cfg.SPIRE.Timeout)
}

func TestLoad_NonexistentFile(t *testing.T) {
	cfg, err := Load("/nonexistent/path/config.yaml")
	assert.Error(t, err)
	assert.Nil(t, cfg)
}

func TestLoad_InvalidYAML(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "invalid.yaml")

	invalidContent := `
http:
  address: ":8443"
  timeout: "not a duration"
    bad indentation
`

	err := os.WriteFile(configPath, []byte(invalidContent), 0o644)
	require.NoError(t, err)

	cfg, err := Load(configPath)
	assert.Error(t, err)
	assert.Nil(t, cfg)
}

func TestLoad_UnknownYAMLKeys(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "unknown.yaml")

	unknownContent := `
http:
  address: ":8443"
  unknown_field: "should fail"

spire:
  socket_path: unix:///test/socket
  trust_domain: example.org
`

	err := os.WriteFile(configPath, []byte(unknownContent), 0o644)
	require.NoError(t, err)

	cfg, err := Load(configPath)
	assert.Error(t, err)
	assert.Nil(t, cfg)
	assert.Contains(t, err.Error(), "unknown")
}

func TestLoad_FromStdin(t *testing.T) {
	// This test would require mocking os.Stdin, which is complex in Go
	// In production, stdin loading is tested manually with: echo "yaml..." | app -
	t.Skip("Stdin loading requires manual testing")
}

func TestLoad_FromEnv(t *testing.T) {
	// Save original env vars
	origSocket := os.Getenv("SPIRE_AGENT_SOCKET")
	origTrust := os.Getenv("SPIRE_TRUST_DOMAIN")
	origAddress := os.Getenv("HTTP_ADDRESS")
	origPeerVerification := os.Getenv("AUTH_PEER_VERIFICATION")
	origAllowedID := os.Getenv("ALLOWED_ID")

	// Clean up env vars after test
	defer func() {
		os.Setenv("SPIRE_AGENT_SOCKET", origSocket)
		os.Setenv("SPIRE_TRUST_DOMAIN", origTrust)
		os.Setenv("HTTP_ADDRESS", origAddress)
		os.Setenv("AUTH_PEER_VERIFICATION", origPeerVerification)
		os.Setenv("ALLOWED_ID", origAllowedID)
	}()

	// Set test env vars
	os.Setenv("SPIRE_AGENT_SOCKET", "unix:///test/socket")
	os.Setenv("SPIRE_TRUST_DOMAIN", "test.org")
	os.Setenv("HTTP_ADDRESS", ":9999")
	os.Setenv("AUTH_PEER_VERIFICATION", "specific-id")
	os.Setenv("ALLOWED_ID", "spiffe://test.org/client")

	cfg, err := Load("") // Empty path = env-only
	require.NoError(t, err)
	require.NotNil(t, cfg)

	assert.Equal(t, "unix:///test/socket", cfg.SPIRE.SocketPath)
	assert.Equal(t, "test.org", cfg.SPIRE.TrustDomain)
	assert.Equal(t, ":9999", cfg.HTTP.Address)
	assert.Equal(t, "specific-id", cfg.HTTP.Auth.PeerVerification)
	assert.Equal(t, []string{"spiffe://test.org/client"}, cfg.HTTP.Auth.AllowedIDs)
}

func TestEnvOverrides(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	configContent := `
http:
  address: ":8443"
  authentication:
    peer_verification: any

spire:
  socket_path: unix:///default/api.sock
  trust_domain: default.org
`

	err := os.WriteFile(configPath, []byte(configContent), 0o644)
	require.NoError(t, err)

	// Save and set env var
	origSocket := os.Getenv("SPIRE_AGENT_SOCKET")
	defer os.Setenv("SPIRE_AGENT_SOCKET", origSocket)
	os.Setenv("SPIRE_AGENT_SOCKET", "unix:///override/socket")

	cfg, err := Load(configPath)
	require.NoError(t, err)

	// Env var should override file value
	assert.Equal(t, "unix:///override/socket", cfg.SPIRE.SocketPath)
	// File value should remain for non-overridden fields
	assert.Equal(t, "default.org", cfg.SPIRE.TrustDomain)
}

func TestEnvOverrides_AllowedIDs_MutualExclusivity(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	configContent := `
http:
  address: ":8443"
  authentication:
    peer_verification: one-of
    allowed_ids:
      - spiffe://example.org/workload1

spire:
  socket_path: unix:///default/api.sock
  trust_domain: example.org
`

	err := os.WriteFile(configPath, []byte(configContent), 0o644)
	require.NoError(t, err)

	// Save original env vars
	origID := os.Getenv("ALLOWED_ID")
	origIDs := os.Getenv("ALLOWED_IDS")
	defer func() {
		os.Setenv("ALLOWED_ID", origID)
		os.Setenv("ALLOWED_IDS", origIDs)
	}()

	// Set BOTH env vars (should fail)
	os.Setenv("ALLOWED_ID", "spiffe://example.org/single")
	os.Setenv("ALLOWED_IDS", "spiffe://example.org/one,spiffe://example.org/two")

	cfg, err := Load(configPath)
	assert.Error(t, err)
	assert.Nil(t, cfg)
	assert.Contains(t, err.Error(), "mutually exclusive")
}

func TestEnvOverrides_PeerVerification_Invalid(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	configContent := `
http:
  address: ":8443"
  authentication:
    peer_verification: any

spire:
  socket_path: unix:///default/api.sock
  trust_domain: example.org
`

	err := os.WriteFile(configPath, []byte(configContent), 0o644)
	require.NoError(t, err)

	// Save original env var
	origMode := os.Getenv("AUTH_PEER_VERIFICATION")
	defer os.Setenv("AUTH_PEER_VERIFICATION", origMode)

	// Set invalid mode
	os.Setenv("AUTH_PEER_VERIFICATION", "invalid-mode")

	cfg, err := Load(configPath)
	assert.Error(t, err)
	assert.Nil(t, cfg)
	assert.Contains(t, err.Error(), "invalid AUTH_PEER_VERIFICATION")
}

func TestValidate_Valid(t *testing.T) {
	tests := []struct {
		name string
		cfg  *MTLSConfig
	}{
		{
			name: "any peer verification",
			cfg: &MTLSConfig{
				SPIRE: SPIREConfig{
					SocketPath:  "unix:///test/socket",
					TrustDomain: "test.org",
				},
				HTTP: HTTPConfig{
					Address: ":8443",
					Auth: AuthConfig{
						PeerVerification: "any",
					},
				},
			},
		},
		{
			name: "trust-domain peer verification",
			cfg: &MTLSConfig{
				SPIRE: SPIREConfig{
					SocketPath:  "unix:///test/socket",
					TrustDomain: "test.org",
				},
				HTTP: HTTPConfig{
					Address: ":8443",
					Auth: AuthConfig{
						PeerVerification: "trust-domain",
						TrustDomain:      "test.org",
					},
				},
			},
		},
		{
			name: "specific-id peer verification",
			cfg: &MTLSConfig{
				SPIRE: SPIREConfig{
					SocketPath:  "unix:///test/socket",
					TrustDomain: "test.org",
				},
				HTTP: HTTPConfig{
					Address: ":8443",
					Auth: AuthConfig{
						PeerVerification: "specific-id",
						AllowedIDs:       []string{"spiffe://test.org/client"},
					},
				},
			},
		},
		{
			name: "one-of peer verification",
			cfg: &MTLSConfig{
				SPIRE: SPIREConfig{
					SocketPath:  "unix:///test/socket",
					TrustDomain: "test.org",
				},
				HTTP: HTTPConfig{
					Address: ":8443",
					Auth: AuthConfig{
						PeerVerification: "one-of",
						AllowedIDs:       []string{"spiffe://test.org/client1", "spiffe://test.org/client2"},
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
				HTTP: HTTPConfig{
					Address: ":8443",
					Auth: AuthConfig{
						PeerVerification: "any",
					},
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
				HTTP: HTTPConfig{
					Address: ":8443",
					Auth: AuthConfig{
						PeerVerification: "any",
					},
				},
			},
			expectedErr: "trust_domain is required",
		},
		{
			name: "invalid peer verification",
			cfg: &MTLSConfig{
				SPIRE: SPIREConfig{
					SocketPath:  "unix:///test/socket",
					TrustDomain: "test.org",
				},
				HTTP: HTTPConfig{
					Address: ":8443",
					Auth: AuthConfig{
						PeerVerification: "invalid-policy",
					},
				},
			},
			expectedErr: "invalid peer_verification",
		},
		{
			name: "trust-domain without trust_domain",
			cfg: &MTLSConfig{
				SPIRE: SPIREConfig{
					SocketPath:  "unix:///test/socket",
					TrustDomain: "test.org",
				},
				HTTP: HTTPConfig{
					Address: ":8443",
					Auth: AuthConfig{
						PeerVerification: "trust-domain",
					},
				},
			},
			expectedErr: "requires trust_domain",
		},
		{
			name: "specific-id without allowed_ids",
			cfg: &MTLSConfig{
				SPIRE: SPIREConfig{
					SocketPath:  "unix:///test/socket",
					TrustDomain: "test.org",
				},
				HTTP: HTTPConfig{
					Address: ":8443",
					Auth: AuthConfig{
						PeerVerification: "specific-id",
					},
				},
			},
			expectedErr: "requires exactly one allowed_ids",
		},
		{
			name: "specific-id with multiple allowed_ids",
			cfg: &MTLSConfig{
				SPIRE: SPIREConfig{
					SocketPath:  "unix:///test/socket",
					TrustDomain: "test.org",
				},
				HTTP: HTTPConfig{
					Address: ":8443",
					Auth: AuthConfig{
						PeerVerification: "specific-id",
						AllowedIDs:       []string{"spiffe://test.org/one", "spiffe://test.org/two"},
					},
				},
			},
			expectedErr: "requires exactly one allowed_ids",
		},
		{
			name: "one-of without allowed_ids",
			cfg: &MTLSConfig{
				SPIRE: SPIREConfig{
					SocketPath:  "unix:///test/socket",
					TrustDomain: "test.org",
				},
				HTTP: HTTPConfig{
					Address: ":8443",
					Auth: AuthConfig{
						PeerVerification: "one-of",
					},
				},
			},
			expectedErr: "requires at least 2 allowed_ids",
		},
		{
			name: "one-of with single allowed_id",
			cfg: &MTLSConfig{
				SPIRE: SPIREConfig{
					SocketPath:  "unix:///test/socket",
					TrustDomain: "test.org",
				},
				HTTP: HTTPConfig{
					Address: ":8443",
					Auth: AuthConfig{
						PeerVerification: "one-of",
						AllowedIDs:       []string{"spiffe://test.org/single"},
					},
				},
			},
			expectedErr: "requires at least 2 allowed_ids",
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
