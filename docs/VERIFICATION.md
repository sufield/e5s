# SPIRE Adapter Verification Guide

This guide provides commands to verify the SPIRE production adapter implementation.

## Build Verification

Verify both dev and production builds compile:

```bash
# Production build
make prod-build
# Expected: bin/spire-server (~13MB)

# Dev build
make dev-build
# Expected: bin/cp-minikube (~3MB)

# Verify build separation
strings bin/spire-server | grep -c "BootstrapMinikubeInfra"
# Expected: 0 (no dev code)

strings bin/cp-minikube | grep -c "BootstrapMinikubeInfra"
# Expected: >0 (dev code present)

# Verify SPIRE adapter included
strings bin/spire-server | grep "SPIREClient"
# Expected: *spire.SPIREClient
```

## Unit Tests

Standard unit tests use in-memory implementations, not live SPIRE.

```bash
# Run all unit tests (in-memory adapters only)
go test ./...

# Run with coverage
go test -cover ./...
```

## Static Analysis

```bash
# Format check
go fmt ./...

# Vet check
go vet ./...

# Build all packages
go build ./...
```

## Integration Tests (Requires SPIRE Infrastructure)

Integration tests verify SPIRE production adapters against real SPIRE deployment.

### Quick Integration Test

```bash
# 1. Start SPIRE in Minikube
make minikube-up

# 2. Run integration tests
make test-integration

# Or run directly
go test -tags=integration ./internal/adapters/outbound/spire/... -v
```

**Test Coverage**:
- Client connection to SPIRE Agent
- X.509 SVID fetching
- Trust bundle fetching
- Client reconnection handling
- Timeout handling

### Integration Test Architecture

Tests run inside Kubernetes cluster with access to SPIRE agent socket:

```
┌─────────────────────────────────────────────────────────────┐
│ Your Host Machine                                            │
│  $ make test-integration                                     │
│     └─> kubectl exec (runs tests in pod)                    │
└──────────────────────────────────────────────────────────────┘
                             │
           ┌─────────────────▼──────────────────┐
           │ Minikube Cluster                   │
           │                                    │
           │  ┌────────────────────────────┐    │
           │  │ Minikube Node Filesystem   │    │
           │  │  /tmp/spire-agent/public/  │    │
           │  │         api.sock           │    │
           │  └───────▲─────────▲──────────┘    │
           │          │         │               │
           │    ┌─────┴────┐  ┌─┴────────────┐  │
           │    │ SPIRE    │  │ Test Pod     │  │
           │    │ Agent    │  │ golang:1.23  │  │
           │    │          │  │              │  │
           │    │ Creates  │  │ Mounts via   │  │
           │    │ socket   │  │ hostPath     │  │
           │    │          │  │              │  │
           │    │          │  │ Runs:        │  │
           │    │          │  │ go test ...  │  │
           │    └──────────┘  └──────────────┘  │
           │                                    │
           └────────────────────────────────────┘
```

- Test pod uses `hostPath` volume to access SPIRE agent socket on Minikube node
- Tests execute inside the pod where socket is accessible
- No socket exposure to host machine needed

### Manual SPIRE Tests

Test SPIRE connectivity interactively:

```bash
# Start SPIRE and run dev CLI
make minikube-up
go run -tags=dev ./cmd
```

This connects to the SPIRE agent and fetches identities automatically.

## Troubleshooting

### Connection Refused

```bash
# Check SPIRE Agent is running
kubectl get pods -n spire-system | grep spire-agent

# Check socket exists in pod
kubectl exec -n spire-system spire-agent-xxx -- ls -la /tmp/spire-agent/public/api.sock
```

### No X.509 SVIDs Available

Workload must be registered with SPIRE Server:

```bash
# Register workload
spire-server entry create \
  -parentID spiffe://example.org/agent \
  -spiffeID spiffe://example.org/workload/test \
  -selector unix:uid:1000

# Verify entry created
spire-server entry show
```

### Permission Denied

- Ensure process UID matches registered selectors
- Check SPIRE Agent attestation configuration
- Verify workload entry selectors

## Dependency Verification

```bash
# Check go-spiffe dependency installed
go list -m github.com/spiffe/go-spiffe/v2

# Verify it's used in binary
go version -m bin/spire-server | grep spiffe
```

## Summary

### Automated Verification (No SPIRE Required)
```bash
make prod-build && make dev-build  # Build verification
go test ./...                       # Unit tests
strings bin/spire-server | grep SPIREClient  # Binary inspection
```

### Integration Tests (Requires SPIRE)
```bash
make minikube-up        # Start SPIRE
make test-integration   # Run integration tests
```
