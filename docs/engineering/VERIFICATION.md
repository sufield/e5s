# SPIRE Adapter Verification Guide

This guide covers verification of the SPIRE production adapter implementation.

## Build Verification

Verify the production build compiles:

```bash
# Production build
make build
# Expected: Binary builds successfully

# Verify SPIRE adapter included in binary
go list -deps ./cmd/... | grep "github.com/spiffe/go-spiffe"
# Expected: Lists go-spiffe dependencies

# Check that build excludes test files
go list -f '{{.TestGoFiles}}' ./...
# Expected: Test files not included in build
```

## Unit Tests

Standard unit tests verify domain logic, configuration parsing, and other components without requiring SPIRE infrastructure.

```bash
# Run all unit tests
go test ./...

# Run with coverage
go test -cover ./...

# Run property-based tests (verify algebraic invariants)
go test -run Properties ./...

# Run with race detection
go test -race ./...
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

# Or run directly (inside test pod with SPIRE socket access)
go test ./internal/adapters/outbound/spire/... -v
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

### Manual SPIRE Verification

Verify SPIRE is running and accessible:

```bash
# Check SPIRE pods are running
kubectl get pods -n spire-system

# Check SPIRE agent socket exists
kubectl exec -n spire-system deploy/spire-agent -- ls -la /run/spire/sockets/agent.sock

# View SPIRE server entries
kubectl exec -n spire-system deploy/spire-server -- \
  /opt/spire/bin/spire-server entry show
```

## Summary

### Automated Verification (No SPIRE Required)
```bash
make build                          # Build verification
go test ./...                       # Unit tests
go test -run Properties ./...      # Property-based tests
go test -race ./...                # Race detection
make verify                         # Full verification suite
```

### Integration Tests (Requires SPIRE)
```bash
make minikube-up                   # Start SPIRE infrastructure
make test-integration              # Run integration tests against live SPIRE
```

### Complete Verification Workflow
```bash
# 1. Local verification (fast, no dependencies)
make verify

# 2. Integration verification (requires Minikube)
make minikube-up
make test-integration

# 3. Cleanup
make minikube-down
```

## Troubleshooting

If you encounter issues during verification or testing, see [`docs/guide/TROUBLESHOOTING.md`](../guide/TROUBLESHOOTING.md) for detailed diagnostics and solutions, including:

- Connection refused errors
- SPIRE socket access issues
- Registration and permission problems
- Integration testing specific issues

For build or dependency issues, run:
```bash
# Check dependencies are installed
go list -m github.com/spiffe/go-spiffe/v2

# Verify Go version
go version  # Should be 1.21 or higher

# Clean and rebuild
go clean -cache
make build
```
