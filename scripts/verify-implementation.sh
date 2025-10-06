#!/bin/bash
set -e

echo "============================================"
echo "SPIRE Adapter Implementation Verification"
echo "============================================"
echo ""

# Colors for output
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

success() {
    echo -e "${GREEN}✅ $1${NC}"
}

error() {
    echo -e "${RED}❌ $1${NC}"
}

info() {
    echo -e "${YELLOW}ℹ️  $1${NC}"
}

# Change to project root
cd "$(dirname "$0")/.."

echo "Step 1: Build Verification"
echo "-------------------------------------------"

# Clean previous builds
info "Cleaning previous builds..."
rm -rf bin/
mkdir -p bin/
rm -f coverage.out 2>/dev/null || true

# Production build
info "Building production binary..."
if go build -trimpath -ldflags "-s -w" -o bin/spire-server ./cmd/agent 2>/dev/null; then
    PROD_SIZE=$(ls -lh bin/spire-server | awk '{print $5}')
    success "Production build successful (size: $PROD_SIZE)"
else
    error "Production build failed"
    exit 1
fi

# Dev build
info "Building dev binary..."
if go build -trimpath -ldflags "-s -w" -tags=dev -o bin/cp-minikube ./cmd/agent 2>/dev/null; then
    DEV_SIZE=$(ls -lh bin/cp-minikube | awk '{print $5}')
    success "Dev build successful (size: $DEV_SIZE)"
else
    error "Dev build failed"
    exit 1
fi

echo ""
echo "Step 2: Binary Separation Verification"
echo "-------------------------------------------"

# Check production binary excludes dev code
if strings bin/spire-server 2>/dev/null | grep -q "BootstrapMinikubeInfra" 2>/dev/null; then
    error "Production binary contains dev code"
    exit 1
else
    success "Production binary excludes dev code"
fi

# Check dev binary includes in-memory adapters
if strings bin/cp-minikube 2>/dev/null | grep -q "InMemoryAdapterFactory" 2>/dev/null; then
    success "Dev binary includes in-memory adapters"
else
    error "Dev binary missing in-memory adapters"
    exit 1
fi

# Check production binary includes SPIRE adapter
if strings bin/spire-server 2>/dev/null | grep -q "SPIREClient" 2>/dev/null; then
    success "Production binary includes SPIRE adapter"
else
    error "Production binary missing SPIRE adapter"
    exit 1
fi

# Verify build constraints work correctly
info "Testing build constraints..."
if go build -tags=dev -o /tmp/test-build-constraints ./cmd/agent 2>&1 >/dev/null; then
    success "Dev build constraints valid"
    rm -f /tmp/test-build-constraints
else
    error "Dev build constraints failed"
    exit 1
fi

if go list -tags=dev -f '{{.GoFiles}}' ./wiring 2>&1 | grep -q "cp_helm_minikube_dev.go"; then
    success "Dev-tagged files included in dev build"
else
    error "Dev-tagged files not found in dev build"
    exit 1
fi

echo ""
echo "Step 3: Package Compilation"
echo "-------------------------------------------"

info "Building all packages..."
if go build ./... 2>/dev/null; then
    success "All packages compile successfully"
else
    error "Package compilation failed"
    exit 1
fi

echo ""
echo "Step 4: Unit Tests"
echo "-------------------------------------------"

info "Running unit tests with coverage..."
if go test ./... -short -coverprofile=coverage.out -covermode=atomic 2>&1 | grep -q "FAIL"; then
    error "Unit tests failed"
    go test ./... -short -v
    exit 1
else
    success "All unit tests pass"

    # Display coverage summary
    if [ -f coverage.out ]; then
        COVERAGE=$(go tool cover -func=coverage.out | grep total | awk '{print $3}')
        info "Total test coverage: $COVERAGE"
    fi
fi

info "Running race detector tests..."
if go test ./... -short -race 2>&1 | tee /tmp/race-test.log | grep -q "FAIL"; then
    error "Race detector found issues"
    cat /tmp/race-test.log
    exit 1
else
    success "No race conditions detected"
fi

echo ""
echo "Step 5: Code Quality Checks"
echo "-------------------------------------------"

# Check formatting
info "Checking code formatting..."
UNFORMATTED=$(gofmt -l . | grep -v vendor || true)
if [ -z "$UNFORMATTED" ]; then
    success "Code is properly formatted"
else
    error "Code needs formatting:"
    echo "$UNFORMATTED"
fi

# Run go vet
info "Running go vet..."
if go vet ./... 2>&1 | grep -q "vet:"; then
    error "Go vet found issues"
    go vet ./...
    exit 1
else
    success "Go vet passed"
fi

echo ""
echo "Step 6: Dependency Verification"
echo "-------------------------------------------"

# Check go-spiffe dependency
SPIFFE_VERSION=$(go list -m github.com/spiffe/go-spiffe/v2 2>/dev/null || echo "not found")
if [[ "$SPIFFE_VERSION" == *"github.com/spiffe/go-spiffe/v2"* ]]; then
    success "go-spiffe dependency installed: $SPIFFE_VERSION"
else
    error "go-spiffe dependency missing"
    exit 1
fi

echo ""
echo "Step 7: File Structure Verification"
echo "-------------------------------------------"

REQUIRED_FILES=(
    "internal/adapters/outbound/spire/client.go"
    "internal/adapters/outbound/spire/agent.go"
    "internal/adapters/outbound/spire/server.go"
    "internal/adapters/outbound/spire/identity_provider.go"
    "internal/adapters/outbound/spire/bundle_provider.go"
    "internal/adapters/outbound/spire/attestor.go"
    "internal/adapters/outbound/spire/translation.go"
    "internal/adapters/outbound/compose/spire.go"
    "cmd/agent/main_prod.go"
    "cmd/agent/main_dev.go"
)

for file in "${REQUIRED_FILES[@]}"; do
    if [ -f "$file" ]; then
        success "Found: $file"
    else
        error "Missing: $file"
        exit 1
    fi
done

# Verify SPIRE adapter file count
SPIRE_FILE_COUNT=$(find internal/adapters/outbound/spire -name "*.go" -type f | wc -l)
EXPECTED_MIN=7
if [ "$SPIRE_FILE_COUNT" -ge "$EXPECTED_MIN" ]; then
    success "SPIRE adapter has $SPIRE_FILE_COUNT Go files (expected >= $EXPECTED_MIN)"
else
    error "SPIRE adapter only has $SPIRE_FILE_COUNT Go files (expected >= $EXPECTED_MIN)"
    exit 1
fi

echo ""
echo "============================================"
echo "VERIFICATION SUMMARY"
echo "============================================"
echo ""
success "All automated checks passed! ✅"
echo ""
echo "Build artifacts:"
echo "  - Production: bin/spire-server ($PROD_SIZE)"
echo "  - Dev:        bin/cp-minikube ($DEV_SIZE)"
echo ""
if [ -f coverage.out ]; then
    echo "Test coverage:"
    echo "  - Total: $COVERAGE"
    echo "  - Report: coverage.out"
    info "View HTML coverage: go tool cover -html=coverage.out"
fi
echo ""
info "To test against live SPIRE infrastructure:"
echo "  1. Start SPIRE: make minikube-up"
echo "  2. Follow docs/VERIFICATION.md for integration tests"
echo ""
