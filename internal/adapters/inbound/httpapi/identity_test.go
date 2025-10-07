package httpapi

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

func TestMatchesTrustDomain(t *testing.T) {
	tests := []struct {
		name        string
		id          string
		trustDomain string
		want        bool
	}{
		{
			name:        "matches",
			id:          "spiffe://example.org/service",
			trustDomain: "example.org",
			want:        true,
		},
		{
			name:        "does not match",
			id:          "spiffe://example.org/service",
			trustDomain: "other.org",
			want:        false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/test", nil)
			testID := spiffeid.RequireFromString(tt.id)
			req = WithSPIFFEID(req, testID)

			result := MatchesTrustDomain(req, tt.trustDomain)
			assert.Equal(t, tt.want, result)
		})
	}

	t.Run("no ID in context", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/test", nil)
		result := MatchesTrustDomain(req, "example.org")
		assert.False(t, result)
	})
}

func TestHasPathPrefix(t *testing.T) {
	tests := []struct {
		name   string
		id     string
		prefix string
		want   bool
	}{
		{
			name:   "has prefix",
			id:     "spiffe://example.org/service/frontend",
			prefix: "/service/",
			want:   true,
		},
		{
			name:   "does not have prefix",
			id:     "spiffe://example.org/workload/backend",
			prefix: "/service/",
			want:   false,
		},
		{
			name:   "empty prefix",
			id:     "spiffe://example.org/service",
			prefix: "",
			want:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/test", nil)
			testID := spiffeid.RequireFromString(tt.id)
			req = WithSPIFFEID(req, testID)

			result := HasPathPrefix(req, tt.prefix)
			assert.Equal(t, tt.want, result)
		})
	}

	t.Run("no ID in context", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/test", nil)
		result := HasPathPrefix(req, "/service/")
		assert.False(t, result)
	})
}

func TestHasPathSuffix(t *testing.T) {
	tests := []struct {
		name   string
		id     string
		suffix string
		want   bool
	}{
		{
			name:   "has suffix",
			id:     "spiffe://example.org/service/admin",
			suffix: "/admin",
			want:   true,
		},
		{
			name:   "does not have suffix",
			id:     "spiffe://example.org/service/user",
			suffix: "/admin",
			want:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/test", nil)
			testID := spiffeid.RequireFromString(tt.id)
			req = WithSPIFFEID(req, testID)

			result := HasPathSuffix(req, tt.suffix)
			assert.Equal(t, tt.want, result)
		})
	}
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

func TestMatchesID(t *testing.T) {
	tests := []struct {
		name       string
		id         string
		expectedID string
		want       bool
	}{
		{
			name:       "exact match",
			id:         "spiffe://example.org/service/frontend",
			expectedID: "spiffe://example.org/service/frontend",
			want:       true,
		},
		{
			name:       "does not match",
			id:         "spiffe://example.org/service/frontend",
			expectedID: "spiffe://example.org/service/backend",
			want:       false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/test", nil)
			testID := spiffeid.RequireFromString(tt.id)
			req = WithSPIFFEID(req, testID)

			result := MatchesID(req, tt.expectedID)
			assert.Equal(t, tt.want, result)
		})
	}
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

func TestRequireAuthentication(t *testing.T) {
	handlerCalled := false
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handlerCalled = true
		w.WriteHeader(http.StatusOK)
	})

	middleware := RequireAuthentication(handler)

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

func TestRequireTrustDomain(t *testing.T) {
	handlerCalled := false
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handlerCalled = true
		w.WriteHeader(http.StatusOK)
	})

	middleware := RequireTrustDomain("example.org", handler)

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
