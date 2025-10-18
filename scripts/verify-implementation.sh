#!/usr/bin/env bash
set -Eeuo pipefail

# ---------- util ----------
trap 'echo -e "\033[0;31m❌ Error at line $LINENO: $BASH_COMMAND\033[0m"; exit 1' ERR

GREEN='\033[0;32m'; RED='\033[0;31m'; YELLOW='\033[1;33m'; NC='\033[0m'
success(){ echo -e "${GREEN}✅ $1${NC}"; }
error(){   echo -e "${RED}❌ $1${NC}"; }
info(){    echo -e "${YELLOW}ℹ️  $1${NC}"; }

script_dir="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" &>/dev/null && pwd)"
cd "$script_dir/.."

echo "============================================"
echo "SPIRE Adapter Implementation Verification"
echo "============================================"
echo ""

prod_bin="bin/agent-prod"
dev_bin="bin/agent-dev"

size_of() {
  # Human readable size with fallback
  if stat --version &>/dev/null; then
    # GNU
    stat -c %s "$1"
  else
    # BSD/macOS
    stat -f %z "$1"
  fi
}

human_readable_size() {
  local bytes=$1
  if [ "$bytes" -lt 1024 ]; then
    echo "${bytes}B"
  elif [ "$bytes" -lt 1048576 ]; then
    echo "$((bytes / 1024))KB"
  else
    echo "$((bytes / 1048576))MB"
  fi
}

# ---------- step 1: builds ----------
echo "Step 1: Build Verification"
echo "-------------------------------------------"
info "Cleaning previous builds..."
rm -rf bin coverage.out || true
mkdir -p bin

info "Building production binary..."
GOFLAGS='-mod=readonly' go build -trimpath -ldflags "-s -w" -o "$prod_bin" ./cmd
PROD_SIZE_BYTES="$(size_of "$prod_bin")"
PROD_SIZE_HR="$(human_readable_size "$PROD_SIZE_BYTES")"
success "Production build ok (size: $PROD_SIZE_HR)"

info "Building dev binary..."
GOFLAGS='-mod=readonly' go build -trimpath -ldflags "-s -w" -tags=dev -o "$dev_bin" ./cmd
DEV_SIZE_BYTES="$(size_of "$dev_bin")"
DEV_SIZE_HR="$(human_readable_size "$DEV_SIZE_BYTES")"
success "Dev build ok (size: $DEV_SIZE_HR)"

# ---------- step 2: build-tag truth tests ----------
echo ""
echo "Step 2: Build-Tag Source Inclusion"
echo "-------------------------------------------"

# Assert dev-tagged files appear only in dev builds.
# Check via go list on the package that owns the dev files.
info "Checking dev-tagged file visibility..."
dev_files_dev=$(go list -tags=dev -f '{{range .GoFiles}}{{println .}}{{end}}' ./wiring | grep -E '(_dev\.go|cp_.*_dev\.go)' || true)
dev_files_prod=$(go list -f '{{range .GoFiles}}{{println .}}{{end}}' ./wiring | grep -E '(_dev\.go|cp_.*_dev\.go)' || true)

if [[ -n "$dev_files_dev" && -z "$dev_files_prod" ]]; then
  success "Dev-tagged files are included only in dev builds"
else
  error "Dev-tagged files selection is incorrect
dev with -tags=dev:
$dev_files_dev

prod (no tags):
$dev_files_prod"
  exit 1
fi

# Verify SPIRE adapter files are present in prod selection
info "Verifying SPIRE adapter files are part of prod selection..."
spire_pkg="./internal/adapters/outbound/spire"
prod_spire_files=$(go list -f '{{range .GoFiles}}{{println .}}{{end}}' "$spire_pkg" || true)
if [[ -n "$prod_spire_files" ]]; then
  success "SPIRE adapter package files selected in prod build"
else
  error "SPIRE adapter package not selected in prod build"
  exit 1
fi

# Verify compose/spire.go is selected in prod
info "Verifying compose SPIRE factory is part of prod selection..."
compose_pkg="./internal/adapters/outbound/compose"
prod_compose_files=$(go list -f '{{range .GoFiles}}{{println .}}{{end}}' "$compose_pkg" | grep -E 'spire\.go' || true)
if [[ -n "$prod_compose_files" ]]; then
  success "Compose SPIRE factory selected in prod build"
else
  error "Compose SPIRE factory not selected in prod build"
  exit 1
fi

# ---------- step 3: package compilation ----------
echo ""
echo "Step 3: Package Compilation"
echo "-------------------------------------------"
info "Building all packages (no tags)..."
GOFLAGS='-mod=readonly' go build ./...
success "All packages compile"

# ---------- step 4: tests ----------
echo ""
echo "Step 4: Unit Tests"
echo "-------------------------------------------"

info "Running unit tests (short, coverage)..."
GOFLAGS='-mod=readonly' go test ./... -short -count=1 -coverprofile=coverage.out -covermode=atomic
success "All unit tests pass"

if [[ -f coverage.out ]]; then
  total_cov="$(go tool cover -func=coverage.out | awk '/^total:/ {print $3}')"
  info "Total test coverage: $total_cov"
fi

info "Running race detector tests (short)..."
GOFLAGS='-mod=readonly' go test ./... -short -race -count=1
success "No race conditions detected"

# ---------- step 5: code quality ----------
echo ""
echo "Step 5: Code Quality Checks"
echo "-------------------------------------------"

info "Checking formatting..."
unformatted=$(gofmt -l . | grep -v '^vendor/' || true)
if [[ -z "$unformatted" ]]; then
  success "Code is properly formatted"
else
  error "Code needs formatting:
$unformatted"
  exit 1
fi

info "Running go vet..."
# Capture output to show on failure
if output=$(go vet ./... 2>&1); then
  success "Go vet passed"
else
  error "Go vet found issues:
$output"
  exit 1
fi

# ---------- step 6: deps ----------
echo ""
echo "Step 6: Dependency Verification"
echo "-------------------------------------------"
if mod_info=$(go list -m -json github.com/spiffe/go-spiffe/v2 2>/dev/null); then
  # Parse JSON more reliably
  mod_path=$(echo "$mod_info" | awk -F'"' '/"Path"/{print $4}')
  mod_version=$(echo "$mod_info" | awk -F'"' '/"Version"/{print $4}')
  if [[ -n "$mod_path" && -n "$mod_version" ]]; then
    success "go-spiffe present: $mod_path $mod_version"
  else
    error "Failed to parse go-spiffe module info"
    exit 1
  fi
else
  error "go-spiffe dependency missing"
  exit 1
fi

# ---------- step 7: file structure ----------
echo ""
echo "Step 7: File Structure Verification"
echo "-------------------------------------------"
declare -a required=(
  "internal/adapters/outbound/spire/client.go"
  "internal/adapters/outbound/spire/agent.go"
  "internal/adapters/outbound/spire/identity_provider.go"
  "internal/adapters/outbound/spire/bundle_provider.go"
  "internal/adapters/outbound/spire/document_provider.go"
  "internal/adapters/outbound/spire/identity_credential_parser.go"
  "internal/adapters/outbound/spire/trust_domain_parser.go"
  "internal/adapters/outbound/spire/translation.go"
  "internal/adapters/outbound/compose/spire.go"
  "cmd/main_prod.go"
  "cmd/main.go"
)
for f in "${required[@]}"; do
  [[ -s "$f" ]] && success "Found: $f" || { error "Missing: $f"; exit 1; }
done

spire_count=$(find internal/adapters/outbound/spire -type f -name '*.go' ! -name '*_test.go' | wc -l | awk '{print $1}')
[[ "$spire_count" -ge 8 ]] && success "SPIRE adapter file count: $spire_count (>=8)" || { error "SPIRE adapter file count too low: $spire_count"; exit 1; }

# ---------- summary ----------
echo ""
echo "============================================"
echo "VERIFICATION SUMMARY"
echo "============================================"
echo ""
success "All automated checks passed! ✅"
echo ""
echo "Build artifacts:"
echo "  - Prod: $prod_bin ($PROD_SIZE_HR - $PROD_SIZE_BYTES bytes)"
echo "  - Dev : $dev_bin  ($DEV_SIZE_HR - $DEV_SIZE_BYTES bytes)"
echo ""
if [[ -f coverage.out ]]; then
  echo "Test coverage:"
  echo "  - Total: $total_cov"
  echo "  - Report: coverage.out"
  info "View HTML coverage: go tool cover -html=coverage.out"
fi
echo ""
info "To test against live SPIRE infrastructure:"
echo "  1. Start SPIRE: make minikube-up"
echo "  2. Follow docs/VERIFICATION.md for integration tests"
echo ""
