### Static Analysis

```bash
# Go vet
go vet ./...
# Result: PASS - No issues

# Staticcheck
staticcheck ./...
# Result: PASS - No issues

# Cyclomatic complexity
gocyclo -over 15 .
# Result: PASS - No functions exceed threshold (was 26, now max 4)
```

### Coverage Analysis

```bash
# Test coverage
go test ./... -cover
# Result: 85%+ coverage maintained across all packages
```

---

## Patterns Established

### 1. Test File Organization Patterns

#### Pattern A: Split by Functional Area
**Use when**: Test file covers multiple functional concerns

**Example**: `identity_test.go` → 3 files
- `identity_context_test.go` - Context extraction/management
- `identity_matching_test.go` - Path/trust domain matching
- `identity_middleware_test.go` - HTTP middleware

**Benefits**: Clear separation, easier navigation

#### Pattern B: Split by Test Type
**Use when**: Test file has distinct test types (integration, errors, compliance)

**Example**: `client_test.go` → 4 files
- `client_integration_test.go` - Full server integration
- `client_errors_test.go` - Error handling
- `client_response_test.go` - Response objects
- `client_compliance_test.go` - Interface compliance

**Benefits**: Separate fast/slow tests, isolate scenarios

#### Pattern C: Split by Component
**Use when**: Large test file covers multiple components

**Example**: `coverage_boost_test.go` → 5 files
- `agent_coverage_test.go` - Agent operations
- `server_coverage_test.go` - Server operations
- `trust_bundle_coverage_test.go` - Trust bundle provider
- `parser_coverage_test.go` - Parsers
- `validator_coverage_test.go` - Validators

**Benefits**: One file per component, aligns with source structure

### 2. Source File Organization Patterns

#### Pattern: Split by Concern
**Use when**: Source file >350 lines with multiple responsibilities

**Example**: `mtls.go` → 6 files
- `mtls.go` - Types and constants only
- `mtls_loader.go` - File/env loading
- `mtls_env.go` - Environment overrides
- `mtls_defaults.go` - Default values
- `mtls_validation.go` - Validation logic
- `mtls_conversion.go` - Port conversion

**Naming**: `{package}_{concern}.go`

**Benefits**: Main file contains only types, clear concern boundaries

### 3. Complexity Reduction Patterns

#### Pattern: Extract Method for Validation
**Use when**: Function cyclomatic complexity >15

**Example**: `Validate()` method
- **Before**: 76 lines, complexity 26
- **After**: 11 lines, complexity 4 (orchestrator + 5 focused methods)

**Benefits**: 84% complexity reduction, single responsibility

### 4. Documentation Patterns

#### Pattern: Package-Level Test Documentation
**Use for**: All test files

**Template**:
```go
// {Component} {Type} Tests
//
// These tests verify {description}.
// Tests cover {specific areas}.
//
// Run these tests with:
//
//	go test ./internal/path/... -v
//	go test ./internal/path/... -run TestComponent -v
//	go test ./internal/path/... -cover
```

**Benefits**: Self-documenting, clear execution instructions

### 5. Automation Patterns

#### Pattern: Bash Script for File Splits
**Use for**: Repeatable file split operations

**Template**: See `scripts/split_*.sh`

**Benefits**: Automatic backups, repeatable, auditable

---

### Verify File Sizes
```bash
find . -name "*.go" -not -path "./vendor/*" -exec wc -l {} + | sort -rn | head -20
```

### Verify Complexity
```bash
gocyclo -over 15 .
```

### Run All Tests
```bash
go test ./... -v -count=1
```

### Check Coverage
```bash
go test ./... -cover
```

### Static Analysis
```bash
staticcheck ./...
go vet ./...
```

### Build with Tags
```bash
go build -tags dev ./...
```
