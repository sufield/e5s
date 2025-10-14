//go:build !dev

package app

import (
	"github.com/pocket/hexagon/spire/internal/ports"
)

// Application is the composition root that wires all dependencies for production.
// Production version doesn't include Registry - workloads only fetch identities via Workload API.
type Application struct {
	Config                *ports.Config
	Service               ports.Service
	IdentityClientService *IdentityClientService
	Agent                 ports.Agent
	// No Registry in production
}
