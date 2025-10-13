package httpcontext

// Identity Context Tests
//
// These tests verify SPIFFE ID extraction and context management for HTTP requests.
// Tests cover getting SPIFFE IDs, trust domains, paths, and path segments from request context.
//
// Run these tests with:
//
//	go test ./internal/adapters/inbound/httpapi/... -v -run TestGet
//	go test ./internal/adapters/inbound/httpapi/... -v -run TestWith
//	go test ./internal/adapters/inbound/httpapi/... -cover

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/spiffe/go-spiffe/v2/spiffeid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetSPIFFEID(t *testing.T) {
	tests := []struct {
		name     string
		setupReq func() *http.Request
		wantOK   bool
		wantID   string
	}{
		{
			name: "ID present in context",
			setupReq: func() *http.Request {
				req := httptest.NewRequest("GET", "/test", nil)
				testID := spiffeid.RequireFromString("spiffe://example.org/test")
				return WithSPIFFEID(req, testID)
			},
			wantOK: true,
			wantID: "spiffe://example.org/test",
		},
		{
			name: "ID not present",
			setupReq: func() *http.Request {
				return httptest.NewRequest("GET", "/test", nil)
			},
			wantOK: false,
			wantID: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := tt.setupReq()
			id, ok := GetSPIFFEID(req)

			assert.Equal(t, tt.wantOK, ok)
			if tt.wantOK {
				assert.Equal(t, tt.wantID, id.String())
			}
		})
	}
}

func TestMustGetSPIFFEID(t *testing.T) {
	t.Run("ID present", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/test", nil)
		testID := spiffeid.RequireFromString("spiffe://example.org/test")
		req = WithSPIFFEID(req, testID)

		id := MustGetSPIFFEID(req)
		assert.Equal(t, testID, id)
	})

	t.Run("ID not present - panics", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/test", nil)

		assert.Panics(t, func() {
			MustGetSPIFFEID(req)
		})
	})
}

func TestGetTrustDomain(t *testing.T) {
	tests := []struct {
		name   string
		id     string
		wantTD string
		wantOK bool
	}{
		{
			name:   "valid ID",
			id:     "spiffe://example.org/service/frontend",
			wantTD: "example.org",
			wantOK: true,
		},
		{
			name:   "different trust domain",
			id:     "spiffe://test.com/service",
			wantTD: "test.com",
			wantOK: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/test", nil)
			testID := spiffeid.RequireFromString(tt.id)
			req = WithSPIFFEID(req, testID)

			td, ok := GetTrustDomain(req)
			assert.Equal(t, tt.wantOK, ok)
			if tt.wantOK {
				assert.Equal(t, tt.wantTD, td.String())
			}
		})
	}

	t.Run("no ID in context", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/test", nil)
		td, ok := GetTrustDomain(req)
		assert.False(t, ok)
		assert.Equal(t, spiffeid.TrustDomain{}, td)
	})
}

func TestGetPath(t *testing.T) {
	tests := []struct {
		name     string
		id       string
		wantPath string
		wantOK   bool
	}{
		{
			name:     "service path",
			id:       "spiffe://example.org/service/frontend",
			wantPath: "/service/frontend",
			wantOK:   true,
		},
		{
			name:     "nested path",
			id:       "spiffe://example.org/ns/prod/service/api",
			wantPath: "/ns/prod/service/api",
			wantOK:   true,
		},
		{
			name:     "simple path",
			id:       "spiffe://example.org/workload",
			wantPath: "/workload",
			wantOK:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/test", nil)
			testID := spiffeid.RequireFromString(tt.id)
			req = WithSPIFFEID(req, testID)

			path, ok := GetPath(req)
			assert.Equal(t, tt.wantOK, ok)
			if tt.wantOK {
				assert.Equal(t, tt.wantPath, path)
			}
		})
	}

	t.Run("no ID in context", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/test", nil)
		path, ok := GetPath(req)
		assert.False(t, ok)
		assert.Empty(t, path)
	})
}

func TestGetPathSegments(t *testing.T) {
	tests := []struct {
		name         string
		id           string
		wantSegments []string
		wantOK       bool
	}{
		{
			name:         "multiple segments",
			id:           "spiffe://example.org/service/frontend/prod",
			wantSegments: []string{"service", "frontend", "prod"},
			wantOK:       true,
		},
		{
			name:         "single segment",
			id:           "spiffe://example.org/service",
			wantSegments: []string{"service"},
			wantOK:       true,
		},
		{
			name:         "no nested segments",
			id:           "spiffe://example.org/workload",
			wantSegments: []string{"workload"},
			wantOK:       true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/test", nil)
			testID := spiffeid.RequireFromString(tt.id)
			req = WithSPIFFEID(req, testID)

			segments, ok := GetPathSegments(req)
			assert.Equal(t, tt.wantOK, ok)
			if tt.wantOK {
				assert.Equal(t, tt.wantSegments, segments)
			}
		})
	}

	t.Run("no ID in context", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/test", nil)
		segments, ok := GetPathSegments(req)
		assert.False(t, ok)
		assert.Nil(t, segments)
	})
}

func TestGetIDString(t *testing.T) {
	t.Run("ID present", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/test", nil)
		testID := spiffeid.RequireFromString("spiffe://example.org/service")
		req = WithSPIFFEID(req, testID)

		result := GetIDString(req)
		assert.Equal(t, "spiffe://example.org/service", result)
	})

	t.Run("ID not present", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/test", nil)
		result := GetIDString(req)
		assert.Empty(t, result)
	})
}

func TestWithSPIFFEID(t *testing.T) {
	req := httptest.NewRequest("GET", "/test", nil)
	testID := spiffeid.RequireFromString("spiffe://example.org/test")

	req = WithSPIFFEID(req, testID)

	id, ok := GetSPIFFEID(req)
	require.True(t, ok)
	assert.Equal(t, testID, id)
}
