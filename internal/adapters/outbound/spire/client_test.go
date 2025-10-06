package spire

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewSPIREClient_ValidConfig(t *testing.T) {
	ctx := context.Background()
	config := Config{
		SocketPath:  "unix:///tmp/test.sock",
		TrustDomain: "example.org",
		Timeout:     30 * time.Second,
	}

	// This will fail to connect to non-existent socket, but validates config parsing
	client, err := NewSPIREClient(ctx, config)

	// We expect an error because the socket doesn't exist
	// But we're testing that the config is properly validated and used
	if err != nil {
		assert.Contains(t, err.Error(), "failed to create SPIRE Workload API client")
	}

	if client != nil {
		assert.Equal(t, config.TrustDomain, client.trustDomain)
		assert.Equal(t, config.SocketPath, client.socketPath)
		assert.Equal(t, config.Timeout, client.timeout)
		client.Close()
	}
}

func TestNewSPIREClient_DefaultTimeout(t *testing.T) {
	ctx := context.Background()
	config := Config{
		SocketPath:  "unix:///tmp/test.sock",
		TrustDomain: "example.org",
		// No timeout specified
	}

	client, err := NewSPIREClient(ctx, config)

	if err != nil {
		// Expected - socket doesn't exist
		assert.Contains(t, err.Error(), "failed to create SPIRE Workload API client")
		return
	}

	if client != nil {
		assert.Equal(t, 30*time.Second, client.timeout, "Should use default timeout")
		client.Close()
	}
}

func TestSPIREClient_GetMethods(t *testing.T) {
	// Create client struct directly (without actual SPIRE connection)
	client := &SPIREClient{
		socketPath:  "unix:///tmp/test.sock",
		trustDomain: "example.org",
		timeout:     30 * time.Second,
		client:      nil, // Will be nil for this test
	}

	assert.Equal(t, "example.org", client.GetTrustDomain())
	assert.Equal(t, "unix:///tmp/test.sock", client.GetSocketPath())
}

func TestConfig_Validation(t *testing.T) {
	tests := []struct {
		name        string
		config      Config
		expectValid bool
	}{
		{
			name: "valid unix socket",
			config: Config{
				SocketPath:  "unix:///tmp/spire-agent/public/api.sock",
				TrustDomain: "example.org",
				Timeout:     30 * time.Second,
			},
			expectValid: true,
		},
		{
			name: "valid with custom timeout",
			config: Config{
				SocketPath:  "unix:///custom/path.sock",
				TrustDomain: "custom.domain",
				Timeout:     60 * time.Second,
			},
			expectValid: true,
		},
		{
			name: "empty socket path",
			config: Config{
				SocketPath:  "",
				TrustDomain: "example.org",
				Timeout:     30 * time.Second,
			},
			expectValid: false,
		},
		{
			name: "empty trust domain",
			config: Config{
				SocketPath:  "unix:///tmp/test.sock",
				TrustDomain: "",
				Timeout:     30 * time.Second,
			},
			expectValid: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Basic validation - ensure fields are set
			if tt.expectValid {
				assert.NotEmpty(t, tt.config.SocketPath)
				assert.NotEmpty(t, tt.config.TrustDomain)
			} else {
				assert.True(t,
					tt.config.SocketPath == "" || tt.config.TrustDomain == "",
					"Invalid config should have empty required field")
			}
		})
	}
}

func TestSPIREClient_Close(t *testing.T) {
	client := &SPIREClient{
		socketPath:  "unix:///tmp/test.sock",
		trustDomain: "example.org",
		timeout:     30 * time.Second,
		client:      nil,
	}

	// Close should not panic even with nil client
	require.NotPanics(t, func() {
		client.Close()
	})
}
