package app

import (
	"github.com/pocket/hexagon/spire/internal/ports"
)

// Application is the composition root that wires all dependencies
// This is an infrastructure/bootstrap logic
type Application struct {
	Config                *ports.Config
	Service               ports.Service
	IdentityClientService *IdentityClientService
	Agent                 ports.Agent
	Registry              ports.IdentityMapperRegistry
}

// NOTE: Bootstrap function is implemented in build-specific files:
// - bootstrap_dev.go (//go:build !production) - development mode with in-memory implementations
// - bootstrap_prod.go (//go:build production) - production mode with SPIRE infrastructure
//
// This allows development-only interfaces (AdapterFactory, DevelopmentAdapterFactory, etc.)
// to be completely excluded from production builds.
