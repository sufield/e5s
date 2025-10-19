checklist to run **security checks** for a Go library that uses the **SPIFFE/SPIRE SDK** for identity-based mTLS. It covers static analysis, dependency vulns, secret leaks, hardening, and runtime/authZ tests. Pick what you need; you can wire all of it into CI.

---

# 1) Prereqs

```bash
# In your repo root
go version         # Go 1.21+ recommended (has govulncheck support)
```

Optional tooling (install once):

```bash
# Vulnerability scanning (Go advisories + OSV)
go install golang.org/x/vuln/cmd/govulncheck@latest

# Security-focused static analysis
go install github.com/securego/gosec/v2/cmd/gosec@latest

# Meta-linter with security rules (fast, broad)
go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest

# Secret scanning
go install github.com/gitleaks/gitleaks/v8@latest
```

---

# 2) Dependency & Binary Vulnerability Scans

### A. Go dependency vulnerabilities

```bash
govulncheck ./...
```

### B. Module hygiene (stale/indirect)

```bash
go list -m -u all | grep -E '=>|v[0-9]+\.[0-9]+'    # see upgradable mods
go mod tidy && go mod verify
```

---

# 3) Static Code Security Analysis

### A. gosec (security rules: crypto misuse, file perms, unsafe ops, etc.)

```bash
gosec ./...
# Or narrow to your packages
gosec ./internal/... ./pkg/...
```

Useful flags:

* `-fmt=json -out gosec.json` (CI artifact)
* `-exclude-dir examples` (if examples are intentionally less strict)

### B. golangci-lint (enable security linters)

Create `.golangci.yml`:

```yaml
run:
  timeout: 5m
linters:
  enable:
    - gosec
    - govet
    - staticcheck
    - errcheck
    - gocritic
    - revive
    - ineffassign
    - bodyclose
issues:
  exclude-rules:
    - path: _test\.go
      linters:
        - gosec  # allow benign test patterns
```

Run:

```bash
golangci-lint run ./...
```

---

# 4) Secrets / Keys / Tokens

```bash
gitleaks detect --no-git -v
# Or scan history too:
gitleaks detect -v
```

---

# 5) Build Hardening & Tests

### A. Hardened build (static, stripped) for adapters/binaries

```bash
CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" ./cmd/...
```

### B. Unit tests + race detector + coverage

```bash
go test -race -covermode=atomic -coverprofile=coverage.out ./...
```

### C. Fuzz high-risk parsers (IDs, selectors, config)

```bash
# Example fuzz target in *_test.go:
# func FuzzParseSelector(f *testing.F) { ... }
go test -fuzz=Fuzz -fuzztime=20s ./internal/domain
```

---

# 6) SPIFFE/SPIRE-Specific Security Tests (must-have)

Create Go tests that **prove** the critical auth properties:

1. **Deny by default** (no identity → 401)

```go
// Make a plain HTTP client to your server. Expect 401/403.
```

2. **Authorize exact SPIFFE ID** (positive/negative)

```go
// Client A: spiffe://example.org/client   → 200
// Client B: spiffe://example.org/other    → 403
```

3. **Authorize trust domain** (member-of)

```go
// Member of trust domain example.org → 200
// Different trust domain → 403
```

4. **TLS policy** (TLS1.3 enforced)

```go
// Force TLS 1.2 client → handshake should fail
```

5. **SVID rotation** (no downtime)

```go
// Advance time / wait for rotation; repeat request → still 200
```

6. **Expired SVID rejected**

```go
// Inject expired leaf in test double; client/server must fail handshake
```

7. **Workload API error handling**

```go
// Simulate socket missing/unreachable → clear, wrapped error (no panic)
```

8. **Selector/registration correctness (dev/inmem)**

```go
// Ensure AND semantics for selectors; extra selectors ignored
```

> If you run integration tests against Minikube+SPIRE, keep a target like:

```bash
make minikube-up
make test-integration
```

---

# 7) Kubernetes/Container Hardening Checks (if you ship a demo pod)

Validate these in your manifests (or via `kube-score` / `kubescape`, optional):

* `readOnlyRootFilesystem: true`
* `allowPrivilegeEscalation: false`
* `securityContext.capabilities.drop: ["ALL"]`
* `runAsNonRoot: true`, `runAsUser: 65532`, `fsGroup: 65532`
* Mount the **SPIRE socket read-only**, and only that path
* Use **distroless** (or minimal) images for runtime
* **No shell** in production images (initContainer handles chmod if needed)
* Liveness/readiness probes on HTTPS endpoints (e.g., `/health`)

Example (fragment):

```yaml
securityContext:
  runAsNonRoot: true
  runAsUser: 65532
  fsGroup: 65532
containers:
- name: server
  securityContext:
    allowPrivilegeEscalation: false
    readOnlyRootFilesystem: true
    capabilities:
      drop: ["ALL"]
  volumeMounts:
    - name: spire-socket
      mountPath: /spire-socket
      readOnly: true
```

---

# 8) Makefile Targets (one-liners for your team/CI)

```makefile
.PHONY: sec deps lint test fuzz all-sec

deps:
	go mod tidy && go mod verify
	govulncheck ./...

lint:
	golangci-lint run ./...
	gosec ./...

secrets:
	gitleaks detect --no-git -v

test:
	go test -race -covermode=atomic ./...

fuzz:
	go test -fuzz=Fuzz -fuzztime=20s ./internal/...

all-sec: deps lint secrets test
```

---

# 9) GitHub Actions (drop-in CI)

```yaml
name: security
on:
  push:
  pull_request:

jobs:
  sec:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      - uses: actions/setup-go@v5
        with:
          go-version: '1.22'

      - name: Cache Go build
        uses: actions/cache@v4
        with:
          path: |
            ~/.cache/go-build
            ~/go/pkg/mod
          key: ${{ runner.os }}-go-${{ hashFiles('**/go.sum') }}
          restore-keys: |
            ${{ runner.os }}-go-

      - name: govulncheck
        run: go install golang.org/x/vuln/cmd/govulncheck@latest && govulncheck ./...

      - name: golangci-lint
        uses: golangci/golangci-lint-action@v6
        with:
          version: latest

      - name: gosec
        run: go install github.com/securego/gosec/v2/cmd/gosec@latest && gosec ./...

      - name: gitleaks
        run: go install github.com/gitleaks/gitleaks/v8@latest && gitleaks detect --no-git -v

      - name: unit tests (race + coverage)
        run: go test -race -covermode=atomic -coverprofile=coverage.out ./...

      - name: upload coverage
        uses: actions/upload-artifact@v4
        with:
          name: coverage
          path: coverage.out
```

---

# 10) Library-Specific Security Gates (design checks)

* Server/client MUST use:

  * `tlsconfig.MTLSServerConfig(...)` / `MTLSClientConfig(...)`
  * Explicit **authorizers**: `AuthorizeID(...)` or `AuthorizeMemberOf(trustDomain)`
  * `MinVersion = tls.VersionTLS13`
* Don’t expose a way for app code to inject identity into context (only adapters call `WithIdentity`).
* Log **SPIFFE IDs only** (avoid sensitive material like private keys).
* Never persist private keys in domain; keep keys within adapter scope (signer held by SDK).

---

## TL;DR runnable set

```bash
# 1) Vulns
govulncheck ./...

# 2) Static analysis (quick)
golangci-lint run ./...
gosec ./...

# 3) Secrets
gitleaks detect --no-git -v

# 4) Tests (race + coverage) + fuzz where applicable
go test -race -covermode=atomic ./...
go test -fuzz=Fuzz -fuzztime=20s ./internal/domain
```

If you wire these into a `make all-sec` and a CI workflow, you’ll have a solid, repeatable **security baseline** tailored for a SPIFFE/SPIRE-based Go mTLS library.
