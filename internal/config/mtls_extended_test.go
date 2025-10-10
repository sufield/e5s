package config

import (
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestEnvOverrides_InvalidValues tests error handling for invalid environment variables
func TestEnvOverrides_InvalidValues(t *testing.T) {
	t.Parallel() // Run in parallel to avoid env var conflicts

	tests := []struct {
		name   string
		envVar string
		value  string
		errMsg string
	}{
		// Port parsing errors (strconv.Atoi failures)
		{
			name:   "invalid port - not a number",
			envVar: "HTTP_PORT",
			value:  "abc",
			errMsg: "invalid HTTP_PORT",
		},
		{
			name:   "invalid port - decimal",
			envVar: "HTTP_PORT",
			value:  "8080.5",
			errMsg: "invalid HTTP_PORT",
		},
		{
			name:   "invalid port - with spaces",
			envVar: "HTTP_PORT",
			value:  " 8080 ",
			errMsg: "invalid HTTP_PORT",
		},
		// Boolean parsing errors
		{
			name:   "invalid boolean",
			envVar: "HTTP_ENABLED",
			value:  "maybe",
			errMsg: "invalid HTTP_ENABLED",
		},
		{
			name:   "invalid boolean - number",
			envVar: "HTTP_ENABLED",
			value:  "2",
			errMsg: "invalid HTTP_ENABLED",
		},
		// Duration parsing errors
		{
			name:   "invalid timeout - not a duration",
			envVar: "HTTP_TIMEOUT",
			value:  "not-a-duration",
			errMsg: "invalid HTTP_TIMEOUT",
		},
		{
			name:   "invalid timeout - missing unit",
			envVar: "HTTP_TIMEOUT",
			value:  "30",
			errMsg: "invalid HTTP_TIMEOUT",
		},
		{
			name:   "invalid timeout - spaces in unit",
			envVar: "HTTP_TIMEOUT",
			value:  "30 s",
			errMsg: "invalid HTTP_TIMEOUT",
		},
		{
			name:   "invalid timeout - empty",
			envVar: "HTTP_TIMEOUT",
			value:  "invalid",
			errMsg: "invalid HTTP_TIMEOUT",
		},
		{
			name:   "invalid read header timeout",
			envVar: "HTTP_READ_HEADER_TIMEOUT",
			value:  "5x",
			errMsg: "invalid HTTP_READ_HEADER_TIMEOUT",
		},
		{
			name:   "invalid read timeout",
			envVar: "HTTP_READ_TIMEOUT",
			value:  "10wrong",
			errMsg: "invalid HTTP_READ_TIMEOUT",
		},
		{
			name:   "invalid write timeout",
			envVar: "HTTP_WRITE_TIMEOUT",
			value:  "bad",
			errMsg: "invalid HTTP_WRITE_TIMEOUT",
		},
		{
			name:   "invalid idle timeout",
			envVar: "HTTP_IDLE_TIMEOUT",
			value:  "nope",
			errMsg: "invalid HTTP_IDLE_TIMEOUT",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Save original value
			orig := os.Getenv(tt.envVar)
			defer os.Setenv(tt.envVar, orig)

			// Set invalid env var
			os.Setenv(tt.envVar, tt.value)

			// Attempt to load from env
			_, err := LoadFromEnv()

			// Empty string should be ignored (not cause error)
			if tt.errMsg == "" {
				assert.NoError(t, err, "Empty env var should be ignored")
			} else {
				// Should fail with descriptive error
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)
			}
		})
	}
}

// TestParseBool_AllVariants tests all boolean parsing variants
func TestParseBool_AllVariants(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected bool
		wantErr  bool
	}{
		// True variants
		{"true lowercase", "true", true, false},
		{"true uppercase", "TRUE", true, false},
		{"true mixed", "True", true, false},
		{"1 string", "1", true, false},
		{"yes lowercase", "yes", true, false},
		{"yes uppercase", "YES", true, false},
		{"on lowercase", "on", true, false},
		{"on uppercase", "ON", true, false},

		// False variants
		{"false lowercase", "false", false, false},
		{"false uppercase", "FALSE", false, false},
		{"0 string", "0", false, false},
		{"no lowercase", "no", false, false},
		{"no uppercase", "NO", false, false},
		{"off lowercase", "off", false, false},
		{"off uppercase", "OFF", false, false},

		// Invalid
		{"maybe", "maybe", false, true},
		{"empty", "", false, true},
		{"random", "xyz", false, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := parseBool(tt.input)

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}

// TestEnvOverrides_CommaSeparatedIDs tests ALLOWED_IDS parsing
func TestEnvOverrides_CommaSeparatedIDs(t *testing.T) {
	tests := []struct {
		name     string
		envValue string
		expected []string
	}{
		{
			name:     "single ID",
			envValue: "spiffe://example.org/service1",
			expected: []string{"spiffe://example.org/service1"},
		},
		{
			name:     "multiple IDs with spaces",
			envValue: "spiffe://example.org/service1, spiffe://example.org/service2, spiffe://example.org/service3",
			expected: []string{
				"spiffe://example.org/service1",
				"spiffe://example.org/service2",
				"spiffe://example.org/service3",
			},
		},
		{
			name:     "multiple IDs no spaces",
			envValue: "spiffe://example.org/service1,spiffe://example.org/service2",
			expected: []string{
				"spiffe://example.org/service1",
				"spiffe://example.org/service2",
			},
		},
		{
			name:     "IDs with extra whitespace",
			envValue: "  spiffe://example.org/service1  ,  spiffe://example.org/service2  ",
			expected: []string{
				"spiffe://example.org/service1",
				"spiffe://example.org/service2",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Save original values
			origAllowedIDs := os.Getenv("ALLOWED_IDS")
			origSocket := os.Getenv("SPIRE_AGENT_SOCKET")
			defer func() {
				os.Setenv("ALLOWED_IDS", origAllowedIDs)
				os.Setenv("SPIRE_AGENT_SOCKET", origSocket)
			}()

			os.Setenv("ALLOWED_IDS", tt.envValue)
			os.Setenv("SPIRE_AGENT_SOCKET", "unix:///tmp/test")

			cfg, err := LoadFromEnv()
			require.NoError(t, err)

			assert.Equal(t, tt.expected, cfg.HTTP.Auth.AllowedIDs)
		})
	}
}

// TestValidate_EnhancedChecks tests the new validation rules
func TestValidate_EnhancedChecks(t *testing.T) {
	tests := []struct {
		name    string
		config  *MTLSConfig
		wantErr string
	}{
		{
			name: "invalid socket path - no unix prefix",
			config: &MTLSConfig{
				SPIRE: SPIREConfig{
					SocketPath:  "/tmp/socket",
					TrustDomain: "example.org",
				},
			},
			wantErr: "must start with 'unix://'",
		},
		{
			name: "http enabled without address",
			config: &MTLSConfig{
				SPIRE: SPIREConfig{
					SocketPath:  "unix:///tmp/socket",
					TrustDomain: "example.org",
				},
				HTTP: HTTPConfig{
					Enabled: true,
					Address: "",
				},
			},
			wantErr: "http.address is required when http.enabled is true",
		},
		{
			name: "trust domain with scheme",
			config: &MTLSConfig{
				SPIRE: SPIREConfig{
					SocketPath:  "unix:///tmp/socket",
					TrustDomain: "https://example.org",
				},
			},
			wantErr: "must not contain scheme",
		},
		{
			name: "trust domain with path",
			config: &MTLSConfig{
				SPIRE: SPIREConfig{
					SocketPath:  "unix:///tmp/socket",
					TrustDomain: "example.org/path",
				},
			},
			wantErr: "must not contain path",
		},
		{
			name: "port too high",
			config: &MTLSConfig{
				SPIRE: SPIREConfig{
					SocketPath:  "unix:///tmp/socket",
					TrustDomain: "example.org",
				},
				HTTP: HTTPConfig{
					Port: 70000,
				},
			},
			wantErr: "must be between",
		},
		{
			name: "negative timeout",
			config: &MTLSConfig{
				SPIRE: SPIREConfig{
					SocketPath:  "unix:///tmp/socket",
					TrustDomain: "example.org",
				},
				HTTP: HTTPConfig{
					Timeout: -5 * time.Second,
				},
			},
			wantErr: "must be positive",
		},
		{
			name: "invalid SPIFFE ID format",
			config: &MTLSConfig{
				SPIRE: SPIREConfig{
					SocketPath:  "unix:///tmp/socket",
					TrustDomain: "example.org",
				},
				HTTP: HTTPConfig{
					Auth: AuthConfig{
						PeerVerification: "specific-id",
						AllowedID:        "https://example.org/service",
					},
				},
			},
			wantErr: "must be a valid SPIFFE ID",
		},
		{
			name: "invalid SPIFFE ID in allowed_ids array",
			config: &MTLSConfig{
				SPIRE: SPIREConfig{
					SocketPath:  "unix:///tmp/socket",
					TrustDomain: "example.org",
				},
				HTTP: HTTPConfig{
					Auth: AuthConfig{
						PeerVerification: "one-of",
						AllowedIDs: []string{
							"spiffe://example.org/service1",
							"http://example.org/service2", // Invalid
						},
					},
				},
			},
			wantErr: "allowed_ids[1]",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Apply defaults first (unless we're testing the enabled/address case)
			// The "http enabled without address" test should not apply defaults
			// because applyDefaults would set a default address
			if tt.name != "http enabled without address" {
				applyDefaults(tt.config)
			}

			err := tt.config.Validate()
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantErr)
		})
	}
}

// TestTimeoutOverrides tests all timeout environment variables
func TestTimeoutOverrides(t *testing.T) {
	// Save original values
	origSocket := os.Getenv("SPIRE_AGENT_SOCKET")
	origHTTPTimeout := os.Getenv("HTTP_TIMEOUT")
	origReadHeaderTimeout := os.Getenv("HTTP_READ_HEADER_TIMEOUT")
	origReadTimeout := os.Getenv("HTTP_READ_TIMEOUT")
	origWriteTimeout := os.Getenv("HTTP_WRITE_TIMEOUT")
	origIdleTimeout := os.Getenv("HTTP_IDLE_TIMEOUT")

	defer func() {
		os.Setenv("SPIRE_AGENT_SOCKET", origSocket)
		os.Setenv("HTTP_TIMEOUT", origHTTPTimeout)
		os.Setenv("HTTP_READ_HEADER_TIMEOUT", origReadHeaderTimeout)
		os.Setenv("HTTP_READ_TIMEOUT", origReadTimeout)
		os.Setenv("HTTP_WRITE_TIMEOUT", origWriteTimeout)
		os.Setenv("HTTP_IDLE_TIMEOUT", origIdleTimeout)
	}()

	os.Setenv("SPIRE_AGENT_SOCKET", "unix:///tmp/test")
	os.Setenv("HTTP_TIMEOUT", "45s")
	os.Setenv("HTTP_READ_HEADER_TIMEOUT", "15s")
	os.Setenv("HTTP_READ_TIMEOUT", "60s")
	os.Setenv("HTTP_WRITE_TIMEOUT", "50s")
	os.Setenv("HTTP_IDLE_TIMEOUT", "180s")

	cfg, err := LoadFromEnv()
	require.NoError(t, err)

	assert.Equal(t, 45*time.Second, cfg.HTTP.Timeout)
	assert.Equal(t, 15*time.Second, cfg.HTTP.ReadHeaderTimeout)
	assert.Equal(t, 60*time.Second, cfg.HTTP.ReadTimeout)
	assert.Equal(t, 50*time.Second, cfg.HTTP.WriteTimeout)
	assert.Equal(t, 180*time.Second, cfg.HTTP.IdleTimeout)
}

// TestDefaults_AllTimeouts tests that all timeout defaults are applied
func TestDefaults_AllTimeouts(t *testing.T) {
	cfg := &MTLSConfig{}
	applyDefaults(cfg)

	assert.Equal(t, DefaultHTTPTimeout, cfg.HTTP.Timeout)
	assert.Equal(t, DefaultReadHeaderTimeout, cfg.HTTP.ReadHeaderTimeout)
	assert.Equal(t, DefaultReadTimeout, cfg.HTTP.ReadTimeout)
	assert.Equal(t, DefaultWriteTimeout, cfg.HTTP.WriteTimeout)
	assert.Equal(t, DefaultIdleTimeout, cfg.HTTP.IdleTimeout)
}
