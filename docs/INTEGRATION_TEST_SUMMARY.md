# Integration Test Fix - Quick Summary

## Problem
Integration tests failed with `context deadline exceeded` when calling `workloadapi.NewX509Source()`.

## Root Cause
Missing workload registration entries in SPIRE server - agent couldn't issue SVIDs to test workloads.

## Solution (3 Parts)

### 1. Registration Setup Script ✨ NEW
**File**: `scripts/setup-spire-registrations.sh`

Automatically creates SPIRE registration entries for test workloads.

**Usage**:
```bash
./scripts/setup-spire-registrations.sh
```

**Creates**:
- `spiffe://example.org/integration-test`
- `spiffe://example.org/test-client`
- `spiffe://example.org/test-server`

### 2. Updated CI Script ✨ IMPROVED
**File**: `scripts/run-integration-tests-ci.sh`

**Changes**:
- ✅ Automatically calls registration script before running tests
- ✅ Added socket-wait init container (waits for SPIRE agent to be ready)
- ✅ Enhanced monitoring of init container completion

### 3. Socket-Wait Init Container ✨ NEW
Added to test pod YAML (lines 179-210):

```yaml
initContainers:
  # 1. Wait for SPIRE socket (up to 120s)
  - name: wait-for-socket
    # Verifies socket exists + agent initialized

  # 2. Wait for test binary
  - name: setup
    # Prepares test binary
```

## Quick Start

### First Time Setup

#### If SPIRE Server is Distroless (Common Issue)
```bash
# 1. Enable shell access in SPIRE server
make spire-server-shell-enable

# 2. Create registration entries
make register-test-workload

# 3. Run integration tests
make test-integration-ci

# 4. Optional: Switch back to distroless
make spire-server-shell-disable
```

#### If SPIRE Server Has Shell Access
```bash
# Create registration entries
./scripts/setup-spire-registrations.sh

# Run integration tests
./scripts/run-integration-tests-ci.sh
```

### Subsequent Runs
```bash
# CI script now auto-creates registrations if needed
./scripts/run-integration-tests-ci.sh
```

## Before vs After

### Before ❌
```
Test pod starts → NewX509Source() → No registration → Timeout (30s)
```

### After ✅
```
CI creates registrations → Init waits for socket → NewX509Source() → Gets SVID → Tests pass (<1s)
```

## Expected Test Output

**Success**:
```
✅ Registration entries verified/created
✅ Socket is available and ready
✅ Integration tests passed!
```

**Failure (no registration)**:
```
=== FAIL: TestClientConnection (30.00s)
    Error: create X509 source: context deadline exceeded
```

**Fix**: Run `./scripts/setup-spire-registrations.sh`

## Troubleshooting

| Issue | Solution |
|-------|----------|
| Tests still timeout | Verify registration: `kubectl exec <server-pod> -- spire-server entry list` |
| Socket wait fails | Check agent logs: `kubectl logs -l app=spire-agent` |
| Can't create entries (distroless) | Run `make spire-server-shell-enable` then `make register-test-workload` |
| "executable file not found" | Server is distroless - see [Distroless Workaround](SPIRE_DISTROLESS_WORKAROUND.md) |

## Key Files

| File | Purpose |
|------|---------|
| `scripts/spire-server-enable-shell.sh` | Switch SPIRE server image (distroless ↔ non-distroless) |
| `scripts/setup-spire-registrations.sh` | Create/verify SPIRE registrations |
| `scripts/run-integration-tests-ci.sh` | Run integration tests (auto-registration) |
| `docs/SPIRE_DISTROLESS_WORKAROUND.md` | Fix distroless server issues |
| `docs/INTEGRATION_TEST_IMPROVEMENTS.md` | Full documentation |
| `docs/SPIRE_INTEGRATION_TEST_FIX.md` | Detailed troubleshooting guide |

## Related Documentation

- [Integration Test Improvements (Full Docs)](INTEGRATION_TEST_IMPROVEMENTS.md)
- [SPIRE Integration Test Fix Guide](SPIRE_INTEGRATION_TEST_FIX.md)
- [SPIRE Integration Test Issue (SO Format)](SPIRE_INTEGRATION_TEST_ISSUE.md)
