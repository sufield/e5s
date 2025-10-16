# Integration Test Optimization

## Problem That Was Solved

Integration tests need to access the SPIRE Agent socket at `/tmp/spire-agent/public/api.sock`, which lives **inside the Minikube node**, not on the host machine.

**Solution:** Run tests inside Kubernetes where the socket exists (Option A from architecture review).

## Recent Security & Robustness Improvements

**What was fixed (2025-10-16):**
- ✅ **Removed unnecessary privileges** - Dropped `hostPID` and `hostNetwork` (not needed for socket access)
- ✅ **Tolerant label selector** - Works with both `app.kubernetes.io/name=agent` and `app=spire-agent`
- ✅ **Parameterized configuration** - All settings configurable via environment variables
- ✅ **Stricter shell safety** - `set -Eeuo pipefail` with trap for guaranteed cleanup
- ✅ **Explicit permissions** - `chmod +x` after binary copy
- ✅ **Pod reuse option** - `KEEP=true` for faster iteration (2-4x faster)
- ✅ **Resource limits** - CPU/memory constraints on test pods
- ✅ **Better error handling** - Detailed failure messages and automatic cleanup

## Three Implementations

### Standard: `test-integration` (Full project, good for debugging)

**Approach:**
```bash
make test-integration  # Uses scripts/run-integration-tests.sh
```

**How it works:**
1. Creates pod with `golang:1.23` image
2. Mounts socket via `hostPath`
3. Copies **entire project** to pod (~100MB+)
4. Runs `go test` inside pod (downloads dependencies)
5. Cleans up

**When to use:**
- ✅ Debugging test failures
- ✅ Modifying tests frequently
- ✅ First-time setup

**Transfer time:** ~30-60 seconds

---

### Optimized: `test-integration-fast` (Pre-compiled, hardened)

**Approach:**
```bash
make test-integration-fast  # Uses scripts/run-integration-tests-optimized.sh
```

**How it works:**
1. **Compiles test binary locally** (`go test -c`)
2. Creates pod with minimal `debian:bookworm-slim` image
3. Mounts socket via `hostPath` (read-only)
4. Copies **only test binary** (~10MB)
5. Runs binary directly in pod
6. Cleans up (or keeps with `KEEP=true`)

**Security improvements:**
- ✅ No `hostPID` or `hostNetwork`
- ✅ Read-only socket mount
- ✅ Resource limits enforced
- ✅ Tolerant label selectors

**When to use:**
- ✅ Repeated test runs
- ✅ CI/CD pipelines
- ✅ Standard development workflow

**Transfer time:** ~10-15 seconds

---

### CI/Distroless: `test-integration-ci` (Maximum security)

**Approach:**
```bash
make test-integration-ci  # Uses scripts/run-integration-tests-ci.sh
```

**How it works:**
1. **Compiles static test binary** (`CGO_ENABLED=0 go test -c`)
2. Creates pod with `gcr.io/distroless/static-debian12:nonroot`
3. Mounts socket via `hostPath` (read-only)
4. Copies static binary (~10MB)
5. Runs binary directly (no shell available)
6. Cleans up

**Security hardening:**
- ✅ Distroless (no shell, no package manager, minimal attack surface)
- ✅ Static binary (no dependencies)
- ✅ `runAsNonRoot: true`
- ✅ `readOnlyRootFilesystem: true`
- ✅ `allowPrivilegeEscalation: false`
- ✅ Capabilities dropped (`drop: ["ALL"]`)
- ✅ Seccomp profile enabled

**When to use:**
- ✅ Production CI/CD
- ✅ Security-critical environments
- ✅ Compliance requirements

**Transfer time:** ~10-15 seconds

---

### Fast Iteration: `test-integration-keep` (Pod reuse)

**Approach:**
```bash
make test-integration-keep  # Reuses pod for faster iteration
```

**How it works:**
1. First run: Same as `test-integration-fast`
2. Subsequent runs: Only copies binary (~2-3 seconds)
3. Pod stays running between tests
4. Manual cleanup: `kubectl delete pod -n spire-system spire-integration-test`

**When to use:**
- ✅ Rapid local development
- ✅ Testing multiple times in a row
- ✅ Debugging test issues

**Transfer time:**
- First run: ~10-15 seconds
- Subsequent: ~2-3 seconds ⚡

---

## Performance Comparison

| Aspect | Standard | Optimized | CI/Distroless | Keep (repeat) |
|--------|----------|-----------|---------------|---------------|
| **Transfer size** | ~100MB+ | ~10MB | ~10MB | ~10MB |
| **Pod image size** | 800MB+ | 80MB | 25MB | 80MB |
| **Privileges** | Normal | Minimal | Hardened | Minimal |
| **Security** | Standard | Good | Maximum | Good |
| **Dependencies** | Runtime | Compiled | Static | Compiled |
| **Determinism** | Low | High | Maximum | High |
| **First run** | ~30-60s | ~10-15s | ~10-15s | ~10-15s |
| **Repeat run** | ~30-60s | ~10-15s | ~10-15s | ~2-3s ⚡ |
| **CI-friendly** | Fair | Good | Best | N/A |
| **Debugging** | Easy | Good | Limited | Good |

## Quick Decision Guide

**Choose your implementation:**

```
Development (first time)      → make test-integration
Development (normal use)      → make test-integration-fast
Development (rapid iteration) → make test-integration-keep
CI/CD (GitHub Actions, etc.)  → make test-integration-ci
Security-critical envs        → make test-integration-ci
Debugging test failures       → make test-integration
```

## Configuration Examples

All optimized scripts support environment variable configuration:

### Custom Namespace

```bash
NS=my-namespace make test-integration-fast
```

### Custom Socket Path

```bash
SOCKET_DIR=/custom/path SOCKET_FILE=custom.sock make test-integration-fast
```

### Different Package

```bash
PKG=./internal/adapters/inbound/identityserver make test-integration-fast
```

### Rapid Development Workflow

```bash
# First run - creates pod
make test-integration-keep

# Edit your tests...
# vim internal/adapters/outbound/spire/integration_test.go

# Second run - reuses pod (2-3 seconds!)
make test-integration-keep

# More edits...

# Third run - still fast
make test-integration-keep

# Done - cleanup
kubectl delete pod -n spire-system spire-integration-test
```

### CI Pipeline Configuration

**GitHub Actions:**
```yaml
- name: Integration Tests
  run: make test-integration-ci
```

**GitLab CI:**
```yaml
integration-test:
  script:
    - make test-integration-ci
```

## Architecture Review Context

This is **Option A** from the architecture review:

> **Option A — Run the tests inside Kubernetes (recommended)**
>
> Run your integration tests in a pod that mounts the agent socket via `hostPath`.
> Compile tests to a single binary and run it in the pod for **speed and determinism**.

**Why Option A was chosen:**
- ❌ Option B (test Job): Requires building Docker image - too heavyweight
- ❌ Option C (prod binary): Only tests binary, not `go test` suite
- ❌ Option D (UDS bridging): Fragile, negates Workload API design
- ✅ **Option A**: Clean, fast, works with existing test suite

## Implementation Details

### Current Script (`run-integration-tests.sh`)

```bash
# Create pod with full Go toolchain
image: golang:1.23

# Copy entire project
kubectl cp . spire-system/spire-integration-test:/workspace

# Run go test (downloads modules, compiles, runs)
go test -tags=integration -race -v ./internal/adapters/outbound/spire/...
```

### Optimized Script (`run-integration-tests-optimized.sh`)

```bash
# Compile test binary locally
go test -tags=integration -c -o /tmp/spire-integration.test ./internal/adapters/outbound/spire

# Create minimal pod
image: debian:bookworm-slim

# Copy only binary
kubectl cp /tmp/spire-integration.test spire-system/spire-integration-test:/work/integration.test

# Run binary directly
/work/integration.test -test.v
```

## Future: Option B (Kubernetes Job)

For fully automated CI, consider **Option B** - package test binary into a Docker image:

```dockerfile
FROM gcr.io/distroless/base-debian12
COPY integration.test /bin/integration.test
ENTRYPOINT ["/bin/integration.test", "-test.v"]
```

Then create a **Job** that:
- Mounts socket via `hostPath`
- Runs test image
- Reports results
- Auto-cleans up

**Benefits:**
- Zero manual steps
- Perfect for CI pipelines
- Version-controlled test images
- Can run multiple test suites in parallel

## Switching Between Implementations

Both work correctly. Choose based on your workflow:

```bash
# Development workflow (easier debugging)
make test-integration

# CI workflow (faster, more deterministic)
make test-integration-fast

# Or just use the fast one for everything
alias test-it='make test-integration-fast'
```

## Measuring the Difference

Time both approaches:

```bash
# Current
time make test-integration

# Optimized
time make test-integration-fast
```

Expected difference: **~20-40 seconds faster** with optimized version.

## References

- Architecture review: Original detailed options analysis
- `scripts/run-integration-tests.sh`: Current implementation
- `scripts/run-integration-tests-optimized.sh`: Optimized implementation following Option A
