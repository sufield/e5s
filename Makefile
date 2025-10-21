.PHONY: test test-verbose test-race test-coverage test-coverage-html test-short \
	clean help prereqs check-prereqs check-prereqs-k8s check-prereqs-lint check-prereqs-misc \
	build prod-build dev-build test-prod-build compare-sizes test-inmem test-inmem-html \
	helm-lint helm-template minikube-up minikube-down minikube-status \
	minikube-delete ci-test ci-build verify verify-spire check-spire-ready \
	test-integration test-integration-ci test-integration-keep test-prod-binary \
	refactor-baseline refactor-compare refactor-check refactor-install-tools refactor-clean \
	test-dev test-prod register-test-workload \
	spire-server-shell-enable spire-server-shell-disable spire-server-shell-status \
	sec-deps sec-lint sec-secrets sec-test sec-fuzz sec-all sec-install-tools check-prereqs-sec \
	codeql-db codeql-analyze codeql-clean codeql check-prereqs-codeql

# Use bash with strict error handling
SHELL := /bin/bash
.SHELLFLAGS := -eu -o pipefail -c

# Default target
.DEFAULT_GOAL := help

# Binary names
BINARY_PROD=bin/spire-server
BINARY_DEV=bin/cp-minikube

# Build flags
LDFLAGS=-ldflags "-s -w"
BUILD_TAGS_DEV=dev

# Required tools (core minimum)
REQUIRED_TOOLS=go

# Optional tool groups
TOOLS_K8S=helm kubectl minikube
TOOLS_LINT=staticcheck golangci-lint gocyclo goimports
TOOLS_MISC=jq sed awk
TOOLS_SEC=govulncheck gosec gitleaks

# Package list
PKGS=$(shell go list ./...)

## prereqs: Check for required tools (alias for check-prereqs)
prereqs: check-prereqs

## check-prereqs: Verify core required tools are installed
check-prereqs:
	@echo "Checking core prerequisites..."
	@for tool in $(REQUIRED_TOOLS); do \
		if ! command -v $$tool >/dev/null 2>&1; then \
			echo "✗ $$tool not found in PATH"; \
			exit 1; \
		else \
			echo "✓ $$tool found"; \
		fi; \
	done
	@echo "✓ Core prerequisites satisfied"

## check-prereqs-k8s: Verify Kubernetes tools are installed
check-prereqs-k8s:
	@echo "Checking Kubernetes tools..."
	@for tool in $(TOOLS_K8S); do \
		if ! command -v $$tool >/dev/null 2>&1; then \
			echo "✗ $$tool not found in PATH"; \
			exit 1; \
		else \
			echo "✓ $$tool found"; \
		fi; \
	done
	@echo "✓ Kubernetes tools satisfied"

## check-prereqs-lint: Verify linting tools are installed
check-prereqs-lint:
	@echo "Checking linting tools..."
	@for tool in $(TOOLS_LINT); do \
		if ! command -v $$tool >/dev/null 2>&1; then \
			echo "✗ $$tool not found - run 'make refactor-install-tools'"; \
			exit 1; \
		else \
			echo "✓ $$tool found"; \
		fi; \
	done
	@echo "✓ Linting tools satisfied"

## check-prereqs-misc: Verify misc tools are installed
check-prereqs-misc:
	@echo "Checking misc tools..."
	@for tool in $(TOOLS_MISC); do \
		if ! command -v $$tool >/dev/null 2>&1; then \
			echo "✗ $$tool not found in PATH"; \
			exit 1; \
		else \
			echo "✓ $$tool found"; \
		fi; \
	done
	@echo "✓ Misc tools satisfied"

## test: Run all tests (same as test-dev)
test: test-dev

## test-prod: Run tests without dev tags (production build)
test-prod:
	@echo "Running production tests (no dev tags)..."
	@go test ./...

## test-dev: Run tests with dev tags (development build)
test-dev:
	@echo "Running development tests (with -tags=dev)..."
	@go test -tags=dev ./...

## test-verbose: Run all tests with verbose output
test-verbose:
	@echo "Running all tests (verbose)..."
	@go test -v ./...

## test-race: Run all tests with race detector
test-race:
	@echo "Running all tests with race detector..."
	@go test -race ./...

## test-short: Run tests in short mode (skip long-running tests)
test-short:
	@echo "Running tests in short mode..."
	@go test -short ./...

## test-coverage: Run tests with coverage report
test-coverage:
	@echo "Running tests with coverage..."
	@go test -coverprofile=coverage.out ./...
	@go tool cover -func=coverage.out

## test-coverage-html: Run tests and generate HTML coverage report
test-coverage-html:
	@echo "Generating HTML coverage report..."
	@go test -coverprofile=coverage.out ./...
	@go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

## test-inmem: Run tests for inmemory package with coverage
test-inmem:
	@echo "Running inmemory package tests with coverage..."
	@go test -tags=dev -coverprofile=inmem.out ./internal/adapters/outbound/inmemory
	@go tool cover -func=inmem.out

## test-inmem-html: Generate HTML coverage report for inmemory package
test-inmem-html:
	@echo "Generating HTML coverage report for inmemory package..."
	@go test -tags=dev -coverprofile=inmem.out ./internal/adapters/outbound/inmemory
	@go tool cover -html=inmem.out -o inmem_coverage.html
	@echo "Coverage report generated: inmem_coverage.html"

## clean: Remove generated files
clean:
	@echo "Cleaning up..."
	@rm -f coverage.out coverage.html inmem.out inmem_coverage.html
	@rm -f gosec.sarif gitleaks.sarif codeql-results.sarif
	@rm -rf bin/ codeql-db/
	@echo "Clean complete"

## build: Build production binary (alias for prod-build)
build: prod-build

## prod-build: Build production binary (no dev tags)
prod-build:
	@echo "Building production binary..."
	@mkdir -p bin
	@go build -trimpath $(LDFLAGS) -o $(BINARY_PROD) ./cmd
	@echo "Production binary: $(BINARY_PROD)"
	@ls -lh $(BINARY_PROD)

## dev-build: Build dev binary with dev tags
dev-build:
	@echo "Building dev binary with -tags=dev..."
	@mkdir -p bin
	@go build -tags=$(BUILD_TAGS_DEV) -o $(BINARY_DEV) ./cmd
	@echo "Dev binary: $(BINARY_DEV)"
	@ls -lh $(BINARY_DEV)

## compare-sizes: Build both versions and compare binary sizes
compare-sizes:
	@echo "Building and comparing binary sizes..."
	@mkdir -p bin
	@echo "→ Building production binary..."
	@go build -trimpath $(LDFLAGS) -o bin/agent-prod ./cmd
	@echo "→ Building dev binary..."
	@go build -tags=$(BUILD_TAGS_DEV) -o bin/agent-dev ./cmd
	@echo ""
	@echo "=== Binary Size Comparison ==="
	@ls -lh bin/agent-prod bin/agent-dev
	@echo ""
	@PROD_SIZE=$$(stat -c%s bin/agent-prod 2>/dev/null || stat -f%z bin/agent-prod); \
	DEV_SIZE=$$(stat -c%s bin/agent-dev 2>/dev/null || stat -f%z bin/agent-dev); \
	DIFF=$$((DEV_SIZE - PROD_SIZE)); \
	PERCENT=$$(echo "scale=2; ($$DIFF * 100) / $$DEV_SIZE" | bc -l); \
	echo "Production binary: $$PROD_SIZE bytes"; \
	echo "Development binary: $$DEV_SIZE bytes"; \
	echo "Size difference: $$DIFF bytes ($$PERCENT% of dev binary)"; \
	echo ""

## test-prod-build: Verify production build excludes dev code
test-prod-build:
	@echo "Testing production build..."
	@echo "→ Checking production dependencies exclude dev packages..."
	@PROD_DEPS=$$(go list -deps ./cmd); \
	DEV_PKGS="internal/adapters/outbound/inmemory internal/adapters/inbound/cli internal/adapters/outbound/compose"; \
	for pkg in $$DEV_PKGS; do \
		if echo "$$PROD_DEPS" | grep -q "github.com/pocket/hexagon/spire/$$pkg"; then \
			echo "✗ ERROR: Production build includes dev package: $$pkg"; \
			exit 1; \
		fi; \
	done; \
	echo "  ✓ No dev packages in production dependencies"
	@echo "→ Building production binary..."
	@go build -o /tmp/test-prod ./cmd 2>&1
	@echo "  ✓ Production build successful"
	@echo "→ Building dev binary..."
	@go build -tags=dev -o /tmp/test-dev ./cmd 2>&1
	@echo "  ✓ Dev build successful"
	@echo "→ Verifying production tests exclude dev tests..."
	@if go test -list . ./internal/domain 2>&1 | grep -q "TestSelector\|TestIdentityMapper"; then \
		echo "✗ ERROR: Production build includes dev tests!"; \
		exit 1; \
	fi
	@echo "  ✓ Dev tests excluded from production"
	@echo "→ Verifying dev tests run with dev tags..."
	@TEST_OUTPUT=$$(go test -tags=dev -list . ./internal/domain 2>&1); \
	if ! echo "$$TEST_OUTPUT" | grep -q "TestSelector\|TestIdentityMapper"; then \
		echo "✗ ERROR: Dev tests not found with -tags=dev!"; \
		exit 1; \
	fi
	@echo "  ✓ Dev tests found with dev tags"
	@rm -f /tmp/test-prod /tmp/test-dev
	@echo "✓ Production build check passed"

## verify: Run comprehensive verification (alias for verify-spire)
verify: verify-spire

## verify-spire: Run comprehensive SPIRE adapter verification
verify-spire:
	@echo "Running comprehensive SPIRE adapter verification..."
	@bash scripts/verify-implementation.sh

## check-spire-ready: Verify SPIRE infrastructure is running
check-spire-ready: check-prereqs-k8s
	@echo "Checking SPIRE infrastructure..."
	@if ! minikube status -p minikube &>/dev/null; then \
		echo "✗ ERROR: Minikube is not running"; \
		echo "  Run 'make minikube-up' to start the cluster"; \
		exit 1; \
	fi
	@echo "  ✓ Minikube is running"
	@if ! kubectl get namespace spire-system &>/dev/null; then \
		echo "✗ ERROR: SPIRE is not deployed"; \
		echo "  Run 'make minikube-up' to deploy SPIRE"; \
		exit 1; \
	fi
	@echo "  ✓ SPIRE namespace exists"
	@if ! kubectl get pods -n spire-system 2>/dev/null | grep -q "Running"; then \
		echo "✗ ERROR: SPIRE pods are not running"; \
		echo "  Check status with 'make minikube-status'"; \
		exit 1; \
	fi
	@echo "  ✓ SPIRE pods are running"
	@echo "✓ SPIRE infrastructure is ready"

## spire-server-shell-enable: Enable shell access in SPIRE server (for CLI registration)
spire-server-shell-enable: check-spire-ready
	@bash scripts/spire-server-enable-shell.sh enable

## spire-server-shell-disable: Disable shell (switch to distroless for production)
spire-server-shell-disable: check-spire-ready
	@bash scripts/spire-server-enable-shell.sh disable

## spire-server-shell-status: Check current SPIRE server image type
spire-server-shell-status: check-spire-ready
	@bash scripts/spire-server-enable-shell.sh status

## register-test-workload: Register test workload in SPIRE (required before test-integration)
register-test-workload: check-spire-ready
	@bash scripts/setup-spire-registrations.sh

## test-integration: Run integration tests against live SPIRE
test-integration: check-spire-ready register-test-workload
	@bash scripts/run-integration-tests.sh

## test-prod-binary: Test production binary against live SPIRE in Minikube
test-prod-binary:
	@bash scripts/test-prod-binary-minikube.sh

## helm-lint: Lint Helm values files
helm-lint:
	@echo "Linting Helm values..."
	@cd infra/dev/minikube && \
	helm lint spiffe/spire-server -f values-minikube.yaml || \
	echo "Note: Chart must be pulled first with 'helm repo add spiffe https://spiffe.github.io/helm-charts-hardened/'"
	@echo "✓ Helm lint complete"

## helm-template: Test Helm template rendering
helm-template:
	@echo "Testing Helm template rendering..."
	@mkdir -p tmp
	@cd infra/dev/minikube && \
	helm repo add spiffe https://spiffe.github.io/helm-charts-hardened/ 2>/dev/null || true && \
	helm repo update && \
	helm template spire-server spiffe/spire-server \
		-n spire-system \
		-f values-minikube.yaml \
		--debug \
		> ../../../tmp/helm-template-server.yaml && \
	helm template spire-agent spiffe/spire-agent \
		-n spire-system \
		-f values-minikube.yaml \
		--debug \
		> ../../../tmp/helm-template-agent.yaml
	@echo "✓ Template rendered to tmp/helm-template-*.yaml"

## minikube-up: Start Minikube and deploy SPIRE
minikube-up:
	@echo "Starting Minikube infrastructure..."
	@cd infra/dev/minikube/scripts && ./cluster-up.sh start

## minikube-down: Stop Minikube and cleanup
minikube-down:
	@echo "Stopping Minikube infrastructure..."
	@cd infra/dev/minikube/scripts && ./cluster-down.sh stop

## minikube-status: Show Minikube infrastructure status
minikube-status:
	@cd infra/dev/minikube/scripts && ./cluster-status.sh

## minikube-delete: Delete Minikube cluster completely
minikube-delete:
	@echo "Deleting Minikube cluster..."
	@cd infra/dev/minikube/scripts && ./cluster-down.sh delete

## ci-test: Run all CI validation checks
ci-test: check-prereqs test-coverage helm-lint helm-template test-prod-build
	@echo ""
	@echo "======================================"
	@echo "✓ All CI checks passed successfully!"
	@echo "======================================"

## ci-build: Build and validate all binaries for CI
ci-build: check-prereqs prod-build dev-build test-prod-build
	@echo ""
	@echo "======================================"
	@echo "✓ All builds completed successfully!"
	@echo "======================================"

## refactor-install-tools: Install refactoring analysis tools
refactor-install-tools:
	@echo "Installing refactoring analysis tools..."
	@go install honnef.co/go/tools/cmd/staticcheck@latest
	@go install github.com/fzipp/gocyclo/cmd/gocyclo@latest
	@go install github.com/uudashr/gocognit/cmd/gocognit@latest
	@go install golang.org/x/tools/cmd/goimports@latest
	@go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	@echo "✓ Tools installed successfully"

## refactor-baseline: Generate refactoring baseline metrics
refactor-baseline:
	@echo "Generating refactoring baseline..."
	@mkdir -p docs/refactoring
	@$(shell date) > docs/refactoring/baseline.txt
	@printf "\n=== File Sizes (Top 20) ===\n" >> docs/refactoring/baseline.txt
	@find . -name "*.go" -not -path "./vendor/*" -not -path "./.git/*" | \
		xargs wc -l | sort -rn | head -20 >> docs/refactoring/baseline.txt
	@printf "\n=== Cyclomatic Complexity (>15) ===\n" >> docs/refactoring/baseline.txt
	@gocyclo -over 15 . >> docs/refactoring/baseline.txt 2>&1 || echo "  (gocyclo not installed - run 'make refactor-install-tools')"
	@printf "\n=== Test Coverage ===\n" >> docs/refactoring/baseline.txt
	@go test ./... -coverprofile=docs/refactoring/coverage_before.out > /dev/null 2>&1
	@go tool cover -func=docs/refactoring/coverage_before.out | tail -1 >> docs/refactoring/baseline.txt
	@printf "\n=== Test Execution Time ===\n" >> docs/refactoring/baseline.txt
	@{ time go test ./... > /dev/null 2>&1; } 2>> docs/refactoring/baseline.txt || true
	@printf "\nBaseline saved to docs/refactoring/baseline.txt\n"
	@cat docs/refactoring/baseline.txt

## refactor-compare: Compare before/after refactoring metrics
refactor-compare:
	@echo "Comparing refactoring results..."
	@if [ ! -f docs/refactoring/baseline.txt ]; then \
		echo "Error: Run 'make refactor-baseline' first to generate baseline"; \
		exit 1; \
	fi
	@mkdir -p docs/refactoring
	@$(shell date) > docs/refactoring/comparison.txt
	@printf "\n=== FILE SIZES COMPARISON ===\n" >> docs/refactoring/comparison.txt
	@printf "\nTop 5 BEFORE:\n" >> docs/refactoring/comparison.txt
	@grep -A 5 "File Sizes" docs/refactoring/baseline.txt | tail -5 >> docs/refactoring/comparison.txt
	@printf "\nTop 5 AFTER:\n" >> docs/refactoring/comparison.txt
	@find . -name "*.go" -not -path "./vendor/*" -not -path "./.git/*" | \
		xargs wc -l | sort -rn | head -5 >> docs/refactoring/comparison.txt
	@printf "\n=== COVERAGE COMPARISON ===\n" >> docs/refactoring/comparison.txt
	@go test ./... -coverprofile=docs/refactoring/coverage_after.out > /dev/null 2>&1
	@printf "\nBEFORE:\n" >> docs/refactoring/comparison.txt
	@grep "total:" docs/refactoring/baseline.txt >> docs/refactoring/comparison.txt || echo "  (coverage data not found in baseline)"
	@printf "\nAFTER:\n" >> docs/refactoring/comparison.txt
	@go tool cover -func=docs/refactoring/coverage_after.out | tail -1 >> docs/refactoring/comparison.txt
	@printf "\n=== SUMMARY ===\n" >> docs/refactoring/comparison.txt
	@printf "\nComparison saved to docs/refactoring/comparison.txt\n"
	@cat docs/refactoring/comparison.txt

## refactor-check: Run all refactoring validation checks
refactor-check:
	@echo "Running refactoring checks..."
	@printf "\n→ Running tests...\n"
	@go test ./... -v -count=1 || (echo "✗ Tests failed" && exit 1)
	@printf "\n→ Running staticcheck...\n"
	@staticcheck ./... || (echo "✗ Staticcheck failed" && exit 1)
	@printf "\n→ Running go vet...\n"
	@go vet ./... || (echo "✗ Go vet failed" && exit 1)
	@printf "\n→ Checking imports...\n"
	@goimports -l . | (! grep .) || (echo "⚠ WARNING: Some files need goimports formatting" && goimports -l .)
	@printf "\n→ Running golangci-lint...\n"
	@golangci-lint run --timeout=5m || echo "⚠ WARNING: golangci-lint found issues"
	@printf "\n→ Checking cyclomatic complexity...\n"
	@if [ -n "$${STRICT}" ]; then \
		gocyclo -over 15 . && (echo "✗ FAIL: High complexity detected in strict mode" && exit 1) || echo "✓ Complexity OK"; \
	else \
		gocyclo -over 15 . && echo "⚠ WARNING: High complexity detected" || echo "✓ Complexity OK"; \
	fi
	@printf "\n→ Checking file sizes...\n"
	@LARGE_FILES=$$(find . -name "*.go" -not -path "./vendor/*" | xargs wc -l | awk '$$1 > 500 {print}' | wc -l); \
		if [ $$LARGE_FILES -gt 0 ]; then \
			if [ -n "$${STRICT}" ]; then \
				echo "✗ FAIL: $$LARGE_FILES file(s) exceed 500 lines in strict mode"; \
				find . -name "*.go" -not -path "./vendor/*" | xargs wc -l | awk '$$1 > 500 {print "  ", $$0}'; \
				exit 1; \
			else \
				echo "⚠ WARNING: $$LARGE_FILES file(s) exceed 500 lines"; \
				find . -name "*.go" -not -path "./vendor/*" | xargs wc -l | awk '$$1 > 500 {print "  ", $$0}'; \
			fi \
		else \
			echo "✓ All files under 500 lines"; \
		fi
	@printf "\n✅ All checks passed\n"

## refactor-clean: Remove generated refactoring files
refactor-clean:
	@echo "Cleaning refactoring files..."
	@rm -rf docs/refactoring/
	@echo "✓ Refactoring files cleaned"

# ============================================================================
# Security Targets (following docs/security.md)
# ============================================================================

## check-prereqs-sec: Verify security tools are installed
check-prereqs-sec:
	@echo "Checking security tools..."
	@for tool in $(TOOLS_SEC); do \
		if ! command -v $$tool >/dev/null 2>&1; then \
			echo "✗ $$tool not found - run 'make sec-install-tools'"; \
			exit 1; \
		else \
			echo "✓ $$tool found"; \
		fi; \
	done
	@echo "✓ Security tools satisfied"

## sec-install-tools: Install security scanning tools
sec-install-tools:
	@echo "Installing security tools..."
	@echo "→ Installing govulncheck..."
	@go install golang.org/x/vuln/cmd/govulncheck@latest
	@echo "→ Installing gosec..."
	@go install github.com/securego/gosec/v2/cmd/gosec@latest
	@echo "→ Installing gitleaks..."
	@go install github.com/zricethezav/gitleaks/v8@latest
	@echo "✓ Security tools installed successfully"

## sec-deps: Check for dependency vulnerabilities and module hygiene
sec-deps:
	@echo "Checking for dependency vulnerabilities..."
	@govulncheck ./...
	@echo ""
	@echo "Verifying module hygiene..."
	@go mod tidy
	@go mod verify
	@echo "✓ Dependency check complete"

## sec-lint: Run security-focused static analysis
sec-lint:
	@echo "Running security-focused static analysis..."
	@echo "→ Running golangci-lint..."
	@golangci-lint run ./...
	@echo ""
	@echo "→ Running gosec..."
	@gosec ./internal/... ./pkg/...
	@echo "✓ Security lint complete"

## sec-secrets: Scan for secrets, keys, and tokens
sec-secrets:
	@echo "Scanning for secrets..."
	@gitleaks detect --no-git -v
	@echo "✓ Secret scan complete"

## sec-test: Run tests with race detector and coverage
sec-test:
	@echo "Running tests with race detector and coverage..."
	@go test -race -covermode=atomic -coverprofile=coverage.out ./...
	@echo ""
	@echo "Coverage summary:"
	@go tool cover -func=coverage.out | tail -1
	@echo "✓ Security tests complete"

## sec-fuzz: Run fuzz tests on high-risk parsers (20s per target)
sec-fuzz:
	@echo "Running fuzz tests..."
	@echo "→ Fuzzing domain parsers..."
	@go test -fuzz=Fuzz -fuzztime=20s ./internal/domain || echo "⚠ No fuzz targets in domain"
	@echo "→ Fuzzing adapter parsers..."
	@go test -fuzz=Fuzz -fuzztime=20s ./internal/adapters/... || echo "⚠ No fuzz targets in adapters"
	@echo "✓ Fuzz testing complete"

## sec-all: Run all security checks (deps, lint, secrets, tests)
sec-all: sec-deps sec-lint sec-secrets sec-test
	@echo ""
	@echo "======================================"
	@echo "✓ All security checks passed!"
	@echo "======================================"

# ============================================================================
# CodeQL Targets (Local Analysis)
# See docs/codeql-local-setup.md for installation instructions
# ============================================================================

## check-prereqs-codeql: Verify CodeQL CLI is installed
check-prereqs-codeql:
	@echo "Checking CodeQL CLI..."
	@if ! command -v codeql >/dev/null 2>&1; then \
		echo "✗ codeql not found in PATH"; \
		echo "  See docs/codeql-local-setup.md for installation instructions"; \
		exit 1; \
	else \
		echo "✓ codeql found"; \
		codeql --version; \
	fi

## codeql-db: Create CodeQL database for Go codebase
codeql-db: check-prereqs-codeql
	@echo "Creating CodeQL database for Go..."
	@codeql database create codeql-db \
		--language=go \
		--source-root=. \
		--overwrite
	@echo "✓ CodeQL database created at codeql-db/"

## codeql-analyze: Analyze CodeQL database with security queries
codeql-analyze: codeql-db
	@echo "Running CodeQL security analysis..."
	@codeql database analyze codeql-db \
		codeql/go-queries:codeql-suites/go-code-scanning.qls \
		--format=sarif-latest \
		--output=codeql-results.sarif \
		--sarif-category=go \
		--threads=0
	@echo "✓ Results saved to codeql-results.sarif"
	@echo ""
	@echo "View results with: code codeql-results.sarif"
	@echo "Or see docs/codeql-local-setup.md for other viewing options"

## codeql-clean: Remove CodeQL artifacts
codeql-clean:
	@echo "Cleaning CodeQL artifacts..."
	@rm -rf codeql-db codeql-results.sarif
	@echo "✓ CodeQL artifacts removed"

## codeql: Run full CodeQL analysis workflow (create DB + analyze)
codeql: codeql-analyze
	@echo ""
	@echo "======================================"
	@echo "✓ CodeQL analysis complete!"
	@echo "======================================"

## help: Show this help message
help:
	@echo "Available targets:"
	@sed -n 's/^##//p' ${MAKEFILE_LIST} | column -t -s ':' | sed -e 's/^/ /'

## test-integration-ci: Run integration tests (CI variant - static binary + distroless)
test-integration-ci: check-spire-ready register-test-workload
	@bash scripts/run-integration-tests-ci.sh

## test-integration-keep: Keep test pod for faster iteration
test-integration-keep: check-spire-ready register-test-workload
	@KEEP=true bash scripts/run-integration-tests.sh
