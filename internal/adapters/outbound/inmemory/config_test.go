package inmemory_test

import (
	"context"
	"testing"

	"github.com/pocket/hexagon/spire/internal/adapters/outbound/inmemory"
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
