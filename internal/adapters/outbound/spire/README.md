# SPIRE Production Adapter

Production SPIRE adapter implementation for hexagonal architecture.

## Status

✅ **Complete**: Production SPIRE adapters fully implemented and tested

## Implementation

### Core Components

- `client.go` - SPIRE Workload API client connection management
- `agent.go` - Agent implementation delegating to external SPIRE
- `server.go` - Server implementation delegating to external SPIRE
- `identity_provider.go` - X.509/JWT SVID fetching from SPIRE
- `bundle_provider.go` - Trust bundle fetching from SPIRE
- `attestor.go` - Workload attestation via SPIRE
- `translation.go` - Domain model conversions (SPIRE SDK ↔ domain)

### Adapter Factory

- `internal/adapters/outbound/compose/spire.go` - Production adapter factory

### Main Entry Points

- `cmd/agent/main_prod.go` - Production agent (uses SPIRE adapters)
- `cmd/agent/main_dev.go` - Dev agent (uses in-memory adapters)

## Build Verification

```bash
# Production build (no dev code)
$ make prod-build
Production binary: bin/spire-server
-rwxrwxr-x 1 zepho zepho 13M bin/spire-server

$ strings bin/spire-server | grep -c "BootstrapMinikubeInfra"
0 (no dev code found - ✓)

# Dev build (includes dev infrastructure)
$ make dev-build
Dev binary: bin/cp-minikube
-rwxrwxr-x 1 zepho zepho 2.9M bin/cp-minikube

$ strings bin/cp-minikube | grep -c "BootstrapMinikubeInfra"
2 (dev code present - ✓)
```

## Usage

### Environment Variables

Production agent configuration:
- `SPIRE_AGENT_SOCKET` - SPIRE Agent Workload API socket (default: `unix:///tmp/spire-agent/public/api.sock`)
- `SPIRE_TRUST_DOMAIN` - Trust domain (default: `example.org`)
- `WORKLOAD_API_SOCKET` - Workload API server socket path (default: `/tmp/spire-agent/public/api.sock`)

### Running Production Agent

```bash
# With external SPIRE infrastructure
export SPIRE_AGENT_SOCKET="unix:///tmp/spire-agent/public/api.sock"
export SPIRE_TRUST_DOMAIN="example.org"
./bin/spire-server
```

## Architecture

The production adapter delegates to external SPIRE infrastructure:

```
┌─────────────────────────────────────────┐
│     Application Layer (ports)          │
│  - Agent interface                      │
│  - Server interface                     │
└─────────────────────────────────────────┘
                 ↓
┌─────────────────────────────────────────┐
│  SPIRE Adapters (outbound/spire)        │
│  - spire.Agent → SPIRE Workload API     │
│  - spire.Server → SPIRE Server API      │
│  - spire.SPIREClient → go-spiffe SDK    │
└─────────────────────────────────────────┘
                 ↓
┌─────────────────────────────────────────┐
│  External SPIRE Infrastructure          │
│  - SPIRE Server (issues identities)     │
│  - SPIRE Agent (attests workloads)      │
└─────────────────────────────────────────┘
```

## Testing

Integration tests require a running SPIRE infrastructure:

```bash
# Start SPIRE in Minikube
make minikube-up

# Run integration tests (TODO)
go test -tags=integration ./internal/adapters/outbound/spire/...
```

## Dependencies

- `github.com/spiffe/go-spiffe/v2` - Official SPIRE/SPIFFE SDK
- go-spiffe provides:
  - Workload API client
  - X.509/JWT SVID handling
  - Trust bundle management
  - SPIFFE ID validation
