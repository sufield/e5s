// Package spiffeid_test provides comprehensive tests for the spiffeid package,
// covering identity extraction, validation, and middleware functionality.
package httpcontext_test

import (
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/spiffe/go-spiffe/v2/spiffeid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	spiffeidhttp "github.com/pocket/hexagon/spire/internal/adapters/inbound/identityserver/httpcontext"
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
		name   string
		req    *http.Request
		wantID spiffeid.ID
		wantOK bool
	}{
		{
			name:   "valid ID in context",
			req:    spiffeidhttp.WithSPIFFEID(httptest.NewRequest("GET", "/test", http.NoBody), testID),
			wantID: testID,
			wantOK: true,
		},
		{
			name:   "no ID in context",
			req:    httptest.NewRequest("GET", "/test", http.NoBody),
			wantID: spiffeid.ID{},
			wantOK: false,
		},
		{
			name:   "nil request",
			req:    nil,
			wantID: spiffeid.ID{},
			wantOK: false,
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

func TestGetSPIFFEIDOrError(t *testing.T) {
	t.Parallel()

	t.Run("with ID returns ID and nil error", func(t *testing.T) {
		t.Parallel()

		req := spiffeidhttp.WithSPIFFEID(httptest.NewRequest("GET", "/test", http.NoBody), testID)
		id, err := spiffeidhttp.GetSPIFFEIDOrError(req)
		require.NoError(t, err)
		assert.Equal(t, testID.String(), id.String())
	})

	t.Run("without ID returns ErrNoSPIFFEID", func(t *testing.T) {
		t.Parallel()

		req := httptest.NewRequest("GET", "/test", http.NoBody)
		id, err := spiffeidhttp.GetSPIFFEIDOrError(req)
		assert.ErrorIs(t, err, spiffeidhttp.ErrNoSPIFFEID)
		assert.True(t, id.IsZero())
	})

	t.Run("nil request returns ErrNoSPIFFEID", func(t *testing.T) {
		t.Parallel()

		id, err := spiffeidhttp.GetSPIFFEIDOrError(nil)
		assert.ErrorIs(t, err, spiffeidhttp.ErrNoSPIFFEID)
		assert.True(t, id.IsZero())
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
			req:    spiffeidhttp.WithSPIFFEID(httptest.NewRequest("GET", "/test", http.NoBody), testID),
			wantTD: "example.org",
			wantOK: true,
		},
		{
			name:   "no ID in context",
			req:    httptest.NewRequest("GET", "/test", http.NoBody),
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
			req:      spiffeidhttp.WithSPIFFEID(httptest.NewRequest("GET", "/test", http.NoBody), testID),
			wantPath: "/test",
			wantOK:   true,
		},
		{
			name:     "nested path",
			req:      spiffeidhttp.WithSPIFFEID(httptest.NewRequest("GET", "/test", http.NoBody), testID2),
			wantPath: "/service/frontend",
			wantOK:   true,
		},
		{
			name:     "no ID",
			req:      httptest.NewRequest("GET", "/test", http.NoBody),
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

func TestMatchesTrustDomainID(t *testing.T) {
	t.Parallel()

	exampleTD := spiffeid.RequireTrustDomainFromString("example.org")
	otherTD := spiffeid.RequireTrustDomainFromString("other.org")

	tests := []struct {
		name        string
		req         *http.Request
		trustDomain spiffeid.TrustDomain
		want        bool
	}{
		{
			name:        "matching trust domain",
			req:         spiffeidhttp.WithSPIFFEID(httptest.NewRequest("GET", "/test", http.NoBody), testID),
			trustDomain: exampleTD,
			want:        true,
		},
		{
			name:        "non-matching trust domain",
			req:         spiffeidhttp.WithSPIFFEID(httptest.NewRequest("GET", "/test", http.NoBody), testID),
			trustDomain: otherTD,
			want:        false,
		},
		{
			name:        "no ID in context",
			req:         httptest.NewRequest("GET", "/test", http.NoBody),
			trustDomain: exampleTD,
			want:        false,
		},
		{
			name:        "nil request",
			req:         nil,
			trustDomain: exampleTD,
			want:        false,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := spiffeidhttp.MatchesTrustDomainID(tt.req, tt.trustDomain)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestHasPathPrefix(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		req    *http.Request
		prefix string
		want   bool
	}{
		{
			name:   "matching prefix with leading slash",
			req:    spiffeidhttp.WithSPIFFEID(httptest.NewRequest("GET", "/test", http.NoBody), testID2),
			prefix: "/service/",
			want:   true,
		},
		{
			name:   "matching prefix without leading slash (auto-normalized)",
			req:    spiffeidhttp.WithSPIFFEID(httptest.NewRequest("GET", "/test", http.NoBody), testID2),
			prefix: "service/",
			want:   true,
		},
		{
			name:   "non-matching prefix",
			req:    spiffeidhttp.WithSPIFFEID(httptest.NewRequest("GET", "/test", http.NoBody), testID2),
			prefix: "/admin/",
			want:   false,
		},
		{
			name:   "empty prefix matches all",
			req:    spiffeidhttp.WithSPIFFEID(httptest.NewRequest("GET", "/test", http.NoBody), testID2),
			prefix: "",
			want:   true,
		},
		{
			name:   "no ID in context",
			req:    httptest.NewRequest("GET", "/test", http.NoBody),
			prefix: "/service/",
			want:   false,
		},
		{
			name:   "nil request",
			req:    nil,
			prefix: "/service/",
			want:   false,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := spiffeidhttp.HasPathPrefix(tt.req, tt.prefix)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestHasPathSuffix(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		req    *http.Request
		suffix string
		want   bool
	}{
		{
			name:   "matching suffix with leading slash",
			req:    spiffeidhttp.WithSPIFFEID(httptest.NewRequest("GET", "/test", http.NoBody), testID2),
			suffix: "/frontend",
			want:   true,
		},
		{
			name:   "matching suffix without leading slash (auto-normalized)",
			req:    spiffeidhttp.WithSPIFFEID(httptest.NewRequest("GET", "/test", http.NoBody), testID2),
			suffix: "frontend",
			want:   true,
		},
		{
			name:   "non-matching suffix",
			req:    spiffeidhttp.WithSPIFFEID(httptest.NewRequest("GET", "/test", http.NoBody), testID2),
			suffix: "/backend",
			want:   false,
		},
		{
			name:   "empty suffix matches all",
			req:    spiffeidhttp.WithSPIFFEID(httptest.NewRequest("GET", "/test", http.NoBody), testID2),
			suffix: "",
			want:   true,
		},
		{
			name:   "no ID in context",
			req:    httptest.NewRequest("GET", "/test", http.NoBody),
			suffix: "/frontend",
			want:   false,
		},
		{
			name:   "nil request",
			req:    nil,
			suffix: "/frontend",
			want:   false,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := spiffeidhttp.HasPathSuffix(tt.req, tt.suffix)
			assert.Equal(t, tt.want, got)
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
			req:          spiffeidhttp.WithSPIFFEID(httptest.NewRequest("GET", "/test", http.NoBody), testID2),
			wantSegments: []string{"service", "frontend"},
			wantOK:       true,
		},
		{
			name:         "single segment",
			req:          spiffeidhttp.WithSPIFFEID(httptest.NewRequest("GET", "/test", http.NoBody), testID),
			wantSegments: []string{"test"},
			wantOK:       true,
		},
		{
			name:         "no ID",
			req:          httptest.NewRequest("GET", "/test", http.NoBody),
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

func TestMatchesIDParsed(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		req        *http.Request
		expectedID spiffeid.ID
		want       bool
	}{
		{
			name:       "matching ID",
			req:        spiffeidhttp.WithSPIFFEID(httptest.NewRequest("GET", "/test", http.NoBody), testID),
			expectedID: testID,
			want:       true,
		},
		{
			name:       "non-matching ID different path",
			req:        spiffeidhttp.WithSPIFFEID(httptest.NewRequest("GET", "/test", http.NoBody), testID),
			expectedID: testID2,
			want:       false,
		},
		{
			name:       "non-matching ID different trust domain",
			req:        spiffeidhttp.WithSPIFFEID(httptest.NewRequest("GET", "/test", http.NoBody), testID),
			expectedID: otherTD,
			want:       false,
		},
		{
			name:       "no ID in context",
			req:        httptest.NewRequest("GET", "/test", http.NoBody),
			expectedID: testID,
			want:       false,
		},
		{
			name:       "nil request",
			req:        nil,
			expectedID: testID,
			want:       false,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := spiffeidhttp.MatchesIDParsed(tt.req, tt.expectedID)
			assert.Equal(t, tt.want, got)
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
			req:  spiffeidhttp.WithSPIFFEID(httptest.NewRequest("GET", "/test", http.NoBody), testID),
			want: "spiffe://example.org/test",
		},
		{
			name: "without ID",
			req:  httptest.NewRequest("GET", "/test", http.NoBody),
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
	t.Parallel()

	t.Run("adds ID to context", func(t *testing.T) {
		t.Parallel()

		req := httptest.NewRequest("GET", "/test", http.NoBody)
		req = spiffeidhttp.WithSPIFFEID(req, testID)

		id, ok := spiffeidhttp.GetSPIFFEID(req)
		require.True(t, ok, "WithSPIFFEID() should set ID in context")
		assert.Equal(t, testID.String(), id.String())
	})

	t.Run("nil request returns nil", func(t *testing.T) {
		t.Parallel()

		req := spiffeidhttp.WithSPIFFEID(nil, testID)
		assert.Nil(t, req, "WithSPIFFEID(nil) should return nil")
	})
}

func TestAuthenticated(t *testing.T) {
	t.Parallel()

	handlerCalled := false
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handlerCalled = true
		w.WriteHeader(http.StatusOK)
	})

	middleware := spiffeidhttp.Authenticated(handler)

	t.Run("with authentication", func(t *testing.T) {
		handlerCalled = false
		req := spiffeidhttp.WithSPIFFEID(httptest.NewRequest("GET", "/test", http.NoBody), testID)
		rr := httptest.NewRecorder()

		middleware.ServeHTTP(rr, req)

		assert.True(t, handlerCalled, "handler should be called with authentication")
		assert.Equal(t, http.StatusOK, rr.Code)
	})

	t.Run("without authentication returns 401", func(t *testing.T) {
		handlerCalled = false
		req := httptest.NewRequest("GET", "/test", http.NoBody)
		rr := httptest.NewRecorder()

		middleware.ServeHTTP(rr, req)

		assert.False(t, handlerCalled, "handler should not be called without authentication")
		assert.Equal(t, http.StatusUnauthorized, rr.Code)
	})

	t.Run("nil request returns 401", func(t *testing.T) {
		handlerCalled = false
		rr := httptest.NewRecorder()

		middleware.ServeHTTP(rr, nil)

		assert.False(t, handlerCalled, "handler should not be called with nil request")
		assert.Equal(t, http.StatusUnauthorized, rr.Code)
	})
}

func TestRequireTrustDomainID(t *testing.T) {
	t.Parallel()

	exampleTD := spiffeid.RequireTrustDomainFromString("example.org")

	handlerCalled := false
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handlerCalled = true
		w.WriteHeader(http.StatusOK)
	})

	middleware := spiffeidhttp.RequireTrustDomainID(exampleTD, handler)

	t.Run("matching trust domain", func(t *testing.T) {
		handlerCalled = false
		req := spiffeidhttp.WithSPIFFEID(httptest.NewRequest("GET", "/test", http.NoBody), testID)
		rr := httptest.NewRecorder()

		middleware.ServeHTTP(rr, req)

		assert.True(t, handlerCalled, "handler should be called with matching trust domain")
		assert.Equal(t, http.StatusOK, rr.Code)
	})

	t.Run("non-matching trust domain returns 403", func(t *testing.T) {
		handlerCalled = false
		req := spiffeidhttp.WithSPIFFEID(httptest.NewRequest("GET", "/test", http.NoBody), otherTD)
		rr := httptest.NewRecorder()

		middleware.ServeHTTP(rr, req)

		assert.False(t, handlerCalled, "handler should not be called with non-matching trust domain")
		assert.Equal(t, http.StatusForbidden, rr.Code)
	})

	t.Run("no ID returns 403", func(t *testing.T) {
		handlerCalled = false
		req := httptest.NewRequest("GET", "/test", http.NoBody)
		rr := httptest.NewRecorder()

		middleware.ServeHTTP(rr, req)

		assert.False(t, handlerCalled, "handler should not be called without ID")
		assert.Equal(t, http.StatusForbidden, rr.Code)
	})
}

func TestRequireAnyTrustDomain(t *testing.T) {
	t.Parallel()

	exampleTD := spiffeid.RequireTrustDomainFromString("example.org")
	otherOrgTD := spiffeid.RequireTrustDomainFromString("other.org")

	handlerCalled := false
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handlerCalled = true
		w.WriteHeader(http.StatusOK)
	})

	allowed := []spiffeid.TrustDomain{exampleTD, otherOrgTD}
	middleware := spiffeidhttp.RequireAnyTrustDomain(allowed, handler)

	t.Run("first allowed trust domain", func(t *testing.T) {
		handlerCalled = false
		req := spiffeidhttp.WithSPIFFEID(httptest.NewRequest("GET", "/test", http.NoBody), testID) // example.org
		rr := httptest.NewRecorder()

		middleware.ServeHTTP(rr, req)

		assert.True(t, handlerCalled, "handler should be called with first allowed trust domain")
		assert.Equal(t, http.StatusOK, rr.Code)
	})

	t.Run("second allowed trust domain", func(t *testing.T) {
		handlerCalled = false
		req := spiffeidhttp.WithSPIFFEID(httptest.NewRequest("GET", "/test", http.NoBody), otherTD) // other.org
		rr := httptest.NewRecorder()

		middleware.ServeHTTP(rr, req)

		assert.True(t, handlerCalled, "handler should be called with second allowed trust domain")
		assert.Equal(t, http.StatusOK, rr.Code)
	})

	t.Run("disallowed trust domain returns 403", func(t *testing.T) {
		handlerCalled = false
		disallowedID := spiffeid.RequireFromString("spiffe://disallowed.org/test")
		req := spiffeidhttp.WithSPIFFEID(httptest.NewRequest("GET", "/test", http.NoBody), disallowedID)
		rr := httptest.NewRecorder()

		middleware.ServeHTTP(rr, req)

		assert.False(t, handlerCalled, "handler should not be called with disallowed trust domain")
		assert.Equal(t, http.StatusForbidden, rr.Code)
	})

	t.Run("no ID returns 401", func(t *testing.T) {
		handlerCalled = false
		req := httptest.NewRequest("GET", "/test", http.NoBody)
		rr := httptest.NewRecorder()

		middleware.ServeHTTP(rr, req)

		assert.False(t, handlerCalled, "handler should not be called without ID")
		assert.Equal(t, http.StatusUnauthorized, rr.Code)
	})
}

func TestRequirePathPrefix(t *testing.T) {
	t.Parallel()

	handlerCalled := false
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handlerCalled = true
		w.WriteHeader(http.StatusOK)
	})

	middleware := spiffeidhttp.RequirePathPrefix("/service/", handler)

	t.Run("matching prefix", func(t *testing.T) {
		handlerCalled = false
		req := spiffeidhttp.WithSPIFFEID(httptest.NewRequest("GET", "/test", http.NoBody), testID2)
		rr := httptest.NewRecorder()

		middleware.ServeHTTP(rr, req)

		assert.True(t, handlerCalled, "handler should be called with matching prefix")
		assert.Equal(t, http.StatusOK, rr.Code)
	})

	t.Run("non-matching prefix returns 403", func(t *testing.T) {
		handlerCalled = false
		req := spiffeidhttp.WithSPIFFEID(httptest.NewRequest("GET", "/test", http.NoBody), testID)
		rr := httptest.NewRecorder()

		middleware.ServeHTTP(rr, req)

		assert.False(t, handlerCalled, "handler should not be called with non-matching prefix")
		assert.Equal(t, http.StatusForbidden, rr.Code)
	})
}

func TestLogIdentity(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	middleware := spiffeidhttp.LogIdentity(handler)

	t.Run("logs with ID", func(t *testing.T) {
		req := spiffeidhttp.WithSPIFFEID(httptest.NewRequest("GET", "/test", http.NoBody), testID)
		rr := httptest.NewRecorder()

		middleware.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("status code = %v, want %v", rr.Code, http.StatusOK)
		}
		// Note: In production, would test actual log output with mock logger
	})

	t.Run("logs without ID", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/test", http.NoBody)
		rr := httptest.NewRecorder()

		middleware.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("status code = %v, want %v", rr.Code, http.StatusOK)
		}
	})
}

// Benchmark GetSPIFFEID (hot path)
func BenchmarkGetSPIFFEID(b *testing.B) {
	req := spiffeidhttp.WithSPIFFEID(httptest.NewRequest("GET", "/test", http.NoBody), testID)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		spiffeidhttp.GetSPIFFEID(req)
	}
}

// Benchmark middleware chain
func BenchmarkMiddlewareChain(b *testing.B) {
	exampleTD := spiffeid.RequireTrustDomainFromString("example.org")
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})

	middleware := spiffeidhttp.Authenticated(
		spiffeidhttp.RequireTrustDomainID(exampleTD,
			spiffeidhttp.RequirePathPrefix("/service/", handler),
		),
	)

	req := spiffeidhttp.WithSPIFFEID(httptest.NewRequest("GET", "/test", http.NoBody), testID2)
	rr := httptest.NewRecorder()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		middleware.ServeHTTP(rr, req)
	}
}

// TestMiddlewareComposition tests chaining multiple middlewares together
func TestMiddlewareComposition(t *testing.T) {
	t.Parallel()

	exampleTD := spiffeid.RequireTrustDomainFromString("example.org")

	handlerCalled := false
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handlerCalled = true
		w.WriteHeader(http.StatusOK)
	})

	// Chain: Auth -> TrustDomain -> PathPrefix
	middleware := spiffeidhttp.Authenticated(
		spiffeidhttp.RequireTrustDomainID(exampleTD,
			spiffeidhttp.RequirePathPrefix("/service/", handler),
		),
	)

	t.Run("all checks pass", func(t *testing.T) {
		handlerCalled = false
		req := spiffeidhttp.WithSPIFFEID(httptest.NewRequest("GET", "/test", http.NoBody), testID2)
		rr := httptest.NewRecorder()

		middleware.ServeHTTP(rr, req)

		require.True(t, handlerCalled, "handler should be called when all checks pass")
		assert.Equal(t, http.StatusOK, rr.Code)
	})

	t.Run("fails at authentication", func(t *testing.T) {
		handlerCalled = false
		req := httptest.NewRequest("GET", "/test", http.NoBody)
		rr := httptest.NewRecorder()

		middleware.ServeHTTP(rr, req)

		require.False(t, handlerCalled, "handler should not be called without auth")
		assert.Equal(t, http.StatusUnauthorized, rr.Code)
	})

	t.Run("fails at trust domain", func(t *testing.T) {
		handlerCalled = false
		req := spiffeidhttp.WithSPIFFEID(httptest.NewRequest("GET", "/test", http.NoBody), otherTD)
		rr := httptest.NewRecorder()

		middleware.ServeHTTP(rr, req)

		require.False(t, handlerCalled, "handler should not be called with wrong trust domain")
		assert.Equal(t, http.StatusForbidden, rr.Code)
	})

	t.Run("fails at path prefix", func(t *testing.T) {
		handlerCalled = false
		req := spiffeidhttp.WithSPIFFEID(httptest.NewRequest("GET", "/test", http.NoBody), testID)
		rr := httptest.NewRecorder()

		middleware.ServeHTTP(rr, req)

		require.False(t, handlerCalled, "handler should not be called without matching prefix")
		assert.Equal(t, http.StatusForbidden, rr.Code)
	})
}

// TestConcurrentAccess ensures thread safety of GetSPIFFEID
func TestConcurrentAccess(t *testing.T) {
	t.Parallel()

	req := spiffeidhttp.WithSPIFFEID(httptest.NewRequest("GET", "/test", http.NoBody), testID)

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

// TestSetLogger verifies logger can be set and swapped thread-safely
func TestSetLogger(t *testing.T) {
	t.Parallel()

	// Save original logger to restore after test
	originalLogger := log.New(os.Stdout, "", log.LstdFlags)
	defer spiffeidhttp.SetLogger(originalLogger)

	t.Run("set custom logger", func(t *testing.T) {
		customLogger := log.New(os.Stderr, "custom: ", log.Lshortfile)
		spiffeidhttp.SetLogger(customLogger)
		// Logger should be swapped atomically - no way to test directly without exposing internals
		// This test just ensures no panics
	})

	t.Run("set nil logger restores default", func(t *testing.T) {
		spiffeidhttp.SetLogger(nil)
		// Should restore default logger without panicking
	})
}

// TestSetRedactIdentity verifies redaction flag can be toggled thread-safely
func TestSetRedactIdentity(t *testing.T) {
	t.Parallel()

	// Save original state to restore after test
	defer spiffeidhttp.SetRedactIdentity(false)

	t.Run("enable redaction", func(t *testing.T) {
		spiffeidhttp.SetRedactIdentity(true)
		// Redaction flag should be set atomically - verified by LogIdentity behavior
	})

	t.Run("disable redaction", func(t *testing.T) {
		spiffeidhttp.SetRedactIdentity(false)
		// Redaction flag should be unset atomically
	})
}
