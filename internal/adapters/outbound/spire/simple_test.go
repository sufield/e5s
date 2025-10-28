//go:build integration

package spire_test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/sufield/e5s/internal/adapters/outbound/spire"
)

// TestSimpleConnection is a minimal test to verify SPIRE connectivity
func TestSimpleConnection(t *testing.T) {
	ctx := context.Background()

	socketPath := os.Getenv("SPIRE_AGENT_SOCKET")
	if socketPath == "" {
		socketPath = "unix:///tmp/spire-agent/public/api.sock"
	}

	trustDomain := os.Getenv("SPIRE_TRUST_DOMAIN")
	if trustDomain == "" {
		trustDomain = "example.org"
	}

	config := spire.Config{
		SocketPath:  socketPath,
		TrustDomain: trustDomain,
		Timeout:     30 * time.Second,
	}

	client, err := spire.NewClient(ctx, config)
	if err != nil {
		t.Fatalf("Failed to create SPIRE client: %v", err)
	}
	defer client.Close()

	t.Logf("✅ Successfully connected to SPIRE Agent")
	t.Logf("Socket: %s", client.GetSocketPath())
	t.Logf("Trust Domain: %s", client.GetTrustDomain())

	// Fetch X.509 SVID
	doc, err := client.FetchX509SVID(ctx)
	if err != nil {
		t.Fatalf("Failed to fetch X.509 SVID: %v", err)
	}

	t.Logf("✅ Successfully fetched X.509 SVID")
	t.Logf("Identity: %s", doc.IdentityCredential().String())
	t.Logf("Expires: %s", doc.ExpiresAt().Format(time.RFC3339))
	t.Logf("Valid: %v", doc.IsValid())
}
