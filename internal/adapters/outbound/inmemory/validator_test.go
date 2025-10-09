package inmemory_test

import (
	"context"
	"testing"
	"time"

	"github.com/pocket/hexagon/spire/internal/adapters/outbound/inmemory"
	"github.com/pocket/hexagon/spire/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIdentityDocumentValidator_Validate(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	// Mock bundle provider (nil for skip)
	validator := inmemory.NewIdentityDocumentValidator(nil)
	td := domain.NewTrustDomainFromName("example.org")
	expected := domain.NewIdentityCredentialFromComponents(td, "/workload")
	validDoc := domain.NewIdentityDocumentFromComponents(expected, domain.IdentityDocumentTypeX509, nil, nil, nil, time.Now().Add(1*time.Hour))
	expiredDoc := domain.NewIdentityDocumentFromComponents(expected, domain.IdentityDocumentTypeX509, nil, nil, nil, time.Now().Add(-1*time.Hour))

	// Valid
	err := validator.Validate(ctx, validDoc, expected)
	require.NoError(t, err)

	// Expired
	err = validator.Validate(ctx, expiredDoc, expected)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "expired")

	// Mismatch namespace
	wrongNS := domain.NewIdentityCredentialFromComponents(domain.NewTrustDomainFromName("other.org"), "/workload")
	err = validator.Validate(ctx, validDoc, wrongNS)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "mismatch")
}
