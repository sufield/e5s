package main_test

import (
    "context"
    "testing"

    "github.com/pocket/hexagon/spire/internal/app"
    "github.com/pocket/hexagon/spire/internal/adapters/outbound/compose"
    "github.com/pocket/hexagon/spire/internal/adapters/outbound/inmemory"
    "github.com/stretchr/testify/require"
)

func TestMain_BootstrapSuccess(t *testing.T) {
    ctx := context.Background()
    loader := inmemory.NewInMemoryConfig()
    factory := compose.NewInMemoryAdapterFactory()
    _, err := app.Bootstrap(ctx, loader, factory)
    require.NoError(t, err)
}
