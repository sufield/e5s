//go:build dev
// +build dev

package app_test

// Identity Service Invariant Tests
//
// These tests verify domain invariants for the IdentityService ExchangeMessage operation.
// Invariants tested: non-nil credentials requirement, valid (non-expired) documents,
// no partial results on error, input preservation, success guarantees, idempotency.
//
// NOTE: These tests are dev-only since IdentityService is only available in development builds.
//
// Run these tests with:
//
//	go test -tags dev ./internal/app/... -v -run TestExchangeMessage_Invariant
//	go test -tags dev ./internal/app/... -cover

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
// "ExchangeMessage requires non-nil identity credentials"
func TestExchangeMessage_Invariant_RequiresNonNilNamespaces(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	mockAgent := new(MockAgent)
	mockRegistry := new(MockRegistry)
	service := app.NewIdentityService(mockAgent, mockRegistry)

	tests := []struct {
		name      string
		from      ports.Identity
		to        ports.Identity
		wantError bool
		wantErr   error
	}{
		{
			name:      "both namespaces non-nil - valid",
			from:      *createValidIdentity(t, "spiffe://example.org/client", time.Now().Add(1*time.Hour)),
			to:        *createValidIdentity(t, "spiffe://example.org/server", time.Now().Add(1*time.Hour)),
			wantError: false,
		},
		{
			name: "sender credential nil - violates invariant",
			from: ports.Identity{
				IdentityCredential: nil, // Nil credential
				Name:               "client",
			},
			to:        *createValidIdentity(t, "spiffe://example.org/server", time.Now().Add(1*time.Hour)),
			wantError: true,
			wantErr:   domain.ErrInvalidIdentityCredential,
		},
		{
			name: "receiver credential nil - violates invariant",
			from: *createValidIdentity(t, "spiffe://example.org/client", time.Now().Add(1*time.Hour)),
			to: ports.Identity{
				IdentityCredential: nil, // Nil credential
				Name:               "server",
			},
			wantError: true,
			wantErr:   domain.ErrInvalidIdentityCredential,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Act
			msg, err := service.ExchangeMessage(ctx, tt.from, tt.to, "test")

			// Assert invariant
			if tt.wantError {
				assert.Error(t, err, "Invariant enforced: should reject nil credentials")
				assert.Nil(t, msg, "Invariant violated: msg should be nil when error occurs")
				assert.ErrorIs(t, err, tt.wantErr)
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
		name      string
		from      ports.Identity
		to        ports.Identity
		wantError bool
		wantErr   error
	}{
		{
			name:      "both documents valid - ok",
			from:      *createValidIdentity(t, "spiffe://example.org/client", time.Now().Add(1*time.Hour)),
			to:        *createValidIdentity(t, "spiffe://example.org/server", time.Now().Add(1*time.Hour)),
			wantError: false,
		},
		{
			name:      "sender document expired - violates invariant",
			from:      *createValidIdentity(t, "spiffe://example.org/client", time.Now().Add(-1*time.Hour)),
			to:        *createValidIdentity(t, "spiffe://example.org/server", time.Now().Add(1*time.Hour)),
			wantError: true,
			wantErr:   domain.ErrIdentityDocumentExpired,
		},
		{
			name:      "receiver document expired - violates invariant",
			from:      *createValidIdentity(t, "spiffe://example.org/client", time.Now().Add(1*time.Hour)),
			to:        *createValidIdentity(t, "spiffe://example.org/server", time.Now().Add(-1*time.Hour)),
			wantError: true,
			wantErr:   domain.ErrIdentityDocumentExpired,
		},
		{
			name: "sender document nil - violates invariant",
			from: ports.Identity{
				IdentityCredential: domain.NewIdentityCredentialFromComponents(
					domain.NewTrustDomainFromName("example.org"), "/client"),
				Name:             "client",
				IdentityDocument: nil, // Nil document
			},
			to:        *createValidIdentity(t, "spiffe://example.org/server", time.Now().Add(1*time.Hour)),
			wantError: true,
			wantErr:   domain.ErrIdentityDocumentInvalid,
		},
		{
			name: "receiver document nil - violates invariant",
			from: *createValidIdentity(t, "spiffe://example.org/client", time.Now().Add(1*time.Hour)),
			to: ports.Identity{
				IdentityCredential: domain.NewIdentityCredentialFromComponents(
					domain.NewTrustDomainFromName("example.org"), "/server"),
				Name:             "server",
				IdentityDocument: nil, // Nil document
			},
			wantError: true,
			wantErr:   domain.ErrIdentityDocumentInvalid,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Act
			msg, err := service.ExchangeMessage(ctx, tt.from, tt.to, "test")

			// Assert invariant
			if tt.wantError {
				assert.Error(t, err, "Invariant enforced: should reject invalid/expired documents")
				assert.Nil(t, msg, "Invariant violated: msg should be nil when error occurs")
				assert.ErrorIs(t, err, tt.wantErr)
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
			name: "nil sender credential",
			from: ports.Identity{IdentityCredential: nil, Name: "client"},
			to:   *createValidIdentity(t, "spiffe://example.org/server", time.Now().Add(1*time.Hour)),
		},
		{
			name: "nil receiver credential",
			from: *createValidIdentity(t, "spiffe://example.org/client", time.Now().Add(1*time.Hour)),
			to:   ports.Identity{IdentityCredential: nil, Name: "server"},
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

			assert.Equal(t, tt.from.IdentityCredential, msg.From.IdentityCredential,
				"Invariant violated: From.IdentityCredential not preserved")
			assert.Equal(t, tt.to.IdentityCredential, msg.To.IdentityCredential,
				"Invariant violated: To.IdentityCredential not preserved")
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
		assert.Equal(t, from.IdentityCredential, msg.From.IdentityCredential,
			"Invariant violated: msg.From should match input from")
		assert.Equal(t, to.IdentityCredential, msg.To.IdentityCredential,
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
	assert.Equal(t, msg1.From.IdentityCredential, msg2.From.IdentityCredential)
	assert.Equal(t, msg1.To.IdentityCredential, msg2.To.IdentityCredential)
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
