package httpcontext

// Identity Matching Tests
//
// These tests verify path and trust domain matching utilities for SPIFFE identities.
// Tests cover trust domain matching, path prefix/suffix matching, and exact ID matching.
//
// Run these tests with:
//
//	go test ./internal/adapters/inbound/httpapi/... -v -run TestMatches
//	go test ./internal/adapters/inbound/httpapi/... -v -run TestHas
//	go test ./internal/adapters/inbound/httpapi/... -cover

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/spiffe/go-spiffe/v2/spiffeid"
	"github.com/stretchr/testify/assert"
)

func TestMatchesTrustDomainID(t *testing.T) {
	tests := []struct {
		name        string
		id          string
		trustDomain spiffeid.TrustDomain
		want        bool
	}{
		{
			name:        "matches",
			id:          "spiffe://example.org/service",
			trustDomain: spiffeid.RequireTrustDomainFromString("example.org"),
			want:        true,
		},
		{
			name:        "does not match",
			id:          "spiffe://example.org/service",
			trustDomain: spiffeid.RequireTrustDomainFromString("other.org"),
			want:        false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/test", http.NoBody)
			testID := spiffeid.RequireFromString(tt.id)
			req = WithSPIFFEID(req, testID)

			result := MatchesTrustDomainID(req, tt.trustDomain)
			assert.Equal(t, tt.want, result)
		})
	}

	t.Run("no ID in context", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/test", http.NoBody)
		exampleTD := spiffeid.RequireTrustDomainFromString("example.org")
		result := MatchesTrustDomainID(req, exampleTD)
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
			req := httptest.NewRequest("GET", "/test", http.NoBody)
			testID := spiffeid.RequireFromString(tt.id)
			req = WithSPIFFEID(req, testID)

			result := HasPathPrefix(req, tt.prefix)
			assert.Equal(t, tt.want, result)
		})
	}

	t.Run("no ID in context", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/test", http.NoBody)
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
			req := httptest.NewRequest("GET", "/test", http.NoBody)
			testID := spiffeid.RequireFromString(tt.id)
			req = WithSPIFFEID(req, testID)

			result := HasPathSuffix(req, tt.suffix)
			assert.Equal(t, tt.want, result)
		})
	}
}

func TestMatchesIDParsed(t *testing.T) {
	tests := []struct {
		name       string
		id         string
		expectedID spiffeid.ID
		want       bool
	}{
		{
			name:       "exact match",
			id:         "spiffe://example.org/service/frontend",
			expectedID: spiffeid.RequireFromString("spiffe://example.org/service/frontend"),
			want:       true,
		},
		{
			name:       "does not match",
			id:         "spiffe://example.org/service/frontend",
			expectedID: spiffeid.RequireFromString("spiffe://example.org/service/backend"),
			want:       false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/test", http.NoBody)
			testID := spiffeid.RequireFromString(tt.id)
			req = WithSPIFFEID(req, testID)

			result := MatchesIDParsed(req, tt.expectedID)
			assert.Equal(t, tt.want, result)
		})
	}
}
