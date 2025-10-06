.PHONY: test test-verbose test-coverage test-coverage-html clean help

# Default target
.DEFAULT_GOAL := help

## test: Run all tests
test:
	@echo "Running all tests..."
	@go test ./...

## test-verbose: Run all tests with verbose output
test-verbose:
	@echo "Running all tests (verbose)..."
	@go test -v ./...

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
	@echo "Clean complete"

## help: Show this help message
help:
	@echo "Available targets:"
	@sed -n 's/^##//p' ${MAKEFILE_LIST} | column -t -s ':' | sed -e 's/^/ /'
