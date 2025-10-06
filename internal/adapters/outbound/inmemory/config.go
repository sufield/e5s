package inmemory

import (
	"context"

	"github.com/pocket/hexagon/spire/internal/ports"
)

// InMemoryConfig is an outbound adapter that provides hardcoded configuration
// This adapter is responsible only for loading config - not wiring dependencies
type InMemoryConfig struct {
	config *ports.Config
}

// NewInMemoryConfig creates a new in-memory configuration adapter
func NewInMemoryConfig() *InMemoryConfig {
	return &InMemoryConfig{
		config: &ports.Config{
			TrustDomain:   "example.org",
			AgentSpiffeID: "spiffe://example.org/host",
			Workloads: []ports.WorkloadEntry{
				{
					SpiffeID: "spiffe://example.org/server-workload",
					Selector: "unix:user:server-workload",
					UID:      1001,
				},
				{
					SpiffeID: "spiffe://example.org/client-workload",
					Selector: "unix:user:client-workload",
					UID:      1002,
				},
				{
					SpiffeID: "spiffe://example.org/test-workload",
					Selector: "unix:uid:1000",
					UID:      1000,
				},
			},
		},
	}
}

// Load returns the in-memory configuration
func (c *InMemoryConfig) Load(ctx context.Context) (*ports.Config, error) {
	return c.config, nil
}

var _ ports.ConfigLoader = (*InMemoryConfig)(nil)
