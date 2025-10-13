// Package spiffeid_test provides comprehensive tests for the spiffeid package,
// covering identity extraction, validation, and middleware functionality.
package httpcontext_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	spiffeidhttp "github.com/pocket/hexagon/spire/internal/adapters/inbound/identityserver/httpcontext"
	"github.com/spiffe/go-spiffe/v2/spiffeid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Test helpers
var (
	testID  = spiffeid.RequireFromString("spiffe://example.org/test")
	testID2 = spiffeid.RequireFromString("spiffe://example.org/service/frontend")
	otherTD = spiffeid.RequireFromString("spiffe://other.org/test")
)

func TestGetSPIFFEID(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		req     *http.Request
		wantID  spiffeid.ID
		wantOK  bool
	}{
		{
			name:    "valid ID in context",
			req:     spiffeidhttp.WithSPIFFEID(httptest.NewRequest("GET", "/", nil), testID),
			wantID:  testID,
			wantOK:  true,
		},
		{
			name:    "no ID in context",
			req:     httptest.NewRequest("GET", "/", nil),
			wantID:  spiffeid.ID{},
			wantOK:  false,
		},
		{
			name:    "nil request",
			req:     nil,
			wantID:  spiffeid.ID{},
			wantOK:  false,
		},
	}

	for _, tt := range tests {
		tt := tt // Capture range variable
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			gotID, gotOK := spiffeidhttp.GetSPIFFEID(tt.req)
			assert.Equal(t, tt.wantOK, gotOK, "GetSPIFFEID() ok")
			if gotOK {
				assert.Equal(t, tt.wantID.String(), gotID.String(), "GetSPIFFEID() id")
			}
		})
	}
}

func TestMustGetSPIFFEID(t *testing.T) {
	t.Parallel()

	t.Run("with ID returns ID", func(t *testing.T) {
		t.Parallel()

		req := spiffeidhttp.WithSPIFFEID(httptest.NewRequest("GET", "/", nil), testID)
		id := spiffeidhttp.MustGetSPIFFEID(req)
		assert.Equal(t, testID.String(), id.String(), "MustGetSPIFFEID() id")
	})

	t.Run("without ID panics with sentinel error", func(t *testing.T) {
		t.Parallel()

		req := httptest.NewRequest("GET", "/", nil)
		assert.PanicsWithValue(t, spiffeidhttp.ErrNoSPIFFEID, func() {
			spiffeidhttp.MustGetSPIFFEID(req)
		}, "MustGetSPIFFEID should panic with ErrNoSPIFFEID")
	})

	t.Run("nil request panics with sentinel error", func(t *testing.T) {
		t.Parallel()

		assert.PanicsWithValue(t, spiffeidhttp.ErrNoSPIFFEID, func() {
			spiffeidhttp.MustGetSPIFFEID(nil)
		}, "MustGetSPIFFEID should panic with ErrNoSPIFFEID for nil request")
	})
}

func TestGetTrustDomain(t *testing.T) {
	tests := []struct {
		name   string
		req    *http.Request
		wantTD string
		wantOK bool
	}{
		{
			name:   "valid trust domain",
			req:    spiffeidhttp.WithSPIFFEID(httptest.NewRequest("GET", "/", nil), testID),
			wantTD: "example.org",
			wantOK: true,
		},
		{
			name:   "no ID in context",
			req:    httptest.NewRequest("GET", "/", nil),
			wantTD: "",
			wantOK: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotTD, gotOK := spiffeidhttp.GetTrustDomain(tt.req)
			if gotOK != tt.wantOK {
				t.Errorf("GetTrustDomain() ok = %v, want %v", gotOK, tt.wantOK)
			}
			if gotOK && gotTD.String() != tt.wantTD {
				t.Errorf("GetTrustDomain() = %v, want %v", gotTD, tt.wantTD)
			}
		})
	}
}

func TestGetPath(t *testing.T) {
	tests := []struct {
		name     string
		req      *http.Request
		wantPath string
		wantOK   bool
	}{
		{
			name:     "simple path",
			req:      spiffeidhttp.WithSPIFFEID(httptest.NewRequest("GET", "/", nil), testID),
			wantPath: "/test",
			wantOK:   true,
		},
		{
			name:     "nested path",
			req:      spiffeidhttp.WithSPIFFEID(httptest.NewRequest("GET", "/", nil), testID2),
			wantPath: "/service/frontend",
			wantOK:   true,
		},
		{
			name:     "no ID",
			req:      httptest.NewRequest("GET", "/", nil),
			wantPath: "",
			wantOK:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotPath, gotOK := spiffeidhttp.GetPath(tt.req)
			if gotOK != tt.wantOK {
				t.Errorf("GetPath() ok = %v, want %v", gotOK, tt.wantOK)
			}
			if gotPath != tt.wantPath {
				t.Errorf("GetPath() = %v, want %v", gotPath, tt.wantPath)
			}
		})
	}
}

func TestMatchesTrustDomain(t *testing.T) {
	tests := []struct {
		name        string
		req         *http.Request
		trustDomain string
		want        bool
	}{
		{
			name:        "matching trust domain",
			req:         spiffeidhttp.WithSPIFFEID(httptest.NewRequest("GET", "/", nil), testID),
			trustDomain: "example.org",
			want:        true,
		},
		{
			name:        "non-matching trust domain",
			req:         spiffeidhttp.WithSPIFFEID(httptest.NewRequest("GET", "/", nil), testID),
			trustDomain: "other.org",
			want:        false,
		},
		{
			name:        "no ID",
			req:         httptest.NewRequest("GET", "/", nil),
			trustDomain: "example.org",
			want:        false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := spiffeidhttp.MatchesTrustDomain(tt.req, tt.trustDomain); got != tt.want {
				t.Errorf("MatchesTrustDomain() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestHasPathPrefix(t *testing.T) {
	tests := []struct {
		name   string
		req    *http.Request
		prefix string
		want   bool
	}{
		{
			name:   "matching prefix",
			req:    spiffeidhttp.WithSPIFFEID(httptest.NewRequest("GET", "/", nil), testID2),
			prefix: "/service/",
			want:   true,
		},
		{
			name:   "non-matching prefix",
			req:    spiffeidhttp.WithSPIFFEID(httptest.NewRequest("GET", "/", nil), testID2),
			prefix: "/admin/",
			want:   false,
		},
		{
			name:   "no ID",
			req:    httptest.NewRequest("GET", "/", nil),
			prefix: "/service/",
			want:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := spiffeidhttp.HasPathPrefix(tt.req, tt.prefix); got != tt.want {
				t.Errorf("HasPathPrefix() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestHasPathSuffix(t *testing.T) {
	tests := []struct {
		name   string
		req    *http.Request
		suffix string
		want   bool
	}{
		{
			name:   "matching suffix",
			req:    spiffeidhttp.WithSPIFFEID(httptest.NewRequest("GET", "/", nil), testID2),
			suffix: "/frontend",
			want:   true,
		},
		{
			name:   "non-matching suffix",
			req:    spiffeidhttp.WithSPIFFEID(httptest.NewRequest("GET", "/", nil), testID2),
			suffix: "/backend",
			want:   false,
		},
		{
			name:   "no ID",
			req:    httptest.NewRequest("GET", "/", nil),
			suffix: "/frontend",
			want:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := spiffeidhttp.HasPathSuffix(tt.req, tt.suffix); got != tt.want {
				t.Errorf("HasPathSuffix() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGetPathSegments(t *testing.T) {
	tests := []struct {
		name         string
		req          *http.Request
		wantSegments []string
		wantOK       bool
	}{
		{
			name:         "nested path",
			req:          spiffeidhttp.WithSPIFFEID(httptest.NewRequest("GET", "/", nil), testID2),
			wantSegments: []string{"service", "frontend"},
			wantOK:       true,
		},
		{
			name:         "single segment",
			req:          spiffeidhttp.WithSPIFFEID(httptest.NewRequest("GET", "/", nil), testID),
			wantSegments: []string{"test"},
			wantOK:       true,
		},
		{
			name:         "no ID",
			req:          httptest.NewRequest("GET", "/", nil),
			wantSegments: nil,
			wantOK:       false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotSegments, gotOK := spiffeidhttp.GetPathSegments(tt.req)
			if gotOK != tt.wantOK {
				t.Errorf("GetPathSegments() ok = %v, want %v", gotOK, tt.wantOK)
			}
			if gotOK {
				if len(gotSegments) != len(tt.wantSegments) {
					t.Errorf("GetPathSegments() len = %v, want %v", len(gotSegments), len(tt.wantSegments))
					return
				}
				for i, seg := range gotSegments {
					if seg != tt.wantSegments[i] {
						t.Errorf("GetPathSegments()[%d] = %v, want %v", i, seg, tt.wantSegments[i])
					}
				}
			}
		})
	}
}

func TestMatchesID(t *testing.T) {
	tests := []struct {
		name       string
		req        *http.Request
		expectedID string
		want       bool
	}{
		{
			name:       "matching ID",
			req:        spiffeidhttp.WithSPIFFEID(httptest.NewRequest("GET", "/", nil), testID),
			expectedID: "spiffe://example.org/test",
			want:       true,
		},
		{
			name:       "non-matching ID",
			req:        spiffeidhttp.WithSPIFFEID(httptest.NewRequest("GET", "/", nil), testID),
			expectedID: "spiffe://example.org/other",
			want:       false,
		},
		{
			name:       "no ID",
			req:        httptest.NewRequest("GET", "/", nil),
			expectedID: "spiffe://example.org/test",
			want:       false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := spiffeidhttp.MatchesID(tt.req, tt.expectedID); got != tt.want {
				t.Errorf("MatchesID() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGetIDString(t *testing.T) {
	tests := []struct {
		name string
		req  *http.Request
		want string
	}{
		{
			name: "with ID",
			req:  spiffeidhttp.WithSPIFFEID(httptest.NewRequest("GET", "/", nil), testID),
			want: "spiffe://example.org/test",
		},
		{
			name: "without ID",
			req:  httptest.NewRequest("GET", "/", nil),
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := spiffeidhttp.GetIDString(tt.req); got != tt.want {
				t.Errorf("GetIDString() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestWithSPIFFEID(t *testing.T) {
	req := httptest.NewRequest("GET", "/", nil)
	req = spiffeidhttp.WithSPIFFEID(req, testID)

	id, ok := spiffeidhttp.GetSPIFFEID(req)
	if !ok {
		t.Fatal("WithSPIFFEID() did not set ID in context")
	}
	if id.String() != testID.String() {
		t.Errorf("WithSPIFFEID() set ID = %v, want %v", id, testID)
	}
}

func TestRequireAuthentication(t *testing.T) {
	handlerCalled := false
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handlerCalled = true
		w.WriteHeader(http.StatusOK)
	})

	middleware := spiffeidhttp.RequireAuthentication(handler)

	t.Run("with authentication", func(t *testing.T) {
		handlerCalled = false
		req := spiffeidhttp.WithSPIFFEID(httptest.NewRequest("GET", "/", nil), testID)
		rr := httptest.NewRecorder()

		middleware.ServeHTTP(rr, req)

		if !handlerCalled {
			t.Error("handler was not called with authentication")
		}
		if rr.Code != http.StatusOK {
			t.Errorf("status code = %v, want %v", rr.Code, http.StatusOK)
		}
	})

	t.Run("without authentication", func(t *testing.T) {
		handlerCalled = false
		req := httptest.NewRequest("GET", "/", nil)
		rr := httptest.NewRecorder()

		middleware.ServeHTTP(rr, req)

		if handlerCalled {
			t.Error("handler was called without authentication")
		}
		if rr.Code != http.StatusUnauthorized {
			t.Errorf("status code = %v, want %v", rr.Code, http.StatusUnauthorized)
		}
	})
}

func TestRequireTrustDomain(t *testing.T) {
	handlerCalled := false
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handlerCalled = true
		w.WriteHeader(http.StatusOK)
	})

	middleware := spiffeidhttp.RequireTrustDomain("example.org", handler)

	t.Run("matching trust domain", func(t *testing.T) {
		handlerCalled = false
		req := spiffeidhttp.WithSPIFFEID(httptest.NewRequest("GET", "/", nil), testID)
		rr := httptest.NewRecorder()

		middleware.ServeHTTP(rr, req)

		if !handlerCalled {
			t.Error("handler was not called with matching trust domain")
		}
		if rr.Code != http.StatusOK {
			t.Errorf("status code = %v, want %v", rr.Code, http.StatusOK)
		}
	})

	t.Run("non-matching trust domain", func(t *testing.T) {
		handlerCalled = false
		req := spiffeidhttp.WithSPIFFEID(httptest.NewRequest("GET", "/", nil), otherTD)
		rr := httptest.NewRecorder()

		middleware.ServeHTTP(rr, req)

		if handlerCalled {
			t.Error("handler was called with non-matching trust domain")
		}
		if rr.Code != http.StatusForbidden {
			t.Errorf("status code = %v, want %v", rr.Code, http.StatusForbidden)
		}
	})
}

func TestRequirePathPrefix(t *testing.T) {
	handlerCalled := false
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handlerCalled = true
		w.WriteHeader(http.StatusOK)
	})

	middleware := spiffeidhttp.RequirePathPrefix("/service/", handler)

	t.Run("matching prefix", func(t *testing.T) {
		handlerCalled = false
		req := spiffeidhttp.WithSPIFFEID(httptest.NewRequest("GET", "/", nil), testID2)
		rr := httptest.NewRecorder()

		middleware.ServeHTTP(rr, req)

		if !handlerCalled {
			t.Error("handler was not called with matching prefix")
		}
		if rr.Code != http.StatusOK {
			t.Errorf("status code = %v, want %v", rr.Code, http.StatusOK)
		}
	})

	t.Run("non-matching prefix", func(t *testing.T) {
		handlerCalled = false
		req := spiffeidhttp.WithSPIFFEID(httptest.NewRequest("GET", "/", nil), testID)
		rr := httptest.NewRecorder()

		middleware.ServeHTTP(rr, req)

		if handlerCalled {
			t.Error("handler was called with non-matching prefix")
		}
		if rr.Code != http.StatusForbidden {
			t.Errorf("status code = %v, want %v", rr.Code, http.StatusForbidden)
		}
	})
}

func TestLogIdentity(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	middleware := spiffeidhttp.LogIdentity(handler)

	t.Run("logs with ID", func(t *testing.T) {
		req := spiffeidhttp.WithSPIFFEID(httptest.NewRequest("GET", "/test", nil), testID)
		rr := httptest.NewRecorder()

		middleware.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("status code = %v, want %v", rr.Code, http.StatusOK)
		}
		// Note: In production, would test actual log output with mock logger
	})

	t.Run("logs without ID", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/test", nil)
		rr := httptest.NewRecorder()

		middleware.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("status code = %v, want %v", rr.Code, http.StatusOK)
		}
	})
}

// Benchmark GetSPIFFEID (hot path)
func BenchmarkGetSPIFFEID(b *testing.B) {
	req := spiffeidhttp.WithSPIFFEID(httptest.NewRequest("GET", "/", nil), testID)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		spiffeidhttp.GetSPIFFEID(req)
	}
}

// Benchmark middleware chain
func BenchmarkMiddlewareChain(b *testing.B) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})

	middleware := spiffeidhttp.RequireAuthentication(
		spiffeidhttp.RequireTrustDomain("example.org",
			spiffeidhttp.RequirePathPrefix("/service/", handler),
		),
	)

	req := spiffeidhttp.WithSPIFFEID(httptest.NewRequest("GET", "/", nil), testID2)
	rr := httptest.NewRecorder()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		middleware.ServeHTTP(rr, req)
	}
}

// TestMiddlewareComposition tests chaining multiple middlewares together
func TestMiddlewareComposition(t *testing.T) {
	t.Parallel()

	handlerCalled := false
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handlerCalled = true
		w.WriteHeader(http.StatusOK)
	})

	// Chain: Auth -> TrustDomain -> PathPrefix
	middleware := spiffeidhttp.RequireAuthentication(
		spiffeidhttp.RequireTrustDomain("example.org",
			spiffeidhttp.RequirePathPrefix("/service/", handler),
		),
	)

	t.Run("all checks pass", func(t *testing.T) {
		handlerCalled = false
		req := spiffeidhttp.WithSPIFFEID(httptest.NewRequest("GET", "/api", nil), testID2)
		rr := httptest.NewRecorder()

		middleware.ServeHTTP(rr, req)

		require.True(t, handlerCalled, "handler should be called when all checks pass")
		assert.Equal(t, http.StatusOK, rr.Code)
	})

	t.Run("fails at authentication", func(t *testing.T) {
		handlerCalled = false
		req := httptest.NewRequest("GET", "/api", nil)
		rr := httptest.NewRecorder()

		middleware.ServeHTTP(rr, req)

		require.False(t, handlerCalled, "handler should not be called without auth")
		assert.Equal(t, http.StatusUnauthorized, rr.Code)
	})

	t.Run("fails at trust domain", func(t *testing.T) {
		handlerCalled = false
		req := spiffeidhttp.WithSPIFFEID(httptest.NewRequest("GET", "/api", nil), otherTD)
		rr := httptest.NewRecorder()

		middleware.ServeHTTP(rr, req)

		require.False(t, handlerCalled, "handler should not be called with wrong trust domain")
		assert.Equal(t, http.StatusForbidden, rr.Code)
	})

	t.Run("fails at path prefix", func(t *testing.T) {
		handlerCalled = false
		req := spiffeidhttp.WithSPIFFEID(httptest.NewRequest("GET", "/api", nil), testID)
		rr := httptest.NewRecorder()

		middleware.ServeHTTP(rr, req)

		require.False(t, handlerCalled, "handler should not be called without matching prefix")
		assert.Equal(t, http.StatusForbidden, rr.Code)
	})
}

// TestConcurrentAccess ensures thread safety of GetSPIFFEID
func TestConcurrentAccess(t *testing.T) {
	t.Parallel()

	req := spiffeidhttp.WithSPIFFEID(httptest.NewRequest("GET", "/", nil), testID)

	// Spawn 100 goroutines reading the same request context
	done := make(chan bool, 100)
	for i := 0; i < 100; i++ {
		go func() {
			id, ok := spiffeidhttp.GetSPIFFEID(req)
			assert.True(t, ok, "should find ID")
			assert.Equal(t, testID.String(), id.String(), "ID should match")
			done <- true
		}()
	}

	// Wait for all goroutines
	for i := 0; i < 100; i++ {
		<-done
	}
}
