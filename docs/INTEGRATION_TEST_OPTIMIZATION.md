# Integration Test Optimization

## Problem That Was Solved

Integration tests need to access the SPIRE Agent socket at `/tmp/spire-agent/public/api.sock`, which lives **inside the Minikube node**, not on the host machine.

**Solution:** Run tests inside Kubernetes where the socket exists (Option A from architecture review).

## Two Implementations

### Current: `test-integration` (Works, but slower)

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

**Transfer time:**
- ~100MB+ project files
- Module downloads in pod
- **Total: ~30-60 seconds**

---

### Optimized: `test-integration-fast` (Recommended)

**Approach:**
```bash
make test-integration-fast  # Uses scripts/run-integration-tests-optimized.sh
```

**How it works:**
1. **Compiles test binary locally** (`go test -c`)
2. Creates pod with minimal `debian:bookworm-slim` image
3. Mounts socket via `hostPath`
4. Copies **only test binary** (~10MB)
5. Runs binary directly in pod
6. Cleans up

**Transfer time:**
- ~10MB test binary
- No module downloads
- **Total: ~10-15 seconds**

---

## Performance Comparison

| Aspect | Current (`test-integration`) | Optimized (`test-integration-fast`) |
|--------|----------------------------|-------------------------------------|
| **Transfer size** | ~100MB+ (full project) | ~10MB (test binary) |
| **Pod image** | `golang:1.23` (800MB+) | `debian:bookworm-slim` (80MB) |
| **Dependencies** | Downloaded at runtime | Compiled in binary |
| **Determinism** | Variable (network dependent) | High (frozen binary) |
| **Speed** | ~30-60 seconds | ~10-15 seconds |
| **CI-friendly** | Good | Better |

## When to Use Each

### Use `test-integration` (current) when:
- ✅ Debugging tests (easier to iterate)
- ✅ You need to modify test code frequently
- ✅ First time running tests

### Use `test-integration-fast` (optimized) when:
- ✅ Running tests in CI/CD pipelines
- ✅ Running tests repeatedly
- ✅ You want faster feedback
- ✅ You need deterministic builds

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
