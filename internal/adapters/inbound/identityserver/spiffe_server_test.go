package identityserver

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/pocket/hexagon/spire/internal/ports"
	"github.com/spiffe/go-spiffe/v2/spiffeid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func contains(s, substr string) bool {
	return strings.Contains(s, substr)
}

func TestNew_MissingSocketPath(t *testing.T) {
	ctx := context.Background()
	cfg := ports.MTLSConfig{
		SPIFFE: ports.SPIFFEConfig{
			AllowedPeerID: "spiffe://example.org/client",
		},
	}

	server, err := New(ctx, cfg)
	require.Error(t, err)
	assert.Nil(t, server)
	assert.Contains(t, err.Error(), "socket path is required")
}

func TestNew_MissingAllowedClientID(t *testing.T) {
	ctx := context.Background()
	cfg := ports.MTLSConfig{
		WorkloadAPI: ports.WorkloadAPIConfig{
			SocketPath: "unix:///tmp/socket",
		},
	}

	server, err := New(ctx, cfg)
	require.Error(t, err)
	assert.Nil(t, server)
	assert.Contains(t, err.Error(), "authorization policy required")
}

func TestNew_InvalidClientID(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cfg := ports.MTLSConfig{
		WorkloadAPI: ports.WorkloadAPIConfig{
			SocketPath: "unix:///tmp/spire-agent/public/api.sock",
		},
		SPIFFE: ports.SPIFFEConfig{
			AllowedPeerID: "not-a-valid-spiffe-id",
		},
	}

	server, err := New(ctx, cfg)
	if server != nil {
		defer server.Close()
	}

	// Should either fail to create X509Source (SPIRE not running)
	// or fail to parse invalid SPIFFE ID
	require.Error(t, err)
	assert.True(t,
		contains(err.Error(), "parse allowed peer ID") ||
			contains(err.Error(), "create X509Source"),
		"Expected error about parsing peer ID or creating X509Source, got: %v", err,
	)
}

func TestNew_ValidConfig(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cfg := ports.MTLSConfig{
		WorkloadAPI: ports.WorkloadAPIConfig{
			SocketPath: "unix:///tmp/spire-agent/public/api.sock",
		},
		SPIFFE: ports.SPIFFEConfig{
			AllowedPeerID: "spiffe://example.org/client",
		},
		HTTP: ports.HTTPConfig{
			Address: ":18443",
		},
	}

	server, err := New(ctx, cfg)

	// If SPIRE is not running, this will fail - that's expected
	if err != nil {
		t.Skipf("Skipping test - SPIRE agent not available: %v", err)
		return
	}

	require.NotNil(t, server)
	defer server.Close()
}

func TestNew_AppliesDefaults(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cfg := ports.MTLSConfig{
		WorkloadAPI: ports.WorkloadAPIConfig{
			SocketPath: "unix:///tmp/spire-agent/public/api.sock",
		},
		SPIFFE: ports.SPIFFEConfig{
			AllowedPeerID: "spiffe://example.org/client",
		},
		// No HTTP config - should apply defaults
	}

	server, err := New(ctx, cfg)

	if err != nil {
		t.Skipf("Skipping test - SPIRE agent not available: %v", err)
		return
	}

	require.NotNil(t, server)
	defer server.Close()

	// Verify defaults were applied (through implementation)
	spiffeServer, ok := server.(*spiffeServer)
	require.True(t, ok)
	assert.Equal(t, ":8443", spiffeServer.cfg.HTTP.Address)
	assert.Equal(t, 10*time.Second, spiffeServer.cfg.HTTP.ReadHeaderTimeout)
	assert.Equal(t, 30*time.Second, spiffeServer.cfg.HTTP.WriteTimeout)
	assert.Equal(t, 120*time.Second, spiffeServer.cfg.HTTP.IdleTimeout)
}

func TestSpiffeServer_Handle(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cfg := ports.MTLSConfig{
		WorkloadAPI: ports.WorkloadAPIConfig{
			SocketPath: "unix:///tmp/spire-agent/public/api.sock",
		},
		SPIFFE: ports.SPIFFEConfig{
			AllowedPeerID: "spiffe://example.org/client",
		},
		HTTP: ports.HTTPConfig{
			Address: ":18444",
		},
	}

	server, err := New(ctx, cfg)
	if err != nil {
		t.Skipf("Skipping test - SPIRE agent not available: %v", err)
		return
	}
	defer server.Close()

	// Register a test handler
	handlerCalled := false
	server.Handle("/test", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handlerCalled = true
		w.WriteHeader(http.StatusOK)
	}))

	// Verify handler was registered
	// (can't easily test without TLS, but we can verify no panic)
	assert.False(t, handlerCalled) // Not called yet
}

func TestSpiffeServer_Close_Idempotent(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cfg := ports.MTLSConfig{
		WorkloadAPI: ports.WorkloadAPIConfig{
			SocketPath: "unix:///tmp/spire-agent/public/api.sock",
		},
		SPIFFE: ports.SPIFFEConfig{
			AllowedPeerID: "spiffe://example.org/client",
		},
		HTTP: ports.HTTPConfig{
			Address: ":18445",
		},
	}

	server, err := New(ctx, cfg)
	if err != nil {
		t.Skipf("Skipping test - SPIRE agent not available: %v", err)
		return
	}

	// Close multiple times - should be idempotent
	err1 := server.Close()
	assert.NoError(t, err1)

	err2 := server.Close()
	assert.NoError(t, err2)
}

func TestGetIdentity_Present(t *testing.T) {
	req := httptest.NewRequest("GET", "/test", nil)
	testID := spiffeid.RequireFromString("spiffe://example.org/test")
	ctx := context.WithValue(req.Context(), spiffeIDKey, testID)
	req = req.WithContext(ctx)

	id, ok := GetIdentity(req)

	assert.True(t, ok)
	assert.Equal(t, testID, id)
}

func TestGetIdentity_NotPresent(t *testing.T) {
	req := httptest.NewRequest("GET", "/test", nil)

	id, ok := GetIdentity(req)

	assert.False(t, ok)
	assert.Equal(t, spiffeid.ID{}, id)
}

func TestMustGetIdentity_Present(t *testing.T) {
	req := httptest.NewRequest("GET", "/test", nil)
	testID := spiffeid.RequireFromString("spiffe://example.org/test")
	ctx := context.WithValue(req.Context(), spiffeIDKey, testID)
	req = req.WithContext(ctx)

	id := MustGetIdentity(req)

	assert.Equal(t, testID, id)
}

func TestMustGetIdentity_Panics(t *testing.T) {
	req := httptest.NewRequest("GET", "/test", nil)

	assert.Panics(t, func() {
		MustGetIdentity(req)
	})
}

func TestWrapHandler_NoTLS(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cfg := ports.MTLSConfig{
		WorkloadAPI: ports.WorkloadAPIConfig{
			SocketPath: "unix:///tmp/spire-agent/public/api.sock",
		},
		SPIFFE: ports.SPIFFEConfig{
			AllowedPeerID: "spiffe://example.org/client",
		},
		HTTP: ports.HTTPConfig{
			Address: ":18446",
		},
	}

	server, err := New(ctx, cfg)
	if err != nil {
		t.Skipf("Skipping test - SPIRE agent not available: %v", err)
		return
	}
	defer server.Close()

	spiffeServer := server.(*spiffeServer)

	// Create test handler
	handlerCalled := false
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handlerCalled = true
	})

	wrapped := spiffeServer.wrapHandler(handler)

	// Request without TLS
	req := httptest.NewRequest("GET", "/test", nil)
	rec := httptest.NewRecorder()

	wrapped.ServeHTTP(rec, req)

	// Should return 400 Bad Request
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.False(t, handlerCalled)
	assert.Contains(t, rec.Body.String(), "TLS connection required")
}
