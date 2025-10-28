.PHONY: test test-verbose test-race test-coverage test-coverage-html test-short \
	clean help examples \
	example-highlevel-server example-highlevel-client example-minikube-server example-minikube-client \
	sec-deps sec-lint sec-secrets sec-test sec-all sec-install-tools

# Use bash with strict error handling
SHELL := /bin/bash
.SHELLFLAGS := -o pipefail -o errexit -o nounset -c

# Default target
.DEFAULT_GOAL := help

# Required tools
REQUIRED_TOOLS=go
TOOLS_SEC=govulncheck gosec gitleaks

## help: Show this help message
help:
	@echo "e5s - SPIFFE/SPIRE mTLS Library"
	@echo ""
	@echo "Available targets:"
	@sed -n 's/^##//p' ${MAKEFILE_LIST} | column -t -s ':' | sed -e 's/^/ /'

## test: Run all tests
test:
	@echo "Running library tests..."
	@go test ./pkg/...

## test-verbose: Run tests with verbose output
test-verbose:
	@echo "Running tests (verbose)..."
	@go test -v ./pkg/...

## test-race: Run tests with race detector
test-race:
	@echo "Running tests with race detector..."
	@go test -race ./pkg/...

## test-short: Run tests in short mode
test-short:
	@echo "Running tests in short mode..."
	@go test -short ./pkg/...

## test-coverage: Run tests with coverage report
test-coverage:
	@echo "Running tests with coverage..."
	@go test -coverprofile=coverage.out ./pkg/...
	@go tool cover -func=coverage.out

## test-coverage-html: Generate HTML coverage report
test-coverage-html:
	@echo "Generating HTML coverage report..."
	@go test -coverprofile=coverage.out ./pkg/...
	@go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report: coverage.html"

## clean: Remove generated files
clean:
	@echo "Cleaning up..."
	@rm -f coverage.out coverage.html
	@rm -f gosec.sarif gitleaks.sarif
	@if [ -d bin/ ]; then rm -rf bin/ && echo "  Removed bin/"; fi
	@echo "Clean complete"

## example-highlevel-server: Build highlevel example mTLS server (high-level API)
example-highlevel-server:
	@echo "Building highlevel example server..."
	@mkdir -p bin
	@cd examples/highlevel && go build -o ../../bin/highlevel-server ./cmd/server
	@echo "Binary: bin/highlevel-server"

## example-highlevel-client: Build highlevel example mTLS client (high-level API)
example-highlevel-client:
	@echo "Building highlevel example client..."
	@mkdir -p bin
	@cd examples/highlevel && go build -o ../../bin/highlevel-client ./cmd/client
	@echo "Binary: bin/highlevel-client"

## example-minikube-server: Build minikube-lowlevel example mTLS server (low-level API)
example-minikube-server:
	@echo "Building minikube-lowlevel example server..."
	@mkdir -p bin
	@cd examples/minikube-lowlevel && go build -o ../../bin/minikube-server ./cmd/server
	@echo "Binary: bin/minikube-server"

## example-minikube-client: Build minikube-lowlevel example mTLS client (low-level API)
example-minikube-client:
	@echo "Building minikube-lowlevel example client..."
	@mkdir -p bin
	@cd examples/minikube-lowlevel && go build -o ../../bin/minikube-client ./cmd/client
	@echo "Binary: bin/minikube-client"

## examples: Build all examples
examples: example-highlevel-server example-highlevel-client example-minikube-server example-minikube-client

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
	@go vet ./pkg/...
	@echo ""
	@gosec ./pkg/...
	@echo "✓ Security lint complete"

## sec-secrets: Scan for secrets and credentials
sec-secrets:
	@echo "Scanning for secrets..."
	@gitleaks detect --no-git -v
	@echo "✓ Secret scan complete"

## sec-test: Run tests with race detector and coverage
sec-test:
	@echo "Running security tests..."
	@go test -race -covermode=atomic -coverprofile=coverage.out ./pkg/...
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
