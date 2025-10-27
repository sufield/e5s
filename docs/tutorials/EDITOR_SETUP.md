---
type: tutorial
audience: beginner
---

# Editor Setup - gopls Configuration

## Overview

This project uses minimal Go build tags for development infrastructure tooling. Most code has no build tags and works in all contexts.

## Understanding Build Tags

> **Note**: As of the selector/inmemory cleanup, build tags are minimal. Only infrastructure tooling (Helm installer) uses `//go:build dev` tags.

- **`//go:build dev`** - Development infrastructure tools (Helm installer for local SPIRE setup)
- **No tag** - Available in all builds (production application code)

## Configuration Files

> **Note**: With minimal dev tags, profile switching is largely unnecessary. The default configuration works for most development.

### Default Configuration

**Active files:**
- `gopls.yaml` - Editor-agnostic gopls config
- `.vscode/settings.json` - VSCode-specific config

**Behavior:**
- ✅ Analyzes all code including Helm installer (`//go:build dev`)
- ✅ Works for: All development work

### Advanced: Profile Switching (Rarely Needed)

If you want to exclude the Helm installer from analysis:

### Method 1: File Rename (Quick Switch)

```bash
# Switch to PROD profile
mv gopls.yaml gopls.dev.yaml
mv gopls.prod.yaml gopls.yaml

# Switch back to DEV profile
mv gopls.yaml gopls.prod.yaml
mv gopls.dev.yaml gopls.yaml
```

### Method 2: VSCode Workspace Settings (Recommended)

Create `.vscode/spire-prod.code-workspace`:

```json
{
  "folders": [
    {
      "path": "."
    }
  ],
  "settings": {
    "go.buildTags": "",
    "gopls": {
      "build.buildFlags": [],
      "build.directoryFilters": ["-vendor", "-node_modules", "-.git"],
      "completion.usePlaceholders": true,
      "ui.semanticTokens": true
    }
  }
}
```

Then: **File > Open Workspace from File** → Select `spire-prod.code-workspace`

### Method 3: Editor Profiles (Per-Project)

Most modern editors support multiple configuration profiles. Create separate profiles for dev and prod work.

## Configuration Details

### Current Dev Config (`gopls.yaml`)

```yaml
build:
  buildFlags: ["-tags=dev"]
  directoryFilters: ["-vendor", "-node_modules", "-.git"]

completion:
  usePlaceholders: true

ui:
  semanticTokens: true
```

**Fixes from review:**
- ✅ Removed duplicate `GOFLAGS` env var (was conflicting with `buildFlags`)
- ✅ Moved `usePlaceholders` to correct location (`completion.*` not `ui.completion.*`)
- ✅ Added directory filters to speed up analysis

### Production Config Template (`gopls.prod.yaml`)

```yaml
build:
  # No buildFlags - analyzes production code only
  directoryFilters: ["-vendor", "-node_modules", "-.git"]

completion:
  usePlaceholders: true

ui:
  semanticTokens: true
```

## Common Issues

### Issue: "Cannot find definition" for dev files

**Cause:** Using prod config while working on dev files
**Fix:** Switch to dev configuration

### Issue: "Cannot find definition" for production adapter code

**Cause:** Using dev config while working on production files
**Fix:** Switch to prod configuration

### Issue: Slow gopls analysis

**Cause:** Not filtering out large directories
**Fix:** Verify `build.directoryFilters` includes:
- `-vendor`
- `-node_modules`
- `-.git`

### Issue: Duplicate build tag errors

**Cause:** Setting tags in both `buildFlags` AND `env.GOFLAGS`
**Fix:** Use only `build.buildFlags` (already fixed in current config)

## Editor-Specific Setup

### VSCode

1. Install Go extension
2. Use `.vscode/settings.json` for dev work (default)
3. Create workspace file for prod work (see Method 2 above)

### Neovim / vim-go

```vim
" Dev profile (default)
let g:go_build_tags = 'dev'

" Prod profile
let g:go_build_tags = ''
```

### Emacs / go-mode

```elisp
;; Dev profile
(setq go-build-tags "dev")

;; Prod profile
(setq go-build-tags "")
```

### GoLand / IntelliJ IDEA

1. **Settings** → **Go** → **Build Tags & Vendoring**
2. **Custom tags:** Enter `dev` for dev profile, leave empty for prod
3. Can create separate run configurations for each profile

## Testing Your Configuration

### Verify Dev Profile

Open `internal/adapters/outbound/inmemory/server.go`:
- ✅ Should have full syntax highlighting and navigation
- ✅ Can "Go to Definition" on types and methods

### Verify Prod Profile

Open `internal/adapters/outbound/spire/x509source.go`:
- ✅ Should have full syntax highlighting and navigation
- ✅ Can "Go to Definition" on SPIRE SDK types

## Related Documentation

- [Build Tags Architecture](BUILD_TAGS.md) - Understanding the build tag strategy
- [Test Architecture](TEST_ARCHITECTURE.md) - Testing with build tags
- [Development Guide](DEVELOPMENT.md) - Setting up the development environment

## References

- [gopls Settings Documentation](https://github.com/golang/tools/blob/master/gopls/doc/settings.md)
- [Go Build Constraints](https://pkg.go.dev/cmd/go#hdr-Build_constraints)
