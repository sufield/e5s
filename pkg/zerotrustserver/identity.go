package zerotrustserver

import (
	"context"

	"github.com/pocket/hexagon/spire/internal/ports"
)

// Identity returns the authenticated identity injected by the adapter.
func Identity(ctx context.Context) (ports.Identity, bool) {
	return ports.IdentityFrom(ctx)
}
