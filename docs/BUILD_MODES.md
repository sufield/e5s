# Build Modes: Development vs Production

This document explains build tags that separates development and production code.

## Overview

The codebase uses Go build tags to maintain two distinct deployment modes:

1. **Development Mode** (`dev` tag): Full in-memory implementations for local testing
2. **Production Mode** (default): Minimal code that delegates to external SPIRE infrastructure

## Build Tags

### Development Build (`-tags=dev`)
```bash
go build -tags=dev ./...
go test -tags=dev ./...
go run -tags=dev ./cmd/agent
```

**Includes:**
- `//go:build dev` files
- In-memory implementations (registry, attestor, server, agent)
- Full `AdapterFactory` interface with seeding/configuration methods
- Development-specific bootstrap logic
- Extra test utilities

**Excludes:**
- `//go:build !dev` files (production-only code)

### Production Build (default)
```bash
go build ./...              # No tag needed
go test ./...
go run ./cmd/agent
```

**Includes:**
- Default files (no build tag)
- `//go:build !dev` files
- SPIRE client adapters
- Minimal `CoreAdapterFactory` interface
- Production-specific bootstrap logic

**Excludes:**
- `//go:build dev` files (development-only code)

## File Organization

### Files with `//go:build dev`
These files are **only included in development builds**:

```
internal/adapters/outbound/compose/inmemory.go         # In-memory factory
internal/app/bootstrap_dev.go                          # Dev bootstrap logic
internal/ports/outbound_dev.go                         # Dev-only interfaces
```

### Files with `//go:build !dev`
These files are **only included in production builds**:

```
internal/app/bootstrap_prod.go                         # Production bootstrap logic
```

### Files with no build tag
These files are **included in both modes**:

```
internal/adapters/outbound/compose/spire.go           # SPIRE factory (prod-oriented)
internal/adapters/outbound/compose/doc.go             # Package docs
internal/ports/outbound.go                            # Core interfaces
internal/domain/**                                    # Domain logic
```

## Interface Segregation

### Development Mode Interfaces
```go
// Only available with -tags=dev
type AdapterFactory interface {
    CoreAdapterFactory              // Base methods
    DevelopmentAdapterFactory       // CreateRegistry, CreateAttestor
    RegistryConfigurator            // SeedRegistry, SealRegistry
    AttestorConfigurator            // RegisterWorkloadUID
}

type DevelopmentAdapterFactory interface {
    BaseAdapterFactory
    CreateRegistry() IdentityMapperRegistry
    CreateAttestor() WorkloadAttestor
    CreateDevelopmentServer(...) (IdentityServer, error)
    CreateDevelopmentAgent(...) (Agent, error)  // 7 params - full control
}
```

### Production Mode Interfaces
```go
// Available in both modes, but prod uses only these
type CoreAdapterFactory interface {
    ProductionServerFactory
    ProductionAgentFactory
}

type ProductionAgentFactory interface {
    BaseAdapterFactory
    CreateProductionAgent(ctx, spiffeID, parser) (Agent, error)  // 3 params - clean!
}
```

## Binary Size Comparison

Development mode includes significantly more code:

| Component | Development | Production |
|-----------|-------------|------------|
| In-memory registry | ✅ | ❌ |
| In-memory attestor | ✅ | ❌ |
| In-memory server | ✅ | ❌ |
| Seeding logic | ✅ | ❌ |
| Extra interfaces | ✅ | ❌ |
| SPIRE client | ✅ | ✅ |

**Estimated reduction:** ~30-40% smaller production binary

## Usage Examples

### Development: Local Testing
```go
// Imports work with -tags=dev
import "github.com/pocket/hexagon/spire/internal/ports"

// Full interface available
var factory ports.AdapterFactory = compose.NewInMemoryAdapterFactory()

// Can seed registry
factory.SeedRegistry(registry, ctx, mapper)

// Can configure attestor
factory.RegisterWorkloadUID(attestor, 1000, "unix:uid:1000")

// Can create in-memory components
agent, _ := factory.CreateDevelopmentAgent(ctx, id, server, registry, attestor, parser, provider)
```

### Production: SPIRE Infrastructure
```go
// No build tag needed
import "github.com/pocket/hexagon/spire/internal/ports"

// Only core interface available (dev interfaces don't exist)
var factory ports.CoreAdapterFactory = compose.NewSPIREAdapterFactory(ctx, cfg)

// Clean signature - only what's needed
agent, _ := factory.CreateProductionAgent(ctx, spiffeID, parser)

// No seeding methods (would not compile)
// factory.SeedRegistry(...)  // ❌ Method doesn't exist without -tags=dev
```

## CI/CD Integration

### Testing Both Modes
```yaml
# .github/workflows/test.yml
jobs:
  test-dev:
    runs-on: ubuntu-latest
    steps:
      - run: go test -tags=dev ./...

  test-prod:
    runs-on: ubuntu-latest
    steps:
      - run: go test ./...  # No tag = production
```

### Building for Production
```bash
# Dockerfile
FROM golang:1.21 AS builder
WORKDIR /app
COPY . .
RUN go build -o /bin/agent ./cmd/agent  # Excludes dev code

FROM alpine:latest
COPY --from=builder /bin/agent /bin/agent
CMD ["/bin/agent"]
```

## Makefile Targets

```makefile
# Run development mode
.PHONY: run-dev
run-dev:
	go run -tags=dev ./cmd/agent

# Build production binary
.PHONY: build-prod
build-prod:
	go build -o bin/agent-prod ./cmd/agent

# Test both modes
.PHONY: test-all
test-all:
	go test -tags=dev ./...
	go test ./...

# Check binary sizes
.PHONY: size-compare
size-compare:
	go build -tags=dev -o bin/agent-dev ./cmd/agent
	go build -o bin/agent-prod ./cmd/agent
	ls -lh bin/agent-*
```

## Troubleshooting

### gopls/IDE Issues
If your editor shows "undefined" errors or "No packages found" for development-only code (files with `//go:build dev`), you need to configure gopls to use the `dev` build tag.

**Global Configuration (gopls.yaml):**
The project includes a `gopls.yaml` file at the repository root:
```yaml
# gopls configuration for SPIRE project
build:
  buildFlags: ["-tags=dev"]
  env:
    GOFLAGS: "-tags=dev"
```

This configuration is automatically used by gopls and should fix most IDE issues.

**VS Code (.vscode/settings.json):**
The project also includes VS Code-specific settings:
```json
{
  "go.buildTags": "dev",
  "gopls": {
    "build.buildFlags": ["-tags=dev"]
  }
}
```

**Neovim/Vim:**
```lua
require('lspconfig').gopls.setup({
  settings = {
    gopls = {
      buildFlags = {"-tags=dev"}
    }
  }
})
```

**After changing configuration:**
1. Reload your IDE/editor window
2. Restart the gopls language server
3. Run `Go: Restart Language Server` (VS Code) or `:LspRestart` (Neovim)

### Verification Commands
```bash
# Check what files are included in each mode
go list -tags=dev -f '{{.GoFiles}}' ./internal/adapters/outbound/compose
go list -f '{{.GoFiles}}' ./internal/adapters/outbound/compose

# Verify interfaces compile
go build -tags=dev ./internal/ports/...  # Should include dev interfaces
go build ./internal/ports/...            # Should exclude dev interfaces
```

## Benefits

1. **Smaller Production Binaries**: Excludes development-only code
2. **Cleaner Production APIs**: ISP-compliant interfaces without dev baggage
3. **Security**: No test/dev code in production
4. **Flexibility**: Easy local development without SPIRE infrastructure
5. **Type Safety**: Compile-time enforcement of mode-specific code

## Related Documentation

- [Interface Segregation](./INTERFACE_SEGREGATION.md)
- [SPIRE Integration](./SPIRE_INTEGRATION.md)
- [Development Workflow](./DEVELOPMENT.md)
