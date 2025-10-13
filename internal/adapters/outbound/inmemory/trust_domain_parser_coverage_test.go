//go:build dev

package inmemory_test

// Trust Domain Parser Coverage Tests
//
// These tests verify edge cases and defensive improvements for the InMemory trust domain parser.
// Tests cover DNS-like validation, label rules, length limits, and illegal character rejection.
//
// Run these tests with:
//
//	go test ./internal/adapters/outbound/inmemory/... -v -run TestTrustDomainParser_Coverage
//	go test ./internal/adapters/outbound/inmemory/... -cover

import (
	"context"
	"strings"
	"testing"

	"github.com/pocket/hexagon/spire/internal/adapters/outbound/inmemory"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestTrustDomainParser_Coverage_ValidInputs tests acceptance of valid trust domains
func TestTrustDomainParser_Coverage_ValidInputs(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	parser := inmemory.NewInMemoryTrustDomainParser()

	tests := []struct {
		name  string
		input string
	}{
		{"simple domain", "example.org"},
		{"subdomain", "dev.example.org"},
		{"internationalized (punycode)", "xn--bcher-kva.example"},
		{"local domain", "dev.local"},
		{"single label", "localhost"},
		{"hyphen in label", "my-service.example.org"},
		{"numbers", "service1.example.org"},
		{"multiple subdomains", "api.v1.prod.example.org"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Act
			td, err := parser.FromString(ctx, tt.input)

			// Assert
			require.NoError(t, err)
			assert.NotNil(t, td)
			assert.Equal(t, strings.ToLower(tt.input), td.String())
		})
	}
}

// TestTrustDomainParser_Coverage_RejectScheme tests rejection of trust domains with scheme
func TestTrustDomainParser_Coverage_RejectScheme(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	parser := inmemory.NewInMemoryTrustDomainParser()

	tests := []struct {
		name  string
		input string
	}{
		{"spiffe scheme", "spiffe://example.org"},
		{"https scheme", "https://example.org"},
		{"http scheme", "http://example.org"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Act
			td, err := parser.FromString(ctx, tt.input)

			// Assert
			assert.Error(t, err)
			assert.Nil(t, td)
			assert.Contains(t, err.Error(), "scheme")
		})
	}
}

// TestTrustDomainParser_Coverage_RejectPath tests rejection of trust domains with path
func TestTrustDomainParser_Coverage_RejectPath(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	parser := inmemory.NewInMemoryTrustDomainParser()

	tests := []struct {
		name  string
		input string
	}{
		{"path with slash", "example.org/abc"},
		{"multiple path segments", "example.org/abc/def"},
		{"trailing slash", "example.org/"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Act
			td, err := parser.FromString(ctx, tt.input)

			// Assert
			assert.Error(t, err)
			assert.Nil(t, td)
			assert.Contains(t, err.Error(), "path")
		})
	}
}

// TestTrustDomainParser_Coverage_RejectDots tests rejection of malformed dot usage
func TestTrustDomainParser_Coverage_RejectDots(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	parser := inmemory.NewInMemoryTrustDomainParser()

	tests := []struct {
		name  string
		input string
		match string
	}{
		{"leading dot", ".example.org", "start with dot"},
		{"trailing dot", "example.org.", "end with dot"},
		{"consecutive dots", "example..org", "consecutive dots"},
		{"multiple consecutive dots", "example...org", "consecutive dots"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Act
			td, err := parser.FromString(ctx, tt.input)

			// Assert
			assert.Error(t, err)
			assert.Nil(t, td)
			assert.Contains(t, err.Error(), tt.match)
		})
	}
}

// TestTrustDomainParser_Coverage_RejectIllegalCharacters tests rejection of illegal characters
func TestTrustDomainParser_Coverage_RejectIllegalCharacters(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	parser := inmemory.NewInMemoryTrustDomainParser()

	tests := []struct {
		name  string
		input string
	}{
		{"underscore", "exa_mple.org"},
		{"space", "exa mple.org"},
		{"uppercase (should be normalized, not rejected)", "EXAMPLE.ORG"}, // This should actually pass
		{"exclamation", "example!.org"},
		{"asterisk", "*.example.org"},
		{"at sign", "user@example.org"},
		{"colon", "example:8080.org"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Act
			td, err := parser.FromString(ctx, tt.input)

			// Assert - Special case for uppercase (should normalize, not reject)
			if tt.input == "EXAMPLE.ORG" {
				require.NoError(t, err)
				assert.NotNil(t, td)
				assert.Equal(t, "example.org", td.String())
			} else {
				assert.Error(t, err)
				assert.Nil(t, td)
				assert.Contains(t, err.Error(), "illegal character")
			}
		})
	}
}

// TestTrustDomainParser_Coverage_RejectBadLabels tests rejection of invalid labels
func TestTrustDomainParser_Coverage_RejectBadLabels(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	parser := inmemory.NewInMemoryTrustDomainParser()

	tests := []struct {
		name  string
		input string
		match string
	}{
		{"leading hyphen", "-ex.org", "start or end with hyphen"},
		{"trailing hyphen", "ex-.org", "start or end with hyphen"},
		{"both leading and trailing hyphen", "-ex-.org", "start or end with hyphen"},
		{"leading hyphen in subdomain", "example.-sub.org", "start or end with hyphen"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Act
			td, err := parser.FromString(ctx, tt.input)

			// Assert
			assert.Error(t, err)
			assert.Nil(t, td)
			assert.Contains(t, err.Error(), tt.match)
		})
	}
}

// TestTrustDomainParser_Coverage_RejectEmpty tests rejection of empty input
func TestTrustDomainParser_Coverage_RejectEmpty(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	parser := inmemory.NewInMemoryTrustDomainParser()

	tests := []struct {
		name  string
		input string
	}{
		{"empty string", ""},
		{"whitespace only", "   "},
		{"tab only", "\t"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Act
			td, err := parser.FromString(ctx, tt.input)

			// Assert
			assert.Error(t, err)
			assert.Nil(t, td)
			assert.Contains(t, err.Error(), "empty")
		})
	}
}

// TestTrustDomainParser_Coverage_LongLabel tests label length limits
func TestTrustDomainParser_Coverage_LongLabel(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	parser := inmemory.NewInMemoryTrustDomainParser()

	tests := []struct {
		name      string
		labelLen  int
		wantError bool
	}{
		{"exactly 63 chars (valid)", 63, false},
		{"64 chars (invalid)", 64, true},
		{"100 chars (invalid)", 100, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Arrange - Create label of specified length
			label := strings.Repeat("a", tt.labelLen)
			input := label + ".example.org"

			// Act
			td, err := parser.FromString(ctx, input)

			// Assert
			if tt.wantError {
				assert.Error(t, err)
				assert.Nil(t, td)
				assert.Contains(t, err.Error(), "exceeds 63 characters")
			} else {
				require.NoError(t, err)
				assert.NotNil(t, td)
			}
		})
	}
}

// TestTrustDomainParser_Coverage_TotalLength tests total length limits
func TestTrustDomainParser_Coverage_TotalLength(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	parser := inmemory.NewInMemoryTrustDomainParser()

	tests := []struct {
		name      string
		totalLen  int
		wantError bool
	}{
		{"exactly 253 chars (valid)", 253, false},
		{"254 chars (invalid)", 254, true},
		{"300 chars (invalid)", 300, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Arrange - Create domain of specified total length
			// Use pattern: "aaa.bbb.ccc..." to reach target length
			// Each segment is 3 chars + 1 dot = 4 chars per segment
			var input string
			if tt.totalLen <= 63 {
				// Single label
				input = strings.Repeat("a", tt.totalLen)
			} else {
				// Multiple 63-char labels separated by dots
				numLabels := (tt.totalLen + 63) / 64 // Rough estimate
				labels := make([]string, numLabels)
				for i := range labels {
					labels[i] = strings.Repeat("a", 63)
				}
				input = strings.Join(labels, ".")

				// Trim to exact length
				if len(input) > tt.totalLen {
					input = input[:tt.totalLen]
					// Ensure no trailing dot
					input = strings.TrimSuffix(input, ".")
				} else if len(input) < tt.totalLen {
					// Pad the last label
					input += strings.Repeat("a", tt.totalLen-len(input))
				}
			}

			// Act
			td, err := parser.FromString(ctx, input)

			// Assert
			if tt.wantError {
				assert.Error(t, err)
				assert.Nil(t, td)
				assert.Contains(t, err.Error(), "exceeds 253 characters")
			} else {
				require.NoError(t, err)
				assert.NotNil(t, td)
			}
		})
	}
}

// TestTrustDomainParser_Coverage_Normalization tests case normalization
func TestTrustDomainParser_Coverage_Normalization(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	parser := inmemory.NewInMemoryTrustDomainParser()

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"all uppercase", "EXAMPLE.ORG", "example.org"},
		{"mixed case", "Example.Org", "example.org"},
		{"already lowercase", "example.org", "example.org"},
		{"with whitespace", "  Example.Org  ", "example.org"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Act
			td, err := parser.FromString(ctx, tt.input)

			// Assert
			require.NoError(t, err)
			assert.NotNil(t, td)
			assert.Equal(t, tt.expected, td.String())
		})
	}
}
