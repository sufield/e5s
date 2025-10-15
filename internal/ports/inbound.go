package ports

import (
	"context"

	"github.com/pocket/hexagon/spire/internal/dto"
)

// IdentityProvider issues the caller's identity credential.
type IdentityProvider interface {
	FetchIdentity(ctx context.Context) (*dto.Identity, error)
	Close() error
}

// CLI drives the application via command line.
type CLI interface {
	Run(ctx context.Context) error
}
