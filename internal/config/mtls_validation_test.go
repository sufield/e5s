package config

// Validation Tests
//
// These tests verify comprehensive validation rules for mTLS configuration.
// Tests cover socket path validation, trust domain validation, HTTP configuration
// validation, timeout validation, and SPIFFE ID format validation.
//
// Run these tests with:
//
//	go test ./internal/config/... -v -run TestValidate
//	go test ./internal/config/... -cover

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestValidate_EnhancedChecks tests the validation rules
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
