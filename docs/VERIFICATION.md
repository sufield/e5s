# SPIRE Adapter Verification Guide

This guide provides steps to verify the SPIRE production adapter implementation works correctly.

## Quick Verification Checklist

- [x] **Build Verification** - Production and dev builds compile successfully
- [x] **Binary Separation** - Production binary excludes dev code
- [x] **Unit Tests** - All existing tests pass
- [x] **Integration Tests** - Live SPIRE connectivity (implemented in `internal/adapters/outbound/spire/integration_test.go`)
- [x] **End-to-End Tests** - Full workload identity flow across multiple services (implemented in `test/e2e/e2e_test.go`)

## 1. Build Verification ✅

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

**Status**: ✅ PASSED

## 2. Unit Test Verification ✅

**IMPORTANT**: Standard unit tests test the **in-memory implementation**, NOT live SPIRE!

```bash
# Run all unit tests (in-memory adapters only)
go test ./...

# Run with coverage
go test -cover ./...

# Note: SPIRE adapter has no unit tests (requires live SPIRE for integration tests)
# Output shows: ?  github.com/pocket/hexagon/spire/internal/adapters/outbound/spire [no test files]
```

**What's tested**:
- ✅ Domain layer (pure business logic)
- ✅ In-memory adapters (dev/test implementation)
- ✅ Application layer (with in-memory backends)
- ✅ Inbound adapters (with mocks)

**What's NOT tested**:
- ❌ SPIRE production adapters (requires integration tests below)
- ❌ Live SPIRE connectivity
- ❌ Actual SVID fetching from SPIRE

**Status**: ✅ PASSED (all in-memory tests pass, 45.8% coverage)

## 3. Static Analysis

Check for common issues:

```bash
# Format check
go fmt ./...

# Vet check
go vet ./...

# Build all packages
go build ./...
```

**Expected**: All commands should complete without errors.

## 4. Integration Testing (Requires SPIRE Infrastructure) ✅

**THIS TESTS THE ACTUAL SPIRE ADAPTERS AGAINST LIVE SPIRE!**

Integration tests verify the SPIRE production adapters work correctly by connecting to a real SPIRE deployment.

**Status**: ✅ **Implemented** in `internal/adapters/outbound/spire/integration_test.go`

**Test Coverage**:
- ✅ `TestSPIREClientConnection` - Verify client can connect to SPIRE Agent
- ✅ `TestFetchX509SVID` - Test X.509 SVID fetching
- ✅ `TestFetchX509Bundle` - Test trust bundle fetching
- ✅ `TestFetchJWTSVID` - Test JWT SVID fetching and validation
- ✅ `TestValidateJWTSVID` - Test JWT token validation
- ✅ `TestAttestation` - Test workload attestation
- ✅ `TestSPIREClientReconnect` - Test client reconnection
- ✅ `TestSPIREClientReconnectFailure` - Test connection failure handling
- ✅ `TestSPIREClientTimeout` - Test timeout handling

### Quick Integration Test

```bash
# 1. Start SPIRE in Minikube
make minikube-up

# 2. Run integration tests
go test -tags=integration ./internal/adapters/outbound/spire/... -v

# Expected output:
# === RUN   TestSPIREClientConnection
# --- PASS: TestSPIREClientConnection (0.05s)
# === RUN   TestFetchX509SVID
# --- PASS: TestFetchX509SVID (0.12s)
#     Fetched SVID for identity: spiffe://example.org/...
# === RUN   TestFetchX509Bundle
# --- PASS: TestFetchX509Bundle (0.08s)
# === RUN   TestFetchJWTSVID
# --- PASS: TestFetchJWTSVID (0.15s)
# === RUN   TestValidateJWTSVID
# --- PASS: TestValidateJWTSVID (0.10s)
# === RUN   TestAttestation
# --- PASS: TestAttestation (0.08s)
# ...
# PASS
# ok  	github.com/pocket/hexagon/spire/internal/adapters/outbound/spire	1.234s
```

### Prerequisites

Start SPIRE infrastructure in Minikube:

```bash
# Start Minikube with SPIRE
make minikube-up

# Verify SPIRE is running
kubectl get pods -n spire-system
# Expected output:
# NAME              READY   STATUS    RESTARTS   AGE
# spire-agent-xxx   1/1     Running   0          1m
# spire-server-0    1/1     Running   0          1m
```

### Test 1: SPIRE Client Connection

Create a simple test to verify the SPIRE client can connect:

```bash
# Create test file
cat > /tmp/test_spire_connection.go <<'EOF'
//go:build integration
package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/pocket/hexagon/spire/internal/adapters/outbound/spire"
)

func main() {
	ctx := context.Background()

	config := &spire.Config{
		SocketPath:  "unix:///tmp/spire-agent/public/api.sock",
		TrustDomain: "example.org",
		Timeout:     30 * time.Second,
	}

	client, err := spire.NewSPIREClient(ctx, *config)
	if err != nil {
		log.Fatalf("Failed to create SPIRE client: %v", err)
	}
	defer client.Close()

	fmt.Println("✅ Successfully connected to SPIRE Agent")
	fmt.Printf("Trust Domain: %s\n", client.GetTrustDomain())
}
EOF

# Run test (requires SPIRE agent running)
go run -tags=integration /tmp/test_spire_connection.go
```

**Expected Output**:
```
✅ Successfully connected to SPIRE Agent
Trust Domain: example.org
```

### Test 2: Fetch X.509 SVID

Test fetching X.509 SVIDs from SPIRE:

```bash
cat > /tmp/test_fetch_svid.go <<'EOF'
//go:build integration
package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/pocket/hexagon/spire/internal/adapters/outbound/spire"
)

func main() {
	ctx := context.Background()

	config := &spire.Config{
		SocketPath:  "unix:///tmp/spire-agent/public/api.sock",
		TrustDomain: "example.org",
		Timeout:     30 * time.Second,
	}

	client, err := spire.NewSPIREClient(ctx, *config)
	if err != nil {
		log.Fatalf("Failed to create SPIRE client: %v", err)
	}
	defer client.Close()

	// Fetch X.509 SVID
	doc, err := client.FetchX509SVID(ctx)
	if err != nil {
		log.Fatalf("Failed to fetch X.509 SVID: %v", err)
	}

	fmt.Println("✅ Successfully fetched X.509 SVID")
	fmt.Printf("Identity: %s\n", doc.IdentityNamespace().String())
	fmt.Printf("Expires: %s\n", doc.ExpiresAt().Format(time.RFC3339))
	fmt.Printf("Valid: %v\n", doc.IsValid())
	fmt.Printf("Certificate CN: %s\n", doc.Certificate().Subject.CommonName)
}
EOF

go run -tags=integration /tmp/test_fetch_svid.go
```

**Expected Output**:
```
✅ Successfully fetched X.509 SVID
Identity: spiffe://example.org/...
Expires: 2024-10-06T16:30:00Z
Valid: true
Certificate CN: ...
```

### Test 3: Fetch Trust Bundle

Test fetching CA certificates:

```bash
cat > /tmp/test_fetch_bundle.go <<'EOF'
//go:build integration
package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/pocket/hexagon/spire/internal/adapters/outbound/spire"
)

func main() {
	ctx := context.Background()

	config := &spire.Config{
		SocketPath:  "unix:///tmp/spire-agent/public/api.sock",
		TrustDomain: "example.org",
		Timeout:     30 * time.Second,
	}

	client, err := spire.NewSPIREClient(ctx, *config)
	if err != nil {
		log.Fatalf("Failed to create SPIRE client: %v", err)
	}
	defer client.Close()

	// Fetch X.509 bundle
	certs, err := client.FetchX509Bundle(ctx)
	if err != nil {
		log.Fatalf("Failed to fetch X.509 bundle: %v", err)
	}

	fmt.Println("✅ Successfully fetched X.509 trust bundle")
	fmt.Printf("CA Certificates: %d\n", len(certs))
	for i, cert := range certs {
		fmt.Printf("  CA %d: %s (expires: %s)\n",
			i+1,
			cert.Subject.CommonName,
			cert.NotAfter.Format(time.RFC3339))
	}
}
EOF

go run -tags=integration /tmp/test_fetch_bundle.go
```

**Expected Output**:
```
✅ Successfully fetched X.509 trust bundle
CA Certificates: 1
  CA 1: example.org (expires: 2025-10-06T12:00:00Z)
```

### Test 4: Production Agent End-to-End

Test the full production agent against SPIRE:

```bash
# Set environment variables
export SPIRE_AGENT_SOCKET="unix:///tmp/spire-agent/public/api.sock"
export SPIRE_TRUST_DOMAIN="example.org"
export WORKLOAD_API_SOCKET="/tmp/hexagon-workload-api.sock"

# Run production agent
./bin/spire-server
```

**Expected Output**:
```
=== SPIRE Agent (Production Mode) ===
Connecting to SPIRE Agent: unix:///tmp/spire-agent/public/api.sock
Trust Domain: example.org
Agent Identity: spiffe://example.org/agent
Workload API socket: /tmp/hexagon-workload-api.sock
Registered workloads: 2
  - spiffe://example.org/workload/server (UID: 1000)
  - spiffe://example.org/workload/client (UID: 1001)

Agent is running. Press Ctrl+C to stop.
```

## 5. Manual Verification Steps

### Verify SPIRE Agent Socket Accessibility

```bash
# Check socket exists
ls -la /tmp/spire-agent/public/api.sock
# Expected: srwxrwxrwx ... /tmp/spire-agent/public/api.sock

# Test socket connectivity (requires spire-agent CLI)
spire-agent api fetch x509 \
  -socketPath /tmp/spire-agent/public/api.sock
```

### Verify go-spiffe Dependency

```bash
# Check dependency is installed
go list -m github.com/spiffe/go-spiffe/v2
# Expected: github.com/spiffe/go-spiffe/v2 v2.x.x

# Verify it's used in binary
go version -m bin/spire-server | grep spiffe
# Expected: dep github.com/spiffe/go-spiffe/v2 v2.x.x
```

## 6. Troubleshooting

### Issue: "Failed to create SPIRE client: connection refused"

**Cause**: SPIRE Agent not running or socket path incorrect

**Solution**:
```bash
# Check SPIRE Agent is running
kubectl get pods -n spire-system | grep spire-agent

# Check socket path in pod
kubectl exec -n spire-system spire-agent-xxx -- ls -la /tmp/spire-agent/public/api.sock

# For local testing, forward the socket (if needed)
kubectl port-forward -n spire-system spire-agent-xxx 8081:8081
```

### Issue: "No X.509 SVIDs available"

**Cause**: Workload not registered with SPIRE Server

**Solution**:
```bash
# Register workload with SPIRE Server
spire-server entry create \
  -parentID spiffe://example.org/agent \
  -spiffeID spiffe://example.org/workload/test \
  -selector unix:uid:1000

# Verify entry created
spire-server entry show
```

### Issue: "Failed to fetch X.509 context: permission denied"

**Cause**: Process not attested or selector mismatch

**Solution**:
- Ensure your process UID matches registered selectors
- Check SPIRE Agent attestation configuration
- Verify workload entry selectors match your process

## 7. End-to-End Test Suite ✅

End-to-end tests verify the complete workload identity flow across multiple services.

**Status**: ✅ **Implemented** in `test/e2e/e2e_test.go`

**Test Coverage**:
- ✅ `TestE2EMultiServiceAttestation` - Verify all services can attest and get valid SVIDs
- ✅ `TestE2EServiceAToServiceBMTLS` - Test Service A → Service B mTLS connection
- ✅ `TestE2EServiceBToServiceCMTLS` - Test Service B → Service C mTLS connection
- ✅ `TestE2EChainedMTLS` - Test complete chain: A → B → C
- ✅ `TestE2EIdentityRotation` - Test SVID rotation mechanism
- ✅ `TestE2EUnauthorizedAccess` - Test unauthorized request rejection
- ✅ `TestE2ETrustBundleValidation` - Test trust bundle retrieval and usage

**Test Architecture**:
```
┌─────────────┐       mTLS        ┌─────────────┐       mTLS        ┌─────────────┐
│  Service A  │ ───────────────> │  Service B  │ ───────────────> │  Service C  │
│  (Client)   │    Authorize B    │  (Server)   │    Authorize C    │ (Database)  │
└─────────────┘                   └─────────────┘                   └─────────────┘
       │                                 │                                 │
       └─────────────────────────────────┴─────────────────────────────────┘
                            Attest via SPIRE Agent
```

**Running E2E Tests**:
```bash
# 1. Start SPIRE infrastructure
make minikube-up

# 2. Deploy test services
kubectl apply -f test/e2e/manifests/

# 3. Create SPIRE registration entries (see test/e2e/README.md)

# 4. Run E2E test suite
go test -tags=e2e ./test/e2e/... -v

# Expected output:
# === RUN   TestE2EMultiServiceAttestation
# --- PASS: TestE2EMultiServiceAttestation (1.23s)
# === RUN   TestE2EServiceAToServiceBMTLS
# --- PASS: TestE2EServiceAToServiceBMTLS (2.45s)
# === RUN   TestE2EServiceBToServiceCMTLS
# --- PASS: TestE2EServiceBToServiceCMTLS (2.31s)
# === RUN   TestE2EChainedMTLS
# --- PASS: TestE2EChainedMTLS (3.12s)
# === RUN   TestE2EIdentityRotation
# --- PASS: TestE2EIdentityRotation (5.67s)
# === RUN   TestE2EUnauthorizedAccess
# --- PASS: TestE2EUnauthorizedAccess (0.89s)
# === RUN   TestE2ETrustBundleValidation
# --- PASS: TestE2ETrustBundleValidation (1.01s)
# PASS
```

See `test/e2e/README.md` for detailed setup instructions, prerequisites, and troubleshooting.

## Summary

### Automated Verification (No SPIRE Required)
- ✅ Build verification: `make prod-build && make dev-build`
- ✅ Unit tests: `go test ./...`
- ✅ Binary inspection: `strings bin/spire-server | grep SPIREClient`

### Integration Tests (Requires SPIRE)
1. Start SPIRE: `make minikube-up`
2. Run integration tests: `go test -tags=integration ./internal/adapters/outbound/spire/... -v`

### End-to-End Tests (Requires SPIRE + Kubernetes)
1. Start SPIRE: `make minikube-up`
2. Deploy test services: `kubectl apply -f test/e2e/manifests/`
3. Create registration entries (see `test/e2e/README.md`)
4. Run E2E tests: `go test -tags=e2e ./test/e2e/... -v`

### Next Steps

**Completed**:
- ✅ Document SPIRE registration requirements (see `docs/REGISTRATION_ENTRY_VERIFICATION.md`)

**Pending**:
- [ ] Create formal integration test suite in `test/integration/`
  - Current: Integration tests exist in `internal/adapters/outbound/spire/integration_test.go`
  - Goal: Move to dedicated `test/integration/` directory with better organization
- [ ] Add CI pipeline for integration tests
  - No CI configuration exists yet (GitHub Actions, GitLab CI, Jenkins, etc.)
  - Integration tests currently run manually with `go test -tags=integration`
- [ ] Add observability (metrics, tracing) to adapters
  - No instrumentation currently in adapter code
  - Consider: Prometheus metrics, OpenTelemetry tracing
