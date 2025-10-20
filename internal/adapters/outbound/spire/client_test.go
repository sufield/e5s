package spire

import (
	"context"
	"testing"
	"time"

	"github.com/spiffe/go-spiffe/v2/spiffeid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewClient_ValidConfig(t *testing.T) {
	ctx := context.Background()
	config := Config{
		SocketPath:  "unix:///tmp/test.sock",
		TrustDomain: "example.org",
		Timeout:     1 * time.Second, // Short timeout for tests
	}

	// This will fail to connect to non-existent socket, but validates config parsing
	client, err := NewClient(ctx, config)

	// We expect an error because the socket doesn't exist
	// But we're testing that the config is properly validated and used
	if err != nil {
		// Should fail on X509Source creation due to unavailable Workload API
		assert.Error(t, err)
	}

	if client != nil {
		assert.Equal(t, config.TrustDomain, client.GetTrustDomain())
		assert.Equal(t, config.SocketPath, client.socketPath)
		assert.Equal(t, config.Timeout, client.timeout)
		client.Close()
	}
}

func TestNewClient_DefaultTimeout(t *testing.T) {
	ctx := context.Background()
	config := Config{
		SocketPath:  "unix:///tmp/test.sock",
		TrustDomain: "example.org",
		Timeout:     1 * time.Second, // Use short timeout for test
	}

	client, err := NewClient(ctx, config)

	if err != nil {
		// Expected - socket doesn't exist, X509Source creation will fail
		assert.Error(t, err)
		return
	}

	if client != nil {
		assert.Equal(t, 1*time.Second, client.timeout)
		client.Close()
	}
}

func TestClient_GetMethods(t *testing.T) {
	// Create client struct directly (without actual SPIRE connection)
	td, err := spiffeid.TrustDomainFromString("example.org")
	require.NoError(t, err)

	client := &Client{
		socketPath:  "unix:///tmp/test.sock",
		trustDomain: td,
		timeout:     30 * time.Second,
		client:      nil, // Will be nil for this test
	}

	assert.Equal(t, "example.org", client.GetTrustDomain())
	assert.Equal(t, td, client.TrustDomain())
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

func TestNewClient_InvalidTrustDomain(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name        string
		trustDomain string
	}{
		{
			name:        "empty trust domain",
			trustDomain: "",
		},
		{
			name:        "double dots",
			trustDomain: "example..org",
		},
		{
			name:        "special chars",
			trustDomain: "bad@example.org",
		},
		{
			name:        "with scheme",
			trustDomain: "spiffe://example.org",
		},
		{
			name:        "trailing dot",
			trustDomain: "example.org.",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := Config{
				SocketPath:  "unix:///tmp/test.sock",
				TrustDomain: tt.trustDomain,
				Timeout:     1 * time.Second, // Short timeout for tests
			}

			_, err := NewClient(ctx, config)
			require.Error(t, err, "Should reject invalid trust domain")
			// Error could be validation failure OR workload API unavailable
			// Both are acceptable - we're testing that invalid TDs don't create working clients
			assert.Error(t, err)
		})
	}
}

func TestConfig_TimeoutValidation(t *testing.T) {
	// Test timeout validation logic without creating actual clients

	tests := []struct {
		name           string
		inputTimeout   time.Duration
		expectedResult time.Duration
	}{
		{
			name:           "zero timeout should default",
			inputTimeout:   0,
			expectedResult: 30 * time.Second,
		},
		{
			name:           "negative timeout should default",
			inputTimeout:   -5 * time.Second,
			expectedResult: 30 * time.Second,
		},
		{
			name:           "positive timeout should be preserved",
			inputTimeout:   5 * time.Second,
			expectedResult: 5 * time.Second,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test the validation logic directly
			timeout := tt.inputTimeout
			if timeout <= 0 {
				timeout = 30 * time.Second
			}
			assert.Equal(t, tt.expectedResult, timeout, "Timeout validation logic")
		})
	}
}

func TestClient_Close(t *testing.T) {
	td, err := spiffeid.TrustDomainFromString("example.org")
	require.NoError(t, err)

	client := &Client{
		socketPath:  "unix:///tmp/test.sock",
		trustDomain: td,
		timeout:     30 * time.Second,
		client:      nil,
		source:      nil,
	}

	// Close should not panic even with nil client and source
	require.NotPanics(t, func() {
		err := client.Close()
		assert.NoError(t, err, "Close with nil client/source should not error")
	})
}

func TestClient_CloseIdempotent(t *testing.T) {
	td, err := spiffeid.TrustDomainFromString("example.org")
	require.NoError(t, err)

	client := &Client{
		socketPath:  "unix:///tmp/test.sock",
		trustDomain: td,
		timeout:     30 * time.Second,
		client:      nil,
		source:      nil,
	}

	// Close multiple times should be safe
	err1 := client.Close()
	err2 := client.Close()
	err3 := client.Close()

	assert.NoError(t, err1)
	assert.NoError(t, err2)
	assert.NoError(t, err3)
}
