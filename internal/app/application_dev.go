//go:build dev

package app

import (
	"github.com/pocket/hexagon/spire/internal/ports"
)

// Application is the composition root that wires all dependencies for development.
// Development version includes Registry for local identity mapping.
type Application struct {
	Config                *ports.Config
	Service               ports.Service
	IdentityClientService *IdentityClientService
	Agent                 ports.Agent
	Registry              ports.IdentityMapperRegistry
}
