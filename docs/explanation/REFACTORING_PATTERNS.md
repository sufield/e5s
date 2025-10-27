# Refactoring Patterns

This document captures successful patterns and best practices discovered during the codebase refactoring effort (Weeks 1-3).

## Table of Contents

1. [Test File Organization](#test-file-organization)
2. [Source File Organization](#source-file-organization)
3. [Complexity Reduction](#complexity-reduction)
4. [Documentation Patterns](#documentation-patterns)
5. [Automation Scripts](#automation-scripts)

---

## Test File Organization

### Pattern: Split by Functional Area

**When to Use**: Test files >350 lines with multiple responsibilities

**Example**: `identity_test.go` (535 lines) → 3 files
- `identity_context_test.go` (250 lines) - Context extraction/management
- `identity_matching_test.go` (170 lines) - Path/trust domain matching
- `identity_middleware_test.go` (156 lines) - HTTP middleware

**Benefits**:
- Clear separation of concerns
- Easier navigation and maintenance
- Focused test documentation
- Better test organization

**Implementation**:
```bash
# 1. Analyze test structure
grep "^func Test" original_test.go | wc -l

# 2. Group tests by functional area
# - Context operations (Get*, With*)
# - Matching operations (Matches*, Has*)
# - Middleware operations (Require*, Log*)

# 3. Create new files with package documentation
# 4. Delete original after verification
```

### Pattern: Split by Test Type

**When to Use**: Test files with distinct test types (integration, errors, compliance)

**Example**: `client_test.go` (427 lines) → 4 files
- `client_integration_test.go` (86 lines) - Full server integration tests
- `client_errors_test.go` (256 lines) - Error handling and edge cases
- `client_response_test.go` (46 lines) - Response object tests
- `client_compliance_test.go` (105 lines) - Interface compliance and concurrency

**Benefits**:
- Separate fast unit tests from slow integration tests
- Isolate error scenarios for debugging
- Clear compliance verification
- Easier to run specific test types

**Implementation Pattern**:
```go
// client_integration_test.go
// These tests use full application bootstrap with real server
func TestClient_FetchX509SVID_Success(t *testing.T) {
    t.Parallel()
    ctx := context.Background()

    // Bootstrap real application
    loader := inmemory.NewInMemoryConfig()
    factory := compose.NewInMemoryAdapterFactory()
    application, err := app.Bootstrap(ctx, loader, factory)
    // ... rest of test
}

// client_errors_test.go
// These tests use mock HTTP servers for error scenarios
func TestClient_FetchX509SVID_ServerError(t *testing.T) {
    t.Parallel()

    // Create mock server with error response
    listener, err := net.Listen("unix", socketPath)
    ts := &http.Server{
        Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            w.WriteHeader(http.StatusInternalServerError)
        }),
    }
    // ... rest of test
}
```

### Pattern: Split by Component

**When to Use**: Large test files covering multiple components

**Example**: `coverage_boost_test.go` (936 lines, 26 tests) → 5 files
- `agent_coverage_test.go` (326 lines, 7 tests) - Agent operations
- `server_coverage_test.go` (76 lines, 2 tests) - Server operations
- `trust_bundle_coverage_test.go` (189 lines, 6 tests) - Trust bundle provider
- `parser_coverage_test.go` (254 lines, 6 tests) - Parser tests
- `validator_coverage_test.go` (156 lines, 5 tests) - Validator tests

**Benefits**:
- One file per component
- Aligns with source file organization
- Easier to find related tests
- Clear component boundaries

---

## Source File Organization

### Pattern: Split by Concern

**When to Use**: Source files >350 lines with multiple responsibilities

**Example**: `mtls.go` (400 lines) → 6 files
- `mtls.go` (61 lines) - Types and constants only
- `mtls_loader.go` (41 lines) - File/env loading
- `mtls_env.go` (115 lines) - Environment variable overrides
- `mtls_defaults.go` (46 lines) - Default value application
- `mtls_validation.go` (120 lines) - Configuration validation
- `mtls_conversion.go` (36 lines) - Port conversion methods

**Benefits**:
- Main file contains only type definitions
- Each concern in dedicated file
- Easy to locate functionality
- Maintainable file sizes

**File Naming Convention**:
```
{package}_{concern}.go
```

Examples:
- `mtls_loader.go` - Loading concern
- `mtls_validation.go` - Validation concern
- `mtls_env.go` - Environment concern

**Main File Pattern**:
```go
// mtls.go - Types and constants only
package config

import "time"

// Default configuration constants
const (
    DefaultSPIRESocket = "unix:///tmp/spire-agent/public/api.sock"
    DefaultTrustDomain = "example.org"
    // ... other constants
)

// MTLSConfig holds configuration for mTLS server and client
type MTLSConfig struct {
    HTTP  HTTPConfig  `yaml:"http"`
    SPIRE SPIREConfig `yaml:"spire"`
}

// HTTPConfig configures the HTTP server
type HTTPConfig struct {
    Enabled bool   `yaml:"enabled"`
    Address string `yaml:"address"`
    // ... other fields
}
// ... other types
```

**Concern File Pattern**:
```go
// mtls_validation.go - Single concern implementation
package config

import "fmt"

// Validate validates the entire configuration
func (c *MTLSConfig) Validate() error {
    if err := c.validateSPIREConfig(); err != nil {
        return err
    }
    if err := c.validateHTTPConfig(); err != nil {
        return err
    }
    return c.validateAuthConfig()
}

// validateSPIREConfig validates SPIRE configuration
func (c *MTLSConfig) validateSPIREConfig() error {
    // ... validation logic
}
// ... other validation methods
```

---

## Complexity Reduction

### Pattern: Extract Method for Validation

**When to Use**: Functions with cyclomatic complexity >15

**Example**: `Validate()` method (complexity 26) → complexity 4

**Before**:
```go
func (c *MTLSConfig) Validate() error {
    // 76 lines of inline validation
    if c.SPIRE.SocketPath == "" {
        return fmt.Errorf("spire.socket_path is required")
    }
    if !strings.HasPrefix(c.SPIRE.SocketPath, "unix://") {
        return fmt.Errorf("spire.socket_path must start with 'unix://'")
    }
    // ... 70+ more lines
}
```

**After**:
```go
// Main validation method (complexity 4)
func (c *MTLSConfig) Validate() error {
    if err := c.validateSPIREConfig(); err != nil {
        return err
    }
    if err := c.validateHTTPConfig(); err != nil {
        return err
    }
    return c.validateAuthConfig()
}

// Focused validation methods
func (c *MTLSConfig) validateSPIREConfig() error {
    if c.SPIRE.SocketPath == "" {
        return fmt.Errorf("spire.socket_path is required")
    }
    if !strings.HasPrefix(c.SPIRE.SocketPath, "unix://") {
        return fmt.Errorf("spire.socket_path must start with 'unix://'")
    }
    // ... rest of SPIRE validation
}

func (c *MTLSConfig) validateHTTPConfig() error {
    // ... HTTP validation
}

func (c *MTLSConfig) validateAuthConfig() error {
    // ... Auth validation
}
```

**Benefits**:
- Reduced cyclomatic complexity (26 → 4 = 84% reduction)
- Each method has single responsibility
- Easier to test individual validations
- Clear error context

**Complexity Thresholds**:
- Target: <15 per function
- Warning: 15-20
- Critical: >20 (requires refactoring)

### Pattern: Sequential Validation

**Structure**:
```go
func (c *Config) Validate() error {
    validators := []func() error{
        c.validateField1,
        c.validateField2,
        c.validateField3,
    }

    for _, validate := range validators {
        if err := validate(); err != nil {
            return err
        }
    }
    return nil
}
```

**Alternative (Recommended)**:
```go
func (c *Config) Validate() error {
    if err := c.validateField1(); err != nil {
        return err
    }
    if err := c.validateField2(); err != nil {
        return err
    }
    return c.validateField3()
}
```

---

## Documentation Patterns

### Pattern: Package-Level Test Documentation

**When to Use**: All test files

**Template**:
```go
package mypackage_test

// {Component} {Type} Tests
//
// These tests verify {description of what is tested}.
// Tests cover {specific areas covered}.
//
// Run these tests with:
//
//	go test ./internal/path/... -v
//	go test ./internal/path/... -run TestComponent -v
//	go test ./internal/path/... -cover
```

**Examples**:

```go
// Workload API Client Integration Tests
//
// These tests verify end-to-end client behavior with a real server.
// Tests use full application bootstrap with inmemory adapters.
//
// Run these tests with:
//
//	go test ./internal/adapters/outbound/workloadapi/... -v -run TestClient.*Success
//	go test ./internal/adapters/outbound/workloadapi/... -cover
```

```go
// Identity Matching Tests
//
// These tests verify path and trust domain matching utilities for SPIFFE identities.
// Tests cover trust domain matching, path prefix/suffix matching, and exact ID matching.
//
// Run these tests with:
//
//	go test ./internal/adapters/inbound/httpapi/... -v -run TestMatches
//	go test ./internal/adapters/inbound/httpapi/... -v -run TestHas
//	go test ./internal/adapters/inbound/httpapi/... -cover
```

**Benefits**:
- Self-documenting tests
- Easy onboarding for new developers
- Clear test execution instructions
- Coverage tracking guidance

### Pattern: Source File Documentation

**Template**:
```go
// Package {name} provides {description}.
package name

// {Description of file purpose}
//
// This file contains {what it contains}.
```

**Example**:
```go
// Package config provides mTLS configuration management.
package config

// Configuration validation
//
// This file contains validation logic for mTLS configuration including
// SPIRE settings, HTTP settings, and authentication configuration.
```

---

## Automation Scripts

### Pattern: Bash Script for File Splits

**When to Use**: Splitting test files (repeatable, auditable)

**Template**:
```bash
#!/bin/bash

set -e  # Exit on error

ORIG="path/to/original.go"
BACKUP="path/to/original.go.bak"

# Backup original
cp "$ORIG" "$BACKUP"

echo "Creating file1.go..."
cat > path/to/file1.go << 'EOF'
package name

// Documentation
// ...

import (...)

// Tests...
EOF

echo "Creating file2.go..."
cat > path/to/file2.go << 'EOF'
...
EOF

echo "Removing original..."
rm "$ORIG"

echo "✅ Split complete!"
echo ""
echo "Created files:"
echo "  - file1.go (description)"
echo "  - file2.go (description)"
echo ""
echo "Original backed up to: $BACKUP"
```

**Benefits**:
- Repeatable process
- Automatic backup
- Clear output
- Easy to review before execution

**Example Usage**:
```bash
chmod +x scripts/split_client_test.sh
bash scripts/split_client_test.sh
go test ./internal/adapters/outbound/workloadapi/... -v
```

### Pattern: Verification After Split

**Process**:
```bash
# 1. Run split script
bash scripts/split_test_file.sh

# 2. Verify tests pass
go test ./path/to/package/... -v

# 3. Check file sizes
wc -l path/to/package/*_test.go

# 4. Check coverage maintained
go test ./path/to/package/... -cover
```

**Rollback if needed**:
```bash
# Restore from backup
cp path/to/file_test.go.bak path/to/file_test.go

# Remove new files
rm path/to/new_file1_test.go path/to/new_file2_test.go
```

---

## File Size Guidelines

### Target Sizes

| File Type | Target | Warning | Critical |
|-----------|--------|---------|----------|
| Test files | <250 lines | 250-350 | >350 |
| Source files | <200 lines | 200-300 | >300 |
| Config files | <150 lines | 150-250 | >250 |

### Exceptions

**Acceptable larger files**:
- Table-driven tests with many test cases (up to 400 lines)
- Type definition files with many structs (up to 300 lines)
- Generated code

**When to split**:
- Multiple responsibilities (different concerns)
- Hard to navigate (scrolling fatigue)
- Hard to understand (cognitive overload)
- High cyclomatic complexity

---

## Quick Reference Checklist

### Before Splitting a File

- [ ] File >350 lines (test) or >300 lines (source)?
- [ ] Multiple clear responsibilities?
- [ ] Would split improve maintainability?
- [ ] Clear naming for new files?

### During Split

- [ ] Create backup of original
- [ ] Add package documentation to each new file
- [ ] Preserve all imports
- [ ] Use bash script for automation
- [ ] Clear, descriptive file names

### After Split

- [ ] All tests pass: `go test ./... -v`
- [ ] Coverage maintained: `go test ./... -cover`
- [ ] Complexity reduced: `gocyclo -over 15 .`
- [ ] Static checks pass: `staticcheck ./...`
- [ ] File sizes within targets
- [ ] Commit with clear message

---

## Results Summary (Weeks 1-3)

### Metrics Achieved

**Test File Splits**:
- `coverage_boost_test.go`: 936 → 5 files (avg 187 lines each)
- `mtls_extended_test.go`: 424 → 3 files (avg 154 lines each)
- `identity_test.go`: 535 → 3 files (avg 192 lines each)
- `client_test.go`: 427 → 4 files (avg 123 lines each)
- `server_test.go`: 411 → 3 files (avg 154 lines each)

**Source File Splits**:
- `mtls.go`: 400 → 6 files (main file reduced 85%)

**Complexity Reductions**:
- `mtls.go Validate()`: 26 → 4 (84% reduction)

**Zero Regressions**:
- All 100+ tests passing
- Coverage maintained
- No functionality changes

### Key Learnings

1. **Always analyze before splitting** - Don't follow plan blindly; adapt to actual content
2. **Use automation scripts** - Bash scripts ensure repeatability and reduce errors
3. **Test after each change** - Immediate feedback prevents compound errors
4. **Document patterns** - Capture learnings for future refactoring
5. **Backup originals** - Easy rollback if needed

### Recommended Next Steps

1. Apply these patterns to remaining large files
2. Document domain-specific patterns as discovered
3. Create linting rules to prevent pattern violations
4. Regular refactoring sessions to maintain code quality
