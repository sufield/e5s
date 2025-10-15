//go:build dev

package ports

import (
	"context"

	"github.com/pocket/hexagon/spire/internal/domain"
	"github.com/pocket/hexagon/spire/internal/dto"
)

// Service defines dev-only business operations used in examples/tests.
type Service interface {
	ExchangeMessage(ctx context.Context, from dto.Identity, to dto.Identity, content string) (*dto.Message, error)
}

// IdentityIssuer issues an identity for a verified workload (dev-only).
type IdentityIssuer interface {
	IssueIdentity(ctx context.Context, workload *domain.Workload) (*dto.Identity, error)
}
