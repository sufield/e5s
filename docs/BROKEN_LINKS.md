# Broken Links Report

**Generated:** 2025-11-13
**Tool:** lychee v0.21.0
**Command:** `lychee '**/*.md' --exclude-loopback --max-concurrency 128 --timeout 20 --format detailed`

## Summary

- 🔍 Total Links: 199
- ✅ Successful: 146
- 🔀 Redirected: 11
- 🚫 **Errors: 27**
- ⛔ Unsupported: 27

## Issues Found

### 1. Broken External URLs (404 Not Found)

#### GitHub Discussion Links
**Status:** Feature not enabled
**Files affected:**
- `.github/SECURITY.md`
- `docs/explanation/faq.md`

**URLs:**
- `https://github.com/sufield/e5s/discussions`

**Fix:** Either:
- Enable GitHub Discussions in repository settings
- Replace with alternative (Issues page, external forum, etc.)
- Remove references

#### Keybase Profile
**File:** `.github/SECURITY.md`
**URL:** `https://keybase.io/sufield`
**Fix:**
- Update URL if profile exists at different location
- Remove if not used

#### SPIRE Documentation
**Files affected:**
- `COMPATIBILITY.md`
- `docs/how-to/debug-mtls.md`
- `docs/how-to/deploy-helm.md`
- `examples/highlevel/TUTORIAL.md`
- `examples/minikube-lowlevel/infra/README.md`
- `hack/README.md`
- `TESTING.md`

**URLs:**
- `https://spiffe.io/docs/latest/spire/`
- `https://spiffe.io/docs/latest/spire/developing/`
- `https://github.com/spiffe/spire/blob/main/support/k8s/k8s-workload-registrar/README.md`

**Fix:** Update to current SPIRE documentation URLs (check spiffe.io)

### 2. Broken Internal File References

#### Missing Documentation Files

| File Reference | Referenced From | Status |
|----------------|-----------------|--------|
| `docs/integration-tests.md` | `CONTRIBUTING.md` | Missing |
| `docs/CONTRIBUTING.md` | `docs/explanation/faq.md` | Wrong path (should be `../CONTRIBUTING.md`) |
| `docs/how-to/faq.md` | `docs/how-to/debug-mtls.md` | Missing |
| `docs/VERSION.md` | `docs/how-to/falco-guide.md` | Missing |
| `docs/reference/low-level-api.md` | `docs/reference/api.md` | Missing (renamed to `api.md`) |
| `VERSIONS.md` | `examples/minikube-lowlevel/infra/README.md` | Missing |
| `examples/highlevel/SPIRE_SETUP.md` | `examples/highlevel/TESTING_PRERELEASE.md` | Missing |

#### Missing Directory References

| Directory Reference | Referenced From | Status |
|---------------------|-----------------|--------|
| `docs/examples/middleware` | `docs/explanation/comparison.md` | Wrong path (should be `../../examples/middleware`) |
| `docs/examples/middleware/main.go` | `docs/explanation/faq.md` | Wrong path |
| `docs/explanation/.github/ISSUE_TEMPLATE/bug_report.yml` | `docs/explanation/faq.md` | Wrong path |
| `docs/examples/debug` | `docs/how-to/debug-mtls.md` | Missing |
| `docs/chart/e5s-demo/README.md` | `docs/how-to/deploy-helm.md` | Wrong path (should be `../../chart/e5s-demo/README.md`) |
| `docs/cmd/e5s/README.md` | `docs/how-to/deploy-helm.md` | Wrong path |
| `docs/examples` | `docs/how-to/deploy-helm.md` | Wrong path |
| `docs/examples/minikube-lowlevel` | `docs/how-to/deploy-helm.md` | Wrong path |
| `docs/pkg/spiffehttp` | `docs/how-to/monitor-with-falco.md` | Wrong path |
| `security` | `README.md` | Wrong path (should be `.github/SECURITY.md`) |

## Recommended Actions

### Priority 1: Fix Critical Broken Links (Do Now)

1. **Update SPIRE documentation URLs**
   - Search and replace all `https://spiffe.io/docs/latest/spire/` references
   - Verify new URLs work

2. **Fix path references from docs/ directory**
   - Many files in `docs/` incorrectly reference other files
   - Should use relative paths: `../` to go up, then navigate to target
   - Examples:
     ```markdown
     ❌ docs/examples/middleware
     ✅ ../../examples/middleware

     ❌ docs/CONTRIBUTING.md
     ✅ ../CONTRIBUTING.md
     ```

3. **Fix or remove GitHub Discussions links**
   - Either enable feature or use alternative

### Priority 2: Fix Missing Files (Do Soon)

1. **Create or remove references to:**
   - `docs/integration-tests.md` (or update reference in CONTRIBUTING.md)
   - `docs/how-to/faq.md` (or remove link)
   - `docs/VERSION.md` (or remove reference)
   - `VERSIONS.md` (or remove reference)

2. **Update renamed file reference:**
   - Replace `docs/reference/low-level-api.md` with `docs/reference/api.md`

### Priority 3: Optimize Documentation Structure (Do Later)

1. **Consider creating:**
   - `docs/examples/` directory with symlinks or docs pointing to actual examples
   - Central FAQ document
   - Version tracking document

2. **Standardize path conventions:**
   - All docs use relative paths
   - Document the docs/ directory structure in docs/README.md

## Quick Fixes

### Fix Path Issues in docs/ Directory

```bash
# Fix CONTRIBUTING.md path references
sed -i 's|docs/CONTRIBUTING\.md|../CONTRIBUTING.md|g' docs/explanation/faq.md

# Fix examples path references
sed -i 's|docs/examples/middleware|../../examples/middleware|g' docs/explanation/comparison.md
sed -i 's|docs/examples/middleware/main\.go|../../examples/middleware/main.go|g' docs/explanation/faq.md

# Fix chart path reference
sed -i 's|docs/chart/e5s-demo/README\.md|../../chart/e5s-demo/README.md|g' docs/how-to/deploy-helm.md

# Fix security file reference in README.md
sed -i 's|\[SECURITY\.md\](security)|[SECURITY.md](.github/SECURITY.md)|g' README.md
```

### Remove or Update SPIRE URLs

Check the current SPIRE documentation structure and update URLs accordingly. The pattern seems to be:
- Old: `https://spiffe.io/docs/latest/spire/`
- Possible new: `https://spiffe.io/docs/latest/spire/install/` or similar

## Testing

After fixes, re-run link checker:

```bash
lychee '**/*.md' --exclude-loopback --max-concurrency 128 --timeout 20 --format detailed
```

Expected result: 0 errors

## Tracking

- [ ] Fix Priority 1 items (critical broken links)
- [ ] Fix Priority 2 items (missing files)
- [ ] Consider Priority 3 items (structure improvements)
- [ ] Re-run link checker and verify 0 errors
- [ ] Add to CI/CD to prevent future broken links
- [ ] Document in docs/maintenance.md

## Notes

- Some GitHub security advisory links redirect to login page (expected behavior)
- Some external redirects are normal and acceptable (e.g., badge images)
- Consider adding `.lychee.toml` config file to exclude known acceptable patterns
