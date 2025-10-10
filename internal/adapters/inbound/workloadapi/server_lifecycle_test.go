package workloadapi_test

// Workload API Server Lifecycle Tests
//
// These tests verify server initialization, startup, shutdown, and constructor behavior.
// Tests cover socket creation, cleanup, and basic server lifecycle operations.
//
// Run these tests with:
//
//	go test ./internal/adapters/inbound/workloadapi/... -v -run TestServer_Start
//	go test ./internal/adapters/inbound/workloadapi/... -v -run TestServer_Stop
//	go test ./internal/adapters/inbound/workloadapi/... -cover

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/pocket/hexagon/spire/internal/adapters/inbound/workloadapi"
	"github.com/pocket/hexagon/spire/internal/adapters/outbound/compose"
	"github.com/pocket/hexagon/spire/internal/adapters/outbound/inmemory"
	"github.com/pocket/hexagon/spire/internal/app"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestServer_Start tests server initialization
func TestServer_Start(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	// Bootstrap application
	loader := inmemory.NewInMemoryConfig()
	factory := compose.NewInMemoryAdapterFactory()
	application, err := app.Bootstrap(ctx, loader, factory)
	require.NoError(t, err)

	// Create temporary socket path
	socketPath := filepath.Join(t.TempDir(), "test.sock")

	// Create and start server
	server := workloadapi.NewServer(application.IdentityClientService, socketPath)
	err = server.Start(ctx)
	require.NoError(t, err)

	// Verify socket exists
	_, err = os.Stat(socketPath)
	assert.NoError(t, err, "Socket file should exist")

	// Cleanup
	server.Stop(ctx)
}

// TestServer_Stop tests server shutdown
func TestServer_Stop(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	loader := inmemory.NewInMemoryConfig()
	factory := compose.NewInMemoryAdapterFactory()
	application, err := app.Bootstrap(ctx, loader, factory)
	require.NoError(t, err)

	socketPath := filepath.Join(t.TempDir(), "test.sock")

	server := workloadapi.NewServer(application.IdentityClientService, socketPath)
	err = server.Start(ctx)
	require.NoError(t, err)

	// Stop server
	err = server.Stop(ctx)
	assert.NoError(t, err)

	// Verify socket removed
	_, err = os.Stat(socketPath)
	assert.True(t, os.IsNotExist(err), "Socket file should be removed")
}

// TestServer_NewServer tests constructor
func TestServer_NewServer(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	loader := inmemory.NewInMemoryConfig()
	factory := compose.NewInMemoryAdapterFactory()
	application, err := app.Bootstrap(ctx, loader, factory)
	require.NoError(t, err)

	socketPath := "/tmp/test.sock"
	server := workloadapi.NewServer(application.IdentityClientService, socketPath)

	assert.NotNil(t, server)
}
