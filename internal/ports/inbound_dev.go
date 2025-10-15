//go:build dev

package ports

import "context"

// Service defines dev-only business operations used in examples/tests.
type Service interface {
	ExchangeMessage(ctx context.Context, from Identity, to Identity, content string) (*Message, error)
}

// IdentityIssuer issues an identity for a verified workload (dev-only).
type IdentityIssuer interface {
	IssueIdentity(ctx context.Context, workload ProcessIdentity) (*Identity, error)
}
