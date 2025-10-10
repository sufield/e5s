package app_test

import (
	"context"
	"testing"
	"time"

	"github.com/pocket/hexagon/spire/internal/app"
	"github.com/pocket/hexagon/spire/internal/domain"
	"github.com/pocket/hexagon/spire/internal/ports"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// MockAgent is a mock implementation of ports.Agent for testing
type MockAgent struct {
	mock.Mock
}

func (m *MockAgent) GetIdentity(ctx context.Context) (*ports.Identity, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*ports.Identity), args.Error(1)
}

func (m *MockAgent) FetchIdentityDocument(ctx context.Context, workload ports.ProcessIdentity) (*ports.Identity, error) {
	args := m.Called(ctx, workload)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*ports.Identity), args.Error(1)
}

// MockRegistry is a mock implementation of ports.IdentityMapperRegistry for testing
type MockRegistry struct {
	mock.Mock
}

func (m *MockRegistry) FindBySelectors(ctx context.Context, selectors *domain.SelectorSet) (*domain.IdentityMapper, error) {
	args := m.Called(ctx, selectors)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.IdentityMapper), args.Error(1)
}

func (m *MockRegistry) ListAll(ctx context.Context) ([]*domain.IdentityMapper, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.IdentityMapper), args.Error(1)
}

func TestIdentityService_ExchangeMessage_Success(t *testing.T) {
	t.Parallel()

	// Arrange
	ctx := context.Background()
	mockAgent := new(MockAgent)
	mockRegistry := new(MockRegistry)
	service := app.NewIdentityService(mockAgent, mockRegistry)

	// Create valid identities
	fromID := createValidIdentity(t, "spiffe://example.org/client", time.Now().Add(1*time.Hour))
	toID := createValidIdentity(t, "spiffe://example.org/server", time.Now().Add(1*time.Hour))

	// Act
	msg, err := service.ExchangeMessage(ctx, *fromID, *toID, "Hello server")

	// Assert
	require.NoError(t, err)
	require.NotNil(t, msg)
	assert.Equal(t, "Hello server", msg.Content)
	assert.Equal(t, fromID.IdentityCredential, msg.From.IdentityCredential)
	assert.Equal(t, toID.IdentityCredential, msg.To.IdentityCredential)
}

func TestIdentityService_ExchangeMessage_NilSenderNamespace(t *testing.T) {
	t.Parallel()

	// Arrange
	ctx := context.Background()
	mockAgent := new(MockAgent)
	mockRegistry := new(MockRegistry)
	service := app.NewIdentityService(mockAgent, mockRegistry)

	fromID := ports.Identity{
		IdentityCredential: nil, // Invalid: nil credential
		Name:               "client",
	}
	toID := createValidIdentity(t, "spiffe://example.org/server", time.Now().Add(1*time.Hour))

	// Act
	msg, err := service.ExchangeMessage(ctx, fromID, *toID, "Hello")

	// Assert
	assert.Error(t, err)
	assert.Nil(t, msg)
	assert.ErrorIs(t, err, domain.ErrInvalidIdentityCredential)
}

func TestIdentityService_ExchangeMessage_NilReceiverNamespace(t *testing.T) {
	t.Parallel()

	// Arrange
	ctx := context.Background()
	mockAgent := new(MockAgent)
	mockRegistry := new(MockRegistry)
	service := app.NewIdentityService(mockAgent, mockRegistry)

	fromID := createValidIdentity(t, "spiffe://example.org/client", time.Now().Add(1*time.Hour))
	toID := ports.Identity{
		IdentityCredential: nil, // Invalid: nil credential
		Name:               "server",
	}

	// Act
	msg, err := service.ExchangeMessage(ctx, *fromID, toID, "Hello")

	// Assert
	assert.Error(t, err)
	assert.Nil(t, msg)
	assert.ErrorIs(t, err, domain.ErrInvalidIdentityCredential)
}

func TestIdentityService_ExchangeMessage_ExpiredSenderDocument(t *testing.T) {
	t.Parallel()

	// Arrange
	ctx := context.Background()
	mockAgent := new(MockAgent)
	mockRegistry := new(MockRegistry)
	service := app.NewIdentityService(mockAgent, mockRegistry)

	// Create identity with expired document
	fromID := createValidIdentity(t, "spiffe://example.org/client", time.Now().Add(-1*time.Hour))
	toID := createValidIdentity(t, "spiffe://example.org/server", time.Now().Add(1*time.Hour))

	// Act
	msg, err := service.ExchangeMessage(ctx, *fromID, *toID, "Hello")

	// Assert
	assert.Error(t, err)
	assert.Nil(t, msg)
	assert.ErrorIs(t, err, domain.ErrIdentityDocumentExpired)
}

func TestIdentityService_ExchangeMessage_ExpiredReceiverDocument(t *testing.T) {
	t.Parallel()

	// Arrange
	ctx := context.Background()
	mockAgent := new(MockAgent)
	mockRegistry := new(MockRegistry)
	service := app.NewIdentityService(mockAgent, mockRegistry)

	fromID := createValidIdentity(t, "spiffe://example.org/client", time.Now().Add(1*time.Hour))
	toID := createValidIdentity(t, "spiffe://example.org/server", time.Now().Add(-1*time.Hour))

	// Act
	msg, err := service.ExchangeMessage(ctx, *fromID, *toID, "Hello")

	// Assert
	assert.Error(t, err)
	assert.Nil(t, msg)
	assert.ErrorIs(t, err, domain.ErrIdentityDocumentExpired)
}

func TestIdentityService_ExchangeMessage_NilSenderDocument(t *testing.T) {
	t.Parallel()

	// Arrange
	ctx := context.Background()
	mockAgent := new(MockAgent)
	mockRegistry := new(MockRegistry)
	service := app.NewIdentityService(mockAgent, mockRegistry)

	td := domain.NewTrustDomainFromName("example.org")
	fromID := &ports.Identity{
		IdentityCredential: domain.NewIdentityCredentialFromComponents(td, "/client"),
		Name:               "client",
		IdentityDocument:   nil, // Invalid: nil document
	}
	toID := createValidIdentity(t, "spiffe://example.org/server", time.Now().Add(1*time.Hour))

	// Act
	msg, err := service.ExchangeMessage(ctx, *fromID, *toID, "Hello")

	// Assert
	assert.Error(t, err)
	assert.Nil(t, msg)
	assert.ErrorIs(t, err, domain.ErrIdentityDocumentExpired)
}

func TestIdentityService_ExchangeMessage_NilReceiverDocument(t *testing.T) {
	t.Parallel()

	// Arrange
	ctx := context.Background()
	mockAgent := new(MockAgent)
	mockRegistry := new(MockRegistry)
	service := app.NewIdentityService(mockAgent, mockRegistry)

	td := domain.NewTrustDomainFromName("example.org")
	fromID := createValidIdentity(t, "spiffe://example.org/client", time.Now().Add(1*time.Hour))
	toID := &ports.Identity{
		IdentityCredential: domain.NewIdentityCredentialFromComponents(td, "/server"),
		Name:               "server",
		IdentityDocument:   nil, // Invalid: nil document
	}

	// Act
	msg, err := service.ExchangeMessage(ctx, *fromID, *toID, "Hello")

	// Assert
	assert.Error(t, err)
	assert.Nil(t, msg)
	assert.ErrorIs(t, err, domain.ErrIdentityDocumentExpired)
}

func TestIdentityService_ExchangeMessage_EmptyContent(t *testing.T) {
	t.Parallel()

	// Arrange
	ctx := context.Background()
	mockAgent := new(MockAgent)
	mockRegistry := new(MockRegistry)
	service := app.NewIdentityService(mockAgent, mockRegistry)

	fromID := createValidIdentity(t, "spiffe://example.org/client", time.Now().Add(1*time.Hour))
	toID := createValidIdentity(t, "spiffe://example.org/server", time.Now().Add(1*time.Hour))

	// Act - Empty content should be allowed
	msg, err := service.ExchangeMessage(ctx, *fromID, *toID, "")

	// Assert
	require.NoError(t, err)
	require.NotNil(t, msg)
	assert.Equal(t, "", msg.Content)
	assert.Equal(t, fromID.IdentityCredential, msg.From.IdentityCredential)
	assert.Equal(t, toID.IdentityCredential, msg.To.IdentityCredential)
}

func TestIdentityService_ExchangeMessage_LongContent(t *testing.T) {
	t.Parallel()

	// Arrange
	ctx := context.Background()
	mockAgent := new(MockAgent)
	mockRegistry := new(MockRegistry)
	service := app.NewIdentityService(mockAgent, mockRegistry)

	fromID := createValidIdentity(t, "spiffe://example.org/client", time.Now().Add(1*time.Hour))
	toID := createValidIdentity(t, "spiffe://example.org/server", time.Now().Add(1*time.Hour))

	// Create long content (10KB)
	longContent := string(make([]byte, 10*1024))

	// Act - Long content should be allowed (no size validation in current implementation)
	msg, err := service.ExchangeMessage(ctx, *fromID, *toID, longContent)

	// Assert
	require.NoError(t, err)
	require.NotNil(t, msg)
	assert.Equal(t, longContent, msg.Content)
	assert.Equal(t, fromID.IdentityCredential, msg.From.IdentityCredential)
	assert.Equal(t, toID.IdentityCredential, msg.To.IdentityCredential)
}

func TestIdentityService_ExchangeMessage_TableDriven(t *testing.T) {
	tests := []struct {
		name      string
		fromID    *ports.Identity
		toID      *ports.Identity
		content   string
		wantError bool
		wantErr   error
	}{
		{
			name:      "valid exchange",
			fromID:    createValidIdentity(t, "spiffe://example.org/client", time.Now().Add(1*time.Hour)),
			toID:      createValidIdentity(t, "spiffe://example.org/server", time.Now().Add(1*time.Hour)),
			content:   "test message",
			wantError: false,
		},
		{
			name:      "nil sender credential",
			fromID:    &ports.Identity{IdentityCredential: nil, Name: "client"},
			toID:      createValidIdentity(t, "spiffe://example.org/server", time.Now().Add(1*time.Hour)),
			content:   "test",
			wantError: true,
			wantErr:   domain.ErrInvalidIdentityCredential,
		},
		{
			name:      "nil receiver credential",
			fromID:    createValidIdentity(t, "spiffe://example.org/client", time.Now().Add(1*time.Hour)),
			toID:      &ports.Identity{IdentityCredential: nil, Name: "server"},
			content:   "test",
			wantError: true,
			wantErr:   domain.ErrInvalidIdentityCredential,
		},
		{
			name:      "expired sender document",
			fromID:    createValidIdentity(t, "spiffe://example.org/client", time.Now().Add(-1*time.Hour)),
			toID:      createValidIdentity(t, "spiffe://example.org/server", time.Now().Add(1*time.Hour)),
			content:   "test",
			wantError: true,
			wantErr:   domain.ErrIdentityDocumentExpired,
		},
		{
			name:      "expired receiver document",
			fromID:    createValidIdentity(t, "spiffe://example.org/client", time.Now().Add(1*time.Hour)),
			toID:      createValidIdentity(t, "spiffe://example.org/server", time.Now().Add(-1*time.Hour)),
			content:   "test",
			wantError: true,
			wantErr:   domain.ErrIdentityDocumentExpired,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			ctx := context.Background()
			mockAgent := new(MockAgent)
			mockRegistry := new(MockRegistry)
			service := app.NewIdentityService(mockAgent, mockRegistry)

			// Explicit nil checks for clarity in error cases
			require.NotNil(t, tt.fromID)
			require.NotNil(t, tt.toID)

			// Act
			msg, err := service.ExchangeMessage(ctx, *tt.fromID, *tt.toID, tt.content)

			// Assert
			if tt.wantError {
				assert.Error(t, err)
				assert.Nil(t, msg)
				if tt.wantErr != nil {
					assert.ErrorIs(t, err, tt.wantErr)
				}
			} else {
				require.NoError(t, err)
				require.NotNil(t, msg)
				assert.Equal(t, tt.content, msg.Content)
				assert.Equal(t, tt.fromID.IdentityCredential, msg.From.IdentityCredential)
				assert.Equal(t, tt.toID.IdentityCredential, msg.To.IdentityCredential)
			}
		})
	}
}

// Helper function to create a valid identity for testing
func createValidIdentity(t *testing.T, spiffeID string, expiresAt time.Time) *ports.Identity {
	t.Helper()

	td := domain.NewTrustDomainFromName("example.org")
	path := "/client"
	if len(spiffeID) > len("spiffe://example.org") {
		path = spiffeID[len("spiffe://example.org"):]
	}

	identityCredential := domain.NewIdentityCredentialFromComponents(td, path)

	return &ports.Identity{
		IdentityCredential: identityCredential,
		Name:               "test-identity",
		IdentityDocument: domain.NewIdentityDocumentFromComponents(
			identityCredential,
			nil, // cert
			nil, // privateKey
			nil, // chain
			expiresAt,
		),
	}
}
