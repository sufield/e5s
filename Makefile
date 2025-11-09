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

# Required tools
REQUIRED_TOOLS=go
TOOLS_SEC=govulncheck gosec gitleaks

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
# Development (Minikube):
#   make start-stack     - Start complete stack (Minikube + SPIRE + apps)
#   make stop-stack      - Stop complete stack (apps + SPIRE + Minikube)
#   make restart-server  - Rebuild and restart server in one command
#   make test-client     - Rebuild, run, and show logs for client in one command
#
# Setup (Ubuntu 24.04 only):
#   make install-tools   - Install all required tools (Go, Docker, kubectl, etc.)
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
	@mkdir -p bin
	@go build -o bin/example-server ./cmd/example-server
	@go build -o bin/example-client ./cmd/example-client
	@echo "✓ Binaries: bin/example-server, bin/example-client"

## build-cli: Build e5s CLI tool
build-cli:
	@echo "Building e5s CLI..."
	@mkdir -p bin
	@go build -o bin/e5s ./cmd/e5s
	@echo "✓ Binary: bin/e5s"

## build-examples: Build all example code (ensures examples compile)
build-examples:
	@echo "Building examples..."
	@mkdir -p bin
	@echo "  Building middleware example..."
	@cd examples/middleware && go build -o ../../bin/example-middleware .
	@echo "✓ Examples built: bin/example-middleware"

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
	@helm install spire spire \
		--repo https://spiffe.github.io/helm-charts-hardened/ \
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
	@CGO_ENABLED=0 go build -o bin/example-server ./cmd/example-server
	@echo "  2. Setting Minikube docker environment..."
	@eval $$(minikube docker-env) && \
		echo "  3. Removing old Docker image..." && \
		docker rmi e5s-server:dev 2>/dev/null || true && \
		echo "  4. Building new Docker image..." && \
		docker build -t e5s-server:dev -f - . <<'EOF' \
FROM alpine:latest \
WORKDIR /app \
COPY bin/example-server . \
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
	@CGO_ENABLED=0 go build -o bin/example-client ./cmd/example-client
	@echo "  2. Setting Minikube docker environment..."
	@eval $$(minikube docker-env) && \
		echo "  3. Removing old Docker image..." && \
		docker rmi e5s-client:dev 2>/dev/null || true && \
		echo "  4. Building new Docker image..." && \
		docker build -t e5s-client:dev -f - . <<'EOF' \
FROM alpine:latest \
WORKDIR /app \
COPY bin/example-client . \
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
	@if [ -n "$$(gofmt -l .)" ]; then \
		echo "::error::Code is not formatted. Run 'make fmt'"; \
		gofmt -l .; \
		exit 1; \
	fi
	@echo "✓ Code is properly formatted"

## vet: Run go vet
vet:
	@echo "Running go vet..."
	@go vet ./...
	@echo "✓ Vet passed"

## tidy: Tidy go modules
tidy:
	@echo "Tidying go modules..."
	@go mod tidy
	@echo "✓ Modules tidied"

## verify: Verify go modules
verify:
	@echo "Verifying go modules..."
	@go mod verify
	@echo "✓ Modules verified"

## ci-local: Run all checks that CI runs (local version)
ci-local: tidy verify lint vet test-race
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
	@go test ./...

## test-verbose: Run tests with verbose output
test-verbose:
	@echo "Running tests (verbose)..."
	@go test -v ./...

## test-race: Run tests with race detector
test-race:
	@echo "Running tests with race detector..."
	@go test -race ./...

## test-short: Run tests in short mode
test-short:
	@echo "Running tests in short mode..."
	@go test -short ./...

## test-coverage: Run tests with coverage report
test-coverage:
	@echo "Running tests with coverage..."
	@go test -coverprofile=coverage.out ./...
	@go tool cover -func=coverage.out

## test-coverage-html: Generate HTML coverage report
test-coverage-html:
	@echo "Generating HTML coverage report..."
	@go test -coverprofile=coverage.out ./...
	@go tool cover -html=coverage.out -o coverage.html
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
	@go install golang.org/x/vuln/cmd/govulncheck@latest
	@go install github.com/securego/gosec/v2/cmd/gosec@latest
	@go install github.com/zricethezav/gitleaks/v8@latest
	@echo "✓ Security tools installed"

## sec-deps: Check for dependency vulnerabilities
sec-deps:
	@echo "Checking for vulnerabilities..."
	@govulncheck ./...
	@echo ""
	@echo "Verifying module hygiene..."
	@go mod tidy
	@go mod verify
	@echo "✓ Dependency check complete"

## sec-lint: Run security-focused static analysis
sec-lint:
	@echo "Running security analysis..."
	@go vet ./...
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
	@go test -race -covermode=atomic -coverprofile=coverage.out ./...
	@echo ""
	@echo "Coverage summary:"
	@go tool cover -func=coverage.out | tail -1
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
	@GOOS=linux GOARCH=amd64 go build -o dist/e5s-linux-amd64 ./cmd/e5s
	@echo "Building e5s CLI for Linux/arm64..."
	@GOOS=linux GOARCH=arm64 go build -o dist/e5s-linux-arm64 ./cmd/e5s
	@echo "Building e5s CLI for macOS/amd64..."
	@GOOS=darwin GOARCH=amd64 go build -o dist/e5s-darwin-amd64 ./cmd/e5s
	@echo "Building e5s CLI for macOS/arm64..."
	@GOOS=darwin GOARCH=arm64 go build -o dist/e5s-darwin-arm64 ./cmd/e5s
	@echo "Building e5s CLI for Windows/amd64..."
	@GOOS=windows GOARCH=amd64 go build -o dist/e5s-windows-amd64.exe ./cmd/e5s
	@echo ""
	@echo "✓ Release binaries built in dist/"
	@ls -lh dist/

## install-tools: Install all required tools (Ubuntu 24.04 only)
install-tools:
	@echo "Installing required tools for Ubuntu 24.04..."
	@echo ""
	@# Check OS
	@if [ ! -f /etc/os-release ]; then \
		echo "Error: /etc/os-release not found. This target is for Ubuntu 24.04 only."; \
		exit 1; \
	fi
	@if ! grep -q "Ubuntu" /etc/os-release || ! grep -q "24.04" /etc/os-release; then \
		echo "Error: This target is designed for Ubuntu 24.04 only."; \
		echo "Current OS:"; \
		cat /etc/os-release | grep PRETTY_NAME; \
		exit 1; \
	fi
	@echo "Detected Ubuntu 24.04 ✓"
	@echo ""
	@# Install Go 1.25.3
	@echo "Installing Go 1.25.3..."
	@if command -v go >/dev/null 2>&1; then \
		CURRENT_GO=$$(go version | awk '{print $$3}' | sed 's/go//'); \
		echo "  Current Go version: $$CURRENT_GO"; \
		if [ "$$CURRENT_GO" != "1.25.3" ]; then \
			echo "  Removing old Go version..."; \
			sudo rm -rf /usr/local/go; \
		else \
			echo "  Go 1.25.3 already installed ✓"; \
		fi; \
	fi
	@if ! command -v go >/dev/null 2>&1 || ! go version | grep -q "go1.25.3"; then \
		echo "  Downloading Go 1.25.3..."; \
		cd /tmp && curl -LO https://go.dev/dl/go1.25.3.linux-amd64.tar.gz; \
		echo "  Installing to /usr/local/go..."; \
		sudo tar -C /usr/local -xzf go1.25.3.linux-amd64.tar.gz; \
		rm go1.25.3.linux-amd64.tar.gz; \
		if ! grep -q "/usr/local/go/bin" ~/.bashrc; then \
			echo 'export PATH=$$PATH:/usr/local/go/bin' >> ~/.bashrc; \
			echo "  Added Go to PATH in ~/.bashrc"; \
		fi; \
		export PATH=$$PATH:/usr/local/go/bin; \
		/usr/local/go/bin/go version; \
		echo "  ✓ Go 1.25.3 installed"; \
	fi
	@echo ""
	@# Install Docker
	@echo "Installing Docker..."
	@if command -v docker >/dev/null 2>&1; then \
		echo "  Docker already installed: $$(docker --version) ✓"; \
	else \
		echo "  Adding Docker repository..."; \
		sudo apt-get update -qq; \
		sudo apt-get install -y -qq ca-certificates curl; \
		sudo install -m 0755 -d /etc/apt/keyrings; \
		sudo curl -fsSL https://download.docker.com/linux/ubuntu/gpg -o /etc/apt/keyrings/docker.asc; \
		sudo chmod a+r /etc/apt/keyrings/docker.asc; \
		echo "deb [arch=$$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/docker.asc] https://download.docker.com/linux/ubuntu $$(. /etc/os-release && echo $$VERSION_CODENAME) stable" | \
			sudo tee /etc/apt/sources.list.d/docker.list > /dev/null; \
		echo "  Installing Docker..."; \
		sudo apt-get update -qq; \
		sudo apt-get install -y -qq docker-ce docker-ce-cli containerd.io docker-buildx-plugin docker-compose-plugin; \
		echo "  Adding user to docker group..."; \
		sudo usermod -aG docker $$USER; \
		echo "  ✓ Docker installed (logout/login required for docker group)"; \
	fi
	@echo ""
	@# Install kubectl
	@echo "Installing kubectl..."
	@if command -v kubectl >/dev/null 2>&1; then \
		echo "  kubectl already installed: $$(kubectl version --client --short 2>/dev/null || kubectl version --client) ✓"; \
	else \
		echo "  Downloading latest kubectl..."; \
		cd /tmp && curl -LO "https://dl.k8s.io/release/$$(curl -L -s https://dl.k8s.io/release/stable.txt)/bin/linux/amd64/kubectl"; \
		sudo install -o root -g root -m 0755 kubectl /usr/local/bin/kubectl; \
		rm kubectl; \
		echo "  ✓ kubectl installed: $$(kubectl version --client --short 2>/dev/null || kubectl version --client)"; \
	fi
	@echo ""
	@# Install Minikube
	@echo "Installing Minikube..."
	@if command -v minikube >/dev/null 2>&1; then \
		echo "  Minikube already installed: $$(minikube version --short) ✓"; \
	else \
		echo "  Downloading latest Minikube..."; \
		cd /tmp && curl -LO https://storage.googleapis.com/minikube/releases/latest/minikube-linux-amd64; \
		sudo install minikube-linux-amd64 /usr/local/bin/minikube; \
		rm minikube-linux-amd64; \
		echo "  ✓ Minikube installed: $$(minikube version --short)"; \
	fi
	@echo ""
	@# Install Helm
	@echo "Installing Helm..."
	@if command -v helm >/dev/null 2>&1; then \
		echo "  Helm already installed: $$(helm version --short) ✓"; \
	else \
		echo "  Downloading and installing Helm..."; \
		cd /tmp && curl -fsSL https://raw.githubusercontent.com/helm/helm/main/scripts/get-helm-3 | bash; \
		echo "  ✓ Helm installed: $$(helm version --short)"; \
	fi
	@echo ""
	@echo "======================================"
	@echo "✓ All tools installed successfully!"
	@echo "======================================"
	@echo ""
	@echo "IMPORTANT: If Docker was just installed, you need to:"
	@echo "  1. Logout and login again (for docker group to take effect)"
	@echo "  2. OR run: newgrp docker"
	@echo ""
	@echo "IMPORTANT: If Go was just installed, run:"
	@echo "  source ~/.bashrc"
	@echo "  OR start a new terminal session"
	@echo ""
	@echo "Verify installation with: make verify-tools"

## verify-tools: Verify required tools are installed
verify-tools:
	@echo "Verifying required tools..."
	@echo ""
	@go version
	@docker --version
	@minikube status || echo "Minikube not running (run 'minikube start' to start it)"
	@kubectl version --client
	@echo ""
	@echo "✓ All required tools are available"

## env-versions: Capture current environment versions
env-versions:
	@echo "Capturing environment versions..."
	@E5S_VERSION=$(version) bash scripts/env-versions.sh dev artifacts
	@echo ""
	@echo "Environment version info saved to artifacts/env-versions-dev.txt"
	@echo "Use this information to update VERSIONS.md and CHANGELOG.md"
