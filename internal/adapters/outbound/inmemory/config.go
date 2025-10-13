//go:build dev
// +build dev

package inmemory

import (
	"context"
	"fmt"
	"strings"

	"github.com/pocket/hexagon/spire/internal/ports"
)

// InMemoryConfig is an outbound adapter that provides hardcoded configuration
// This adapter is responsible only for loading config - not wiring dependencies
type InMemoryConfig struct {
	config *ports.Config
}

// NewInMemoryConfig creates a new in-memory configuration adapter.
// Optional functional options can override default values for tests.
func NewInMemoryConfig(opts ...func(*ports.Config)) *InMemoryConfig {
	cfg := &ports.Config{
		TrustDomain:   "example.org",
		AgentSpiffeID: "spiffe://example.org/host",
		Workloads: []ports.WorkloadEntry{
			{
				SpiffeID: "spiffe://example.org/server-workload",
				Selector: "unix:uid:1001",
				UID:      1001,
			},
			{
				SpiffeID: "spiffe://example.org/client-workload",
				Selector: "unix:uid:1002",
				UID:      1002,
			},
			{
				SpiffeID: "spiffe://example.org/test-workload",
				Selector: "unix:uid:1000",
				UID:      1000,
			},
		},
	}
	for _, o := range opts {
		o(cfg)
	}
	return &InMemoryConfig{config: cfg}
}

// Load returns a defensive copy of the in-memory configuration.
// Returns a copy to prevent callers from mutating shared state across tests.
func (c *InMemoryConfig) Load(ctx context.Context) (*ports.Config, error) {
	// Dev-only consistency checks: fail fast on misconfiguration
	if err := validateConfig(c.config); err != nil {
		return nil, fmt.Errorf("inmemory: invalid config: %w", err)
	}

	// Shallow copy of config
	cfg := *c.config

	// Deep copy workloads slice to prevent mutation
	if len(c.config.Workloads) > 0 {
		wl := make([]ports.WorkloadEntry, len(c.config.Workloads))
		copy(wl, c.config.Workloads)
		cfg.Workloads = wl
	}

	return &cfg, nil
}

// validateConfig performs dev-only consistency checks
func validateConfig(cfg *ports.Config) error {
	if cfg == nil {
		return fmt.Errorf("config cannot be nil")
	}
	if cfg.TrustDomain == "" {
		return fmt.Errorf("trust domain cannot be empty")
	}
	if cfg.AgentSpiffeID == "" {
		return fmt.Errorf("agent SPIFFE ID cannot be empty")
	}

	// Ensure agent SPIFFE ID matches trust domain
	expectedPrefix := "spiffe://" + cfg.TrustDomain + "/"
	if !strings.HasPrefix(cfg.AgentSpiffeID, expectedPrefix) {
		return fmt.Errorf("agent SPIFFE ID %q must be in trust domain %q", cfg.AgentSpiffeID, cfg.TrustDomain)
	}

	// Ensure all workload SPIFFE IDs match trust domain
	for i, wl := range cfg.Workloads {
		if !strings.HasPrefix(wl.SpiffeID, expectedPrefix) {
			return fmt.Errorf("workload[%d] SPIFFE ID %q must be in trust domain %q", i, wl.SpiffeID, cfg.TrustDomain)
		}
		// Ensure selector format matches what attestor emits (unix:uid:<UID>)
		if wl.Selector != fmt.Sprintf("unix:uid:%d", wl.UID) {
			return fmt.Errorf("workload[%d] selector %q does not match UID format unix:uid:%d", i, wl.Selector, wl.UID)
		}
	}

	return nil
}

var _ ports.ConfigLoader = (*InMemoryConfig)(nil)
