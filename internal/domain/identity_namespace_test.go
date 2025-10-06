package domain_test

import (
	"testing"

	"github.com/pocket/hexagon/spire/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewIdentityNamespaceFromComponents(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		trustDomain string
		path        string
		wantURI     string
		wantPath    string
	}{
		{
			name:        "valid identity namespace with path",
			trustDomain: "example.org",
			path:        "/workload/server",
			wantURI:     "spiffe://example.org/workload/server",
			wantPath:    "/workload/server",
		},
		{
			name:        "identity namespace with empty path defaults to /",
			trustDomain: "example.org",
			path:        "",
			wantURI:     "spiffe://example.org/",
			wantPath:    "/",
		},
		{
			name:        "identity namespace with root path",
			trustDomain: "example.org",
			path:        "/",
			wantURI:     "spiffe://example.org/",
			wantPath:    "/",
		},
		{
			name:        "identity namespace with nested path",
			trustDomain: "prod.example.org",
			path:        "/ns/prod/svc/api",
			wantURI:     "spiffe://prod.example.org/ns/prod/svc/api",
			wantPath:    "/ns/prod/svc/api",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			td := domain.NewTrustDomainFromName(tt.trustDomain)

			// Act
			id := domain.NewIdentityNamespaceFromComponents(td, tt.path)

			// Assert
			require.NotNil(t, id)
			assert.Equal(t, tt.wantURI, id.String())
			assert.Equal(t, tt.wantPath, id.Path())
			assert.Equal(t, td, id.TrustDomain())
		})
	}
}

func TestIdentityNamespace_Equals(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		id1   *domain.IdentityNamespace
		id2   *domain.IdentityNamespace
		equal bool
	}{
		{
			name:  "equal identity namespaces",
			id1:   domain.NewIdentityNamespaceFromComponents(domain.NewTrustDomainFromName("example.org"), "/workload"),
			id2:   domain.NewIdentityNamespaceFromComponents(domain.NewTrustDomainFromName("example.org"), "/workload"),
			equal: true,
		},
		{
			name:  "different paths",
			id1:   domain.NewIdentityNamespaceFromComponents(domain.NewTrustDomainFromName("example.org"), "/server"),
			id2:   domain.NewIdentityNamespaceFromComponents(domain.NewTrustDomainFromName("example.org"), "/client"),
			equal: false,
		},
		{
			name:  "different trust domains",
			id1:   domain.NewIdentityNamespaceFromComponents(domain.NewTrustDomainFromName("example.org"), "/workload"),
			id2:   domain.NewIdentityNamespaceFromComponents(domain.NewTrustDomainFromName("other.org"), "/workload"),
			equal: false,
		},
		{
			name:  "nil comparison returns false",
			id1:   domain.NewIdentityNamespaceFromComponents(domain.NewTrustDomainFromName("example.org"), "/workload"),
			id2:   nil,
			equal: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Act
			result := tt.id1.Equals(tt.id2)

			// Assert
			assert.Equal(t, tt.equal, result)
		})
	}
}

func TestIdentityNamespace_IsInTrustDomain(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		id          *domain.IdentityNamespace
		trustDomain *domain.TrustDomain
		inDomain    bool
	}{
		{
			name:        "identity in same trust domain",
			id:          domain.NewIdentityNamespaceFromComponents(domain.NewTrustDomainFromName("example.org"), "/workload"),
			trustDomain: domain.NewTrustDomainFromName("example.org"),
			inDomain:    true,
		},
		{
			name:        "identity in different trust domain",
			id:          domain.NewIdentityNamespaceFromComponents(domain.NewTrustDomainFromName("example.org"), "/workload"),
			trustDomain: domain.NewTrustDomainFromName("other.org"),
			inDomain:    false,
		},
		{
			name:        "identity with subdomain trust domain",
			id:          domain.NewIdentityNamespaceFromComponents(domain.NewTrustDomainFromName("prod.example.org"), "/svc"),
			trustDomain: domain.NewTrustDomainFromName("example.org"),
			inDomain:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Act
			result := tt.id.IsInTrustDomain(tt.trustDomain)

			// Assert
			assert.Equal(t, tt.inDomain, result)
		})
	}
}

func TestIdentityNamespace_String(t *testing.T) {
	t.Parallel()

	// Arrange
	td := domain.NewTrustDomainFromName("example.org")
	id := domain.NewIdentityNamespaceFromComponents(td, "/workload/server")

	// Act
	str := id.String()

	// Assert
	assert.Equal(t, "spiffe://example.org/workload/server", str)
}

func TestIdentityNamespace_TrustDomain(t *testing.T) {
	t.Parallel()

	// Arrange
	td := domain.NewTrustDomainFromName("example.org")
	id := domain.NewIdentityNamespaceFromComponents(td, "/workload")

	// Act
	result := id.TrustDomain()

	// Assert
	assert.Equal(t, td, result)
}

func TestIdentityNamespace_Path(t *testing.T) {
	t.Parallel()

	// Arrange
	td := domain.NewTrustDomainFromName("example.org")
	id := domain.NewIdentityNamespaceFromComponents(td, "/workload/server")

	// Act
	result := id.Path()

	// Assert
	assert.Equal(t, "/workload/server", result)
}
