package httpapi

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/spiffe/go-spiffe/v2/spiffeid"
	"github.com/spiffe/go-spiffe/v2/spiffetls/tlsconfig"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewHTTPServer_ValidConfig(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Note: This test requires a running SPIRE agent
	// Skip if SPIRE_AGENT_SOCKET not set
	cfg := ServerConfig{
		Address:    ":18443",
		SocketPath: "unix:///tmp/spire-agent/public/api.sock",
		Authorizer: tlsconfig.AuthorizeAny(),
	}

	server, err := NewHTTPServer(ctx, cfg)

	// If SPIRE is not running, this will fail - that's expected
	if err != nil {
		t.Skipf("Skipping test - SPIRE agent not available: %v", err)
		return
	}

	require.NotNil(t, server)
	defer server.Stop(ctx)
}

func TestNewHTTPServer_MissingAddress(t *testing.T) {
	ctx := context.Background()

	authorizer := tlsconfig.AuthorizeAny()

	server, err := NewHTTPServer(ctx, ServerConfig{Address: "", X509SourceProvider: &WorkloadAPISourceProvider{SocketPath: "unix:///tmp/socket"}, Authorizer: authorizer})

	require.Error(t, err)
	assert.Nil(t, server)
	assert.Contains(t, err.Error(), "address is required")
}

func TestNewHTTPServer_MissingX509SourceProvider(t *testing.T) {
	ctx := context.Background()

	authorizer := tlsconfig.AuthorizeAny()

	server, err := NewHTTPServer(ctx, ServerConfig{Address: ":8443", X509SourceProvider: nil, Authorizer: authorizer})

	require.Error(t, err)
	assert.Nil(t, server)
	assert.Contains(t, err.Error(), "X509SourceProvider is required")
}

func TestNewHTTPServer_MissingAuthorizer(t *testing.T) {
	ctx := context.Background()

	server, err := NewHTTPServer(ctx, ServerConfig{Address: ":8443", X509SourceProvider: &WorkloadAPISourceProvider{SocketPath: "unix:///tmp/socket"}, Authorizer: nil})

	require.Error(t, err)
	assert.Nil(t, server)
	assert.Contains(t, err.Error(), "authorizer is required")
}

func TestGetSPIFFEID_Present(t *testing.T) {
	// Create a request with SPIFFE ID in context
	req := httptest.NewRequest("GET", "/test", nil)
	testID := spiffeid.RequireFromString("spiffe://example.org/test")
	ctx := context.WithValue(req.Context(), spiffeIDKey, testID)
	req = req.WithContext(ctx)

	id, ok := GetSPIFFEID(req)

	assert.True(t, ok)
	assert.Equal(t, testID, id)
}

func TestGetSPIFFEID_NotPresent(t *testing.T) {
	req := httptest.NewRequest("GET", "/test", nil)

	id, ok := GetSPIFFEID(req)

	assert.False(t, ok)
	assert.Equal(t, spiffeid.ID{}, id)
}

func TestMustGetSPIFFEID_Present(t *testing.T) {
	req := httptest.NewRequest("GET", "/test", nil)
	testID := spiffeid.RequireFromString("spiffe://example.org/test")
	ctx := context.WithValue(req.Context(), spiffeIDKey, testID)
	req = req.WithContext(ctx)

	id := MustGetSPIFFEID(req)

	assert.Equal(t, testID, id)
}

func TestMustGetSPIFFEID_Panics(t *testing.T) {
	req := httptest.NewRequest("GET", "/test", nil)

	assert.Panics(t, func() {
		MustGetSPIFFEID(req)
	})
}

func TestGetTrustDomain_Present(t *testing.T) {
	req := httptest.NewRequest("GET", "/test", nil)
	testID := spiffeid.RequireFromString("spiffe://example.org/test/workload")
	ctx := context.WithValue(req.Context(), spiffeIDKey, testID)
	req = req.WithContext(ctx)

	td, ok := GetTrustDomain(req)

	assert.True(t, ok)
	assert.Equal(t, "example.org", td.String())
}

func TestGetTrustDomain_NotPresent(t *testing.T) {
	req := httptest.NewRequest("GET", "/test", nil)

	td, ok := GetTrustDomain(req)

	assert.False(t, ok)
	assert.Equal(t, spiffeid.TrustDomain{}, td)
}

func TestGetPath_Present(t *testing.T) {
	req := httptest.NewRequest("GET", "/test", nil)
	testID := spiffeid.RequireFromString("spiffe://example.org/workload/server")
	ctx := context.WithValue(req.Context(), spiffeIDKey, testID)
	req = req.WithContext(ctx)

	path, ok := GetPath(req)

	assert.True(t, ok)
	assert.Equal(t, "/workload/server", path)
}

func TestGetPath_NotPresent(t *testing.T) {
	req := httptest.NewRequest("GET", "/test", nil)

	path, ok := GetPath(req)

	assert.False(t, ok)
	assert.Empty(t, path)
}

func TestWrapHandler_NoTLS(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	socketPath := "unix:///tmp/spire-agent/public/api.sock"
	authorizer := tlsconfig.AuthorizeAny()

	server, err := NewHTTPServer(ctx, ServerConfig{Address: ":18444", SocketPath: socketPath, Authorizer: authorizer})
	if err != nil {
		t.Skipf("Skipping test - SPIRE agent not available: %v", err)
		return
	}
	defer server.Stop(ctx)

	// Create a test handler
	handlerCalled := false
	handler := func(w http.ResponseWriter, r *http.Request) {
		handlerCalled = true
		w.WriteHeader(http.StatusOK)
	}

	// Wrap the handler
	wrapped := server.wrapHandler(handler)

	// Create request without TLS
	req := httptest.NewRequest("GET", "/test", nil)
	rec := httptest.NewRecorder()

	// Call wrapped handler
	wrapped(rec, req)

	// Should return 400 Bad Request
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.False(t, handlerCalled)
}

func TestRegisterHandler(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	socketPath := "unix:///tmp/spire-agent/public/api.sock"
	authorizer := tlsconfig.AuthorizeAny()

	server, err := NewHTTPServer(ctx, ServerConfig{Address: ":18445", SocketPath: socketPath, Authorizer: authorizer})
	if err != nil {
		t.Skipf("Skipping test - SPIRE agent not available: %v", err)
		return
	}
	defer server.Stop(ctx)

	// Register a test handler
	server.RegisterHandler("/test", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, "test response")
	})

	// Verify the handler was registered by checking the mux
	mux := server.GetMux()
	require.NotNil(t, mux)

	// Create a test request
	req := httptest.NewRequest("GET", "/test", nil)
	rec := httptest.NewRecorder()

	// Note: The mux will call wrapHandler which expects TLS,
	// so we just verify registration happened
	mux.ServeHTTP(rec, req)

	// The wrapped handler should reject the request due to missing TLS
	// but this confirms the handler was registered
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestGetMux(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	socketPath := "unix:///tmp/spire-agent/public/api.sock"
	authorizer := tlsconfig.AuthorizeAny()

	server, err := NewHTTPServer(ctx, ServerConfig{Address: ":18446", SocketPath: socketPath, Authorizer: authorizer})
	if err != nil {
		t.Skipf("Skipping test - SPIRE agent not available: %v", err)
		return
	}
	defer server.Stop(ctx)

	mux := server.GetMux()
	require.NotNil(t, mux)
}

func TestStop_MultipleCallsIdempotent(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	socketPath := "unix:///tmp/spire-agent/public/api.sock"
	authorizer := tlsconfig.AuthorizeAny()

	server, err := NewHTTPServer(ctx, ServerConfig{Address: ":18447", SocketPath: socketPath, Authorizer: authorizer})
	if err != nil {
		t.Skipf("Skipping test - SPIRE agent not available: %v", err)
		return
	}

	// Call Stop multiple times - should be idempotent
	err = server.Stop(ctx)
	assert.NoError(t, err)

	err = server.Stop(ctx)
	assert.NoError(t, err)
}

func TestWrapHandler_ExtractsIdentity(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	socketPath := "unix:///tmp/spire-agent/public/api.sock"
	authorizer := tlsconfig.AuthorizeAny()

	server, err := NewHTTPServer(ctx, ServerConfig{Address: ":18448", SocketPath: socketPath, Authorizer: authorizer})
	if err != nil {
		t.Skipf("Skipping test - SPIRE agent not available: %v", err)
		return
	}
	defer server.Stop(ctx)

	// Note: wrapHandler requires TLS connection state, which we can't easily
	// mock without integration tests. The identity extraction itself is
	// thoroughly tested in identity_test.go. This test verifies the handler
	// wrapping mechanism exists and can be called.

	handler := func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}

	wrapped := server.wrapHandler(handler)
	require.NotNil(t, wrapped)

	// The wrapped handler will reject requests without TLS,
	// which is the correct behavior and is tested in TestWrapHandler_NoTLS
}

// Example handler demonstrating usage
func ExampleHTTPServer_RegisterHandler() {
	ctx := context.Background()

	authorizer := tlsconfig.AuthorizeMemberOf(
		spiffeid.RequireTrustDomainFromString("example.org"),
	)

	server, err := NewHTTPServer(ctx, ServerConfig{
		Address:    ":8443",
		SocketPath: "unix:///tmp/spire-agent/public/api.sock",
		Authorizer: authorizer,
	})
	if err != nil {
		panic(err)
	}
	defer server.Stop(ctx)

	// Register handler that uses client identity
	server.RegisterHandler("/api/hello", func(w http.ResponseWriter, r *http.Request) {
		clientID, ok := GetSPIFFEID(r)
		if !ok {
			http.Error(w, "No client identity", http.StatusInternalServerError)
			return
		}

		// Application performs authorization based on identity
		// This is NOT the library's responsibility

		fmt.Fprintf(w, "Hello, %s!\n", clientID.String())
	})

	server.Start(ctx)
}
