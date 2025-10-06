package app_test

import (
	"context"
	"testing"
	"time"

	"github.com/pocket/hexagon/spire/internal/app"
	"github.com/pocket/hexagon/spire/internal/domain"
	"github.com/pocket/hexagon/spire/internal/ports"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestExchangeMessage_Invariant_RequiresNonNilNamespaces tests the invariant:
// "ExchangeMessage requires non-nil identity namespaces"
func TestExchangeMessage_Invariant_RequiresNonNilNamespaces(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	mockAgent := new(MockAgent)
	mockRegistry := new(MockRegistry)
	service := app.NewIdentityService(mockAgent, mockRegistry)

	tests := []struct {
		name        string
		from        ports.Identity
		to          ports.Identity
		expectError bool
		errorContains string
	}{
		{
			name: "both namespaces non-nil - valid",
			from: *createValidIdentity(t, "spiffe://example.org/client", time.Now().Add(1*time.Hour)),
			to:   *createValidIdentity(t, "spiffe://example.org/server", time.Now().Add(1*time.Hour)),
			expectError: false,
		},
		{
			name: "sender namespace nil - violates invariant",
			from: ports.Identity{
				IdentityNamespace: nil, // Nil namespace
				Name:              "client",
			},
			to:          *createValidIdentity(t, "spiffe://example.org/server", time.Now().Add(1*time.Hour)),
			expectError: true,
			errorContains: "sender identity namespace required",
		},
		{
			name: "receiver namespace nil - violates invariant",
			from: *createValidIdentity(t, "spiffe://example.org/client", time.Now().Add(1*time.Hour)),
			to: ports.Identity{
				IdentityNamespace: nil, // Nil namespace
				Name:              "server",
			},
			expectError: true,
			errorContains: "receiver identity namespace required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Act
			msg, err := service.ExchangeMessage(ctx, tt.from, tt.to, "test")

			// Assert invariant
			if tt.expectError {
				assert.Error(t, err, "Invariant enforced: should reject nil namespaces")
				assert.Nil(t, msg, "Invariant violated: msg should be nil when error occurs")
				assert.Contains(t, err.Error(), tt.errorContains)
			} else {
				require.NoError(t, err)
				require.NotNil(t, msg)
			}
		})
	}
}

// TestExchangeMessage_Invariant_RequiresValidDocuments tests the invariant:
// "ExchangeMessage requires valid (non-expired) identity documents"
func TestExchangeMessage_Invariant_RequiresValidDocuments(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	mockAgent := new(MockAgent)
	mockRegistry := new(MockRegistry)
	service := app.NewIdentityService(mockAgent, mockRegistry)

	tests := []struct {
		name        string
		from        ports.Identity
		to          ports.Identity
		expectError bool
		errorContains string
	}{
		{
			name:        "both documents valid - ok",
			from:        *createValidIdentity(t, "spiffe://example.org/client", time.Now().Add(1*time.Hour)),
			to:          *createValidIdentity(t, "spiffe://example.org/server", time.Now().Add(1*time.Hour)),
			expectError: false,
		},
		{
			name:        "sender document expired - violates invariant",
			from:        *createValidIdentity(t, "spiffe://example.org/client", time.Now().Add(-1*time.Hour)),
			to:          *createValidIdentity(t, "spiffe://example.org/server", time.Now().Add(1*time.Hour)),
			expectError: true,
			errorContains: "sender identity document invalid or expired",
		},
		{
			name:        "receiver document expired - violates invariant",
			from:        *createValidIdentity(t, "spiffe://example.org/client", time.Now().Add(1*time.Hour)),
			to:          *createValidIdentity(t, "spiffe://example.org/server", time.Now().Add(-1*time.Hour)),
			expectError: true,
			errorContains: "receiver identity document invalid or expired",
		},
		{
			name: "sender document nil - violates invariant",
			from: ports.Identity{
				IdentityNamespace: domain.NewIdentityNamespaceFromComponents(
					domain.NewTrustDomainFromName("example.org"), "/client"),
				Name:             "client",
				IdentityDocument: nil, // Nil document
			},
			to:          *createValidIdentity(t, "spiffe://example.org/server", time.Now().Add(1*time.Hour)),
			expectError: true,
			errorContains: "sender identity document invalid or expired",
		},
		{
			name: "receiver document nil - violates invariant",
			from: *createValidIdentity(t, "spiffe://example.org/client", time.Now().Add(1*time.Hour)),
			to: ports.Identity{
				IdentityNamespace: domain.NewIdentityNamespaceFromComponents(
					domain.NewTrustDomainFromName("example.org"), "/server"),
				Name:             "server",
				IdentityDocument: nil, // Nil document
			},
			expectError: true,
			errorContains: "receiver identity document invalid or expired",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Act
			msg, err := service.ExchangeMessage(ctx, tt.from, tt.to, "test")

			// Assert invariant
			if tt.expectError {
				assert.Error(t, err, "Invariant enforced: should reject invalid/expired documents")
				assert.Nil(t, msg, "Invariant violated: msg should be nil when error occurs")
				assert.Contains(t, err.Error(), tt.errorContains)
			} else {
				require.NoError(t, err)
				require.NotNil(t, msg)
			}
		})
	}
}

// TestExchangeMessage_Invariant_NeverReturnsPartialResult tests the invariant:
// "ExchangeMessage never returns msg != nil when err != nil"
func TestExchangeMessage_Invariant_NeverReturnsPartialResult(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	mockAgent := new(MockAgent)
	mockRegistry := new(MockRegistry)
	service := app.NewIdentityService(mockAgent, mockRegistry)

	tests := []struct {
		name string
		from ports.Identity
		to   ports.Identity
	}{
		{
			name: "nil sender namespace",
			from: ports.Identity{IdentityNamespace: nil, Name: "client"},
			to:   *createValidIdentity(t, "spiffe://example.org/server", time.Now().Add(1*time.Hour)),
		},
		{
			name: "nil receiver namespace",
			from: *createValidIdentity(t, "spiffe://example.org/client", time.Now().Add(1*time.Hour)),
			to:   ports.Identity{IdentityNamespace: nil, Name: "server"},
		},
		{
			name: "expired sender document",
			from: *createValidIdentity(t, "spiffe://example.org/client", time.Now().Add(-1*time.Hour)),
			to:   *createValidIdentity(t, "spiffe://example.org/server", time.Now().Add(1*time.Hour)),
		},
		{
			name: "expired receiver document",
			from: *createValidIdentity(t, "spiffe://example.org/client", time.Now().Add(1*time.Hour)),
			to:   *createValidIdentity(t, "spiffe://example.org/server", time.Now().Add(-1*time.Hour)),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Act
			msg, err := service.ExchangeMessage(ctx, tt.from, tt.to, "test")

			// Assert invariant: if error, then msg must be nil (no partial result)
			if err != nil {
				assert.Nil(t, msg,
					"Invariant violated: msg must be nil when err != nil (no partial results)")
			}
		})
	}
}

// TestExchangeMessage_Invariant_PreservesInputIdentities tests the invariant:
// "Created message preserves input identities and content"
func TestExchangeMessage_Invariant_PreservesInputIdentities(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	mockAgent := new(MockAgent)
	mockRegistry := new(MockRegistry)
	service := app.NewIdentityService(mockAgent, mockRegistry)

	tests := []struct {
		name    string
		from    *ports.Identity
		to      *ports.Identity
		content string
	}{
		{
			name:    "simple message",
			from:    createValidIdentity(t, "spiffe://example.org/client", time.Now().Add(1*time.Hour)),
			to:      createValidIdentity(t, "spiffe://example.org/server", time.Now().Add(1*time.Hour)),
			content: "Hello server",
		},
		{
			name:    "empty content",
			from:    createValidIdentity(t, "spiffe://example.org/client", time.Now().Add(1*time.Hour)),
			to:      createValidIdentity(t, "spiffe://example.org/server", time.Now().Add(1*time.Hour)),
			content: "",
		},
		{
			name:    "long content",
			from:    createValidIdentity(t, "spiffe://example.org/client", time.Now().Add(1*time.Hour)),
			to:      createValidIdentity(t, "spiffe://example.org/server", time.Now().Add(1*time.Hour)),
			content: string(make([]byte, 1024)),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Act
			msg, err := service.ExchangeMessage(ctx, *tt.from, *tt.to, tt.content)

			// Assert invariant: message preserves inputs
			require.NoError(t, err)
			require.NotNil(t, msg)

			assert.Equal(t, tt.from.IdentityNamespace, msg.From.IdentityNamespace,
				"Invariant violated: From.IdentityNamespace not preserved")
			assert.Equal(t, tt.to.IdentityNamespace, msg.To.IdentityNamespace,
				"Invariant violated: To.IdentityNamespace not preserved")
			assert.Equal(t, tt.content, msg.Content,
				"Invariant violated: Content not preserved")
		})
	}
}

// TestExchangeMessage_Invariant_SuccessImpliesNonNilMessage tests the invariant:
// "If err == nil, then msg != nil and msg.From/To match inputs"
func TestExchangeMessage_Invariant_SuccessImpliesNonNilMessage(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	mockAgent := new(MockAgent)
	mockRegistry := new(MockRegistry)
	service := app.NewIdentityService(mockAgent, mockRegistry)

	from := createValidIdentity(t, "spiffe://example.org/client", time.Now().Add(1*time.Hour))
	to := createValidIdentity(t, "spiffe://example.org/server", time.Now().Add(1*time.Hour))

	// Act
	msg, err := service.ExchangeMessage(ctx, *from, *to, "test")

	// Assert invariant: success implies non-nil message with correct fields
	require.NoError(t, err, "Setup: should succeed with valid inputs")
	assert.NotNil(t, msg, "Invariant violated: msg must be non-nil when err == nil")

	if msg != nil {
		assert.Equal(t, from.IdentityNamespace, msg.From.IdentityNamespace,
			"Invariant violated: msg.From should match input from")
		assert.Equal(t, to.IdentityNamespace, msg.To.IdentityNamespace,
			"Invariant violated: msg.To should match input to")
	}
}

// TestExchangeMessage_Invariant_Idempotency tests the invariant:
// "Multiple calls with same inputs produce equivalent results"
func TestExchangeMessage_Invariant_Idempotency(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	mockAgent := new(MockAgent)
	mockRegistry := new(MockRegistry)
	service := app.NewIdentityService(mockAgent, mockRegistry)

	from := createValidIdentity(t, "spiffe://example.org/client", time.Now().Add(1*time.Hour))
	to := createValidIdentity(t, "spiffe://example.org/server", time.Now().Add(1*time.Hour))
	content := "test message"

	// Act - Call multiple times
	msg1, err1 := service.ExchangeMessage(ctx, *from, *to, content)
	msg2, err2 := service.ExchangeMessage(ctx, *from, *to, content)
	msg3, err3 := service.ExchangeMessage(ctx, *from, *to, content)

	// Assert invariant: idempotent (same results)
	require.NoError(t, err1)
	require.NoError(t, err2)
	require.NoError(t, err3)

	assert.Equal(t, msg1.Content, msg2.Content, "Invariant violated: should be idempotent")
	assert.Equal(t, msg2.Content, msg3.Content, "Invariant violated: should be idempotent")
	assert.Equal(t, msg1.From.IdentityNamespace, msg2.From.IdentityNamespace)
	assert.Equal(t, msg1.To.IdentityNamespace, msg2.To.IdentityNamespace)
}

// TestExchangeMessage_Invariant_LargeContentNoTruncation tests the invariant:
// "Content is preserved without truncation regardless of size"
func TestExchangeMessage_Invariant_LargeContentNoTruncation(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	mockAgent := new(MockAgent)
	mockRegistry := new(MockRegistry)
	service := app.NewIdentityService(mockAgent, mockRegistry)

	from := createValidIdentity(t, "spiffe://example.org/client", time.Now().Add(1*time.Hour))
	to := createValidIdentity(t, "spiffe://example.org/server", time.Now().Add(1*time.Hour))

	// Test with 1MB content
	largeContent := string(make([]byte, 1024*1024))

	// Act
	msg, err := service.ExchangeMessage(ctx, *from, *to, largeContent)

	// Assert invariant: content not truncated
	require.NoError(t, err)
	require.NotNil(t, msg)
	assert.Equal(t, len(largeContent), len(msg.Content),
		"Invariant violated: content length changed (possible truncation)")
	assert.Equal(t, largeContent, msg.Content,
		"Invariant violated: content was modified or truncated")
}
