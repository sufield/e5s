# Integration Test Optimization

## Problem

Integration tests need to access the SPIRE Agent socket at `/tmp/spire-agent/public/api.sock`, which lives inside the Minikube node, not on the host machine.

**Solution:** 

Run tests inside Kubernetes where the socket exists (Option A from architecture review).

## Recent Security & Robustness Improvements

- **Removed unnecessary privileges** - Dropped `hostPID` and `hostNetwork` (not needed for socket access)
- **Tolerant label selector** - Works with multiple label patterns (`app.kubernetes.io/name`, `app`, `name`)
- **Parameterized configuration** - All settings configurable via environment variables
- **Stricter shell safety** - `set -Eeuo pipefail` with trap for guaranteed cleanup
- **Explicit permissions** - `chmod +x` after binary copy
- **Pod reuse option** - `KEEP=true` for faster iteration (12x faster)
- **Resource limits** - CPU/memory constraints on test pods
- **Better error handling** - Detailed failure messages and automatic cleanup
- **Cross-platform support** - Auto-detects node architecture (ARM/x86)
- **Optimized by default** - Pre-compiled binary approach is now the standard

## Three Implementations

### Standard: `test-integration` (Optimized, recommended)

**Approach:**
```bash
make test-integration  # Uses scripts/run-integration-tests.sh
```

**How it works:**
1. Compiles test binary locally (`go test -c`) - uses local Go cache
2. Creates pod with minimal `debian:bookworm-slim` image (~80MB)
3. Mounts socket via `hostPath` (read-only)
4. Copies only test binary (~10MB)
5. Runs binary directly in pod
6. Cleans up automatically

**Security improvements:**
- No `hostPID` or `hostNetwork`
- Read-only socket mount
- Resource limits enforced
- Tolerant label selectors (works with any SPIRE deployment)
- `runAsNonRoot: true`
- `allowPrivilegeEscalation: false`

**When to use:**
- Standard development workflow
- Repeated test runs
- Local testing
- Quick verification

**Speed:** ~15 seconds (first run), ~15 seconds (repeat)

---

### CI/Distroless: `test-integration-ci` (Maximum security)

**Approach:**
```bash
make test-integration-ci  # Uses scripts/run-integration-tests-ci.sh
```

**How it works:**
1. Compiles static test binary (`CGO_ENABLED=0 go test -c`)
2. Creates pod with `gcr.io/distroless/static-debian12:nonroot` (~25MB)
3. Mounts socket via `hostPath` (read-only)
4. Copies static binary (~10MB)
5. Runs binary directly (no shell available)
6. Cleans up automatically

**Security hardening:**
- Distroless (no shell, no package manager, minimal attack surface)
- Static binary (no dependencies)
- `runAsNonRoot: true`
- `readOnlyRootFilesystem: true`
- `allowPrivilegeEscalation: false`
- Capabilities dropped (`drop: ["ALL"]`)
- Seccomp profile enabled

**When to use:**
- Production CI/CD pipelines
- Security-critical environments
- Compliance requirements
- GitHub Actions, GitLab CI, etc.

**Speed:** ~20 seconds

---

### Fast Iteration: `test-integration-keep` (Pod reuse)

**Approach:**
```bash
make test-integration-keep  # Reuses pod for faster iteration
```

**How it works:**
1. First run: Same as `test-integration`
2. Subsequent runs: Only copies binary (~5 seconds)
3. Pod stays running between tests
4. Manual cleanup: `kubectl delete pod -n spire-system spire-integration-test`

**When to use:**
- Rapid local development
- Testing multiple times in a row
- Debugging test issues
- Quick edit-test cycles

**Speed:**
- First run: ~15 seconds
- Subsequent: ~5 seconds ⚡ (12x faster than 60s baseline)

---

## Performance Comparison

| Aspect | Standard (Optimized) | CI/Distroless | Keep (repeat) |
|--------|---------------------|---------------|---------------|
| **Script** | `run-integration-tests.sh` | `run-integration-tests-ci.sh` | `run-integration-tests.sh` |
| **Transfer size** | ~10MB | ~10MB | ~10MB |
| **Pod image size** | 80MB | 25MB | 80MB (reused) |
| **Privileges** | Minimal | Hardened | Minimal |
| **Security** | Good | Maximum | Good |
| **Dependencies** | Compiled | Static | Compiled |
| **Determinism** | High | Maximum | High |
| **First run** | ~15s | ~20s | ~15s |
| **Repeat run** | ~15s | ~20s | ~5s ⚡ |
| **CI-friendly** | Yes | Best | N/A |
| **Debugging** | Good | Limited | Excellent |

## Quick Decision Guide

**Choose your implementation:**

```
Development (normal use)      → make test-integration
Development (rapid iteration) → make test-integration-keep
CI/CD (GitHub Actions, etc.)  → make test-integration-ci
Security-critical envs        → make test-integration-ci
Debugging test failures       → make test-integration-keep
```

## Configuration Examples

All scripts support environment variable configuration:

### Custom Namespace

```bash
NS=my-namespace make test-integration
```

### Custom Socket Path

```bash
SOCKET_DIR=/custom/path SOCKET_FILE=custom.sock make test-integration
```

### Different Package

```bash
PKG=./internal/adapters/inbound/identityserver make test-integration
```

### Rapid Development Workflow

```bash
# First run - creates pod
make test-integration-keep

# Edit your tests...
# vim internal/adapters/outbound/spire/integration_test.go

# Second run - reuses pod (5 seconds!)
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

This is Option A from the architecture review:

> Option A — Run the tests inside Kubernetes (recommended)
>
> Run your integration tests in a pod that mounts the agent socket via `hostPath`.
> Compile tests to a single binary and run it in the pod for speed and determinism.

**Why Option A was chosen:**
- ❌ Option B (test Job): Requires building Docker image - too heavyweight
- ❌ Option C (prod binary): Only tests binary, not `go test` suite
- ❌ Option D (UDS bridging): Fragile, negates Workload API design
- ✅ Option A: Clean, fast, works with existing test suite

## Implementation Details

### Standard Script (`run-integration-tests.sh`)

**Optimized approach (default):**
```bash
# Compile test binary locally (uses local Go cache)
CGO_ENABLED=0 GOOS=linux GOARCH=$NODE_ARCH \
    go test -tags=integration -c -o /tmp/spire-integration.test ./internal/adapters/outbound/spire

# Create minimal pod with Debian slim
image: debian:bookworm-slim

# Copy only binary (~10MB vs ~100MB source)
kubectl cp /tmp/spire-integration.test spire-system/spire-integration-test:/work/integration.test

# Run binary directly
/work/integration.test -test.v -test.timeout=3m
```

**Features:**
- Auto-detects node architecture (ARM/x86)
- Static binary for reliability
- Test timeout (3 minutes) prevents hangs
- Resource limits (500m CPU, 256Mi RAM)
- Security context hardening

### CI Script (`run-integration-tests-ci.sh`)

**Maximum security variant:**
```bash
# Compile static test binary
CGO_ENABLED=0 GOOS=linux GOARCH=$NODE_ARCH \
    go test -tags=integration -c -o /tmp/spire-integration.test ./internal/adapters/outbound/spire

# Create distroless pod (no shell, no packages)
image: gcr.io/distroless/static-debian12:nonroot

# Copy static binary
kubectl cp /tmp/spire-integration.test spire-system/spire-integration-test-ci:/work/integration.test

# Run binary with full security hardening
command: ["/work/integration.test"]
args: ["-test.v", "-test.timeout=3m"]
```

**Security features:**
- Distroless base (minimal attack surface)
- No shell or package manager
- `readOnlyRootFilesystem: true`
- All capabilities dropped
- Seccomp profile enabled

## Switching Between Implementations

All variants work correctly. Choose based on your needs:

```bash
# Standard development (default, recommended)
make test-integration

# Fast iteration (keep pod between runs)
make test-integration-keep

# CI/CD pipeline (maximum security)
make test-integration-ci
```

## Performance Evolution

### Before Optimization (Baseline)
```
Full source copy + in-pod compilation: ~60 seconds
- golang:1.23 image (~800MB)
- Copy entire project (~100MB)
- Download dependencies
- Compile in pod
```

### After Optimization (Current)
```
Pre-compiled binary approach: ~15 seconds (4x faster)
- debian:bookworm-slim (~80MB)
- Copy only binary (~10MB)
- Uses local Go cache
- No in-pod compilation

With pod reuse: ~5 seconds (12x faster)
- Same pod between runs
- Only binary copy needed
```

## Measuring Performance

Time each approach:

```bash
# Standard (optimized, default)
time make test-integration        # ~15 seconds

# Fast iteration (pod reuse)
time make test-integration-keep   # ~5 seconds (repeat runs)

# CI variant (maximum security)
time make test-integration-ci     # ~20 seconds
```

## References

- Architecture review: Original detailed options analysis
- `scripts/run-integration-tests.sh`: Optimized implementation (standard)
- `scripts/run-integration-tests-ci.sh`: Maximum security variant for CI/CD
- `Makefile`: Integration test targets (`test-integration`, `test-integration-ci`, `test-integration-keep`)
