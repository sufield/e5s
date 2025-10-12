//go:build integration

package spire_test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/pocket/hexagon/spire/internal/adapters/outbound/spire"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// getTestConfig returns test configuration from environment or defaults
func getTestConfig() *spire.Config {
	socketPath := os.Getenv("SPIRE_AGENT_SOCKET")
	if socketPath == "" {
		socketPath = "unix:///tmp/spire-agent/public/api.sock"
	}

	trustDomain := os.Getenv("SPIRE_TRUST_DOMAIN")
	if trustDomain == "" {
		trustDomain = "example.org"
	}

	return &spire.Config{
		SocketPath:  socketPath,
		TrustDomain: trustDomain,
		Timeout:     30 * time.Second,
	}
}

// TestSPIREClientConnection tests that we can connect to SPIRE Agent
func TestSPIREClientConnection(t *testing.T) {
	ctx := context.Background()
	config := getTestConfig()

	client, err := spire.NewSPIREClient(ctx, *config)
	require.NoError(t, err, "Failed to create SPIRE client")
	defer client.Close()

	assert.Equal(t, config.TrustDomain, client.GetTrustDomain())
	assert.Equal(t, config.SocketPath, client.GetSocketPath())
}

// TestFetchX509SVID tests fetching X.509 SVID from SPIRE
func TestFetchX509SVID(t *testing.T) {
	ctx := context.Background()
	config := getTestConfig()

	client, err := spire.NewSPIREClient(ctx, *config)
	require.NoError(t, err, "Failed to create SPIRE client")
	defer client.Close()

	// Fetch X.509 SVID
	doc, err := client.FetchX509SVID(ctx)
	require.NoError(t, err, "Failed to fetch X.509 SVID")
	require.NotNil(t, doc, "Identity document should not be nil")

	// Verify document properties
	assert.NotNil(t, doc.IdentityCredential(), "Identity credential should not be nil")
	assert.NotNil(t, doc.Certificate(), "Certificate should not be nil")
	assert.NotNil(t, doc.PrivateKey(), "Private key should not be nil")
	assert.True(t, doc.IsValid(), "Document should be valid")
	assert.False(t, doc.IsExpired(), "Document should not be expired")

	t.Logf("Fetched SVID for identity: %s", doc.IdentityCredential().String())
	t.Logf("Certificate expires: %s", doc.ExpiresAt().Format(time.RFC3339))
}

// TestFetchX509Bundle tests fetching trust bundle from SPIRE
func TestFetchX509Bundle(t *testing.T) {
	ctx := context.Background()
	config := getTestConfig()

	client, err := spire.NewSPIREClient(ctx, *config)
	require.NoError(t, err, "Failed to create SPIRE client")
	defer client.Close()

	// Fetch trust bundle
	certs, err := client.FetchX509Bundle(ctx)
	require.NoError(t, err, "Failed to fetch X.509 bundle")
	require.NotEmpty(t, certs, "Bundle should contain at least one CA certificate")

	// Verify CA certificates
	for i, cert := range certs {
		assert.True(t, cert.IsCA, "Certificate should be a CA")
		t.Logf("CA %d: %s (expires: %s)", i+1, cert.Subject.CommonName, cert.NotAfter.Format(time.RFC3339))
	}
}

// TestFetchJWTSVID tests fetching JWT SVID from SPIRE
func TestFetchJWTSVID(t *testing.T) {
	ctx := context.Background()
	config := getTestConfig()

	client, err := spire.NewSPIREClient(ctx, *config)
	require.NoError(t, err, "Failed to create SPIRE client")
	defer client.Close()

	// Fetch JWT SVID
	audiences := []string{"test-audience", "example.com"}
	token, err := client.FetchJWTSVID(ctx, audiences)
	require.NoError(t, err, "Failed to fetch JWT SVID")
	assert.NotEmpty(t, token, "JWT token should not be empty")

	t.Logf("Fetched JWT SVID (length: %d bytes)", len(token))
}

// TestValidateJWTSVID tests JWT token validation
func TestValidateJWTSVID(t *testing.T) {
	ctx := context.Background()
	config := getTestConfig()

	client, err := spire.NewSPIREClient(ctx, *config)
	require.NoError(t, err, "Failed to create SPIRE client")
	defer client.Close()

	// First, fetch a JWT SVID
	audiences := []string{"test-validation"}
	token, err := client.FetchJWTSVID(ctx, audiences)
	require.NoError(t, err, "Failed to fetch JWT SVID")

	// Validate the token
	err = client.ValidateJWTSVID(ctx, token, "test-validation")
	assert.NoError(t, err, "JWT validation should succeed")

	// Test with wrong audience (should fail)
	err = client.ValidateJWTSVID(ctx, token, "wrong-audience")
	assert.Error(t, err, "JWT validation should fail with wrong audience")
}

// TestSPIREClientReconnect tests client can handle reconnection
func TestSPIREClientReconnect(t *testing.T) {
	ctx := context.Background()
	config := getTestConfig()

	// Create first client
	client1, err := spire.NewSPIREClient(ctx, *config)
	require.NoError(t, err, "Failed to create first SPIRE client")

	// Fetch SVID with first client
	doc1, err := client1.FetchX509SVID(ctx)
	require.NoError(t, err, "Failed to fetch SVID with first client")

	// Close first client
	client1.Close()

	// Create second client (reconnect)
	client2, err := spire.NewSPIREClient(ctx, *config)
	require.NoError(t, err, "Failed to create second SPIRE client")
	defer client2.Close()

	// Fetch SVID with second client
	doc2, err := client2.FetchX509SVID(ctx)
	require.NoError(t, err, "Failed to fetch SVID with second client")

	// Both should work and have same trust domain
	assert.Equal(t, doc1.IdentityCredential().TrustDomain().String(),
		doc2.IdentityCredential().TrustDomain().String(),
		"Trust domains should match across reconnects")
}

// TestSPIREClientReconnectFailure tests that using a closed client fails appropriately
func TestSPIREClientReconnectFailure(t *testing.T) {
	ctx := context.Background()
	config := getTestConfig()

	// Create client
	client, err := spire.NewSPIREClient(ctx, *config)
	require.NoError(t, err, "Failed to create SPIRE client")

	// Fetch SVID successfully
	_, err = client.FetchX509SVID(ctx)
	require.NoError(t, err, "First fetch should succeed")

	// Close the client
	client.Close()

	// Try to fetch SVID with closed client - should fail
	_, err = client.FetchX509SVID(ctx)
	assert.Error(t, err, "Fetch with closed client should fail")
	assert.Contains(t, err.Error(), "failed to fetch X.509 context",
		"Error should indicate connection issue")
}

// TestSPIREClientTimeout tests client handles timeouts gracefully
func TestSPIREClientTimeout(t *testing.T) {
	ctx := context.Background()

	// Create config with very short timeout
	config := getTestConfig()
	config.Timeout = 1 * time.Nanosecond // Extremely short timeout

	client, err := spire.NewSPIREClient(ctx, *config)
	if err != nil {
		// Connection may fail with very short timeout - this is expected
		t.Logf("Client creation failed with short timeout (expected): %v", err)
		return
	}
	defer client.Close()

	// Try to fetch SVID with very short timeout - should likely fail
	_, err = client.FetchX509SVID(ctx)
	if err != nil {
		t.Logf("SVID fetch failed with short timeout (expected): %v", err)
	}
}
