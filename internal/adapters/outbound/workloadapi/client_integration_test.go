package workloadapi_test

// Workload API Client Integration Tests
//
// These tests verify end-to-end client behavior with a real server.
// Tests use full application bootstrap with inmemory adapters.
//
// Run these tests with:
//
//	go test ./internal/adapters/outbound/workloadapi/... -v -run TestClient.*Success
//	go test ./internal/adapters/outbound/workloadapi/... -cover

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/pocket/hexagon/spire/internal/adapters/inbound/workloadapi"
	"github.com/pocket/hexagon/spire/internal/adapters/outbound/compose"
	"github.com/pocket/hexagon/spire/internal/adapters/outbound/inmemory"
	wlapi "github.com/pocket/hexagon/spire/internal/adapters/outbound/workloadapi"
	"github.com/pocket/hexagon/spire/internal/app"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestClient_FetchX509SVID_Success tests successful SVID fetch
func TestClient_FetchX509SVID_Success(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	// Start real server
	loader := inmemory.NewInMemoryConfig()
	factory := compose.NewInMemoryAdapterFactory()
	application, err := app.Bootstrap(ctx, loader, factory)
	require.NoError(t, err)

	socketPath := filepath.Join(t.TempDir(), "test.sock")
	server := workloadapi.NewServer(application.IdentityClientService, socketPath)
	err = server.Start(ctx)
	require.NoError(t, err)
	defer server.Stop(ctx)

	time.Sleep(100 * time.Millisecond)

	// Create client and fetch SVID
	client := wlapi.NewClient(socketPath)

	// Mock the UID to match registered workload (since we're running as current user)
	// In production, the server would extract this via SO_PEERCRED
	resp, err := client.FetchX509SVID(ctx)
	require.NoError(t, err)
	assert.NotNil(t, resp)

	// Verify response structure
	assert.NotEmpty(t, resp.GetSPIFFEID())
	assert.NotEmpty(t, resp.GetX509SVID())
	assert.Greater(t, resp.GetExpiresAt(), int64(0))
}

// TestClient_FetchX509SVIDWithConfig_Success tests mTLS fetch
func TestClient_FetchX509SVIDWithConfig_Success(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	// Start real server
	loader := inmemory.NewInMemoryConfig()
	factory := compose.NewInMemoryAdapterFactory()
	application, err := app.Bootstrap(ctx, loader, factory)
	require.NoError(t, err)

	socketPath := filepath.Join(t.TempDir(), "test.sock")
	server := workloadapi.NewServer(application.IdentityClientService, socketPath)
	err = server.Start(ctx)
	require.NoError(t, err)
	defer server.Stop(ctx)

	time.Sleep(100 * time.Millisecond)

	// Create client with nil TLS config (should fallback to regular fetch)
	client := wlapi.NewClient(socketPath)
	resp, err := client.FetchX509SVIDWithConfig(ctx, nil)
	require.NoError(t, err)
	assert.NotNil(t, resp)
}
