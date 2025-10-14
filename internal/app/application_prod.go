//go:build !dev

package app

import (
	"github.com/pocket/hexagon/spire/internal/ports"
)

// Application is the composition root that wires all dependencies for production.
// Production version doesn't include Registry or demo Service - workloads only fetch identities via Workload API.
type Application struct {
	Config                *ports.Config
	IdentityClientService *IdentityClientService
	Agent                 ports.Agent
	// No Registry or Service in production (dev-only demos)
}
