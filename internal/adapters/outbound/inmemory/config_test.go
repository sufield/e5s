package inmemory_test

import (
	"context"
	"testing"

	"github.com/pocket/hexagon/spire/internal/adapters/outbound/inmemory"
	"github.com/pocket/hexagon/spire/internal/dto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInMemoryConfig_Load(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	config, err := inmemory.NewInMemoryConfig().Load(ctx)
	require.NoError(t, err)
	require.NotNil(t, config)
	require.Equal(t, 3, len(config.Workloads))
	require.Equal(t, "example.org", config.TrustDomain)
}

func TestInMemoryConfig_Load_DefensiveCopy(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	loader := inmemory.NewInMemoryConfig()

	// Load config twice
	cfg1, err := loader.Load(ctx)
	require.NoError(t, err)
	cfg2, err := loader.Load(ctx)
	require.NoError(t, err)

	// Modify first config's workloads
	originalLen := len(cfg1.Workloads)
	cfg1.Workloads[0].UID = 9999
	cfg1.Workloads = append(cfg1.Workloads, dto.WorkloadEntry{
		SpiffeID: "spiffe://example.org/modified",
		Selector: "unix:uid:9999",
		UID:      9999,
	})

	// Second config should be unaffected (defensive copy)
	assert.Equal(t, originalLen, len(cfg2.Workloads), "Second config should not see modifications to first config")
	assert.NotEqual(t, 9999, cfg2.Workloads[0].UID, "Second config workload UID should be unchanged")
}

func TestInMemoryConfig_Validation_TrustDomainMismatch(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	// Create config with mismatched trust domain
	loader := inmemory.NewInMemoryConfig(func(cfg *dto.Config) {
		cfg.AgentSpiffeID = "spiffe://different.org/agent"
	})

	_, err := loader.Load(ctx)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "must be in trust domain")
}

func TestInMemoryConfig_Validation_SelectorFormat(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	// Create config with wrong selector format
	loader := inmemory.NewInMemoryConfig(func(cfg *dto.Config) {
		cfg.Workloads[0].Selector = "unix:user:wrong-format"
	})

	_, err := loader.Load(ctx)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "does not match UID format")
}

func TestInMemoryConfig_WithOptions(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	// Create config with custom trust domain
	loader := inmemory.NewInMemoryConfig(func(cfg *dto.Config) {
		cfg.TrustDomain = "custom.org"
		cfg.AgentSpiffeID = "spiffe://custom.org/agent"
		cfg.Workloads = []dto.WorkloadEntry{
			{
				SpiffeID: "spiffe://custom.org/workload",
				Selector: "unix:uid:5000",
				UID:      5000,
			},
		}
	})

	config, err := loader.Load(ctx)
	require.NoError(t, err)
	assert.Equal(t, "custom.org", config.TrustDomain)
	assert.Equal(t, "spiffe://custom.org/agent", config.AgentSpiffeID)
	assert.Equal(t, 1, len(config.Workloads))
	assert.Equal(t, "spiffe://custom.org/workload", config.Workloads[0].SpiffeID)
}
