package domain_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pocket/hexagon/spire/internal/domain"
)

// TestIdentityCredential_IsZero tests the IsZero method for detecting uninitialized values
func TestIdentityCredential_IsZero(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		setup    func() *domain.IdentityCredential
		wantZero bool
	}{
		{
			name: "nil instance is zero",
			setup: func() *domain.IdentityCredential {
				return nil
			},
			wantZero: true,
		},
		{
			name: "valid instance is not zero",
			setup: func() *domain.IdentityCredential {
				td := domain.NewTrustDomainFromName("example.org")
				return domain.NewIdentityCredentialFromComponents(td, "/workload")
			},
			wantZero: false,
		},
		{
			name: "root identity is not zero",
			setup: func() *domain.IdentityCredential {
				td := domain.NewTrustDomainFromName("example.org")
				return domain.NewIdentityCredentialFromComponents(td, "/")
			},
			wantZero: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Act
			id := tt.setup()
			result := id.IsZero()

			// Assert
			assert.Equal(t, tt.wantZero, result)
		})
	}
}

// TestIdentityCredential_Key tests the Key method for use in maps/sets
func TestIdentityCredential_Key(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		td      string
		path    string
		wantKey string
	}{
		{
			name:    "simple workload",
			td:      "example.org",
			path:    "/workload",
			wantKey: "spiffe://example.org/workload",
		},
		{
			name:    "root identity",
			td:      "example.org",
			path:    "/",
			wantKey: "spiffe://example.org/",
		},
		{
			name:    "nested path",
			td:      "prod.example.org",
			path:    "/ns/prod/svc/api",
			wantKey: "spiffe://prod.example.org/ns/prod/svc/api",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			td := domain.NewTrustDomainFromName(tt.td)
			id := domain.NewIdentityCredentialFromComponents(td, tt.path)

			// Act
			key := id.Key()

			// Assert
			assert.Equal(t, tt.wantKey, key)
			assert.Equal(t, id.String(), key, "Key() should match String()")
		})
	}
}

// TestIdentityCredential_Key_MapUsage tests using Key() in a map
func TestIdentityCredential_Key_MapUsage(t *testing.T) {
	t.Parallel()

	// Arrange
	td := domain.NewTrustDomainFromName("example.org")
	id1 := domain.NewIdentityCredentialFromComponents(td, "/workload")
	id2 := domain.NewIdentityCredentialFromComponents(td, "/workload") // Same as id1
	id3 := domain.NewIdentityCredentialFromComponents(td, "/service")  // Different

	cache := make(map[string]string)

	// Act
	cache[id1.Key()] = "data1"
	cache[id3.Key()] = "data3"

	// Assert
	assert.Equal(t, "data1", cache[id2.Key()], "Same identity should retrieve same data")
	assert.Equal(t, "data3", cache[id3.Key()])
	assert.Len(t, cache, 2, "Should have 2 unique keys")
}

// TestIdentityCredential_PathNormalization tests that paths are normalized correctly
func TestIdentityCredential_PathNormalization(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		inputPath    string
		expectedPath string
		expectedURI  string
	}{
		{
			name:         "empty path becomes root",
			inputPath:    "",
			expectedPath: "/",
			expectedURI:  "spiffe://example.org/",
		},
		{
			name:         "path without leading slash gets one",
			inputPath:    "workload",
			expectedPath: "/workload",
			expectedURI:  "spiffe://example.org/workload",
		},
		{
			name:         "already normalized path unchanged",
			inputPath:    "/workload/server",
			expectedPath: "/workload/server",
			expectedURI:  "spiffe://example.org/workload/server",
		},
		{
			name:         "colons allowed in path segments",
			inputPath:    "/db:rw/user",
			expectedPath: "/db:rw/user",
			expectedURI:  "spiffe://example.org/db:rw/user",
		},
		{
			name:         "multiple colons in path",
			inputPath:    "/service:v1.2.3:prod",
			expectedPath: "/service:v1.2.3:prod",
			expectedURI:  "spiffe://example.org/service:v1.2.3:prod",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			td := domain.NewTrustDomainFromName("example.org")

			// Act
			id := domain.NewIdentityCredentialFromComponents(td, tt.inputPath)

			// Assert
			assert.Equal(t, tt.expectedPath, id.Path(), "Path should be normalized")
			assert.Equal(t, tt.expectedURI, id.String(), "URI should contain normalized path")
		})
	}
}

// TestIdentityCredential_PathValidation_Strict tests that strict validation rejects invalid paths
func TestIdentityCredential_PathValidation_Strict(t *testing.T) {
	t.Parallel()

	td := domain.NewTrustDomainFromName("example.org")

	// Test that invalid paths panic (strict validation)
	invalidPaths := []struct {
		path   string
		reason string
	}{
		{"//foo//bar", "consecutive slashes"},
		{"/foo/bar/", "trailing slash"},
		{"/foo bar", "internal whitespace (space)"},
		{"/foo\tbar", "internal whitespace (tab)"},
		{"/foo\nbar", "internal whitespace (newline)"},
		{"/foo\rbar", "internal whitespace (carriage return)"},
		{"/foo\fbar", "internal whitespace (form feed)"},
		{"/foo\u00A0bar", "internal whitespace (non-breaking space U+00A0)"},
		{"/foo\u2003bar", "internal whitespace (em space U+2003)"},
		{" /foo", "leading whitespace (space)"},
		{"\t/foo", "leading whitespace (tab)"},
		{"/foo ", "trailing whitespace (space)"},
		{"/foo\n", "trailing whitespace (newline)"},
		{"/./foo", "dot segment"},
		{"/../foo", "dotdot segment"},
	}

	for _, tc := range invalidPaths {
		tc := tc // capture range variable
		t.Run(tc.reason, func(t *testing.T) {
			t.Parallel()
			assert.Panics(t, func() {
				domain.NewIdentityCredentialFromComponents(td, tc.path)
			}, "Should panic for path with %s: %q", tc.reason, tc.path)
		})
	}

	// Test that valid paths work
	validPaths := []struct {
		path     string
		expected string
	}{
		{"/foo/bar", "/foo/bar"},
		{"foo/bar", "/foo/bar"}, // Convenience: adds leading slash
		{"/", "/"},
		{"", "/"},
	}

	for _, tc := range validPaths {
		tc := tc
		t.Run(tc.path, func(t *testing.T) {
			t.Parallel()
			id := domain.NewIdentityCredentialFromComponents(td, tc.path)
			assert.Equal(t, tc.expected, id.Path())
		})
	}
}

// TestIdentityCredential_MarshalJSON tests JSON marshaling
func TestIdentityCredential_MarshalJSON(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		setup      func() *domain.IdentityCredential
		wantJSON   string
		wantErr    bool
		errMessage string
	}{
		{
			name: "valid identity marshals to URI string",
			setup: func() *domain.IdentityCredential {
				td := domain.NewTrustDomainFromName("example.org")
				return domain.NewIdentityCredentialFromComponents(td, "/workload")
			},
			wantJSON: `"spiffe://example.org/workload"`,
			wantErr:  false,
		},
		{
			name: "root identity marshals correctly",
			setup: func() *domain.IdentityCredential {
				td := domain.NewTrustDomainFromName("example.org")
				return domain.NewIdentityCredentialFromComponents(td, "/")
			},
			wantJSON: `"spiffe://example.org/"`,
			wantErr:  false,
		},
		{
			name: "nil identity marshals to null",
			setup: func() *domain.IdentityCredential {
				return nil
			},
			wantJSON: `null`,
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			id := tt.setup()

			// Act
			data, err := json.Marshal(id)

			// Assert
			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMessage)
			} else {
				require.NoError(t, err)
				assert.JSONEq(t, tt.wantJSON, string(data))
			}
		})
	}
}

// TestIdentityCredential_MarshalJSON_InStruct tests JSON marshaling within a struct
func TestIdentityCredential_MarshalJSON_InStruct(t *testing.T) {
	t.Parallel()

	// Arrange
	type Workload struct {
		Name     string                     `json:"name"`
		Identity *domain.IdentityCredential `json:"identity"`
	}

	td := domain.NewTrustDomainFromName("example.org")
	id := domain.NewIdentityCredentialFromComponents(td, "/workload/server")

	workload := Workload{
		Name:     "my-server",
		Identity: id,
	}

	// Act
	data, err := json.Marshal(workload)

	// Assert
	require.NoError(t, err)
	expected := `{"name":"my-server","identity":"spiffe://example.org/workload/server"}`
	assert.JSONEq(t, expected, string(data))
}

// TestIdentityCredential_UnmarshalJSON tests that unmarshaling returns ErrNotSupported
func TestIdentityCredential_UnmarshalJSON(t *testing.T) {
	t.Parallel()

	// Arrange
	jsonData := []byte(`"spiffe://example.org/workload"`)
	var id domain.IdentityCredential

	// Act
	err := json.Unmarshal(jsonData, &id)

	// Assert
	require.Error(t, err)
	assert.ErrorIs(t, err, domain.ErrNotSupported)
	assert.Contains(t, err.Error(), "unmarshaling requires IdentityCredentialParser adapter")
}

// TestIdentityCredential_MarshalText tests text marshaling
func TestIdentityCredential_MarshalText(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		setup      func() *domain.IdentityCredential
		wantText   string
		wantErr    bool
		errMessage string
	}{
		{
			name: "valid identity marshals to URI bytes",
			setup: func() *domain.IdentityCredential {
				td := domain.NewTrustDomainFromName("example.org")
				return domain.NewIdentityCredentialFromComponents(td, "/workload")
			},
			wantText: "spiffe://example.org/workload",
			wantErr:  false,
		},
		{
			name: "root identity marshals correctly",
			setup: func() *domain.IdentityCredential {
				td := domain.NewTrustDomainFromName("example.org")
				return domain.NewIdentityCredentialFromComponents(td, "/")
			},
			wantText: "spiffe://example.org/",
			wantErr:  false,
		},
		{
			name: "nil identity returns error",
			setup: func() *domain.IdentityCredential {
				return nil
			},
			wantErr:    true,
			errMessage: "invalid identity credential",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			id := tt.setup()

			// Act
			text, err := id.MarshalText()

			// Assert
			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMessage)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.wantText, string(text))
			}
		})
	}
}

// TestIdentityCredential_UnmarshalText tests that unmarshaling returns ErrNotSupported
func TestIdentityCredential_UnmarshalText(t *testing.T) {
	t.Parallel()

	// Arrange
	textData := []byte("spiffe://example.org/workload")
	var id domain.IdentityCredential

	// Act
	err := id.UnmarshalText(textData)

	// Assert
	require.Error(t, err)
	assert.ErrorIs(t, err, domain.ErrNotSupported)
	assert.Contains(t, err.Error(), "unmarshaling requires IdentityCredentialParser adapter")
}

// TestIdentityCredential_NilTrustDomain_Panics tests that nil trust domain panics
func TestIdentityCredential_NilTrustDomain_Panics(t *testing.T) {
	t.Parallel()

	// Assert
	assert.Panics(t, func() {
		domain.NewIdentityCredentialFromComponents(nil, "/workload")
	}, "Should panic when trust domain is nil")
}

// TestIdentityCredential_DotSegments_Panics tests that dot and dotdot segments panic
func TestIdentityCredential_DotSegments_Panics(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		inputPath string
	}{
		{
			name:      "single dot segment",
			inputPath: "/.",
		},
		{
			name:      "double dot segment",
			inputPath: "/..",
		},
		{
			name:      "dot in middle of path",
			inputPath: "/a/./b",
		},
		{
			name:      "dotdot in middle of path",
			inputPath: "/a/../b",
		},
		{
			name:      "multiple dot segments",
			inputPath: "/./foo/./bar",
		},
		{
			name:      "dotdot at end",
			inputPath: "/foo/bar/..",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			td := domain.NewTrustDomainFromName("example.org")

			// Assert: Should panic on dot/dotdot segments
			assert.Panics(t, func() {
				domain.NewIdentityCredentialFromComponents(td, tt.inputPath)
			}, "Should panic when path contains dot or dotdot segments")
		})
	}
}
