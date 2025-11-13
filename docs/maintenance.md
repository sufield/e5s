# Maintenance Guide

This guide covers routine maintenance tasks for keeping the e5s project healthy and up-to-date.

## Table of Contents

- [Checking for Broken Links](#checking-for-broken-links)
- [Dependency Updates](#dependency-updates)
- [Security Scanning](#security-scanning)
- [Documentation Updates](#documentation-updates)

---

## Checking for Broken Links

Broken links in documentation hurt the user experience and make the project look unmaintained. We use **lychee** to automatically scan all Markdown files for broken links.

> **Current Status:** As of 2025-11-13, we have 27 broken links that need fixing. See [BROKEN_LINKS.md](BROKEN_LINKS.md) for the complete list and action plan.

### Why lychee?

- **Fast**: Written in Rust, parallelized, extremely fast
- **Accurate**: Correctly resolves relative links within the repository
- **CI-friendly**: Excellent exit codes and output formats
- **Comprehensive**: Checks both external URLs and internal file references
- **Battle-tested**: Used by major projects (Kubernetes, Rust, many Go repos)

### Installation

Choose one method:

```bash
# macOS / Linux (Homebrew)
brew install lychee

# Cargo (Rust package manager)
cargo install lychee

# Docker (no installation needed)
docker pull lycheeverse/lychee

# Pre-built binaries
# Download from https://github.com/lycheeverse/lychee/releases
```

### Quick Check

From the repository root:

```bash
# Basic check
lychee '**/*.md'

# Verbose output with progress
lychee '**/*.md' --verbose --no-progress

# Recommended flags for this project
lychee '**/*.md' \
  --exclude-loopback \
  --max-concurrency 128 \
  --timeout 20 \
  --format detailed
```

### Common Issues and Fixes

#### Broken Internal Links

**Example error:**
```
✗ [ERR] docs/how-to/setup.md:42:1 | Failed: File not found
  → docs/reference/low-level-api.md
```

**Fix:**
1. Check if the file exists at that path
2. If renamed, update all references:
   ```bash
   # Find all files referencing the old path
   grep -r "low-level-api.md" --include="*.md" .

   # Replace with correct path
   sed -i 's|low-level-api\.md|api.md|g' **/*.md
   ```

#### Broken External URLs

**Example error:**
```
✗ [ERR] README.md:123:45 | Failed: 404 Not Found
  → https://example.com/old-page
```

**Fix:**
1. Find the current URL (use web.archive.org if page moved)
2. Update the link in the Markdown file
3. If permanently broken, remove or replace with working alternative

#### False Positives

Some links may be flagged incorrectly:

```bash
# Exclude specific domains
lychee '**/*.md' --exclude 'https://internal.example.com'

# Exclude patterns
lychee '**/*.md' --exclude-path 'test/*' --exclude-path 'tmp/*'

# Skip localhost links (for examples)
lychee '**/*.md' --exclude-loopback
```

### Makefile Integration

Add to your `Makefile`:

```makefile
## check-links: Check for broken links in documentation
check-links:
	@echo "Checking for broken links in Markdown files..."
	@lychee '**/*.md' \
		--exclude-loopback \
		--max-concurrency 128 \
		--timeout 20 \
		--format detailed || (echo "❌ Broken links found" && exit 1)
	@echo "✓ All links are valid"
```

Usage:
```bash
make check-links
```

### Automatic Fixing

Many common link issues can be automatically fixed:

```bash
# Dry run - see what would be fixed
./hack/fix-broken-links.sh

# Apply fixes
./hack/fix-broken-links.sh --apply

# Verify fixes
make check-links
```

The script fixes:
- Incorrect relative paths from `docs/` directory
- References to renamed files
- Common path mistakes
- Known moved files

**After auto-fixing:**
1. Review changes: `git diff`
2. Test: `make check-links`
3. Commit: `git add . && git commit -m 'fix: correct broken link paths'`

### CI Integration

We use a **smart CI strategy** that balances preventing broken links with not blocking development:

#### Strategy Overview

| Event | Check Type | Behavior | Purpose |
|-------|------------|----------|---------|
| **Pull Request** | Internal links only | ❌ Fail if internal links broken | Prevent introducing broken internal links |
| **Pull Request** | External links | ⚠️ Warn but don't fail | External sites can break anytime |
| **Push to main** | All links | 📝 Create/update issue | Track issues without blocking |
| **Weekly schedule** | All links | 📝 Create/update issue | Catch external link rot |

**Why this strategy?**
- ✅ Prevents accidental internal link breakage
- ✅ Doesn't block PRs when external sites are down
- ✅ Catches external link rot through scheduled checks
- ✅ Automatically closes issues when fixed

#### GitHub Actions

File: `.github/workflows/links.yml` (already created)

The workflow (`.github/workflows/links.yml`) automatically:
- ✅ Runs on every PR touching Markdown files
- ✅ Runs on every push to main
- ✅ Runs weekly on Mondays at 9 AM UTC
- ✅ Can be triggered manually

**Features:**
- 📊 **Generates detailed report** with categorized errors
- 💬 **Comments on PRs** with findings (non-blocking for external links)
- 📋 **Creates/updates GitHub issue** on main/schedule with full report
- ✅ **Auto-closes issue** when all links are fixed
- 📦 **Uploads artifacts** (JSON report + markdown summary)
- 🎯 **Smart failure logic**: Only fails PRs for internal link errors

**View the workflow:** `.github/workflows/links.yml`

#### GitLab CI

Add to `.gitlab-ci.yml`:

```yaml
check-links:
  image: lycheeverse/lychee:latest
  stage: test
  script:
    - lychee --verbose --no-progress '**/*.md'
  only:
    - merge_requests
    - main
```

### Configuration File

Create `.lychee.toml` in repository root for project-specific settings:

```toml
# Cache results for faster repeated runs
cache = true

# Maximum number of concurrent requests
max_concurrency = 128

# Request timeout in seconds
timeout = 20

# Retry failed requests
max_retries = 3

# Exclude patterns
exclude = [
    # Skip localhost links in examples
    "^https?://localhost",
    "^https?://127.0.0.1",
    "^https?://0.0.0.0",

    # Skip private/internal URLs
    "^https?://internal\\.example\\.com",
]

# Check mailto links (email addresses)
include_mail = false

# Exclude paths
exclude_path = [
    "test/",
    "tmp/",
    "vendor/",
    "node_modules/",
]

# Accept these status codes as valid
accept = [200, 201, 204, 301, 302, 307, 308, 429]

# User agent string
user_agent = "Mozilla/5.0 (compatible; lychee/link-checker)"
```

### CI Output Example

When the workflow runs, you'll see:

**On PR:**
```
🔗 Link Check Report

Summary:
- 🔍 Total: 200
- ✅ Successful: 147
- 🚫 Failures: 27

Internal Link Errors
- `docs/examples/middleware` in docs/explanation/comparison.md

External Link Errors (404)
- https://spiffe.io/docs/latest/spire/ - ❌ Not Found
  - Referenced in: TESTING.md, docs/how-to/debug-mtls.md

ℹ️ External errors are non-blocking. Only internal link errors will fail the build.
```

**Automatic Issue Creation:**
When broken links are found on main or schedule, an issue is automatically created with:
- Full categorized report
- Quick fix suggestions
- Links to workflow run
- Auto-closes when fixed

### Best Practices

1. **Run locally before committing**:
   ```bash
   make check-links
   ```

2. **Use auto-fix for common issues**:
   ```bash
   ./hack/fix-broken-links.sh --apply
   ```

3. **Fix internal links immediately**: These block PRs

4. **Review external link warnings**: Update URLs when possible

5. **Use relative links for internal docs**:
   ```markdown
   ✓ Good: [API Reference](../reference/api.md)
   ✗ Bad:  [API Reference](https://github.com/user/repo/blob/main/docs/reference/api.md)
   ```

6. **Archive important external links**:
   - Use [web.archive.org](https://web.archive.org) for critical references
   - Add archived link as alternative: `[Link](https://example.com) ([archived](https://web.archive.org/...))`

7. **Understand CI behavior**:
   - PR: Internal links must pass, external warnings only
   - Main/Schedule: Creates issue, doesn't block
   - Manual: Can be run anytime via GitHub Actions UI

### Troubleshooting

#### Rate Limiting

If hitting rate limits on external APIs:

```bash
# Reduce concurrency
lychee '**/*.md' --max-concurrency 10

# Increase timeout
lychee '**/*.md' --timeout 60

# Add delays between requests
lychee '**/*.md' --max-concurrency 1 --timeout 30
```

#### GitHub API Rate Limits

GitHub URLs may hit rate limits:

```bash
# Use GitHub token for higher limits
export GITHUB_TOKEN="ghp_your_token_here"
lychee '**/*.md'
```

In CI, use:
```yaml
env:
  GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}
```

#### Certificate Errors

For internal servers with self-signed certs:

```bash
lychee '**/*.md' --insecure
```

---

## Dependency Updates

### Go Module Updates

Check for outdated dependencies:

```bash
# List available updates
go list -u -m all

# Update specific module
go get -u github.com/spiffe/go-spiffe/v2

# Update all dependencies (minor/patch only)
go get -u ./...

# Update all dependencies (including major versions)
go get -u=patch ./...
```

After updating:
```bash
go mod tidy
go mod verify
go test -race ./...
make lint
```

### Security Updates

Check for vulnerabilities:

```bash
# Scan for known vulnerabilities
govulncheck ./...

# Check SARIF output for CI
govulncheck -format=sarif -scan package ./... > govulncheck.sarif
```

### Dependency Review Checklist

Before accepting updates:

- [ ] Read CHANGELOG/release notes
- [ ] Check for breaking changes
- [ ] Run full test suite: `go test -race -tags=integration,container ./...`
- [ ] Build all examples: `make build-all`
- [ ] Review security implications
- [ ] Update documentation if API changed
- [ ] Test in local development environment

---

## Security Scanning

### Regular Scans

Run comprehensive security checks:

```bash
# All security checks
make sec-all

# Individual checks
make sec-deps      # Dependency vulnerabilities
make sec-lint      # Static analysis (gosec)
make sec-secrets   # Secret scanning (gitleaks)
make sec-test      # Tests with race detector
```

### Dependency Auditing

```bash
# Check for known vulnerabilities
govulncheck ./...

# Check for outdated vulnerable dependencies
go list -u -m all | grep -E '\[.*\]'
```

### Code Security

```bash
# Static analysis for security issues
gosec ./...

# With SARIF output for GitHub
gosec -fmt=sarif -out=gosec.sarif ./...
```

### Secret Scanning

```bash
# Scan for committed secrets
gitleaks detect --source . --verbose

# Scan with report
gitleaks detect --source . --report-format sarif --report-path gitleaks.sarif
```

---

## Documentation Updates

### After API Changes

1. **Update godoc comments**:
   ```go
   // PeerID extracts the SPIFFE ID from an HTTP request.
   //
   // Returns the peer's SPIFFE ID and true if found, otherwise empty string and false.
   // The peer ID comes from the client certificate presented during TLS handshake.
   func PeerID(r *http.Request) (string, bool)
   ```

2. **Update example code**:
   ```bash
   # Edit example
   vim examples/basic-server/main.go

   # Verify it compiles
   go build ./examples/basic-server
   ```

3. **Update reference docs**:
   ```bash
   vim docs/reference/api.md
   ```

4. **Check for broken links**:
   ```bash
   make check-links
   ```

### Documentation Review Checklist

- [ ] API changes documented in godoc comments
- [ ] Examples updated and compile successfully
- [ ] README.md updated if user-facing change
- [ ] CHANGELOG.md entry added
- [ ] Migration guide if breaking change
- [ ] All links validated with lychee
- [ ] Spelling and grammar checked

### Spelling Check

Optional but helpful:

```bash
# Install aspell
sudo apt install aspell aspell-en

# Check spelling
aspell check docs/README.md

# Or use codespell (Python)
pip install codespell
codespell docs/
```

---

## Release Maintenance

### Pre-Release Checklist

See `make release-check` for automated checks, plus:

- [ ] All CI checks passing
- [ ] No broken links: `make check-links`
- [ ] All examples build: `make build-all`
- [ ] Integration tests pass: `go test -tags=integration ./...`
- [ ] Container tests pass: `go test -tags=container ./...`
- [ ] CHANGELOG.md updated
- [ ] Version bumped in relevant files
- [ ] Security scan clean: `make sec-all`

### Post-Release

- [ ] Verify release artifacts on GitHub
- [ ] Test installation: `go install github.com/sufield/e5s/cmd/e5s@latest`
- [ ] Verify Docker images published
- [ ] Announce release (if appropriate)
- [ ] Close milestone (if using)

---

## Monitoring and Metrics

### GitHub Insights

Regularly review:
- Open issues and PRs
- Contributor activity
- Traffic and clones
- Dependency updates (Dependabot)

### External Metrics

- **pkg.go.dev**: Check documentation renders correctly
- **Go Report Card**: Maintain A+ rating
- **OpenSSF Scorecard**: Track security posture
- **SLSA**: Track supply chain security level

---

## Routine Maintenance Schedule

### Weekly
- [ ] Review and triage new issues
- [ ] Review open pull requests
- [ ] Check for dependency updates (Dependabot)

### Monthly
- [ ] Run full security scan: `make sec-all`
- [ ] Check for broken links: `make check-links`
- [ ] Review and update documentation
- [ ] Update CHANGELOG.md

### Quarterly
- [ ] Major dependency updates
- [ ] Review and update examples
- [ ] Security audit
- [ ] Performance benchmarking

### Annually
- [ ] Review and update roadmap
- [ ] Major version planning
- [ ] Documentation overhaul review
- [ ] Tooling updates (Go version, linters, etc.)

---

## References

- [lychee documentation](https://github.com/lycheeverse/lychee)
- [govulncheck documentation](https://golang.org/x/vuln/cmd/govulncheck)
- [gosec documentation](https://github.com/securego/gosec)
- [gitleaks documentation](https://github.com/zricethezav/gitleaks)
- [Go modules reference](https://go.dev/ref/mod)
