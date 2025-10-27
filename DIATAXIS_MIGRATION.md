# Diátaxis Documentation Migration

## ✅ What's Been Done

### 1. Created New Documentation Structure

The new Diátaxis framework structure has been set up with four categories:

```
docs/
├── README.md                   ← Comprehensive navigation index (✅ CREATED)
├── tutorials/                  ← Learning-oriented docs (✅ CREATED)
├── how-to-guides/              ← Task-oriented docs (✅ CREATED)
├── reference/                  ← Information-oriented docs (✅ CREATED)
└── explanation/                ← Understanding-oriented docs (✅ CREATED)
```

### 2. Created Comprehensive Index

**File**: `docs/README.md`

Features:
- ✅ Clear explanation of each Diátaxis category
- ✅ "When to use" guidance for each section
- ✅ Quick navigation based on user goals
- ✅ Visual Diátaxis framework table
- ✅ Contributing guidelines
- ✅ Links to all documentation

### 3. Created Migration Script

**File**: `scripts/migrate-to-diataxis.sh`

Features:
- ✅ Automated file movement with git history preservation
- ✅ Backup creation before migration
- ✅ Cleanup of empty old directories
- ✅ Verification of successful migration
- ✅ Clear next steps

---

## 📋 Next Steps (Execute When Ready)

### Step 1: Review the Migration Plan

The migration script will move files as follows:

**TUTORIALS** (learning-oriented):
- `docs/guide/QUICKSTART.md` → `docs/tutorials/`
- `docs/guide/EDITOR_SETUP.md` → `docs/tutorials/`
- `examples/` → `docs/tutorials/examples/`

**HOW-TO GUIDES** (task-oriented):
- `docs/guide/PRODUCTION_WORKLOAD_API.md` → `docs/how-to-guides/`
- `docs/guide/TROUBLESHOOTING.md` → `docs/how-to-guides/`
- `docs/infra-notes/codeql-local-setup.md` → `docs/how-to-guides/`
- `docs/infra-notes/security-tools.md` → `docs/how-to-guides/`
- `docs/infra-notes/SPIRE_DISTROLESS_WORKAROUND.md` → `docs/how-to-guides/`
- `docs/infra-notes/UNIFIED_CONFIG_IMPROVEMENTS.md` → `docs/how-to-guides/`

**REFERENCE** (information-oriented):
- `docs/architecture/PORT_CONTRACTS.md` → `docs/reference/`
- `docs/architecture/INVARIANTS.md` → `docs/reference/`
- `docs/architecture/DOMAIN.md` → `docs/reference/`
- `docs/engineering/TEST_ARCHITECTURE.md` → `docs/reference/`
- `docs/engineering/TESTING_GUIDE.md` → `docs/reference/`
- `docs/engineering/TESTING.md` → `docs/reference/`
- `docs/engineering/END_TO_END_TESTS.md` → `docs/reference/`
- `docs/engineering/INTEGRATION_TEST_SUMMARY.md` → `docs/reference/`
- `docs/engineering/INTEGRATION_TEST_OPTIMIZATION.md` → `docs/reference/`
- `docs/engineering/VERIFICATION.md` → `docs/reference/`
- `docs/engineering/pbt.md` → `docs/reference/`

**EXPLANATION** (understanding-oriented):
- `docs/architecture/ARCHITECTURE.md` → `docs/explanation/`
- `docs/architecture/SPIFFE_ID_REFACTORING.md` → `docs/explanation/`
- `docs/engineering/DESIGN_BY_CONTRACT.md` → `docs/explanation/`
- `docs/engineering/DEBUG_MODE.md` → `docs/explanation/`
- `docs/engineering/ARCHITECTURE_REVIEW.md` → `docs/explanation/`
- `docs/roadmap/REFACTORING_PATTERNS.md` → `docs/explanation/`
- `docs/roadmap/ITERATIONS_SUMMARY.md` → `docs/explanation/`
- `docs/roadmap/PROJECT_SETUP_STATUS.md` → `docs/explanation/`

### Step 2: Execute the Migration

```bash
# Run from repository root
./scripts/migrate-to-diataxis.sh
```

This will:
1. Create a timestamped backup of the `docs/` directory
2. Move all files to their new locations (preserving git history if possible)
3. Clean up empty old directories
4. Verify the migration completed successfully

### Step 3: Update Cross-References

After migration, you'll need to update links in documentation files:

**Find all broken links:**
```bash
# Search for old path references
grep -r "docs/guide/" docs/
grep -r "docs/architecture/" docs/
grep -r "docs/engineering/" docs/
grep -r "docs/roadmap/" docs/
grep -r "docs/infra-notes/" docs/
grep -r "\.\./examples/" docs/
```

**Common replacements:**
```
docs/guide/           → docs/tutorials/  or  docs/how-to-guides/
docs/architecture/    → docs/reference/  or  docs/explanation/
docs/engineering/     → docs/reference/  or  docs/explanation/
docs/roadmap/         → docs/explanation/
docs/infra-notes/     → docs/how-to-guides/
../examples/          → tutorials/examples/  (from within docs/)
examples/             → docs/tutorials/examples/  (from root)
```

### Step 4: Add Document Type Headers

Add metadata headers to each documentation file:

```markdown
---
type: tutorial | how-to | reference | explanation
audience: beginner | intermediate | advanced
---

# Document Title
...
```

**Example for a tutorial:**
```markdown
---
type: tutorial
audience: beginner
---

# Quick Start Guide
...
```

### Step 5: Update Main README.md

Update the main `README.md` to link to the new Diátaxis structure:

```markdown
## 📚 Documentation

This project uses the [Diátaxis framework](https://diataxis.fr/) for clear, user-focused documentation.

**Start here**: [Documentation Index](docs/README.md)

### Quick Links

- 🎓 **[Tutorials](docs/tutorials/)** - Learn by doing
- 🔧 **[How-To Guides](docs/how-to-guides/)** - Solve specific problems
- 📖 **[Reference](docs/reference/)** - Technical specifications
- 💡 **[Explanation](docs/explanation/)** - Understand the design
```

### Step 6: Verify and Test

```bash
# Check that all links work (you can use a link checker tool)
# Example with markdown-link-check (if installed):
find docs -name "*.md" -exec markdown-link-check {} \;

# Verify directory structure
tree docs/

# Check git status
git status

# Review changes
git diff --stat
```

### Step 7: Commit the Changes

```bash
# Stage all changes
git add docs/ examples/ scripts/

# Commit with descriptive message
git commit -m "docs: Restructure with Diátaxis framework

- Create tutorials/, how-to-guides/, reference/, explanation/ categories
- Move files to appropriate Diátaxis categories
- Create comprehensive docs/README.md navigation index
- Move examples/ to docs/tutorials/examples/
- Update documentation structure for better discoverability

Follows Diátaxis framework: https://diataxis.fr/"

# Push changes
git push
```

---

## 🎯 Benefits of This Structure

### Before (Category-Based)
- `docs/guide/` - Mixed tutorials and how-to guides
- `docs/architecture/` - Mixed reference and explanation
- `docs/engineering/` - Mixed everything
- Hard to find what you need based on your goal

### After (Diátaxis)
- **Tutorials**: "I want to learn" → Clear learning path
- **How-to guides**: "I need to solve X" → Task-focused solutions
- **Reference**: "What does Y do?" → Precise specifications
- **Explanation**: "Why Z?" → Design rationale

**Result**: Users can find information based on their **current need**, not document categories.

---

## 🔄 Rollback (If Needed)

If something goes wrong during migration:

```bash
# The script creates a backup: docs-backup-YYYYMMDD-HHMMSS
# Restore it:
rm -rf docs/
mv docs-backup-YYYYMMDD-HHMMSS docs/

# Also restore examples/ if needed
git checkout examples/
```

---

## 📊 Migration Checklist

- [x] Create Diátaxis directory structure
- [x] Create comprehensive `docs/README.md` index
- [x] Create migration script
- [ ] Execute migration script
- [ ] Update cross-references in documentation
- [ ] Add document type headers
- [ ] Update main `README.md`
- [ ] Verify all links work
- [ ] Test navigation flow
- [ ] Commit changes

---

## 🤔 Decision Matrix: Where Does a New Doc Go?

**Is it teaching someone to use the system for the first time?**
→ `tutorials/`

**Is it solving a specific task or problem?**
→ `how-to-guides/`

**Is it documenting an API, contract, or specification?**
→ `reference/`

**Is it explaining why we made a design decision?**
→ `explanation/`

---

## 📞 Questions?

- Review the [Diátaxis documentation](https://diataxis.fr/)
- Check the comprehensive `docs/README.md` for navigation guidance
- The framework is flexible - use your best judgment
- Consistency is more important than perfection

---

## ✅ Ready to Migrate?

Run: `./scripts/migrate-to-diataxis.sh`

Then follow steps 3-7 above to complete the migration.
