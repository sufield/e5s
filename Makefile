.PHONY: test test-verbose test-race test-coverage test-coverage-html test-short \
	clean help prereqs check-prereqs build prod-build dev-build test-prod-build \
	helm-lint helm-template minikube-up minikube-down minikube-status \
	minikube-delete ci-test ci-build

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

## test: Run all tests
test:
	@echo "Running all tests..."
	@go test ./...

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
	@go build -trimpath $(LDFLAGS) -o $(BINARY_PROD) ./cmd/agent
	@echo "Production binary: $(BINARY_PROD)"
	@ls -lh $(BINARY_PROD)

## dev-build: Build dev binary with dev tags
dev-build:
	@echo "Building dev binary with -tags=dev..."
	@mkdir -p bin
	@go build -tags=$(BUILD_TAGS_DEV) -o $(BINARY_DEV) ./cmd/cp-minikube
	@echo "Dev binary: $(BINARY_DEV)"
	@ls -lh $(BINARY_DEV)

## test-prod-build: Verify production build excludes dev code
test-prod-build:
	@echo "Testing production build..."
	@echo "→ Checking for dev package imports in production cmd..."
	@if go list -f '{{.Imports}}' ./cmd/agent 2>&1 | grep -q "controlplane\|cp-minikube"; then \
		echo "✗ ERROR: Production cmd imports dev packages!"; \
		exit 1; \
	fi
	@echo "→ Verifying dev cmd requires dev tags..."
	@if go build -o /tmp/test-dev ./cmd/cp-minikube 2>&1; then \
		echo "✗ ERROR: Dev cmd should not build without -tags=dev!"; \
		rm -f /tmp/test-dev; \
		exit 1; \
	fi
	@echo "→ Verifying dev cmd builds with dev tags..."
	@if ! go build -tags=dev -o /tmp/test-dev ./cmd/cp-minikube 2>&1; then \
		echo "✗ ERROR: Dev cmd failed to build with -tags=dev!"; \
		exit 1; \
	fi
	@rm -f /tmp/test-dev
	@echo "→ Checking production binary for dev strings..."
	@if [ -f "$(BINARY_PROD)" ]; then \
		if strings $(BINARY_PROD) 2>/dev/null | grep -q "cp-minikube\|BootstrapMinikubeInfra"; then \
			echo "✗ ERROR: Dev code found in production binary!"; \
			exit 1; \
		fi; \
	else \
		echo "  (Binary not built, skipping string check)"; \
	fi
	@echo "✓ Production build check passed"

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

## help: Show this help message
help:
	@echo "Available targets:"
	@sed -n 's/^##//p' ${MAKEFILE_LIST} | column -t -s ':' | sed -e 's/^/ /'
