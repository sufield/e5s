#!/bin/bash
set -euo pipefail

# Diátaxis Documentation Migration Script
# This script reorganizes documentation following the Diátaxis framework
# Run this from the repository root: ./scripts/migrate-to-diataxis.sh

echo "🚀 Starting Diátaxis documentation migration..."
echo ""

# Verify we're in the right directory
if [ ! -d "docs" ] || [ ! -d "examples" ]; then
    echo "❌ Error: Run this script from the repository root"
    exit 1
fi

# Create backup
echo "📦 Creating backup..."
BACKUP_DIR="docs-backup-$(date +%Y%m%d-%H%M%S)"
cp -r docs "$BACKUP_DIR"
echo "✅ Backup created at: $BACKUP_DIR"
echo ""

# ============================================================================
# TUTORIALS
# ============================================================================
echo "📚 Moving files to tutorials/..."

# Move from docs/guide/
git mv docs/guide/QUICKSTART.md docs/tutorials/ 2>/dev/null || mv docs/guide/QUICKSTART.md docs/tutorials/
git mv docs/guide/EDITOR_SETUP.md docs/tutorials/ 2>/dev/null || mv docs/guide/EDITOR_SETUP.md docs/tutorials/

# Move examples/ directory
git mv examples docs/tutorials/ 2>/dev/null || mv examples docs/tutorials/

echo "✅ Tutorials moved"
echo ""

# ============================================================================
# HOW-TO GUIDES
# ============================================================================
echo "🔧 Moving files to how-to-guides/..."

# From docs/guide/
git mv docs/guide/PRODUCTION_WORKLOAD_API.md docs/how-to-guides/ 2>/dev/null || mv docs/guide/PRODUCTION_WORKLOAD_API.md docs/how-to-guides/
git mv docs/guide/TROUBLESHOOTING.md docs/how-to-guides/ 2>/dev/null || mv docs/guide/TROUBLESHOOTING.md docs/how-to-guides/

# From docs/infra-notes/
git mv docs/infra-notes/codeql-local-setup.md docs/how-to-guides/ 2>/dev/null || mv docs/infra-notes/codeql-local-setup.md docs/how-to-guides/
git mv docs/infra-notes/security-tools.md docs/how-to-guides/ 2>/dev/null || mv docs/infra-notes/security-tools.md docs/how-to-guides/
git mv docs/infra-notes/SPIRE_DISTROLESS_WORKAROUND.md docs/how-to-guides/ 2>/dev/null || mv docs/infra-notes/SPIRE_DISTROLESS_WORKAROUND.md docs/how-to-guides/
git mv docs/infra-notes/UNIFIED_CONFIG_IMPROVEMENTS.md docs/how-to-guides/ 2>/dev/null || mv docs/infra-notes/UNIFIED_CONFIG_IMPROVEMENTS.md docs/how-to-guides/

echo "✅ How-to guides moved"
echo ""

# ============================================================================
# REFERENCE
# ============================================================================
echo "📖 Moving files to reference/..."

# From docs/architecture/
git mv docs/architecture/PORT_CONTRACTS.md docs/reference/ 2>/dev/null || mv docs/architecture/PORT_CONTRACTS.md docs/reference/
git mv docs/architecture/INVARIANTS.md docs/reference/ 2>/dev/null || mv docs/architecture/INVARIANTS.md docs/reference/
git mv docs/architecture/DOMAIN.md docs/reference/ 2>/dev/null || mv docs/architecture/DOMAIN.md docs/reference/

# From docs/engineering/
git mv docs/engineering/TEST_ARCHITECTURE.md docs/reference/ 2>/dev/null || mv docs/engineering/TEST_ARCHITECTURE.md docs/reference/
git mv docs/engineering/TESTING_GUIDE.md docs/reference/ 2>/dev/null || mv docs/engineering/TESTING_GUIDE.md docs/reference/
git mv docs/engineering/TESTING.md docs/reference/ 2>/dev/null || mv docs/engineering/TESTING.md docs/reference/
git mv docs/engineering/END_TO_END_TESTS.md docs/reference/ 2>/dev/null || mv docs/engineering/END_TO_END_TESTS.md docs/reference/
git mv docs/engineering/INTEGRATION_TEST_SUMMARY.md docs/reference/ 2>/dev/null || mv docs/engineering/INTEGRATION_TEST_SUMMARY.md docs/reference/
git mv docs/engineering/INTEGRATION_TEST_OPTIMIZATION.md docs/reference/ 2>/dev/null || mv docs/engineering/INTEGRATION_TEST_OPTIMIZATION.md docs/reference/
git mv docs/engineering/VERIFICATION.md docs/reference/ 2>/dev/null || mv docs/engineering/VERIFICATION.md docs/reference/
git mv docs/engineering/pbt.md docs/reference/ 2>/dev/null || mv docs/engineering/pbt.md docs/reference/

echo "✅ Reference docs moved"
echo ""

# ============================================================================
# EXPLANATION
# ============================================================================
echo "💡 Moving files to explanation/..."

# From docs/architecture/
git mv docs/architecture/ARCHITECTURE.md docs/explanation/ 2>/dev/null || mv docs/architecture/ARCHITECTURE.md docs/explanation/
git mv docs/architecture/SPIFFE_ID_REFACTORING.md docs/explanation/ 2>/dev/null || mv docs/architecture/SPIFFE_ID_REFACTORING.md docs/explanation/

# From docs/engineering/
git mv docs/engineering/DESIGN_BY_CONTRACT.md docs/explanation/ 2>/dev/null || mv docs/engineering/DESIGN_BY_CONTRACT.md docs/explanation/
git mv docs/engineering/DEBUG_MODE.md docs/explanation/ 2>/dev/null || mv docs/engineering/DEBUG_MODE.md docs/explanation/
git mv docs/engineering/ARCHITECTURE_REVIEW.md docs/explanation/ 2>/dev/null || mv docs/engineering/ARCHITECTURE_REVIEW.md docs/explanation/

# From docs/roadmap/
git mv docs/roadmap/REFACTORING_PATTERNS.md docs/explanation/ 2>/dev/null || mv docs/roadmap/REFACTORING_PATTERNS.md docs/explanation/
git mv docs/roadmap/ITERATIONS_SUMMARY.md docs/explanation/ 2>/dev/null || mv docs/roadmap/ITERATIONS_SUMMARY.md docs/explanation/
git mv docs/roadmap/PROJECT_SETUP_STATUS.md docs/explanation/ 2>/dev/null || mv docs/roadmap/PROJECT_SETUP_STATUS.md docs/explanation/

echo "✅ Explanation docs moved"
echo ""

# ============================================================================
# CLEANUP OLD DIRECTORIES
# ============================================================================
echo "🧹 Cleaning up old directories..."

# Remove empty directories
rmdir docs/guide 2>/dev/null || echo "  docs/guide not empty or already removed"
rmdir docs/infra-notes 2>/dev/null || echo "  docs/infra-notes not empty or already removed"
rmdir docs/architecture 2>/dev/null || echo "  docs/architecture not empty or already removed"
rmdir docs/engineering 2>/dev/null || echo "  docs/engineering not empty or already removed"
rmdir docs/roadmap 2>/dev/null || echo "  docs/roadmap not empty or already removed"

echo "✅ Cleanup complete"
echo ""

# ============================================================================
# VERIFICATION
# ============================================================================
echo "🔍 Verifying migration..."
echo ""

echo "Tutorials:"
ls -1 docs/tutorials/*.md 2>/dev/null | wc -l | xargs echo "  - Markdown files:"
echo "  - Examples directory: $([ -d docs/tutorials/examples ] && echo '✅' || echo '❌')"

echo ""
echo "How-to guides:"
ls -1 docs/how-to-guides/*.md 2>/dev/null | wc -l | xargs echo "  - Files:"

echo ""
echo "Reference:"
ls -1 docs/reference/*.md 2>/dev/null | wc -l | xargs echo "  - Files:"

echo ""
echo "Explanation:"
ls -1 docs/explanation/*.md 2>/dev/null | wc -l | xargs echo "  - Files:"

echo ""
echo "✅ Migration complete!"
echo ""
echo "📋 Next steps:"
echo "  1. Review the changes: git status"
echo "  2. Update cross-references in documentation files"
echo "  3. Test that all links work"
echo "  4. Commit the changes: git add docs/ && git commit -m 'Restructure docs with Diátaxis framework'"
echo ""
echo "If you need to rollback, restore from: $BACKUP_DIR"
