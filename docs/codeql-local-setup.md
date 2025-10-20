# CodeQL Local Setup and Analysis Guide

This guide provides step-by-step instructions for installing and running CodeQL CLI locally on Ubuntu 24.04 LTS for analyzing the SPIRE mTLS codebase.

## Why Local CodeQL?

Running CodeQL locally provides several advantages:
- **Faster feedback**: Analyze code during development without waiting for CI
- **Privacy**: Keep analysis results local during sensitive development
- **Customization**: Run custom queries and configure analysis depth
- **Offline capability**: Analyze code without internet connectivity (after initial setup)

## Prerequisites

- **Operating System**: Ubuntu 24.04 LTS (64-bit x86-64)
- **Administrative access**: `sudo` privileges for installation
- **Disk space**: ~2-3 GB for CodeQL CLI bundle and Go database
- **RAM**: Minimum 8 GB recommended for analyzing medium-sized codebases
- **Basic tools**: Ensure `wget`, `tar`, and `unzip` are installed:
  ```bash
  sudo apt update && sudo apt install wget tar unzip
  ```

## Installation

### Step 1: Download the CodeQL CLI Bundle

Download the latest CodeQL CLI bundle which includes the CLI, queries, and libraries:

```bash
# Create installation directory
sudo mkdir -p /opt/codeql

# Download the latest bundle
cd /tmp
wget https://github.com/github/codeql-action/releases/latest/download/codeql-bundle-linux64.tar.gz

# For a specific version (recommended for reproducibility):
# wget https://github.com/github/codeql-action/releases/download/codeql-bundle-v2.20.1/codeql-bundle-linux64.tar.gz
```

**Note**: By downloading, you agree to the [GitHub CodeQL Terms and Conditions](https://securitylab.github.com/tools/codeql/license).

### Step 2: Extract the Bundle

Extract the downloaded bundle to the installation directory:

```bash
# Extract to /opt/codeql
sudo tar -xzf codeql-bundle-linux64.tar.gz -C /opt/codeql

# Verify extraction
ls -la /opt/codeql/codeql/
# Expected: You should see 'codeql' executable and supporting directories
```

### Step 3: Add CodeQL to Your PATH

Make CodeQL available system-wide:

```bash
# Edit your shell profile (for bash users)
nano ~/.bashrc

# Add this line at the end:
export PATH="/opt/codeql/codeql:$PATH"

# Save and reload profile
source ~/.bashrc
```

**For system-wide access** (optional, requires sudo):
```bash
# Edit system environment
sudo nano /etc/environment

# Add /opt/codeql/codeql to the PATH line, e.g.:
# PATH="/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin:/opt/codeql/codeql"

# Reboot or source the file
```

### Step 4: Verify Installation

Confirm CodeQL is installed correctly:

```bash
# Check version
codeql --version
# Expected output: CodeQL command-line toolchain release 2.xx.x

# Verify query packs are available
codeql resolve packs
# Expected: Lists available query packs (go, javascript, etc.)

# List supported languages
codeql resolve languages
# Expected: Shows languages including "go"
```

If verification fails, ensure you downloaded the **full bundle** (not just the standalone CLI).

## Analyzing the SPIRE Codebase

### Step 1: Create a CodeQL Database

Navigate to the SPIRE project root and create a database:

```bash
cd /home/zepho/work/pocket/hexagon/spire

# Create CodeQL database for Go
codeql database create codeql-db \
  --language=go \
  --source-root=. \
  --overwrite
```

**What happens during database creation:**
- CodeQL extracts the source code structure
- Analyzes Go package dependencies
- Creates a queryable database in `codeql-db/` directory
- Takes 1-3 minutes depending on system resources

**Troubleshooting database creation:**
- If build fails, ensure Go modules are downloaded: `go mod download`
- For build issues, check logs in `codeql-db/log/`
- Use `--threads=4` to limit CPU usage: `codeql database create ... --threads=4`

### Step 2: Run Security Analysis

Analyze the database using CodeQL's standard security queries:

```bash
# Run Go security query suite and output SARIF
codeql database analyze codeql-db \
  codeql/go-queries:codeql-suites/go-code-scanning.qls \
  --format=sarif-latest \
  --output=codeql-results.sarif \
  --sarif-category=go \
  --threads=0
```

**Query suites available:**
- `go-code-scanning.qls`: Standard security queries (recommended)
- `go-security-extended.qls`: Extended security queries (more thorough)
- `go-security-and-quality.qls`: Security + code quality checks

**Alternative: Run specific queries**
```bash
# Example: Check for SQL injection vulnerabilities
codeql database analyze codeql-db \
  codeql/go-queries:Security/CWE-089/SqlInjection.ql \
  --format=csv \
  --output=sql-injection-results.csv
```

**Output formats:**
- `sarif-latest`: Structured JSON for tool integration (recommended)
- `csv`: Tabular format for spreadsheets
- `text`: Human-readable text output

### Step 3: View and Interpret Results

#### Option A: View SARIF in VS Code (Recommended)

1. Install SARIF Viewer extension:
   ```bash
   code --install-extension=MS-SarifVSCode.sarif-viewer
   ```

2. Open results in VS Code:
   ```bash
   code codeql-results.sarif
   ```

3. The SARIF viewer shows:
   - Alert severity (error, warning, note)
   - Affected file and line number
   - Vulnerability description and remediation
   - Code flow path for complex issues

#### Option B: View CSV Results

```bash
# View in terminal
cat results.csv | column -t -s,

# Or open in spreadsheet application
libreoffice --calc results.csv
```

#### Option C: Command-line Summary

```bash
# Extract summary statistics from SARIF
jq '.runs[0].results | length' codeql-results.sarif
# Shows total number of alerts

# Group by severity
jq '.runs[0].results | group_by(.level) | map({level: .[0].level, count: length})' codeql-results.sarif
```

### Step 4: Interpret Common Findings

**Example Alert Structure:**
```json
{
  "ruleId": "go/sql-injection",
  "level": "error",
  "message": {
    "text": "This query depends on a user-provided value."
  },
  "locations": [{
    "physicalLocation": {
      "artifactLocation": {
        "uri": "internal/app/server.go"
      },
      "region": {
        "startLine": 42,
        "startColumn": 15
      }
    }
  }]
}
```

**Severity levels:**
- **error**: High-severity security vulnerabilities (fix immediately)
- **warning**: Medium-severity issues (review and fix)
- **note**: Low-severity or code quality suggestions (optional)

## Makefile Integration

Add CodeQL commands to your Makefile for convenience:

```makefile
# Add to Makefile
.PHONY: codeql-db codeql-analyze codeql-clean

# Create CodeQL database
codeql-db:
	@echo "Creating CodeQL database for Go..."
	codeql database create codeql-db --language=go --source-root=. --overwrite

# Analyze with security queries
codeql-analyze: codeql-db
	@echo "Running CodeQL security analysis..."
	codeql database analyze codeql-db \
		codeql/go-queries:codeql-suites/go-code-scanning.qls \
		--format=sarif-latest \
		--output=codeql-results.sarif \
		--sarif-category=go
	@echo "Results saved to codeql-results.sarif"

# Clean CodeQL artifacts
codeql-clean:
	rm -rf codeql-db codeql-results.sarif

# Run full CodeQL workflow
codeql: codeql-analyze
	@echo "✓ CodeQL analysis complete"
```

**Usage:**
```bash
# Create database and analyze
make codeql

# Clean up artifacts
make codeql-clean
```

## Advanced Usage

### Custom Queries

Create custom queries for project-specific patterns:

1. Create a query file (e.g., `custom-queries/detect-hardcoded-secrets.ql`):
   ```ql
   /**
    * @name Hardcoded secrets
    * @description Detects potential hardcoded API keys or passwords
    * @kind problem
    * @problem.severity warning
    * @id go/hardcoded-secrets
    */
   import go

   from StringLit s
   where s.getValue().regexpMatch("(?i).*(password|api[_-]?key|secret).*=.*")
   select s, "Potential hardcoded secret"
   ```

2. Run custom query:
   ```bash
   codeql database analyze codeql-db custom-queries/detect-hardcoded-secrets.ql \
     --format=sarif-latest \
     --output=custom-results.sarif
   ```

### Incremental Analysis

For faster re-analysis after code changes:

```bash
# Update existing database (faster than full rebuild)
codeql database upgrade codeql-db

# Re-run analysis
codeql database analyze codeql-db ...
```

### Performance Tuning

For large codebases:

```bash
# Limit memory usage (in MB)
codeql database create codeql-db --language=go --ram=8192

# Limit CPU threads
codeql database analyze codeql-db ... --threads=4

# Enable query timeout (in seconds)
codeql database analyze codeql-db ... --timeout=300
```

## Troubleshooting

### Database Creation Fails

**Issue**: `ERROR: Extraction failed`

**Solutions:**
1. Ensure Go modules are downloaded: `go mod download`
2. Check disk space: `df -h`
3. Review logs: `cat codeql-db/log/database-create*.log`
4. Try with verbose output: `codeql database create ... --verbose`

### Out of Memory Errors

**Issue**: `java.lang.OutOfMemoryError`

**Solutions:**
1. Increase RAM allocation: `--ram=16384` (16 GB)
2. Reduce thread count: `--threads=2`
3. Close other applications to free memory
4. Analyze smaller portions of code with `--source-root=./internal/app`

### Query Results Empty

**Issue**: No results in SARIF file

**Solutions:**
1. Verify database was created: `ls -la codeql-db/`
2. Check query suite exists: `codeql resolve queries go-code-scanning.qls`
3. Try a different query suite: `go-security-extended.qls`
4. Enable diagnostics: `--sarif-add-diagnostics`

### CodeQL Not Found After Installation

**Issue**: `command not found: codeql`

**Solutions:**
1. Verify extraction: `ls /opt/codeql/codeql/codeql`
2. Reload shell profile: `source ~/.bashrc`
3. Check PATH: `echo $PATH | grep codeql`
4. Try absolute path: `/opt/codeql/codeql/codeql --version`

## CI/CD Integration (Optional)

While this guide focuses on local analysis, you can optionally upload results to GitHub:

```bash
# Requires GITHUB_TOKEN with security_events scope
export GITHUB_TOKEN="ghp_your_token_here"

codeql github upload-results \
  --repository=pocket/hexagon/spire \
  --ref=refs/heads/main \
  --commit=$(git rev-parse HEAD) \
  --sarif=codeql-results.sarif
```

## Resources

- **Official Documentation**: https://codeql.github.com/docs/
- **Query Reference**: https://codeql.github.com/codeql-query-help/go/
- **CodeQL for Go**: https://codeql.github.com/docs/codeql-language-guides/codeql-for-go/
- **Community Forum**: https://github.com/github/codeql/discussions
- **Query Examples**: https://github.com/github/codeql/tree/main/go/ql/src/Security

## Regular Analysis Workflow

**Recommended practice:**

```bash
# Before committing code
make codeql

# Review results in VS Code
code codeql-results.sarif

# Fix any high-severity issues
# Re-run analysis to verify fixes
make codeql-clean && make codeql

# Commit when clean
git add . && git commit -m "Fix: Resolve CodeQL security findings"
```

## Security Best Practices

1. **Run CodeQL before PRs**: Catch issues early in development
2. **Review all error-level findings**: These are high-confidence vulnerabilities
3. **Triage warnings**: Assess context and fix as appropriate
4. **Keep CodeQL updated**: Run `sudo rm -rf /opt/codeql && <reinstall>` quarterly
5. **Track false positives**: Document with `// lgtm[go/rule-id]` comments
6. **Integrate with IDE**: Use VS Code extension for real-time feedback

## License and Terms

CodeQL is free for research and open source projects. By using CodeQL CLI, you agree to the [GitHub CodeQL Terms and Conditions](https://securitylab.github.com/tools/codeql/license).

For commercial use on proprietary codebases, check GitHub's licensing terms.
