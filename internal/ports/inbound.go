package ports

import "context"

// IdentityProvider issues the caller's identity credential.
type IdentityProvider interface {
	FetchIdentity(ctx context.Context) (*Identity, error)
	Close() error
}

// CLI drives the application via command line.
type CLI interface {
	Run(ctx context.Context) error
}
