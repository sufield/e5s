.PHONY: test test-verbose test-race test-coverage test-coverage-html test-short \
	clean help prereqs check-prereqs build prod-build dev-build test-prod-build \
	helm-lint helm-template minikube-up minikube-down minikube-status \
	minikube-delete ci-test ci-build verify verify-spire test-integration test-prod-binary \
	refactor-baseline refactor-compare refactor-check refactor-install-tools refactor-clean

# Use bash for consistency across platforms
SHELL := /bin/bash

# Default target
.DEFAULT_GOAL := help

# Binary names
BINARY_PROD=bin/spire-server
BINARY_DEV=bin/cp-minikube

# Build flags
LDFLAGS=-ldflags "-s -w"
BUILD_TAGS_DEV=dev

# Required tools
REQUIRED_TOOLS=go helm kubectl minikube

## prereqs: Check for required tools (alias for check-prereqs)
prereqs: check-prereqs

## check-prereqs: Verify all required tools are installed
check-prereqs:
	@echo "Checking prerequisites..."
	@for tool in $(REQUIRED_TOOLS); do \
		if ! command -v $$tool >/dev/null 2>&1; then \
			echo "✗ $$tool not found in PATH"; \
			exit 1; \
		else \
			echo "✓ $$tool found"; \
		fi; \
	done
	@echo "✓ All prerequisites satisfied"

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
	@go test -coverprofile=inmem.out ./internal/adapters/outbound/inmemory
	@go tool cover -func=inmem.out

## test-inmem-html: Generate HTML coverage report for inmemory package
test-inmem-html:
	@echo "Generating HTML coverage report for inmemory package..."
	@go test -coverprofile=inmem.out ./internal/adapters/outbound/inmemory
	@go tool cover -html=inmem.out -o inmem_coverage.html
	@echo "Coverage report generated: inmem_coverage.html"

## clean: Remove generated files
clean:
	@echo "Cleaning up..."
	@rm -f coverage.out coverage.html inmem.out inmem_coverage.html
	@rm -rf bin/
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
	@echo "→ Building production binary..."
	@go build -o /tmp/test-prod ./cmd 2>&1
	@echo "→ Building dev binary..."
	@go build -tags=dev -o /tmp/test-dev ./cmd 2>&1
	@echo "→ Verifying production tests exclude dev tests..."
	@if go test -list . ./internal/domain 2>&1 | grep -q "TestSelector\|TestIdentityMapper"; then \
		echo "✗ ERROR: Production build includes dev tests!"; \
		exit 1; \
	fi
	@echo "→ Verifying dev tests run with dev tags..."
	@if ! go test -tags=dev -list . ./internal/domain 2>&1 | grep -q "TestSelector\|TestIdentityMapper"; then \
		echo "✗ ERROR: Dev tests not found with -tags=dev!"; \
		exit 1; \
	fi
	@echo "→ Checking production binary for dev symbols..."
	@if go tool nm /tmp/test-prod 2>/dev/null | grep -i "selector\|identitymapper"; then \
		echo "✗ ERROR: Dev symbols found in production binary!"; \
		exit 1; \
	fi
	@rm -f /tmp/test-prod /tmp/test-dev
	@echo "✓ Production build check passed"

## verify: Run comprehensive verification (alias for verify-spire)
verify: verify-spire

## verify-spire: Run comprehensive SPIRE adapter verification
verify-spire:
	@echo "Running comprehensive SPIRE adapter verification..."
	@bash scripts/verify-implementation.sh

## register-test-workload: Register test workload in SPIRE (required before test-integration)
register-test-workload:
	@bash scripts/register-test-workload.sh

## test-integration: Run integration tests against live SPIRE (requires minikube-up and register-test-workload)
test-integration:
	@echo "Running integration tests against SPIRE in Kubernetes..."
	@echo "Note: This creates a test pod with socket access"
	@echo "Note: Run 'make register-test-workload' first if tests fail with 'no identity issued'"
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
	@cd infra/dev/minikube/scripts && ./cluster-down.sh status

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
	@echo "\n=== File Sizes (Top 20) ===" >> docs/refactoring/baseline.txt
	@find . -name "*.go" -not -path "./vendor/*" -not -path "./.git/*" | \
		xargs wc -l | sort -rn | head -20 >> docs/refactoring/baseline.txt
	@echo "\n=== Cyclomatic Complexity (>15) ===" >> docs/refactoring/baseline.txt
	@gocyclo -over 15 . >> docs/refactoring/baseline.txt 2>&1 || echo "  (gocyclo not installed - run 'make refactor-install-tools')"
	@echo "\n=== Test Coverage ===" >> docs/refactoring/baseline.txt
	@go test ./... -coverprofile=docs/refactoring/coverage_before.out > /dev/null 2>&1
	@go tool cover -func=docs/refactoring/coverage_before.out | tail -1 >> docs/refactoring/baseline.txt
	@echo "\n=== Test Execution Time ===" >> docs/refactoring/baseline.txt
	@{ time go test ./... > /dev/null 2>&1; } 2>> docs/refactoring/baseline.txt || true
	@echo "\nBaseline saved to docs/refactoring/baseline.txt"
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
	@echo "\n=== FILE SIZES COMPARISON ===" >> docs/refactoring/comparison.txt
	@echo "\nTop 5 BEFORE:" >> docs/refactoring/comparison.txt
	@grep -A 5 "File Sizes" docs/refactoring/baseline.txt | tail -5 >> docs/refactoring/comparison.txt
	@echo "\nTop 5 AFTER:" >> docs/refactoring/comparison.txt
	@find . -name "*.go" -not -path "./vendor/*" -not -path "./.git/*" | \
		xargs wc -l | sort -rn | head -5 >> docs/refactoring/comparison.txt
	@echo "\n=== COVERAGE COMPARISON ===" >> docs/refactoring/comparison.txt
	@go test ./... -coverprofile=docs/refactoring/coverage_after.out > /dev/null 2>&1
	@echo "\nBEFORE:" >> docs/refactoring/comparison.txt
	@grep "total:" docs/refactoring/baseline.txt >> docs/refactoring/comparison.txt || echo "  (coverage data not found in baseline)"
	@echo "\nAFTER:" >> docs/refactoring/comparison.txt
	@go tool cover -func=docs/refactoring/coverage_after.out | tail -1 >> docs/refactoring/comparison.txt
	@echo "\n=== SUMMARY ===" >> docs/refactoring/comparison.txt
	@echo "\nComparison saved to docs/refactoring/comparison.txt"
	@cat docs/refactoring/comparison.txt

## refactor-check: Run all refactoring validation checks
refactor-check:
	@echo "Running refactoring checks..."
	@echo "\n→ Running tests..."
	@go test ./... -v -count=1 || (echo "✗ Tests failed" && exit 1)
	@echo "\n→ Running staticcheck..."
	@staticcheck ./... || (echo "✗ Staticcheck failed" && exit 1)
	@echo "\n→ Running go vet..."
	@go vet ./... || (echo "✗ Go vet failed" && exit 1)
	@echo "\n→ Checking imports..."
	@goimports -l . | (! grep .) || (echo "⚠ WARNING: Some files need goimports formatting" && goimports -l .)
	@echo "\n→ Running golangci-lint..."
	@golangci-lint run --timeout=5m || echo "⚠ WARNING: golangci-lint found issues"
	@echo "\n→ Checking cyclomatic complexity..."
	@if [ -n "$${STRICT}" ]; then \
		gocyclo -over 15 . && (echo "✗ FAIL: High complexity detected in strict mode" && exit 1) || echo "✓ Complexity OK"; \
	else \
		gocyclo -over 15 . && echo "⚠ WARNING: High complexity detected" || echo "✓ Complexity OK"; \
	fi
	@echo "\n→ Checking file sizes..."
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
	@echo "\n✅ All checks passed"

## refactor-clean: Remove generated refactoring files
refactor-clean:
	@echo "Cleaning refactoring files..."
	@rm -rf docs/refactoring/
	@echo "✓ Refactoring files cleaned"

## help: Show this help message
help:
	@echo "Available targets:"
	@sed -n 's/^##//p' ${MAKEFILE_LIST} | column -t -s ':' | sed -e 's/^/ /'
