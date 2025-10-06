package inmemory_test

import (
	"context"
	"crypto/x509"
	"encoding/pem"
	"regexp"
	"testing"
	"time"

	"github.com/pocket/hexagon/spire/internal/adapters/outbound/inmemory"
	"github.com/pocket/hexagon/spire/internal/adapters/outbound/inmemory/attestor"
	"github.com/pocket/hexagon/spire/internal/domain"
	"github.com/pocket/hexagon/spire/internal/ports"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Coverage tests for agent.go error paths

// TestAgent_FetchIdentityDocument_NoSelectorsRegistered tests when attestor returns no selectors
func TestAgent_FetchIdentityDocument_NoSelectorsRegistered(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	tdParser := inmemory.NewInMemoryTrustDomainParser()
	docProvider := inmemory.NewInMemoryIdentityDocumentProvider()

	// Create server and registry
	server, err := inmemory.NewInMemoryServer(ctx, "example.org", tdParser, docProvider)
	require.NoError(t, err)

	registry := inmemory.NewInMemoryRegistry()
	registry.Seal()

	// Create attestor WITHOUT registering any UIDs
	workloadAttestor := attestor.NewUnixWorkloadAttestor()
	parser := inmemory.NewInMemoryIdentityNamespaceParser()

	agent, err := inmemory.NewInMemoryAgent(
		ctx,
		"spiffe://example.org/agent",
		server,
		registry,
		workloadAttestor,
		parser,
		docProvider,
	)
	require.NoError(t, err)

	// Act - Try to fetch with unregistered UID
	_, err = agent.FetchIdentityDocument(ctx, ports.ProcessIdentity{UID: 99999, GID: 99999})

	// Assert - Should fail because UID not registered in attestor
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "attestation")
}

// TestAgent_FetchIdentityDocument_NoMatchingMapper tests when selectors don't match registry
func TestAgent_FetchIdentityDocument_NoMatchingMapper(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	tdParser := inmemory.NewInMemoryTrustDomainParser()
	docProvider := inmemory.NewInMemoryIdentityDocumentProvider()

	// Create server and empty registry (no mappers)
	server, err := inmemory.NewInMemoryServer(ctx, "example.org", tdParser, docProvider)
	require.NoError(t, err)

	registry := inmemory.NewInMemoryRegistry()
	registry.Seal() // Seal with no mappers

	// Create attestor and register a UID
	workloadAttestor := attestor.NewUnixWorkloadAttestor()
	workloadAttestor.RegisterUID(1000, "unix:user:testuser")

	parser := inmemory.NewInMemoryIdentityNamespaceParser()

	agent, err := inmemory.NewInMemoryAgent(
		ctx,
		"spiffe://example.org/agent",
		server,
		registry,
		workloadAttestor,
		parser,
		docProvider,
	)
	require.NoError(t, err)

	// Act - Fetch with UID that has selectors but no matching mapper in registry
	_, err = agent.FetchIdentityDocument(ctx, ports.ProcessIdentity{UID: 1000, GID: 1000})

	// Assert - Should fail with "no mapper found" error
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no identity mapper found")
}

// Coverage tests for trust_bundle_provider.go (currently 0%)

// TestTrustBundleProvider_GetBundle tests bundle retrieval
func TestTrustBundleProvider_GetBundle(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	tdParser := inmemory.NewInMemoryTrustDomainParser()
	docProvider := inmemory.NewInMemoryIdentityDocumentProvider()

	// Create server with CA
	server, err := inmemory.NewInMemoryServer(ctx, "example.org", tdParser, docProvider)
	require.NoError(t, err)

	// Create trust bundle provider with server's CA
	caCerts := []*x509.Certificate{server.GetCA()}
	provider := inmemory.NewInMemoryTrustBundleProvider(caCerts)

	// Act - Get bundle for trust domain
	td := domain.NewTrustDomainFromName("example.org")
	bundle, err := provider.GetBundle(ctx, td)

	// Assert
	require.NoError(t, err)
	assert.NotNil(t, bundle)
	assert.NotEmpty(t, bundle)
	assert.Contains(t, string(bundle), "BEGIN CERTIFICATE")
}

// TestTrustBundleProvider_GetBundleForIdentity tests bundle for specific identity
func TestTrustBundleProvider_GetBundleForIdentity(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	tdParser := inmemory.NewInMemoryTrustDomainParser()
	docProvider := inmemory.NewInMemoryIdentityDocumentProvider()

	server, err := inmemory.NewInMemoryServer(ctx, "example.org", tdParser, docProvider)
	require.NoError(t, err)

	caCerts := []*x509.Certificate{server.GetCA()}
	provider := inmemory.NewInMemoryTrustBundleProvider(caCerts)

	// Create identity namespace
	td := domain.NewTrustDomainFromName("example.org")
	namespace := domain.NewIdentityNamespaceFromComponents(td, "/workload")

	// Act
	bundle, err := provider.GetBundleForIdentity(ctx, namespace)

	// Assert
	require.NoError(t, err)
	assert.NotNil(t, bundle)
	assert.Contains(t, string(bundle), "BEGIN CERTIFICATE")
}

// TestTrustBundleProvider_EmptyCAs tests error case with no CAs
func TestTrustBundleProvider_EmptyCAs(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	// Create provider with empty CA list
	provider := inmemory.NewInMemoryTrustBundleProvider(nil)

	td := domain.NewTrustDomainFromName("example.org")

	// Act
	bundle, err := provider.GetBundle(ctx, td)

	// Assert - Should return error for empty CAs
	assert.Error(t, err)
	assert.Nil(t, bundle)
	assert.Contains(t, err.Error(), "trust bundle not found")
}

// TestTrustBundleProvider_GetBundle_NilTrustDomain tests nil trust domain error
func TestTrustBundleProvider_GetBundle_NilTrustDomain(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	tdParser := inmemory.NewInMemoryTrustDomainParser()
	docProvider := inmemory.NewInMemoryIdentityDocumentProvider()

	server, err := inmemory.NewInMemoryServer(ctx, "example.org", tdParser, docProvider)
	require.NoError(t, err)

	caCerts := []*x509.Certificate{server.GetCA()}
	provider := inmemory.NewInMemoryTrustBundleProvider(caCerts)

	// Act - Pass nil trust domain
	bundle, err := provider.GetBundle(ctx, nil)

	// Assert - Should return error
	assert.Error(t, err)
	assert.Nil(t, bundle)
	assert.Contains(t, err.Error(), "trust domain cannot be nil")
}

// Coverage tests for trust_domain_parser.go gaps

// TestTrustDomainParser_FromString_EmptyString tests empty input
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

// TestIdentityNamespaceParser_ParseFromPath tests path-based parsing
func TestIdentityNamespaceParser_ParseFromPath(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	parser := inmemory.NewInMemoryIdentityNamespaceParser()

	tests := []struct {
		name        string
		trustDomain *domain.TrustDomain
		path        string
		expectError bool
		expectedURI string
	}{
		{
			name:        "valid workload path",
			trustDomain: domain.NewTrustDomainFromName("example.org"),
			path:        "/workload",
			expectError: false,
			expectedURI: "spiffe://example.org/workload",
		},
		{
			name:        "root path",
			trustDomain: domain.NewTrustDomainFromName("example.org"),
			path:        "/",
			expectError: false,
			expectedURI: "spiffe://example.org/",
		},
		{
			name:        "nested path",
			trustDomain: domain.NewTrustDomainFromName("prod.example.org"),
			path:        "/ns/prod/svc",
			expectError: false,
			expectedURI: "spiffe://prod.example.org/ns/prod/svc",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Act
			namespace, err := parser.ParseFromPath(ctx, tt.trustDomain, tt.path)

			// Assert
			if tt.expectError {
				assert.Error(t, err)
				assert.Nil(t, namespace)
			} else {
				require.NoError(t, err)
				assert.NotNil(t, namespace)
				assert.Equal(t, tt.expectedURI, namespace.String())
			}
		})
	}
}

// Coverage tests for validator.go

// TestIdentityDocumentValidator_Validate_NilDocument tests nil document error
func TestIdentityDocumentValidator_Validate_NilDocument(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	validator := inmemory.NewIdentityDocumentValidator(nil)

	td := domain.NewTrustDomainFromName("example.org")
	namespace := domain.NewIdentityNamespaceFromComponents(td, "/workload")

	// Act - Pass nil document
	err := validator.Validate(ctx, nil, namespace)

	// Assert
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "identity document cannot be nil")
}

// TestIdentityDocumentValidator_Validate_NilExpectedID tests nil expected ID error
func TestIdentityDocumentValidator_Validate_NilExpectedID(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	tdParser := inmemory.NewInMemoryTrustDomainParser()
	docProvider := inmemory.NewInMemoryIdentityDocumentProvider()

	server, err := inmemory.NewInMemoryServer(ctx, "example.org", tdParser, docProvider)
	require.NoError(t, err)

	validator := inmemory.NewIdentityDocumentValidator(nil)

	// Create a valid document
	td := domain.NewTrustDomainFromName("example.org")
	namespace := domain.NewIdentityNamespaceFromComponents(td, "/workload")
	doc := domain.NewIdentityDocumentFromComponents(
		namespace,
		domain.IdentityDocumentTypeX509,
		server.GetCA(),
		nil, nil,
		time.Unix(2000000000, 0), // Future expiry
	)

	// Act - Pass nil expected ID
	err = validator.Validate(ctx, doc, nil)

	// Assert
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "expected identity namespace cannot be nil")
}

// TestIdentityDocumentValidator_Validate_ExpiredDocument tests expired document
func TestIdentityDocumentValidator_Validate_ExpiredDocument(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	validator := inmemory.NewIdentityDocumentValidator(nil)

	td := domain.NewTrustDomainFromName("example.org")
	namespace := domain.NewIdentityNamespaceFromComponents(td, "/workload")

	// Create expired document
	doc := domain.NewIdentityDocumentFromComponents(
		namespace,
		domain.IdentityDocumentTypeX509,
		nil, nil, nil,
		time.Unix(1000000000, 0), // Past expiry
	)

	// Act
	err := validator.Validate(ctx, doc, namespace)

	// Assert
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "expired or not yet valid")
}

// TestIdentityDocumentValidator_Validate_MismatchedNamespace tests namespace mismatch
func TestIdentityDocumentValidator_Validate_MismatchedNamespace(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	validator := inmemory.NewIdentityDocumentValidator(nil)

	td := domain.NewTrustDomainFromName("example.org")
	namespace1 := domain.NewIdentityNamespaceFromComponents(td, "/workload1")
	namespace2 := domain.NewIdentityNamespaceFromComponents(td, "/workload2")

	// Create document with namespace1
	doc := domain.NewIdentityDocumentFromComponents(
		namespace1,
		domain.IdentityDocumentTypeX509,
		nil, nil, nil,
		time.Unix(2000000000, 0), // Future expiry
	)

	// Act - Validate against different namespace2
	err := validator.Validate(ctx, doc, namespace2)

	// Assert
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "expected")
	assert.Contains(t, err.Error(), "/workload2")
}

// TestIdentityDocumentValidator_Validate_Success tests successful validation
func TestIdentityDocumentValidator_Validate_Success(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	tdParser := inmemory.NewInMemoryTrustDomainParser()
	docProvider := inmemory.NewInMemoryIdentityDocumentProvider()

	server, err := inmemory.NewInMemoryServer(ctx, "example.org", tdParser, docProvider)
	require.NoError(t, err)

	caCerts := []*x509.Certificate{server.GetCA()}
	bundleProvider := inmemory.NewInMemoryTrustBundleProvider(caCerts)
	validator := inmemory.NewIdentityDocumentValidator(bundleProvider)

	td := domain.NewTrustDomainFromName("example.org")
	namespace := domain.NewIdentityNamespaceFromComponents(td, "/workload")

	// Create valid document
	doc := domain.NewIdentityDocumentFromComponents(
		namespace,
		domain.IdentityDocumentTypeX509,
		nil, nil, nil,
		time.Unix(2000000000, 0), // Future expiry
	)

	// Act
	err = validator.Validate(ctx, doc, namespace)

	// Assert - Should succeed
	assert.NoError(t, err)
}

// Additional coverage tests to push over 70%

// TestTrustBundleProvider_GetBundleForIdentity_NilNamespace tests nil identity namespace error
func TestTrustBundleProvider_GetBundleForIdentity_NilNamespace(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	tdParser := inmemory.NewInMemoryTrustDomainParser()
	docProvider := inmemory.NewInMemoryIdentityDocumentProvider()

	server, err := inmemory.NewInMemoryServer(ctx, "example.org", tdParser, docProvider)
	require.NoError(t, err)

	caCerts := []*x509.Certificate{server.GetCA()}
	provider := inmemory.NewInMemoryTrustBundleProvider(caCerts)

	// Act - Pass nil identity namespace
	bundle, err := provider.GetBundleForIdentity(ctx, nil)

	// Assert - Should return error
	assert.Error(t, err)
	assert.Nil(t, bundle)
	assert.Contains(t, err.Error(), "identity namespace cannot be nil")
}

// TestAgent_GetIdentity tests the GetIdentity method
func TestAgent_GetIdentity(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	tdParser := inmemory.NewInMemoryTrustDomainParser()
	docProvider := inmemory.NewInMemoryIdentityDocumentProvider()

	server, err := inmemory.NewInMemoryServer(ctx, "example.org", tdParser, docProvider)
	require.NoError(t, err)

	registry := inmemory.NewInMemoryRegistry()
	registry.Seal()

	workloadAttestor := attestor.NewUnixWorkloadAttestor()
	parser := inmemory.NewInMemoryIdentityNamespaceParser()

	agent, err := inmemory.NewInMemoryAgent(
		ctx,
		"spiffe://example.org/agent",
		server,
		registry,
		workloadAttestor,
		parser,
		docProvider,
	)
	require.NoError(t, err)

	// Act
	identity, err := agent.GetIdentity(ctx)

	// Assert
	require.NoError(t, err)
	require.NotNil(t, identity)
	assert.NotNil(t, identity.IdentityNamespace)
	assert.Equal(t, "spiffe://example.org/agent", identity.IdentityNamespace.String())
}

// TestIdentityNamespaceParser_ParseFromPath_ErrorCases tests error handling
func TestIdentityNamespaceParser_ParseFromPath_ErrorCases(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	parser := inmemory.NewInMemoryIdentityNamespaceParser()

	tests := []struct {
		name        string
		trustDomain *domain.TrustDomain
		path        string
		expectError bool
	}{
		{
			name:        "nil trust domain",
			trustDomain: nil,
			path:        "/workload",
			expectError: true,
		},
		{
			name:        "empty path becomes root",
			trustDomain: domain.NewTrustDomainFromName("example.org"),
			path:        "",
			expectError: false, // Empty path is normalized to "/"
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Act
			namespace, err := parser.ParseFromPath(ctx, tt.trustDomain, tt.path)

			// Assert
			if tt.expectError {
				assert.Error(t, err)
				assert.Nil(t, namespace)
			} else {
				require.NoError(t, err)
				assert.NotNil(t, namespace)
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
		name        string
		input       string
		expectError bool
		errorMsg    string
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
			if tt.expectError {
				assert.Error(t, err)
				assert.Nil(t, td)
				if tt.errorMsg != "" {
					assert.Contains(t, err.Error(), tt.errorMsg)
				}
			} else {
				require.NoError(t, err)
				assert.NotNil(t, td)
			}
		})
	}
}

// TestIdentityNamespaceParser_ParseFromString_InvalidURI tests error handling for invalid URIs
func TestIdentityNamespaceParser_ParseFromString_InvalidURI(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	parser := inmemory.NewInMemoryIdentityNamespaceParser()

	tests := []struct {
		name        string
		input       string
		expectError bool
		errorMsg    string
	}{
		{"empty string", "", true, "identity namespace cannot be empty"},
		{"missing scheme", "example.org/workload", true, ""},
		{"wrong scheme", "http://example.org/workload", true, "must use 'spiffe' scheme"},
		{"missing host/trust domain", "spiffe:///workload", true, "must contain a trust domain"},
		{"valid spiffe URI", "spiffe://example.org/workload", false, ""},
		{"missing path", "spiffe://example.org", false, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Act
			namespace, err := parser.ParseFromString(ctx, tt.input)

			// Assert
			if tt.expectError {
				assert.Error(t, err)
				assert.Nil(t, namespace)
				if tt.errorMsg != "" {
					assert.Contains(t, err.Error(), tt.errorMsg)
				}
			} else {
				require.NoError(t, err)
				assert.NotNil(t, namespace)
			}
		})
	}
}

// TestAgent_NewInMemoryAgent_ErrorPaths tests agent initialization error handling
func TestAgent_NewInMemoryAgent_ErrorPaths(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	tdParser := inmemory.NewInMemoryTrustDomainParser()
	docProvider := inmemory.NewInMemoryIdentityDocumentProvider()

	server, err := inmemory.NewInMemoryServer(ctx, "example.org", tdParser, docProvider)
	require.NoError(t, err)

	registry := inmemory.NewInMemoryRegistry()
	registry.Seal()

	workloadAttestor := attestor.NewUnixWorkloadAttestor()
	parser := inmemory.NewInMemoryIdentityNamespaceParser()

	tests := []struct {
		name            string
		identityURI     string
		expectError     bool
	}{
		{"invalid URI", "not-a-spiffe-uri", true},
		{"valid URI", "spiffe://example.org/agent", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Act
			agent, err := inmemory.NewInMemoryAgent(
				ctx,
				tt.identityURI,
				server,
				registry,
				workloadAttestor,
				parser,
				docProvider,
			)

			// Assert
			if tt.expectError {
				assert.Error(t, err)
				assert.Nil(t, agent)
			} else {
				require.NoError(t, err)
				assert.NotNil(t, agent)
			}
		})
	}
}

// TestServer_NewInMemoryServer_ErrorPaths tests server initialization error handling
func TestServer_NewInMemoryServer_ErrorPaths(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	tdParser := inmemory.NewInMemoryTrustDomainParser()
	docProvider := inmemory.NewInMemoryIdentityDocumentProvider()

	tests := []struct {
		name        string
		trustDomain string
		expectError bool
	}{
		{"valid trust domain", "example.org", false},
		{"empty trust domain", "", true},
		{"trust domain with scheme", "https://example.org", true},
		{"trust domain with path", "example.org/path", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Act
			server, err := inmemory.NewInMemoryServer(ctx, tt.trustDomain, tdParser, docProvider)

			// Assert
			if tt.expectError {
				assert.Error(t, err)
				assert.Nil(t, server)
			} else {
				require.NoError(t, err)
				assert.NotNil(t, server)
			}
		})
	}
}

// TestAgent_FetchIdentityDocument_InvalidSelector tests invalid selector format
func TestAgent_FetchIdentityDocument_InvalidSelector(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	tdParser := inmemory.NewInMemoryTrustDomainParser()
	docProvider := inmemory.NewInMemoryIdentityDocumentProvider()

	server, err := inmemory.NewInMemoryServer(ctx, "example.org", tdParser, docProvider)
	require.NoError(t, err)

	registry := inmemory.NewInMemoryRegistry()
	registry.Seal()

	// Create attestor that returns invalid selector format
	workloadAttestor := attestor.NewUnixWorkloadAttestor()
	workloadAttestor.RegisterUID(2000, "invalid-selector-no-colon")

	parser := inmemory.NewInMemoryIdentityNamespaceParser()

	agent, err := inmemory.NewInMemoryAgent(
		ctx,
		"spiffe://example.org/agent",
		server,
		registry,
		workloadAttestor,
		parser,
		docProvider,
	)
	require.NoError(t, err)

	// Act - Try to fetch with UID that has invalid selector
	_, err = agent.FetchIdentityDocument(ctx, ports.ProcessIdentity{UID: 2000, GID: 2000})

	// Assert - Should fail with selector parse error
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid selector")
}

// TestServer_IssueIdentity_NilNamespace tests nil namespace error
func TestServer_IssueIdentity_NilNamespace(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	tdParser := inmemory.NewInMemoryTrustDomainParser()
	docProvider := inmemory.NewInMemoryIdentityDocumentProvider()

	server, err := inmemory.NewInMemoryServer(ctx, "example.org", tdParser, docProvider)
	require.NoError(t, err)

	// Act - Pass nil namespace
	doc, err := server.IssueIdentity(ctx, nil)

	// Assert
	assert.Error(t, err)
	assert.Nil(t, doc)
	assert.Contains(t, err.Error(), "identity namespace cannot be nil")
}

// TestAgent_FetchIdentityDocument_FullErrorFlow tests the complete error flow
func TestAgent_FetchIdentityDocument_FullErrorFlow(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	tdParser := inmemory.NewInMemoryTrustDomainParser()
	docProvider := inmemory.NewInMemoryIdentityDocumentProvider()

	server, err := inmemory.NewInMemoryServer(ctx, "example.org", tdParser, docProvider)
	require.NoError(t, err)

	registry := inmemory.NewInMemoryRegistry()

	// Register a mapper
	td := domain.NewTrustDomainFromName("example.org")
	namespace := domain.NewIdentityNamespaceFromComponents(td, "/workload")
	selector, err := domain.ParseSelectorFromString("unix:uid:1000")
	require.NoError(t, err)

	selectorSet := domain.NewSelectorSet()
	selectorSet.Add(selector)
	mapper, err := domain.NewIdentityMapper(namespace, selectorSet)
	require.NoError(t, err)
	err = registry.Seed(ctx, mapper)
	require.NoError(t, err)
	registry.Seal()

	workloadAttestor := attestor.NewUnixWorkloadAttestor()
	workloadAttestor.RegisterUID(1000, "unix:uid:1000")

	parser := inmemory.NewInMemoryIdentityNamespaceParser()

	agent, err := inmemory.NewInMemoryAgent(
		ctx,
		"spiffe://example.org/agent",
		server,
		registry,
		workloadAttestor,
		parser,
		docProvider,
	)
	require.NoError(t, err)

	// Act - This should work end-to-end
	identity, err := agent.FetchIdentityDocument(ctx, ports.ProcessIdentity{UID: 1000, GID: 1000})

	// Assert - Should succeed
	require.NoError(t, err)
	assert.NotNil(t, identity)
	assert.Equal(t, "workload", identity.Name)
	assert.NotNil(t, identity.IdentityDocument)
}

// TestAgent_ExtractName_RootPath tests extractNameFromIdentityNamespace with root path
func TestAgent_ExtractName_RootPath(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	tdParser := inmemory.NewInMemoryTrustDomainParser()
	docProvider := inmemory.NewInMemoryIdentityDocumentProvider()

	server, err := inmemory.NewInMemoryServer(ctx, "example.org", tdParser, docProvider)
	require.NoError(t, err)

	registry := inmemory.NewInMemoryRegistry()

	// Register a mapper with root path "/"
	td := domain.NewTrustDomainFromName("example.org")
	namespace := domain.NewIdentityNamespaceFromComponents(td, "/")
	selector, err := domain.ParseSelectorFromString("unix:uid:2000")
	require.NoError(t, err)

	selectorSet := domain.NewSelectorSet()
	selectorSet.Add(selector)
	mapper, err := domain.NewIdentityMapper(namespace, selectorSet)
	require.NoError(t, err)
	err = registry.Seed(ctx, mapper)
	require.NoError(t, err)
	registry.Seal()

	workloadAttestor := attestor.NewUnixWorkloadAttestor()
	workloadAttestor.RegisterUID(2000, "unix:uid:2000")

	parser := inmemory.NewInMemoryIdentityNamespaceParser()

	agent, err := inmemory.NewInMemoryAgent(
		ctx,
		"spiffe://example.org/agent",
		server,
		registry,
		workloadAttestor,
		parser,
		docProvider,
	)
	require.NoError(t, err)

	// Act - Fetch with root path identity
	identity, err := agent.FetchIdentityDocument(ctx, ports.ProcessIdentity{UID: 2000, GID: 2000})

	// Assert - Name should be trust domain when path is "/"
	require.NoError(t, err)
	assert.NotNil(t, identity)
	assert.Equal(t, "example.org", identity.Name)
}

// TestTrustBundleProvider_MultiCAConcat tests multi-CA bundle concatenation
func TestTrustBundleProvider_MultiCAConcat(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	tdParser := inmemory.NewInMemoryTrustDomainParser()
	docProvider := inmemory.NewInMemoryIdentityDocumentProvider()

	// Create two separate servers to get two different CAs
	server1, err := inmemory.NewInMemoryServer(ctx, "example.org", tdParser, docProvider)
	require.NoError(t, err)

	server2, err := inmemory.NewInMemoryServer(ctx, "example.com", tdParser, docProvider)
	require.NoError(t, err)

	// Create provider with multiple CAs
	caCerts := []*x509.Certificate{server1.GetCA(), server2.GetCA()}
	provider := inmemory.NewInMemoryTrustBundleProvider(caCerts)

	td := domain.NewTrustDomainFromName("example.org")

	// Act
	bundle, err := provider.GetBundle(ctx, td)

	// Assert
	require.NoError(t, err)
	assert.NotNil(t, bundle)

	// Verify it contains multiple PEM blocks
	bundleStr := string(bundle)
	assert.Contains(t, bundleStr, "BEGIN CERTIFICATE")
	assert.Contains(t, bundleStr, "END CERTIFICATE")

	// Count PEM blocks (should be 2)
	beginCount := len(regexp.MustCompile("BEGIN CERTIFICATE").FindAllString(bundleStr, -1))
	endCount := len(regexp.MustCompile("END CERTIFICATE").FindAllString(bundleStr, -1))
	assert.Equal(t, 2, beginCount, "Should have 2 BEGIN CERTIFICATE markers")
	assert.Equal(t, 2, endCount, "Should have 2 END CERTIFICATE markers")

	// Verify the bundle can be parsed back
	block1, rest := pem.Decode(bundle)
	assert.NotNil(t, block1, "First PEM block should decode")
	assert.Equal(t, "CERTIFICATE", block1.Type)

	block2, _ := pem.Decode(rest)
	assert.NotNil(t, block2, "Second PEM block should decode")
	assert.Equal(t, "CERTIFICATE", block2.Type)
}
