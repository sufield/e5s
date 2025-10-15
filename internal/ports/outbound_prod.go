//go:build !dev

package ports

import (
	"context"

	"github.com/pocket/hexagon/spire/internal/dto"
)

// ConfigLoader loads runtime configuration (production).
type ConfigLoader interface {
	Load(ctx context.Context) (*dto.Config, error)
}
