#!/bin/bash
# Auto-fix common broken link patterns
#
# This script automatically fixes common link breakage patterns:
# - Incorrect relative paths from docs/ directory
# - Common path mistakes
# - Known renamed files
#
# Usage:
#   ./hack/fix-broken-links.sh         # Dry run (show what would change)
#   ./hack/fix-broken-links.sh --apply # Apply fixes

set -euo pipefail

DRY_RUN=true
if [[ "${1:-}" == "--apply" ]]; then
    DRY_RUN=false
fi

cd "$(dirname "$0")/.."

echo "🔍 Scanning for fixable broken links..."
echo ""

FIXES=0

# Function to fix a pattern
fix_pattern() {
    local pattern="$1"
    local replacement="$2"
    local description="$3"

    local files=$(grep -rl "$pattern" --include="*.md" . 2>/dev/null || true)

    if [ -n "$files" ]; then
        echo "📝 $description"
        echo "   Pattern: $pattern"
        echo "   Fix: $replacement"
        echo "   Files affected:"

        for file in $files; do
            echo "     - $file"
            if [ "$DRY_RUN" = false ]; then
                sed -i "s|$pattern|$replacement|g" "$file"
            fi
            ((FIXES++))
        done
        echo ""
    fi
}

# Fix common path issues from docs/ directory
fix_pattern \
    "docs/CONTRIBUTING\\.md" \
    "../CONTRIBUTING.md" \
    "Fix CONTRIBUTING.md path from docs/"

fix_pattern \
    "docs/examples/middleware" \
    "../../examples/middleware" \
    "Fix examples/middleware path from docs/"

fix_pattern \
    "docs/examples/middleware/main\\.go" \
    "../../examples/middleware/main.go" \
    "Fix examples/middleware/main.go path from docs/"

fix_pattern \
    "docs/chart/e5s-demo/README\\.md" \
    "../../chart/e5s-demo/README.md" \
    "Fix chart README path from docs/"

fix_pattern \
    "docs/cmd/e5s/README\\.md" \
    "../../cmd/e5s/README.md" \
    "Fix cmd/e5s README path from docs/"

fix_pattern \
    "docs/examples/minikube-lowlevel" \
    "../../examples/minikube-lowlevel" \
    "Fix examples/minikube-lowlevel path from docs/"

fix_pattern \
    "docs/pkg/spiffehttp" \
    "../../spiffehttp" \
    "Fix spiffehttp package path from docs/"

# Fix renamed file references
fix_pattern \
    "docs/reference/low-level-api\\.md" \
    "docs/reference/api.md" \
    "Fix renamed low-level-api.md reference"

fix_pattern \
    "reference/low-level-api\\.md" \
    "reference/api.md" \
    "Fix renamed low-level-api.md reference (relative)"

# Fix security file reference in README
fix_pattern \
    "\\[SECURITY\\.md\\](security)" \
    "[SECURITY.md](.github/SECURITY.md)" \
    "Fix SECURITY.md path in README"

# Fix .github path issues from docs/
fix_pattern \
    "docs/explanation/\\.github/ISSUE_TEMPLATE" \
    "../../.github/ISSUE_TEMPLATE" \
    "Fix .github/ISSUE_TEMPLATE path from docs/"

# Fix examples path from docs/
fix_pattern \
    "docs/examples\"" \
    "../../examples\"" \
    "Fix examples directory path from docs/"

# Fix debug examples path
fix_pattern \
    "docs/examples/debug" \
    "../../examples/debug" \
    "Fix examples/debug path from docs/"

# Fix how-to/faq.md path
fix_pattern \
    "docs/how-to/faq\\.md" \
    "../reference/troubleshooting.md" \
    "Fix FAQ reference (use troubleshooting.md instead)"

echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
if [ "$DRY_RUN" = true ]; then
    if [ "$FIXES" -gt 0 ]; then
        echo "🔍 DRY RUN: Found $FIXES fixable issues"
        echo ""
        echo "To apply these fixes, run:"
        echo "  $0 --apply"
    else
        echo "✅ No fixable issues found"
    fi
else
    if [ "$FIXES" -gt 0 ]; then
        echo "✅ Applied $FIXES fixes"
        echo ""
        echo "Next steps:"
        echo "  1. Review changes: git diff"
        echo "  2. Test: make check-links"
        echo "  3. Commit if satisfied: git add . && git commit -m 'fix: correct broken link paths'"
    else
        echo "✅ No fixable issues found"
    fi
fi
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
