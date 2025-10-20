//go:build dev

package inmemory_test

// Parser Coverage Tests
//
// These tests verify edge cases and error paths for the inmemory Parser implementations.
// Tests cover TrustDomainParser and IdentityCredentialParser with various valid and invalid inputs.
//
// Run these tests with:
//
//	go test ./internal/adapters/outbound/inmemory/... -v -run TestTrustDomainParser
//	go test ./internal/adapters/outbound/inmemory/... -v -run TestIdentityCredentialParser
//	go test ./internal/adapters/outbound/inmemory/... -cover

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pocket/hexagon/spire/internal/adapters/outbound/inmemory"
	"github.com/pocket/hexagon/spire/internal/domain"
)

func TestTrustDomainParser_FromString_EmptyString(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	parser := inmemory.NewInMemoryTrustDomainParser()

	// Act
	td, err := parser.FromString(ctx, "")

	// Assert - Empty string should be rejected
	assert.Error(t, err)
	assert.Nil(t, td)
}

// TestTrustDomainParser_FromString_ValidCases tests various valid inputs
func TestTrustDomainParser_FromString_ValidCases(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	parser := inmemory.NewInMemoryTrustDomainParser()

	tests := []struct {
		name  string
		input string
	}{
		{"simple domain", "example.org"},
		{"subdomain", "prod.example.org"},
		{"with dash", "my-domain.com"},
		{"multi-level", "a.b.c.example.org"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Act
			td, err := parser.FromString(ctx, tt.input)

			// Assert
			require.NoError(t, err)
			assert.NotNil(t, td)
			assert.Equal(t, tt.input, td.String())
		})
	}
}

// Coverage tests for identity_namespace_parser.go - ParseFromPath

// TestIdentityCredentialParser_ParseFromPath tests path-based parsing
func TestIdentityCredentialParser_ParseFromPath(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	parser := inmemory.NewInMemoryIdentityCredentialParser()

	tests := []struct {
		name        string
		trustDomain *domain.TrustDomain
		path        string
		wantError   bool
		expectedURI string
	}{
		{
			name:        "valid workload path",
			trustDomain: domain.NewTrustDomainFromName("example.org"),
			path:        "/workload",
			wantError:   false,
			expectedURI: "spiffe://example.org/workload",
		},
		{
			name:        "root path",
			trustDomain: domain.NewTrustDomainFromName("example.org"),
			path:        "/",
			wantError:   false,
			expectedURI: "spiffe://example.org/",
		},
		{
			name:        "nested path",
			trustDomain: domain.NewTrustDomainFromName("prod.example.org"),
			path:        "/ns/prod/svc",
			wantError:   false,
			expectedURI: "spiffe://prod.example.org/ns/prod/svc",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Act
			credential, err := parser.ParseFromPath(ctx, tt.trustDomain, tt.path)

			// Assert
			if tt.wantError {
				assert.Error(t, err)
				assert.Nil(t, credential)
			} else {
				require.NoError(t, err)
				assert.NotNil(t, credential)
				assert.Equal(t, tt.expectedURI, credential.String())
			}
		})
	}
}
func TestIdentityCredentialParser_ParseFromPath_ErrorCases(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	parser := inmemory.NewInMemoryIdentityCredentialParser()

	tests := []struct {
		name        string
		trustDomain *domain.TrustDomain
		path        string
		wantError   bool
	}{
		{
			name:        "nil trust domain",
			trustDomain: nil,
			path:        "/workload",
			wantError:   true,
		},
		{
			name:        "empty path becomes root",
			trustDomain: domain.NewTrustDomainFromName("example.org"),
			path:        "",
			wantError:   false, // Empty path is normalized to "/"
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Act
			credential, err := parser.ParseFromPath(ctx, tt.trustDomain, tt.path)

			// Assert
			if tt.wantError {
				assert.Error(t, err)
				assert.Nil(t, credential)
			} else {
				require.NoError(t, err)
				assert.NotNil(t, credential)
			}
		})
	}
}

// TestTrustDomainParser_FromString_InvalidDomain tests invalid domain names
func TestTrustDomainParser_FromString_InvalidDomain(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	parser := inmemory.NewInMemoryTrustDomainParser()

	tests := []struct {
		name       string
		input      string
		wantError  bool
		wantErrMsg string
	}{
		{"valid domain", "example.org", false, ""},
		{"with scheme", "https://example.org", true, "must not contain scheme"},
		{"with path", "example.org/path", true, "must not contain path"},
		{"subdomain", "prod.example.org", false, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Act
			td, err := parser.FromString(ctx, tt.input)

			// Assert
			if tt.wantError {
				assert.Error(t, err)
				assert.Nil(t, td)
				if tt.wantErrMsg != "" {
					assert.Contains(t, err.Error(), tt.wantErrMsg)
				}
			} else {
				require.NoError(t, err)
				assert.NotNil(t, td)
			}
		})
	}
}
func TestIdentityCredentialParser_ParseFromString_InvalidURI(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	parser := inmemory.NewInMemoryIdentityCredentialParser()

	tests := []struct {
		name       string
		input      string
		wantError  bool
		wantErrMsg string
	}{
		{"empty string", "", true, "identity credential cannot be empty"},
		{"whitespace only", "  ", true, "identity credential cannot be empty"},
		{"missing scheme", "example.org/workload", true, ""},
		{"wrong scheme", "http://example.org/workload", true, "must use 'spiffe' scheme"},
		{"missing host/trust domain", "spiffe:///workload", true, "must contain a trust domain"},
		{"with userinfo", "spiffe://user:pass@example.org/workload", true, "must not include userinfo"},
		{"with port", "spiffe://example.org:8080/workload", true, "must not include userinfo, port, query, or fragment"},
		{"with query", "spiffe://example.org/workload?key=value", true, "must not include userinfo, port, query, or fragment"},
		{"with fragment", "spiffe://example.org/workload#section", true, "must not include userinfo, port, query, or fragment"},
		{"valid spiffe URI", "spiffe://example.org/workload", false, ""},
		{"missing path", "spiffe://example.org", false, ""},
		{"with leading/trailing spaces", "  spiffe://example.org/workload  ", false, ""},
		{"uppercase trust domain", "spiffe://EXAMPLE.ORG/workload", false, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Act
			credential, err := parser.ParseFromString(ctx, tt.input)

			// Assert
			if tt.wantError {
				assert.Error(t, err)
				assert.Nil(t, credential)
				if tt.wantErrMsg != "" {
					assert.Contains(t, err.Error(), tt.wantErrMsg)
				}
			} else {
				require.NoError(t, err)
				assert.NotNil(t, credential)
			}
		})
	}
}

func TestIdentityCredentialParser_ParseFromString_Normalization(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	parser := inmemory.NewInMemoryIdentityCredentialParser()

	tests := []struct {
		name        string
		input       string
		expectedURI string
	}{
		{
			name:        "lowercase trust domain",
			input:       "spiffe://EXAMPLE.ORG/workload",
			expectedURI: "spiffe://example.org/workload",
		},
		{
			name:        "mixed case trust domain",
			input:       "spiffe://Example.Org/workload",
			expectedURI: "spiffe://example.org/workload",
		},
		{
			name:        "trim spaces",
			input:       "  spiffe://example.org/workload  ",
			expectedURI: "spiffe://example.org/workload",
		},
		{
			name:        "empty path becomes root",
			input:       "spiffe://example.org",
			expectedURI: "spiffe://example.org/",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Act
			credential, err := parser.ParseFromString(ctx, tt.input)

			// Assert
			require.NoError(t, err)
			assert.NotNil(t, credential)
			assert.Equal(t, tt.expectedURI, credential.String())
		})
	}
}
