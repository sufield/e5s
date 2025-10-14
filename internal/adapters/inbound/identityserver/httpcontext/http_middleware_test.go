package httpcontext

// Identity Middleware Tests
//
// These tests verify HTTP middleware for SPIFFE identity authentication and authorization.
// Tests cover authentication requirements, trust domain requirements, path prefix requirements,
// and identity logging.
//
// Run these tests with:
//
//	go test ./internal/adapters/inbound/httpapi/... -v -run TestRequire
//	go test ./internal/adapters/inbound/httpapi/... -v -run TestLog
//	go test ./internal/adapters/inbound/httpapi/... -cover

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/spiffe/go-spiffe/v2/spiffeid"
	"github.com/stretchr/testify/assert"
)

func TestAuthenticated(t *testing.T) {
	handlerCalled := false
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handlerCalled = true
		w.WriteHeader(http.StatusOK)
	})

	middleware := Authenticated(handler)

	t.Run("authenticated request", func(t *testing.T) {
		handlerCalled = false
		req := httptest.NewRequest("GET", "/test", nil)
		testID := spiffeid.RequireFromString("spiffe://example.org/test")
		req = WithSPIFFEID(req, testID)
		rec := httptest.NewRecorder()

		middleware.ServeHTTP(rec, req)

		assert.True(t, handlerCalled)
		assert.Equal(t, http.StatusOK, rec.Code)
	})

	t.Run("unauthenticated request", func(t *testing.T) {
		handlerCalled = false
		req := httptest.NewRequest("GET", "/test", nil)
		rec := httptest.NewRecorder()

		middleware.ServeHTTP(rec, req)

		assert.False(t, handlerCalled)
		assert.Equal(t, http.StatusUnauthorized, rec.Code)
	})
}

func TestRequireTrustDomainID(t *testing.T) {
	exampleTD := spiffeid.RequireTrustDomainFromString("example.org")

	handlerCalled := false
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handlerCalled = true
		w.WriteHeader(http.StatusOK)
	})

	middleware := RequireTrustDomainID(exampleTD, handler)

	t.Run("matching trust domain", func(t *testing.T) {
		handlerCalled = false
		req := httptest.NewRequest("GET", "/test", nil)
		testID := spiffeid.RequireFromString("spiffe://example.org/service")
		req = WithSPIFFEID(req, testID)
		rec := httptest.NewRecorder()

		middleware.ServeHTTP(rec, req)

		assert.True(t, handlerCalled)
		assert.Equal(t, http.StatusOK, rec.Code)
	})

	t.Run("non-matching trust domain", func(t *testing.T) {
		handlerCalled = false
		req := httptest.NewRequest("GET", "/test", nil)
		testID := spiffeid.RequireFromString("spiffe://other.org/service")
		req = WithSPIFFEID(req, testID)
		rec := httptest.NewRecorder()

		middleware.ServeHTTP(rec, req)

		assert.False(t, handlerCalled)
		assert.Equal(t, http.StatusForbidden, rec.Code)
	})
}

func TestRequirePathPrefix(t *testing.T) {
	handlerCalled := false
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handlerCalled = true
		w.WriteHeader(http.StatusOK)
	})

	middleware := RequirePathPrefix("/service/", handler)

	t.Run("matching path prefix", func(t *testing.T) {
		handlerCalled = false
		req := httptest.NewRequest("GET", "/test", nil)
		testID := spiffeid.RequireFromString("spiffe://example.org/service/frontend")
		req = WithSPIFFEID(req, testID)
		rec := httptest.NewRecorder()

		middleware.ServeHTTP(rec, req)

		assert.True(t, handlerCalled)
		assert.Equal(t, http.StatusOK, rec.Code)
	})

	t.Run("non-matching path prefix", func(t *testing.T) {
		handlerCalled = false
		req := httptest.NewRequest("GET", "/test", nil)
		testID := spiffeid.RequireFromString("spiffe://example.org/workload/backend")
		req = WithSPIFFEID(req, testID)
		rec := httptest.NewRecorder()

		middleware.ServeHTTP(rec, req)

		assert.False(t, handlerCalled)
		assert.Equal(t, http.StatusForbidden, rec.Code)
	})
}

func TestLogIdentity(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	middleware := LogIdentity(handler)

	t.Run("logs authenticated request", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/test", nil)
		testID := spiffeid.RequireFromString("spiffe://example.org/service")
		req = WithSPIFFEID(req, testID)
		rec := httptest.NewRecorder()

		// Just verify it doesn't panic
		middleware.ServeHTTP(rec, req)
		assert.Equal(t, http.StatusOK, rec.Code)
	})

	t.Run("logs unauthenticated request", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/test", nil)
		rec := httptest.NewRecorder()

		// Just verify it doesn't panic
		middleware.ServeHTTP(rec, req)
		assert.Equal(t, http.StatusOK, rec.Code)
	})
}
