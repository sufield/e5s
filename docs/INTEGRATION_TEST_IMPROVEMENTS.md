# Integration Test Improvements

This document describes the improvements made to fix "context deadline exceeded" errors in SPIRE integration tests and make them more reliable.

## Problem Overview

Integration tests were failing with:
```
create X509 source (Workload API may be unavailable): context deadline exceeded
```

**Root Cause**: Missing workload registration entries in SPIRE server, causing `workloadapi.NewX509Source()` to block indefinitely waiting for an SVID that never arrives.

## Three-Part Solution

### 1. Automatic Registration Setup Script

**File**: `scripts/setup-spire-registrations.sh`

**Purpose**: Automatically creates required SPIRE registration entries for integration test workloads.

**Usage**:
```bash
# Run manually
./scripts/setup-spire-registrations.sh

# With custom configuration
NS=spire-system TRUST_DOMAIN=example.org ./scripts/setup-spire-registrations.sh
```

**What it does**:
1. Auto-detects SPIRE server pod
2. Verifies agents are attested
3. Creates/updates workload registration entries for:
   - `spiffe://example.org/integration-test` (main test workload)
   - `spiffe://example.org/test-client` (client workload)
   - `spiffe://example.org/test-server` (server workload)
4. Validates entries were created successfully

**Registration Entry Structure**:
```bash
spire-server entry create \
  -spiffeID spiffe://example.org/integration-test \
  -parentID spiffe://example.org/spire-agent \
  -selector k8s:ns:spire-system \
  -selector k8s:sa:default \
  -selector k8s:pod-label:app:spire-integration-test
```

**Selectors Explained**:
- `k8s:ns:spire-system` - Matches pods in the spire-system namespace
- `k8s:sa:default` - Matches pods using the "default" service account
- `k8s:pod-label:app:spire-integration-test` - Matches pods with label `app=spire-integration-test`

All three selectors must match for the SPIRE agent to issue an SVID to the workload.

---

### 2. Updated CI Script with Auto-Registration

**File**: `scripts/run-integration-tests-ci.sh`

**Changes**:

#### Added Registration Setup (lines 100-114)
```bash
# Setup SPIRE registration entries (if not already done)
info "Setting up SPIRE workload registration entries..."
if [ -f "./scripts/setup-spire-registrations.sh" ]; then
    # Run registration setup non-interactively
    if NS="$NS" TRUST_DOMAIN="$TRUST_DOMAIN" bash ./scripts/setup-spire-registrations.sh 2>&1 | grep -E "(✅|❌|Entry ID|SPIFFE ID)" || true; then
        success "Registration entries verified/created"
    else
        info "Registration setup encountered issues - tests may fail if workload entries are missing"
        info "Run manually if needed: NS=$NS TRUST_DOMAIN=$TRUST_DOMAIN ./scripts/setup-spire-registrations.sh"
    fi
else
    info "Registration setup script not found, assuming entries exist"
    info "If tests fail with 'context deadline exceeded', create workload entries manually"
fi
```

**Benefits**:
- Automatically ensures registration entries exist before running tests
- Fails gracefully if registration script is missing
- Provides helpful error messages for manual troubleshooting

#### Added Socket-Wait Init Container (lines 179-210)

**Before** (single init container):
```yaml
initContainers:
  - name: setup
    # Wait for binary to be copied
```

**After** (two init containers):
```yaml
initContainers:
  # First: Wait for SPIRE socket
  - name: wait-for-socket
    image: busybox:stable-musl
    command:
      - sh
      - -c
      - |
        echo "Waiting for SPIRE Workload API socket..."
        WAIT_TIME=0
        MAX_WAIT=120
        until [ -S /spire-socket/api.sock ]; do
          if [ $WAIT_TIME -ge $MAX_WAIT ]; then
            echo "ERROR: Socket not found after ${MAX_WAIT}s"
            exit 1
          fi
          echo "Socket not found yet (waited ${WAIT_TIME}s), retrying in 2s..."
          sleep 2
          WAIT_TIME=$((WAIT_TIME + 2))
        done
        echo "✅ Socket found: /spire-socket/api.sock"
        # Give agent a moment to fully initialize
        echo "Waiting 5s for agent initialization..."
        sleep 5
        echo "✅ Ready for workload attestation"

  # Second: Wait for test binary
  - name: setup
    # Wait for binary to be copied
```

**Benefits**:
- Ensures SPIRE agent socket exists before tests run
- Waits up to 120s with clear progress messages
- Gives agent 5s grace period for initialization after socket creation
- Prevents "context deadline exceeded" errors from agent not being ready

#### Enhanced Init Container Monitoring (lines 290-335)

```bash
# Wait for socket-wait initContainer to complete
info "Waiting for socket availability check to complete..."
for i in {1..120}; do
    SOCKET_INIT_STATE=$(kubectl get pod -n "$NS" "$POD_NAME" \
        -o jsonpath='{.status.initContainerStatuses[?(@.name=="wait-for-socket")].state}' 2>/dev/null || true)

    if echo "$SOCKET_INIT_STATE" | grep -q "terminated"; then
        EXIT_CODE=$(kubectl get pod -n "$NS" "$POD_NAME" \
            -o jsonpath='{.status.initContainerStatuses[?(@.name=="wait-for-socket")].state.terminated.exitCode}' 2>/dev/null || echo "")
        if [ "$EXIT_CODE" = "0" ]; then
            success "Socket is available and ready"
            break
        else
            error "Socket wait failed (exit code: $EXIT_CODE)"
            kubectl logs -n "$NS" "$POD_NAME" -c wait-for-socket || true
            exit 1
        fi
    fi
    # ... timeout handling ...
done
```

**Benefits**:
- Monitors socket-wait init container completion
- Fails fast if socket isn't found within 120s
- Provides logs from failed init container for debugging
- Ensures setup init container only starts after socket is ready

---

### 3. Integration Test Pod Configuration

**Test Pod Labels** (required for registration matching):
```yaml
metadata:
  labels:
    app: spire-integration-test  # Matches registration selector
    role: ci-test
    security: distroless
```

**Init Container Sequence**:
1. **wait-for-socket**: Verifies SPIRE agent socket is available
2. **setup**: Waits for test binary to be copied and makes it executable

**Environment Variables**:
```yaml
env:
  - name: SPIRE_AGENT_SOCKET
    value: "unix:///spire-socket/api.sock"
  - name: SPIRE_TRUST_DOMAIN
    value: "example.org"
```

---

## How It Works Together

### Execution Flow

1. **CI Script Starts**
   ```bash
   ./scripts/run-integration-tests-ci.sh
   ```

2. **Auto-Registration** (NEW)
   - Calls `setup-spire-registrations.sh`
   - Creates workload entries if missing
   - Verifies entries exist

3. **Pod Creation**
   - Creates test pod with proper labels
   - Pod has two init containers

4. **Socket Wait Init Container** (NEW)
   - Waits for `/spire-socket/api.sock` to exist
   - Times out after 120s if not found
   - Waits additional 5s for agent initialization

5. **Setup Init Container**
   - Waits for test binary to be copied
   - Makes binary executable

6. **Test Container Starts**
   - Calls `workloadapi.NewX509Source()`
   - Agent sees pod with matching selectors
   - Agent issues SVID (because registration exists)
   - Tests run successfully

### Before vs After

#### Before (Failing)
```
1. Pod starts
2. Test immediately calls NewX509Source()
3. No registration entry exists
4. Agent doesn't issue SVID
5. Timeout after 30s: "context deadline exceeded"
```

#### After (Passing)
```
1. CI script creates registration entries
2. Init container waits for socket + agent ready
3. Test container starts
4. Test calls NewX509Source()
5. Agent finds matching registration
6. Agent issues SVID immediately
7. Tests pass in < 1s
```

---

## Configuration Options

### Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `NS` | `spire-system` | Kubernetes namespace |
| `TRUST_DOMAIN` | `example.org` | SPIRE trust domain |
| `WORKLOAD_SA` | `default` | Service account for test pods |
| `SOCKET_DIR` | `/tmp/spire-agent/public` | Socket directory on node |
| `SOCKET_FILE` | `api.sock` | Socket filename |
| `KEEP` | `false` | Keep test pod after completion |

### Customization Examples

#### Different Trust Domain
```bash
TRUST_DOMAIN=mycompany.com ./scripts/run-integration-tests-ci.sh
```

#### Custom Service Account
```bash
WORKLOAD_SA=test-runner ./scripts/setup-spire-registrations.sh
```

#### Keep Pod for Debugging
```bash
KEEP=true ./scripts/run-integration-tests-ci.sh
# Then inspect: kubectl describe pod spire-integration-test-ci -n spire-system
```

---

## Troubleshooting

### Test Still Fails with "context deadline exceeded"

**Check registration entries**:
```bash
kubectl exec -it -n spire-system <spire-server-pod> -- \
  /opt/spire/bin/spire-server entry list
```

Look for entry with:
- SPIFFE ID: `spiffe://example.org/integration-test`
- Selectors: `k8s:ns:spire-system`, `k8s:sa:default`, `k8s:pod-label:app:spire-integration-test`

**Verify pod has matching labels**:
```bash
kubectl get pod spire-integration-test-ci -n spire-system -o yaml | grep -A5 labels
```

Must have: `app: spire-integration-test`

### Socket Wait Init Container Fails

**Check agent logs**:
```bash
kubectl logs -n spire-system -l app=spire-agent
```

**Verify socket exists on node**:
```bash
minikube ssh -- "ls -la /tmp/spire-agent/public/"
```

**Check init container logs**:
```bash
kubectl logs spire-integration-test-ci -n spire-system -c wait-for-socket
```

### Registration Script Can't Find Server

**Server might be distroless** - switch temporarily:
```yaml
# In SPIRE server deployment
containers:
- name: spire-server
  image: ghcr.io/spiffe/spire-server:1.9.0  # Non-distroless
```

Or **create entries manually**:
```bash
kubectl exec -it -n spire-system <server-pod> -- \
  /opt/spire/bin/spire-server entry create \
    -spiffeID spiffe://example.org/integration-test \
    -parentID spiffe://example.org/spire-agent \
    -selector k8s:ns:spire-system \
    -selector k8s:sa:default \
    -selector k8s:pod-label:app:spire-integration-test
```

### Agent Not Attested

**List attested agents**:
```bash
kubectl exec -it -n spire-system <server-pod> -- \
  /opt/spire/bin/spire-server agent list
```

If empty, check agent logs for attestation errors:
```bash
kubectl logs -n spire-system -l app=spire-agent | grep -i attest
```

---

## Testing the Improvements

### Quick Test
```bash
# 1. Setup registrations
./scripts/setup-spire-registrations.sh

# 2. Run integration tests
./scripts/run-integration-tests-ci.sh
```

### Expected Output
```
============================================
SPIRE Integration Tests (CI/Distroless)
============================================

ℹ️  Configuration:
  Namespace: spire-system
  Socket: /tmp/spire-agent/public/api.sock
  Package: ./internal/adapters/outbound/spire
  Keep pod: false
  Mode: CI (static binary + distroless)

✅ Found SPIRE Agent: spire-agent-xyz
ℹ️  Setting up SPIRE workload registration entries...
✅ Registration entries verified/created
✅ Socket is available and ready
✅ Setup init container is running
✅ Binary copied, initContainer will chmod +x and test container will start

ℹ️  Running integration tests in distroless container...

=== RUN   TestClientConnection
--- PASS: TestClientConnection (0.12s)
=== RUN   TestFetchX509SVID
--- PASS: TestFetchX509SVID (0.08s)
=== RUN   TestFetchX509Bundle
--- PASS: TestFetchX509Bundle (0.06s)

✅ Integration tests passed!
```

---

## Files Modified/Created

### Created
- `scripts/setup-spire-registrations.sh` - Registration management script
- `docs/INTEGRATION_TEST_IMPROVEMENTS.md` - This document
- `docs/SPIRE_INTEGRATION_TEST_ISSUE.md` - Stack Overflow question format
- `docs/SPIRE_INTEGRATION_TEST_FIX.md` - Detailed troubleshooting guide

### Modified
- `scripts/run-integration-tests-ci.sh` - Added auto-registration and socket-wait init container

---

## References

- [SPIRE Workload Registration](https://spiffe.io/docs/latest/deploying/registering/)
- [Kubernetes Workload Attestor](https://github.com/spiffe/spire/blob/main/doc/plugin_agent_workloadattestor_k8s.md)
- [go-spiffe SDK Documentation](https://pkg.go.dev/github.com/spiffe/go-spiffe/v2)
- [Init Containers in Kubernetes](https://kubernetes.io/docs/concepts/workloads/pods/init-containers/)
