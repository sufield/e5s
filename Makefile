.PHONY: test test-verbose test-race test-coverage test-coverage-html test-short \
	clean help examples build build-cli build-all \
	start-stack stop-stack restart-server test-client \
	example-highlevel-server example-highlevel-client example-minikube-server example-minikube-client \
	lint lint-fix fmt fmt-check vet tidy verify \
	ci ci-local \
	sec-deps sec-lint sec-secrets sec-test sec-all sec-install-tools \
	release-check release-build install-tools verify-tools env-versions

# Use bash with strict error handling
SHELL := /bin/bash
.SHELLFLAGS := -o pipefail -o errexit -o nounset -c

# Default target
.DEFAULT_GOAL := help

# Go configuration
GO          ?= go
BINDIR      ?= bin
PKG_CLI     := ./cmd/e5s
PKG_SERVER  := ./cmd/example-server
PKG_CLIENT  := ./cmd/example-client
PKG_MW_EX   := ./examples/middleware

# Test configuration
TEST_PKGS   := ./...
TEST_FLAGS  ?= -count=1

# Version (for releases and env-versions)
VERSION     ?= $(shell git describe --tags --dirty --always 2>/dev/null || echo dev)

# Required tools
REQUIRED_TOOLS := go docker minikube kubectl helm
TOOLS_SEC      := govulncheck gosec gitleaks golangci-lint

# ============================================================================
# Quick Reference
# ============================================================================
# Common workflows:
#   make ci              - Run all CI checks locally before pushing
#   make release-check   - Run all pre-release checks (CI + security + build)
#   make lint            - Quick lint check (what CI runs)
#   make test            - Run tests quickly
#   make build           - Build example binaries (cmd/example-server, cmd/example-client)
#   make build-cli       - Build e5s CLI tool
#   make build-examples  - Build all example code (ensures examples compile)
#   make fmt             - Format code
#   make help            - Show all available targets
#
# Complete Setup Flow:
#   make install-tools   - [1] Install prerequisites (Go, Docker, Minikube, kubectl, Helm)
#   make start-stack     - [2] Start testing stack (Minikube + SPIRE)
#   make restart-server  - [3] Rebuild and restart server during development
#   make test-client     - [4] Rebuild, run, and show logs for client
#   make stop-stack      - [5] Stop everything (apps + SPIRE + Minikube)
#
# Individual Commands:
#   make verify-tools    - Verify all required tools are installed
# ============================================================================

## help: Show this help message
help:
	@echo "e5s - SPIFFE/SPIRE mTLS Library"
	@echo ""
	@echo "Available targets:"
	@sed -n 's/^##//p' ${MAKEFILE_LIST} | column -t -s ':' | sed -e 's/^/ /'

# ============================================================================
# Build Targets
# ============================================================================

## build: Build example binaries (server and client)
build:
	@echo "Building example binaries..."
	@mkdir -p $(BINDIR)
	@$(GO) build -o $(BINDIR)/example-server $(PKG_SERVER)
	@$(GO) build -o $(BINDIR)/example-client $(PKG_CLIENT)
	@echo "✓ Binaries: $(BINDIR)/example-server, $(BINDIR)/example-client"

## build-cli: Build e5s CLI tool
build-cli:
	@echo "Building e5s CLI..."
	@mkdir -p $(BINDIR)
	@$(GO) build -o $(BINDIR)/e5s $(PKG_CLI)
	@echo "✓ Binary: $(BINDIR)/e5s"

## build-examples: Build all example code (ensures examples compile)
build-examples:
	@echo "Building examples..."
	@mkdir -p $(BINDIR)
	@echo "  Building middleware example..."
	@cd $(PKG_MW_EX) && $(GO) build -o ../../$(BINDIR)/example-middleware .
	@echo "✓ Examples built: $(BINDIR)/example-middleware"

## build-all: Build all binaries (examples + CLI)
build-all: build build-cli build-examples
	@echo ""
	@echo "✓ All binaries built successfully"

# ============================================================================
# Development/Testing Targets (Minikube)
# ============================================================================

## start-stack: Start complete testing stack (Minikube + SPIRE + e5s apps)
start-stack:
	@echo "Starting complete testing stack..."
	@echo ""
	@echo "Note: If prerequisites are missing, run 'make install-tools' first (Ubuntu 24.04)"
	@echo ""
	@echo "=== Step 1: Starting Minikube ==="
	@minikube status > /dev/null 2>&1 || minikube start --cpus=4 --memory=8192 --driver=docker
	@echo "✓ Minikube is running"
	@echo ""
	@echo "=== Step 2: Installing SPIRE ==="
	@echo "  → Adding Helm repo..."
	@helm repo add spiffe https://spiffe.github.io/helm-charts-hardened/ > /dev/null 2>&1 || true
	@helm repo update > /dev/null 2>&1
	@echo "  → Creating spire namespace..."
	@kubectl create namespace spire 2>/dev/null || true
	@echo "  → Installing SPIRE CRDs..."
	@helm install spire-crds spire-crds \
		--repo https://spiffe.github.io/helm-charts-hardened/ \
		--namespace spire 2>/dev/null || echo "  (CRDs already installed)"
	@echo "  → Installing SPIRE..."
	@helm install spire spiffe/spire \
		--namespace spire \
		--set global.spire.trustDomain=example.org \
		--set global.spire.clusterName=minikube-cluster 2>/dev/null || echo "  (SPIRE already installed)"
	@echo "  → Waiting for SPIRE Server to be ready..."
	@kubectl wait --for=condition=ready pod \
		-l app.kubernetes.io/name=server \
		-n spire \
		--timeout=120s 2>/dev/null || echo "  (timed out or already running)"
	@echo "  → Waiting for SPIRE Agent to be ready..."
	@kubectl wait --for=condition=ready pod \
		-l app.kubernetes.io/name=agent \
		-n spire \
		--timeout=120s 2>/dev/null || echo "  (timed out or already running)"
	@echo "✓ SPIRE is running"
	@echo ""
	@echo "=== Step 3: Checking e5s applications ==="
	@if kubectl get deployment e5s-server 2>/dev/null; then \
		echo "✓ e5s-server deployment exists"; \
	else \
		echo "  Note: e5s-server not deployed yet. Follow examples/highlevel/TESTING_PRERELEASE.md to deploy."; \
	fi
	@echo ""
	@echo "✓ Stack is ready!"
	@echo ""
	@echo "=== Step 4: Configuring Docker environment ==="
	@echo "To build images for Minikube, run this in your shell:"
	@echo "  eval \$$(minikube -p minikube docker-env)"
	@echo ""
	@echo "Next steps:"
	@echo "  1. Follow examples/highlevel/TESTING_PRERELEASE.md to deploy your test apps"
	@echo "  2. Use 'make restart-server' to rebuild and restart server"
	@echo "  3. Use 'make test-client' to test with client"

## stop-stack: Stop complete testing stack (e5s apps + SPIRE + Minikube)
stop-stack:
	@echo "Stopping complete testing stack..."
	@echo ""
	@echo "=== Step 1: Cleaning up e5s applications ==="
	@kubectl delete deployment e5s-server 2>/dev/null || true
	@kubectl delete service e5s-server 2>/dev/null || true
	@kubectl delete job e5s-client e5s-unregistered-client 2>/dev/null || true
	@kubectl delete configmap e5s-config client-config 2>/dev/null || true
	@kubectl delete serviceaccount unregistered-client 2>/dev/null || true
	@echo "✓ e5s applications cleaned up"
	@echo ""
	@echo "=== Step 2: Uninstalling SPIRE ==="
	@helm uninstall spire -n spire 2>/dev/null || true
	@helm uninstall spire-crds -n spire 2>/dev/null || true
	@kubectl delete clusterrole spire-agent spire-server spire-controller-manager 2>/dev/null || true
	@kubectl delete clusterrolebinding spire-agent spire-server spire-controller-manager 2>/dev/null || true
	@kubectl delete csidriver csi.spiffe.io 2>/dev/null || true
	@kubectl delete validatingwebhookconfiguration spire-server 2>/dev/null || true
	@kubectl delete mutatingwebhookconfiguration spire-controller-manager 2>/dev/null || true
	@kubectl delete crd clusterspiffeids.spire.spiffe.io 2>/dev/null || true
	@kubectl delete crd clusterstaticentries.spire.spiffe.io 2>/dev/null || true
	@kubectl delete crd clusterfederatedtrustdomains.spire.spiffe.io 2>/dev/null || true
	@kubectl delete crd controllermanagerconfigs.spire.spiffe.io 2>/dev/null || true
	@kubectl delete namespace spire 2>/dev/null || true
	@echo "✓ SPIRE uninstalled"
	@echo ""
	@echo "=== Step 3: Stopping Minikube ==="
	@minikube stop 2>/dev/null || true
	@echo "✓ Minikube stopped"
	@echo ""
	@echo "✓ Stack completely stopped"

## restart-server: Rebuild and restart e5s-example-server in Minikube
restart-server:
	@echo "Rebuilding and restarting server..."
	@echo "  1. Building server binary..."
	@CGO_ENABLED=0 $(GO) build -o $(BINDIR)/example-server $(PKG_SERVER)
	@echo "  2. Setting Minikube docker environment..."
	@eval $$(minikube docker-env) && \
		echo "  3. Removing old Docker image..." && \
		docker rmi e5s-server:dev 2>/dev/null || true && \
		echo "  4. Building new Docker image..." && \
		docker build -t e5s-server:dev -f - . <<'EOF' \
FROM alpine:latest \
WORKDIR /app \
COPY $(BINDIR)/example-server . \
ENTRYPOINT ["/app/example-server"] \
EOF
	@echo "  5. Deleting pod to force recreation..."
	@kubectl delete pod -l app=e5s-server 2>/dev/null || true
	@echo "  6. Waiting for new pod..."
	@kubectl wait --for=condition=ready pod -l app=e5s-server --timeout=30s
	@echo "✓ Server restarted successfully"

## test-client: Rebuild, run, and show logs for e5s-example-client in Minikube
test-client:
	@echo "Testing client (rebuild + run + logs)..."
	@echo "  1. Building client binary..."
	@CGO_ENABLED=0 $(GO) build -o $(BINDIR)/example-client $(PKG_CLIENT)
	@echo "  2. Setting Minikube docker environment..."
	@eval $$(minikube docker-env) && \
		echo "  3. Removing old Docker image..." && \
		docker rmi e5s-client:dev 2>/dev/null || true && \
		echo "  4. Building new Docker image..." && \
		docker build -t e5s-client:dev -f - . <<'EOF' \
FROM alpine:latest \
WORKDIR /app \
COPY $(BINDIR)/example-client . \
ENTRYPOINT ["/app/example-client"] \
EOF
	@echo "  5. Replacing job (delete + recreate)..."
	@kubectl replace --force -f examples/highlevel/k8s-client-job.yaml 2>/dev/null || kubectl apply -f examples/highlevel/k8s-client-job.yaml
	@echo "  6. Waiting for job to complete..."
	@sleep 10
	@echo ""
	@echo "=== Client Logs ==="
	@kubectl logs -l app=e5s-client

# ============================================================================
# Code Quality Targets (match CI)
# ============================================================================

## lint: Run golangci-lint (same as CI)
lint:
	@echo "Running golangci-lint..."
	@golangci-lint run --timeout=5m
	@echo "✓ Lint passed"

## lint-fix: Run golangci-lint with auto-fix
lint-fix:
	@echo "Running golangci-lint with auto-fix..."
	@golangci-lint run --fix --timeout=5m
	@echo "✓ Lint fixes applied"

## fmt: Format all Go code
fmt:
	@echo "Formatting code..."
	@gofmt -w -s .
	@echo "✓ Code formatted"

## fmt-check: Check if code is formatted
fmt-check:
	@echo "Checking code formatting..."
	@UNFORMATTED=$$(gofmt -l .); \
	if [ -n "$$UNFORMATTED" ]; then \
		echo "::error::Code is not formatted. Run 'make fmt'"; \
		echo "$$UNFORMATTED"; \
		exit 1; \
	fi
	@echo "✓ Code is properly formatted"

## vet: Run go vet
vet:
	@echo "Running go vet..."
	@$(GO) vet ./...
	@echo "✓ Vet passed"

## tidy: Tidy go modules
tidy:
	@echo "Tidying go modules..."
	@$(GO) mod tidy
	@echo "✓ Modules tidied"

## verify: Verify go modules
verify:
	@echo "Verifying go modules..."
	@$(GO) mod verify
	@echo "✓ Modules verified"

## ci-local: Run all checks that CI runs (local version)
ci-local:
	@echo "Running CI checks with readonly modules..."
	@GOFLAGS="-mod=readonly" $(MAKE) tidy verify lint vet test-race
	@echo ""
	@echo "======================================"
	@echo "✓ All CI checks passed locally!"
	@echo "======================================"

## ci: Alias for ci-local
ci: ci-local

# ============================================================================
# Test Targets
# ============================================================================

## test: Run all tests
test:
	@echo "Running library tests..."
	@$(GO) test $(TEST_FLAGS) $(TEST_PKGS)

## test-verbose: Run tests with verbose output
test-verbose:
	@echo "Running tests (verbose)..."
	@$(GO) test $(TEST_FLAGS) -v $(TEST_PKGS)

## test-race: Run tests with race detector
test-race:
	@echo "Running tests with race detector..."
	@$(GO) test $(TEST_FLAGS) -race $(TEST_PKGS)

## test-short: Run tests in short mode
test-short:
	@echo "Running tests in short mode..."
	@$(GO) test $(TEST_FLAGS) -short $(TEST_PKGS)

## test-coverage: Run tests with coverage report
test-coverage:
	@echo "Running tests with coverage..."
	@$(GO) test $(TEST_FLAGS) -coverprofile=coverage.out $(TEST_PKGS)
	@$(GO) tool cover -func=coverage.out

## test-coverage-html: Generate HTML coverage report
test-coverage-html:
	@echo "Generating HTML coverage report..."
	@$(GO) test $(TEST_FLAGS) -coverprofile=coverage.out $(TEST_PKGS)
	@$(GO) tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report: coverage.html"

## clean: Remove generated files
clean:
	@echo "Cleaning up..."
	@rm -f coverage.out coverage.html
	@rm -f gosec.sarif gitleaks.sarif
	@if [ -d bin/ ]; then rm -rf bin/ && echo "  Removed bin/"; fi
	@if [ -d dist/ ]; then rm -rf dist/ && echo "  Removed dist/"; fi
	@if [ -d artifacts/ ]; then rm -rf artifacts/ && echo "  Removed artifacts/"; fi
	@echo "Clean complete"

## examples: Build all examples (alias for build + build-examples)
examples: build build-examples
	@echo "✓ All example code built successfully"

# ============================================================================
# Security Targets
# ============================================================================

## sec-install-tools: Install security scanning tools
sec-install-tools:
	@echo "Installing security tools..."
	@$(GO) install golang.org/x/vuln/cmd/govulncheck@latest
	@$(GO) install github.com/securego/gosec/v2/cmd/gosec@latest
	@$(GO) install github.com/zricethezav/gitleaks/v8@latest
	@echo "✓ Security tools installed"

## sec-deps: Check for dependency vulnerabilities
sec-deps:
	@echo "Checking for vulnerabilities..."
	@govulncheck ./...
	@echo ""
	@echo "Verifying module hygiene..."
	@$(GO) mod tidy
	@$(GO) mod verify
	@echo "✓ Dependency check complete"

## sec-lint: Run security-focused static analysis
sec-lint:
	@echo "Running security analysis..."
	@$(GO) vet ./...
	@echo ""
	@gosec ./...
	@echo "✓ Security lint complete"

## sec-secrets: Scan for secrets and credentials
sec-secrets:
	@echo "Scanning for secrets..."
	@gitleaks detect --no-git -v
	@echo "✓ Secret scan complete"

## sec-test: Run tests with race detector and coverage
sec-test:
	@echo "Running security tests..."
	@$(GO) test $(TEST_FLAGS) -race -covermode=atomic -coverprofile=coverage.out ./...
	@echo ""
	@echo "Coverage summary:"
	@$(GO) tool cover -func=coverage.out | tail -1
	@echo "✓ Security tests complete"

## sec-all: Run all security checks
sec-all: sec-deps sec-lint sec-secrets sec-test
	@echo ""
	@echo "======================================"
	@echo "✓ All security checks passed!"
	@echo "======================================"

# ============================================================================
# Release Targets
# ============================================================================

## release-check: Run all pre-release checks (includes example builds)
release-check: tidy verify lint vet test-race sec-all build-all
	@echo ""
	@echo "======================================"
	@echo "✓ All automated checks passed!"
	@echo "======================================"
	@echo ""
	@echo "Manual pre-release checklist:"
	@echo ""
	@echo "Documentation:"
	@echo "  [ ] Updated CHANGELOG.md with version and changes"
	@echo "  [ ] Updated README.md if public API changed"
	@echo "  [ ] Updated TUTORIAL.md for new features"
	@echo "  [ ] Reviewed all documentation for accuracy"
	@echo ""
	@echo "Testing:"
	@echo "  [ ] Tested examples in Kubernetes/Minikube"
	@echo "  [ ] Verified SPIRE integration works"
	@echo "  [ ] Ran through TESTING_PRERELEASE.md workflow"
	@echo ""
	@echo "Code Review:"
	@echo "  [ ] Reviewed all public API changes"
	@echo "  [ ] Verified backward compatibility"
	@echo "  [ ] Checked for breaking changes"
	@echo ""
	@echo "Release:"
	@echo "  [ ] Updated version in relevant files"
	@echo "  [ ] Tagged release: git tag vX.Y.Z"
	@echo "  [ ] Pushed tag: git push origin vX.Y.Z"
	@echo ""
	@echo "Post-release verification:"
	@echo "  [ ] Test: go install github.com/sufield/e5s/cmd/e5s@latest"
	@echo "  [ ] Verify examples work with released version"
	@echo "  [ ] Check GitHub release page"
	@echo ""

## release-build: Build release binaries for all platforms
release-build:
	@echo "Building release binaries..."
	@mkdir -p dist
	@echo "Building e5s CLI for Linux/amd64..."
	@GOOS=linux GOARCH=amd64 $(GO) build -o dist/e5s-linux-amd64 $(PKG_CLI)
	@echo "Building e5s CLI for Linux/arm64..."
	@GOOS=linux GOARCH=arm64 $(GO) build -o dist/e5s-linux-arm64 $(PKG_CLI)
	@echo "Building e5s CLI for macOS/amd64..."
	@GOOS=darwin GOARCH=amd64 $(GO) build -o dist/e5s-darwin-amd64 $(PKG_CLI)
	@echo "Building e5s CLI for macOS/arm64..."
	@GOOS=darwin GOARCH=arm64 $(GO) build -o dist/e5s-darwin-arm64 $(PKG_CLI)
	@echo "Building e5s CLI for Windows/amd64..."
	@GOOS=windows GOARCH=amd64 $(GO) build -o dist/e5s-windows-amd64.exe $(PKG_CLI)
	@echo ""
	@echo "✓ Release binaries built in dist/"
	@ls -lh dist/

## install-tools: Install all required tools (Ubuntu 24.04 only)
install-tools:
	@bash scripts/install-tools-ubuntu-24.04.sh

## verify-tools: Verify required tools are installed
verify-tools:
	@echo "Verifying required tools..."
	@echo ""
	@echo "Core tools:"
	@for tool in $(REQUIRED_TOOLS); do \
		printf "  %-12s " "$$tool:"; \
		if command -v $$tool >/dev/null 2>&1; then \
			echo "✓"; \
		else \
			echo "✗ MISSING"; \
			exit 1; \
		fi; \
	done
	@echo ""
	@echo "Security tools (optional):"
	@for tool in $(TOOLS_SEC); do \
		printf "  %-12s " "$$tool:"; \
		if command -v $$tool >/dev/null 2>&1; then \
			echo "✓"; \
		else \
			echo "- (run 'make sec-install-tools' to install)"; \
		fi; \
	done
	@echo ""
	@echo "✓ All required tools are available"

## env-versions: Capture current environment versions
env-versions:
	@echo "Capturing environment versions..."
	@E5S_VERSION=$(VERSION) bash scripts/env-versions.sh dev artifacts
	@echo ""
	@echo "Environment version info saved to artifacts/env-versions-dev.txt"
	@echo "Use this information to update VERSIONS.md and CHANGELOG.md"
