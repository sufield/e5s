.PHONY: test test-verbose test-race test-coverage test-coverage-html test-short \
	clean help examples build build-cli build-all \
	example-highlevel-server example-highlevel-client example-minikube-server example-minikube-client \
	lint lint-fix fmt fmt-check vet tidy verify \
	ci ci-local \
	sec-deps sec-lint sec-secrets sec-test sec-all sec-install-tools \
	release-check release-build

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
#   make build           - Build example binaries
#   make build-cli       - Build e5s CLI tool
#   make fmt             - Format code
#   make help            - Show all available targets
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

## build-all: Build all binaries (examples + CLI)
build-all: build build-cli examples
	@echo ""
	@echo "✓ All binaries built successfully"

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

## release-check: Run all pre-release checks
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
