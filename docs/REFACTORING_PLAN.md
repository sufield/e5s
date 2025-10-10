# Refactoring Plan v2.0

**Version**: 2.0
**Date**: October 10, 2025
**Status**: Draft for Team Review
**Owner**: Development Team

---

## Executive Summary

This refactoring plan targets the **top 12 oversized files** (>350 lines) to improve code maintainability, test execution speed, and developer productivity. The primary focus is splitting large test files to enable parallel execution and modularizing the config subsystem for better separation of concerns.

**Expected Outcomes:**
- ✅ Reduce average file size by **17%** (115→95 lines)
- ✅ Eliminate all files >500 lines (currently 5 files)
- ✅ Enable parallel test execution: **~60% reduction in serial test time** (5m→2m)
- ✅ Improve code navigability and reduce cognitive load

**Total Effort**: 20-30 hours over 4 weeks
**Risk Level**: Low-Medium (comprehensive test coverage mitigates risk)
**ROI**: High (faster CI/CD, easier onboarding, reduced bug introduction)

---

## Table of Contents

- [Executive Summary](#executive-summary)
- [Current State Baseline](#current-state-baseline)
- [Priority 1: Source Files](#priority-1-source-files-high-impact)
- [Priority 2: Test Files](#priority-2-test-files-maintainability)
- [Refactoring Priority Matrix](#refactoring-priority-matrix)
- [Implementation Checklist](#implementation-checklist)
- [Metrics & Monitoring](#metrics--monitoring)
- [Quick Wins](#quick-wins-low-effort-high-impact)
- [Tools & Automation](#tools--automation)
- [Timeline & Milestones](#timeline--milestones)
- [Team Assignments](#team-assignments)
- [Success Criteria](#success-criteria)

---

## Current State Baseline

### File Size Metrics (Before Refactoring)

```
Total Go Files:     ~150
Average File Size:  115 lines
Files >300 lines:   50 (33%)
Files >400 lines:   12 (8%)
Files >500 lines:   5 (3%)
Largest File:       936 lines (coverage_boost_test.go)
Test-to-Code Ratio: 84% tests, 16% source
```

### Performance Baseline

| Metric | Current Value | Tool |
|--------|---------------|------|
| **Config Module** | | |
| `mtls.go` size | 364 lines | `wc -l` |
| `mtls.go` cyclomatic complexity | 26 (Validate), 24 (applyEnvOverrides) | `gocyclo` |
| Config test time | 0.45s (cached) | `time go test` |
| **InMemory Adapter** | | |
| `coverage_boost_test.go` size | 936 lines (26 test functions) | `wc -l` |
| Test execution time | 0.38s (serial) | `go test -v` |
| Estimated parallel speedup | **~60%** (0.38s→0.15s) | Projection |
| **Overall Test Suite** | | |
| Full test time | ~2m 30s | `go test ./...` |
| Test coverage | 85%+ | `go test -cover` |

**Measurement Command:**
```bash
# Generate baseline report
make refactor-baseline  # See automation section
```

---

## Priority 1: Source Files (High Impact)

### 1. 🔴 **internal/config/mtls.go** (364 lines) - CRITICAL PRIORITY

**Current State:**
- **Lines**: 364
- **Functions**: 8 (2 large: `applyEnvOverrides` 91 lines, `Validate` 78 lines)
- **Cyclomatic Complexity**: 26 (Validate), 24 (applyEnvOverrides) - **High Risk**
- **Test Coverage**: 95%
- **Dependencies**: Used by all HTTP/mTLS components

**Refactoring Strategy: Split into Focused Modules**

#### Proposed File Structure:
```
internal/config/
├── mtls.go                  (types only - 70 lines)
│   └── MTLSConfig, HTTPConfig, SPIREConfig, AuthConfig structs
├── mtls_loader.go           (loading logic - 40 lines)
│   └── LoadFromFile(), LoadFromEnv()
├── mtls_env.go              (environment overrides - 110 lines)
│   └── applyEnvOverrides(), parseBool()
├── mtls_defaults.go         (default values - 50 lines)
│   └── applyDefaults()
├── mtls_validation.go       (validation logic - 85 lines)
│   └── Validate(), validateSPIRE(), validateHTTP(), validateAuth()
└── mtls_conversion.go       (port conversion - 35 lines)
    └── ToServerConfig(), ToClientConfig()
```

#### Alternative: Keep Single File with Extracted Functions

If splitting is deferred, extract validation into sub-functions:

```go
// Before: 78-line monolithic Validate()
func (c *MTLSConfig) Validate() error {
    // 78 lines of validation logic...
}

// After: 10-line orchestrator + 3 focused validators
func (c *MTLSConfig) Validate() error {
    if err := c.validateSPIRE(); err != nil { return err }    // 20 lines
    if err := c.validateHTTP(); err != nil { return err }      // 25 lines
    if err := c.validateAuth(); err != nil { return err }      // 30 lines
    return nil
}
```

**Benefits (Quantified):**
- ✅ **Cyclomatic Complexity**: Reduce from 26→8 per function (67% reduction)
- ✅ **Cognitive Load**: Max file size 110 lines (70% reduction)
- ✅ **Test Time**: Enable parallel test execution (~20% speedup)
- ✅ **Maintainability**: Clear separation makes changes safer
- ✅ **Code Review**: Reviewers can focus on single concern per file

**Viper Prototype Consideration:**
- If env/validation logic continues growing, prototype with Viper library
- **Potential savings**: 50-70% reduction in boilerplate
- **Effort**: 4-6 hours for prototype + comparison
- **Decision criteria**: If manual env handling exceeds 150 lines, migrate to Viper

**Effort Breakdown:**
- Planning & file splits: 1 hour
- Moving functions & updating imports: 2 hours
- Updating tests (split mtls_test.go accordingly): 1 hour
- Code review & iteration: 1 hour
- **Total**: 5 hours

**Risk**: 🟢 Low (comprehensive tests exist)

**Rollback Plan:**
- Revert commits if any test fails
- Branch strategy: `refactor/config-split` → merge only after full test pass

---

### 2. 🟡 **internal/controlplane/adapters/helm/install_dev.go** (294 lines) - MEDIUM PRIORITY

**Current State:**
- **Lines**: 294
- **Purpose**: DevOps automation for Helm/Kubernetes setup
- **Dependencies**: Likely used in CI/CD pipelines

**Refactoring Strategy:**

#### Proposed Split by Environment:
```
internal/controlplane/adapters/helm/
├── install_common.go       (shared helpers - 80 lines)
├── install_minikube.go     (dev environment - 100 lines)
└── install_prod.go         (production setup - 120 lines)
```

**Risk Mitigation:**
- ⚠️ **Medium Risk**: Deployment logic requires careful testing
- **Shadow Deploy**: Test refactored scripts in staging environment first
- **CI Integration**: Add `helm lint` and `helm test` jobs to validate changes
- **Rollback**: Keep original script tagged as `install_dev_legacy.go` for 1 sprint

**Effort**: 4-6 hours
**Dependencies**: DevOps team approval required

---

### 3. 🟢 **internal/httpclient/client.go** (239 lines) - LOW PRIORITY (Monitor Only)

**Current State:**
- **Lines**: 239 (just below 300-line threshold)
- **Status**: Acceptable size for HTTP client logic

**Action**:
- ✅ **Monitor only** - no immediate refactoring
- 🔔 **Auto-alert**: GitHub Action warns if file exceeds 300 lines in PR
- **Future split** (if grows >300):
  - `client_request.go` (request building)
  - `client_response.go` (response handling)
  - `client_errors.go` (error handling)

**Monitoring Rule:**
```yaml
# .github/workflows/file-size-check.yml
- name: Check file size
  run: |
    if [ $(wc -l < internal/httpclient/client.go) -gt 300 ]; then
      echo "::warning::client.go exceeds 300 lines - consider refactoring"
    fi
```

---

## Priority 2: Test Files (Maintainability)

### 1. 🔴 **internal/adapters/outbound/inmemory/coverage_boost_test.go** (936 lines) - CRITICAL

**Current State:**
- **Lines**: 936 (LARGEST FILE IN CODEBASE)
- **Test Functions**: 26
- **Components Tested**: Agent, Server, TrustBundleProvider, Parsers, Validators
- **Execution Time**: 0.38s serial → **0.15s parallel** (60% speedup potential)
- **Coverage**: Comprehensive edge cases and error paths

**Refactoring Strategy: Split by Component**

#### Proposed Structure:
```
internal/adapters/outbound/inmemory/
├── coverage_agent_test.go          (10 tests, ~300 lines)
│   ├── Agent_FetchIdentityDocument_NoSelectorsRegistered
│   ├── Agent_FetchIdentityDocument_NoMatchingMapper
│   ├── Agent_FetchIdentityDocument_InvalidSelector
│   ├── Agent_FetchIdentityDocument_FullErrorFlow
│   ├── Agent_GetIdentity
│   ├── Agent_ExtractName_RootPath
│   └── Agent_NewInMemoryAgent_ErrorPaths (4 more)
│
├── coverage_trustbundle_test.go    (6 tests, ~200 lines)
│   ├── TrustBundleProvider_GetBundle
│   ├── TrustBundleProvider_GetBundleForIdentity
│   ├── TrustBundleProvider_EmptyCAs
│   ├── TrustBundleProvider_GetBundle_NilTrustDomain
│   ├── TrustBundleProvider_GetBundleForIdentity_NilNamespace
│   └── TrustBundleProvider_MultiCAConcat
│
├── coverage_parsers_test.go        (5 tests, ~250 lines)
│   ├── TrustDomainParser_FromString_EmptyString
│   ├── TrustDomainParser_FromString_ValidCases
│   ├── TrustDomainParser_FromString_InvalidDomain
│   ├── IdentityCredentialParser_ParseFromPath
│   └── IdentityCredentialParser_ParseFromPath_ErrorCases
│
├── coverage_validators_test.go     (4 tests, ~150 lines)
│   ├── IdentityDocumentValidator_Validate_NilDocument
│   ├── IdentityDocumentValidator_Validate_ExpiredDocument
│   ├── IdentityDocumentValidator_Validate_MismatchedNamespace
│   └── IdentityDocumentValidator_Validate_Success
│
└── coverage_server_test.go         (1 test, ~50 lines)
    └── Server_NewInMemoryServer_ErrorPaths
```

**Benefits (Quantified):**
- ✅ **Parallel Execution**: 0.38s → 0.15s (**60% faster**, runs 5 files concurrently)
- ✅ **Navigation**: Find specific test in <5 seconds vs. scrolling 900+ lines
- ✅ **CI/CD**: Faster feedback loop on failures (component isolation)
- ✅ **Maintainability**: Each file <350 lines (62% reduction per file)
- ✅ **Test Organization**: Clear component boundaries

**Migration Steps:**
1. Create new test files with proper naming
2. Move test functions preserving exact logic (use `git mv` for history)
3. Extract shared test helpers to `inmemory_test_helpers.go` if needed
4. Run `go test ./internal/adapters/outbound/inmemory/... -v` after each move
5. Verify coverage unchanged: `go test -cover -coverprofile=after.out`

**Effort**: 6 hours
**Risk**: 🟢 Low (tests verify themselves)
**ROI**: ⭐⭐⭐⭐⭐ **Highest** (4h effort for 4x maintainability gain + test speedup)

---

### 2. 🟡 **internal/adapters/inbound/httpapi/identity_test.go** (523 lines) - HIGH PRIORITY

**Current State:**
- **Lines**: 523
- **Purpose**: HTTP API endpoint tests for identity operations
- **Likely Structure**: Create, Get, List, Update, Delete endpoints

**Refactoring Strategy: Split by HTTP Method/Endpoint**

#### Proposed Structure:
```
internal/adapters/inbound/httpapi/
├── identity_create_test.go    (~130 lines - POST endpoints)
├── identity_get_test.go        (~100 lines - GET single)
├── identity_list_test.go       (~120 lines - GET collection)
├── identity_update_test.go     (~90 lines - PUT/PATCH)
├── identity_delete_test.go     (~60 lines - DELETE)
└── identity_errors_test.go     (~80 lines - 4xx/5xx errors)
```

**Optimization Techniques:**
1. **Convert to Table-Driven Tests** (if not already):
   ```go
   tests := []struct {
       name       string
       input      Request
       wantStatus int
       wantBody   string
   }{
       {"valid input", validReq, 200, "success"},
       {"invalid ID", invalidReq, 400, "bad request"},
   }
   for _, tt := range tests {
       t.Run(tt.name, func(t *testing.T) { /* ... */ })
   }
   ```

2. **Extract Fixtures**: Move test data to `testdata/` or `identity_fixtures_test.go`

3. **Use `t.Parallel()`**: Enable parallel sub-tests immediately (no file split needed)

**Effort**: 4 hours
**Risk**: 🟢 Low

---

### 3. ✅ **Invariant Tests** (5 files, 258-417 lines) - ALREADY OPTIMIZED

**Files:**
| File | Lines | Status |
|------|-------|--------|
| `registry_invariants_test.go` | 417 | ✅ Table-driven |
| `selector_invariants_test.go` | 377 | ✅ Table-driven |
| `identity_credential_invariants_test.go` | 294 | ✅ Optimized |
| `identity_document_invariants_test.go` | 287 | ✅ Optimized |
| `identity_mapper_invariants_test.go` | 258 | ✅ Optimized |

**Action**: ✅ **No immediate refactoring needed**

**Recent Improvement**: `trust_domain_invariants_test.go` recently reduced from 274→162 lines (41% reduction) via consolidation - **excellent reference pattern**

**Monitoring**: If any grows >500 lines, consider extracting common assertions to `domain/invariants_helpers_test.go`

---

### 4. 🟡 **Client/Server Tests** (415-399 lines) - MEDIUM PRIORITY

**Files:**
- `internal/adapters/outbound/workloadapi/client_test.go` (415 lines)
- `internal/adapters/inbound/workloadapi/server_test.go` (399 lines)

**Refactoring Strategy:**

#### For Client Tests:
```
workloadapi/
├── client_test.go              (core logic - 150 lines)
├── client_happy_path_test.go   (success cases - 120 lines)
├── client_errors_test.go       (error handling - 120 lines)
└── client_fixtures_test.go     (test data/mocks - 50 lines)
```

#### For Server Tests:
- Similar split: happy path / errors / fixtures
- Extract shared `setupMockServer(t)` helper

**Techniques:**
- Use `testdata/` directory for request/response samples
- Implement `t.Cleanup()` for test isolation
- Add `t.Parallel()` to independent tests

**Effort**: 3-4 hours each
**Risk**: 🟢 Low

---

### 5. 🟡 **internal/config/mtls_extended_test.go** (412 lines) - MEDIUM PRIORITY

**Current State:**
- **Lines**: 412
- **Status**: Already well-structured with table-driven tests
- **Coverage**: Comprehensive env override and validation tests

**Action**:
- ✅ **Defer until source file (`mtls.go`) is split**
- When source splits, mirror structure in tests:
  ```
  config/
  ├── mtls_env_test.go          (env override tests - 150 lines)
  ├── mtls_validation_test.go   (validation tests - 180 lines)
  └── mtls_integration_test.go  (end-to-end tests - 100 lines)
  ```

**Effort**: 2 hours (after source split)
**Risk**: 🟢 Low

---

## Refactoring Priority Matrix

| Rank | File | Lines | Priority | Effort | Risk | Impact | ROI | Timeline | Dependencies |
|------|------|-------|----------|--------|------|--------|-----|----------|--------------|
| 1 | `coverage_boost_test.go` | 936 | 🔴 Critical | 6h | 🟢 Low | ⭐⭐⭐⭐⭐ | **Highest** | Week 1 | None |
| 2 | `mtls.go` | 364 | 🔴 High | 5h | 🟢 Low | ⭐⭐⭐⭐ | High | Week 1-2 | Tests must pass |
| 3 | `identity_test.go` | 523 | 🟡 High | 4h | 🟢 Low | ⭐⭐⭐ | High | Week 2 | None |
| 4 | `client_test.go` (workload) | 415 | 🟡 Medium | 3h | 🟢 Low | ⭐⭐⭐ | Medium | Week 3 | None |
| 5 | `mtls_extended_test.go` | 412 | 🟡 Medium | 2h | 🟢 Low | ⭐⭐ | Medium | Week 3 | Mtls.go split |
| 6 | `server_test.go` (workload) | 399 | 🟡 Medium | 4h | 🟢 Low | ⭐⭐⭐ | Medium | Week 3 | None |
| 7 | `install_dev.go` | 294 | 🟡 Medium | 5h | 🟡 Med | ⭐⭐ | Medium | Week 4 | DevOps approval |
| 8 | `registry_invariants_test.go` | 417 | 🟢 Low | N/A | N/A | ⭐ | N/A | Monitor | - |
| 9 | `selector_invariants_test.go` | 377 | 🟢 Low | N/A | N/A | ⭐ | N/A | Monitor | - |
| 10 | `service_test.go` | 382 | 🟢 Low | 3h | 🟢 Low | ⭐⭐ | Low | Backlog | - |
| 11 | `service_invariants_test.go` | 357 | 🟢 Low | N/A | N/A | ⭐ | N/A | Monitor | - |
| 12 | `client.go` (httpclient) | 239 | 🟢 Monitor | N/A | N/A | ⭐ | N/A | Auto-alert | - |

**Legend:**
- **Priority**: 🔴 Critical, 🟡 Medium, 🟢 Low/Monitor
- **Risk**: 🔴 High, 🟡 Medium, 🟢 Low
- **Impact**: ⭐⭐⭐⭐⭐ (Highest) to ⭐ (Low)
- **ROI**: Return on Investment (Impact ÷ Effort)

---

## Implementation Checklist

### Pre-Refactoring (Before Any Changes)

- [ ] **Generate Baseline Metrics**
  ```bash
  make refactor-baseline  # Creates baseline.txt with all metrics
  ```
- [ ] **Run Full Test Suite**
  ```bash
  go test ./... -v -count=1 > tests_before.log 2>&1
  ```
- [ ] **Measure Test Coverage**
  ```bash
  go test ./... -coverprofile=coverage_before.out
  go tool cover -func=coverage_before.out | tee coverage_before.txt
  ```
- [ ] **Check Code Quality**
  ```bash
  staticcheck ./... > staticcheck_before.txt
  gocyclo -over 15 . > gocyclo_before.txt
  go vet ./... 2>&1 | tee govet_before.txt
  ```
- [ ] **Document Current Behavior**
  - Add to docs/REFACTORING_BASELINE.md
  - Include screenshots of test output if applicable
- [ ] **Create Refactoring Branch**
  ```bash
  git checkout -b refactor/week1-coverage-tests
  ```
- [ ] **Set Up Monitoring Dashboard** (optional)
  - Track metrics in GitHub wiki or Prometheus

### During Refactoring (For Each File)

- [ ] **Plan the Split**
  - Draw file structure diagram
  - List functions to move
  - Identify shared dependencies
- [ ] **Create New Files**
  ```bash
  touch internal/config/mtls_validation.go
  ```
- [ ] **Move Functions Atomically**
  - Use `git mv` for tracking history when moving entire files
  - Copy-paste when extracting functions from same file
  - Update package comments
- [ ] **Update Imports**
  - Run `goimports -w .` after each move
  - Fix any circular import issues
- [ ] **Run Tests After Each Move**
  ```bash
  go test ./internal/config/... -v -count=1
  ```
- [ ] **Commit Atomically**
  ```bash
  git add .
  git commit -m "refactor(config): extract validation to mtls_validation.go"
  ```
- [ ] **Use `t.Parallel()` in Tests**
  - Add to all independent test functions
  - Verify no shared state issues

### Post-Refactoring (After All Changes)

- [ ] **Verify Full Test Suite Passes**
  ```bash
  go test ./... -v -count=1 > tests_after.log 2>&1
  diff tests_before.log tests_after.log  # Should show only file name changes
  ```
- [ ] **Measure Test Coverage (Must Match or Improve)**
  ```bash
  go test ./... -coverprofile=coverage_after.out
  go tool cover -func=coverage_after.out | tee coverage_after.txt
  diff coverage_before.txt coverage_after.txt
  ```
- [ ] **Run Code Quality Checks**
  ```bash
  staticcheck ./...  # Must pass
  gocyclo -over 15 . > gocyclo_after.txt
  go vet ./...  # Must pass
  ```
- [ ] **Measure Performance Improvements**
  ```bash
  make refactor-compare  # Compares before/after metrics
  ```
- [ ] **Update Documentation**
  - Update package godoc comments
  - Update README if file structure changed
  - Add to CHANGELOG.md
- [ ] **Run Formatting**
  ```bash
  gofmt -w .
  goimports -w .
  ```
- [ ] **Create Pull Request**
  - Title: "refactor: split coverage_boost_test.go by component"
  - Include before/after metrics in description
  - Link to this refactoring plan
  - Assign reviewers
- [ ] **Cleanup**
  - Delete any `_legacy.go` files after 1 sprint
  - Archive baseline reports in docs/refactoring/

---

## Metrics & Monitoring

### Baseline Metrics (October 10, 2025)

| Category | Metric | Current | Target | Improvement |
|----------|--------|---------|--------|-------------|
| **File Size** | | | | |
| Average file size | 115 lines | 95 lines | -17% |
| Files >500 lines | 5 | 0 | -100% |
| Files >400 lines | 12 | 3 | -75% |
| Largest file | 936 lines | <350 lines | -63% |
| **Code Complexity** | | | | |
| Cyclomatic complexity (max) | 26 | <15 | -42% |
| Functions >50 lines | 8 | 2 | -75% |
| **Test Performance** | | | | |
| Config test time | 0.45s | 0.35s | -22% |
| InMemory test time | 0.38s | 0.15s | -60% |
| Full test suite | 2m 30s | 2m 00s | -20% |
| **Test Coverage** | | | | |
| Overall coverage | 85% | 85%+ | ≥0% |
| Config module | 95% | 95%+ | ≥0% |

### Automated Monitoring

#### GitHub Actions Workflow (`.github/workflows/file-size-check.yml`):

```yaml
name: File Size Monitor

on: [pull_request]

jobs:
  check-file-sizes:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3

      - name: Check for oversized files
        run: |
          # Find files >400 lines and warn
          OVERSIZED=$(find . -name "*.go" -not -path "./vendor/*" -exec wc -l {} + | \
                      awk '$1 > 400 {print $0}' | sort -rn)

          if [ -n "$OVERSIZED" ]; then
            echo "::warning::Files exceeding 400 lines:"
            echo "$OVERSIZED"
          fi

          # Fail if any file >500 lines (post-refactor)
          CRITICAL=$(find . -name "*.go" -not -path "./vendor/*" -exec wc -l {} + | \
                     awk '$1 > 500 {print $0}')

          if [ -n "$CRITICAL" ]; then
            echo "::error::Files exceeding 500 lines (refactoring required):"
            echo "$CRITICAL"
            exit 1
          fi

      - name: Check cyclomatic complexity
        run: |
          go install github.com/fzipp/gocyclo/cmd/gocyclo@latest
          gocyclo -over 15 . || echo "::warning::Functions with high complexity detected"
```

#### Makefile Targets:

```makefile
# File: Makefile

.PHONY: refactor-baseline refactor-compare refactor-check

refactor-baseline:
	@echo "Generating refactoring baseline..."
	@mkdir -p docs/refactoring
	@date > docs/refactoring/baseline.txt
	@echo "\n=== File Sizes ===" >> docs/refactoring/baseline.txt
	@find . -name "*.go" -not -path "./vendor/*" -exec wc -l {} + | sort -rn | head -20 >> docs/refactoring/baseline.txt
	@echo "\n=== Cyclomatic Complexity ===" >> docs/refactoring/baseline.txt
	@gocyclo -over 15 . >> docs/refactoring/baseline.txt 2>&1 || true
	@echo "\n=== Test Coverage ===" >> docs/refactoring/baseline.txt
	@go test ./... -coverprofile=docs/refactoring/coverage_before.out > /dev/null 2>&1
	@go tool cover -func=docs/refactoring/coverage_before.out | tail -1 >> docs/refactoring/baseline.txt
	@echo "\n=== Test Time ===" >> docs/refactoring/baseline.txt
	@time go test ./... -v > docs/refactoring/tests_before.log 2>&1
	@echo "Baseline saved to docs/refactoring/baseline.txt"

refactor-compare:
	@echo "Comparing refactoring results..."
	@date > docs/refactoring/comparison.txt
	@echo "\n=== Before/After File Sizes ===" >> docs/refactoring/comparison.txt
	@echo "Top 5 largest files BEFORE:" >> docs/refactoring/comparison.txt
	@grep -A 5 "File Sizes" docs/refactoring/baseline.txt >> docs/refactoring/comparison.txt
	@echo "\nTop 5 largest files AFTER:" >> docs/refactoring/comparison.txt
	@find . -name "*.go" -not -path "./vendor/*" -exec wc -l {} + | sort -rn | head -5 >> docs/refactoring/comparison.txt
	@echo "\n=== Coverage Comparison ===" >> docs/refactoring/comparison.txt
	@go test ./... -coverprofile=docs/refactoring/coverage_after.out > /dev/null 2>&1
	@echo "BEFORE:" >> docs/refactoring/comparison.txt
	@grep "total:" docs/refactoring/baseline.txt >> docs/refactoring/comparison.txt
	@echo "AFTER:" >> docs/refactoring/comparison.txt
	@go tool cover -func=docs/refactoring/coverage_after.out | tail -1 >> docs/refactoring/comparison.txt
	@echo "\nComparison saved to docs/refactoring/comparison.txt"

refactor-check:
	@echo "Running refactoring checks..."
	@go test ./... -v -count=1 || (echo "FAIL: Tests failed" && exit 1)
	@staticcheck ./... || (echo "FAIL: Staticcheck failed" && exit 1)
	@go vet ./... || (echo "FAIL: Go vet failed" && exit 1)
	@gocyclo -over 15 . && echo "WARNING: High complexity detected" || true
	@echo "✅ All checks passed"
```

#### Dashboard Tracking (Optional - Prometheus/Grafana):

```yaml
# metrics.yml - Track in time-series database
metrics:
  - name: file_size_avg
    value: 115
    target: 95

  - name: test_time_seconds
    value: 150
    target: 120

  - name: cyclomatic_complexity_max
    value: 26
    target: 15
```

---

## Quick Wins (Low Effort, High Impact)

### 1. 🏆 **Split `coverage_boost_test.go`** (6 hours) - HIGHEST ROI

**Impact**: ⭐⭐⭐⭐⭐
- **Test Speedup**: 60% faster (0.38s→0.15s via parallelism)
- **Maintainability**: 62% smaller files (936→~200 lines each)
- **CI/CD**: Faster feedback on failures
- **Navigation**: Find tests 5x faster

**Steps**:
1. Create 5 new test files (agent, trustbundle, parsers, validators, server)
2. Move test functions preserving exact logic
3. Run `go test ./internal/adapters/outbound/inmemory/... -v` after each move
4. Verify parallel execution: `go test -parallel 5`

**Effort**: 6 hours
**Risk**: 🟢 Low (tests verify themselves)
**Timeline**: Week 1, Days 1-2

---

### 2. 🥈 **Extract Validation from `mtls.go`** (2 hours) - HIGH ROI

**Impact**: ⭐⭐⭐⭐
- **File Size**: Reduce main file by 22% (364→282 lines)
- **Complexity**: Reduce cyclomatic complexity 26→8
- **Readability**: Clearer separation of concerns

**Steps**:
1. Create `mtls_validation.go`
2. Move `Validate()` function and sub-validators
3. Update imports
4. Run `go test ./internal/config/... -v`

**Effort**: 2 hours
**Risk**: 🟢 Low
**Timeline**: Week 1, Day 3

---

### 3. 🥉 **Add Test Command Examples** (30 minutes) - INSTANT WIN

**Impact**: ⭐⭐⭐
- **Developer Onboarding**: New devs can run tests immediately
- **Documentation**: Self-documenting test files
- **Reproducibility**: Standardized test commands

**Steps**:
1. Add package comment to all large test files:
   ```go
   // Package config tests mTLS configuration loading and validation.
   //
   // Run these tests with:
   //     go test ./internal/config/... -v
   //     go test ./internal/config/... -run TestMTLS -v
   package config
   ```

2. Add to files:
   - `coverage_boost_test.go` (or new split files)
   - `identity_test.go`
   - `mtls_extended_test.go`
   - All other >300 line test files

**Effort**: 30 minutes
**Risk**: 🟢 None
**Timeline**: Week 1, Day 1 (morning)

---

### 4. 🎖️ **Enable `t.Parallel()` in Existing Tests** (1 hour) - QUICK WIN

**Impact**: ⭐⭐⭐
- **Test Speedup**: 20-30% faster without file splits
- **Zero Risk**: Tests already independent
- **Immediate**: No refactoring needed

**Steps**:
1. Add `t.Parallel()` to top of each test function:
   ```go
   func TestAgent_FetchIdentity(t *testing.T) {
       t.Parallel()  // ← Add this line
       // ... rest of test
   }
   ```

2. Target files:
   - `internal/config/mtls_test.go`
   - `internal/domain/*_test.go`
   - Any test file with independent tests

3. Verify: `go test -parallel 8 ./...`

**Effort**: 1 hour
**Risk**: 🟢 Low (verify no shared state)
**Timeline**: Week 1, Day 1 (afternoon)

---

## Tools & Automation

### Essential Tools

Install these tools for analysis:

```bash
# Cyclomatic complexity
go install github.com/fzipp/gocyclo/cmd/gocyclo@latest

# Cognitive complexity (alternative)
go install github.com/uudashr/gocognit/cmd/gocognit@latest

# Import management
go install golang.org/x/tools/cmd/goimports@latest

# Linting (comprehensive)
go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
```

### Analysis Commands

#### 1. Find Oversized Files

```bash
# Files >300 lines (excluding vendor)
find . -name "*.go" -not -path "./vendor/*" -not -path "./.git/*" | \
    xargs wc -l | \
    awk '$1 > 300 {print $1 "\t" $2}' | \
    sort -rn

# Save to file for tracking
find . -name "*.go" -not -path "./vendor/*" | \
    xargs wc -l | \
    awk '$1 > 300' | \
    sort -rn > docs/refactoring/large_files.txt
```

#### 2. Count Test Functions Per File

```bash
# Count test functions in each file
for file in **/*_test.go; do
    count=$(grep -c "^func Test" "$file" 2>/dev/null || echo 0)
    lines=$(wc -l < "$file" 2>/dev/null || echo 0)
    if [ $lines -gt 300 ]; then
        echo "$lines lines, $count tests: $file"
    fi
done | sort -rn
```

#### 3. Analyze Function Lengths

```bash
# Find long functions (>50 lines)
awk '/^func / {
    if (func && lines > 50) print func, lines;
    func=$0; lines=0; next
}
func {lines++}
END {
    if (func && lines > 50) print func, lines
}' internal/config/mtls.go | sort -k2 -rn

# Apply to all files
find . -name "*.go" -not -path "./vendor/*" -exec sh -c '
    awk "/^func / {if (func && lines > 50) print FILENAME, func, lines; func=\$0; lines=0; next} func {lines++} END {if (func && lines > 50) print FILENAME, func, lines}" {}
' \; | sort -k3 -rn | head -20
```

#### 4. Check Cyclomatic Complexity

```bash
# Functions >15 complexity
gocyclo -over 15 . | sort -rn

# All functions with complexity
gocyclo -avg . > docs/refactoring/complexity.txt
```

#### 5. Cognitive Complexity (Alternative)

```bash
# More nuanced than cyclomatic
gocognit -over 15 . | sort -rn
```

#### 6. Test Coverage by Package

```bash
# Coverage report
go test ./... -cover -coverprofile=coverage.out
go tool cover -func=coverage.out | sort -k3 -rn > docs/refactoring/coverage.txt

# HTML coverage visualization
go tool cover -html=coverage.out -o coverage.html
```

#### 7. Measure Test Execution Time

```bash
# Verbose test time (shows per-test timing)
go test ./internal/config/... -v 2>&1 | grep "PASS.*s$"

# Benchmark specific package
go test ./internal/adapters/outbound/inmemory/... -bench . -benchtime=10x

# Compare serial vs parallel
time go test ./... -parallel 1 > serial.log 2>&1
time go test ./... -parallel 8 > parallel.log 2>&1
```

#### 8. Run Linting (Comprehensive)

```bash
# Staticcheck (essential)
staticcheck ./...

# Go vet (builtin)
go vet ./...

# Golangci-lint (combines 50+ linters)
golangci-lint run --enable-all --disable=exhaustivestruct,exhaustruct

# Custom config (.golangci.yml)
golangci-lint run
```

#### 9. Detect Code Duplication

```bash
# Install dupl
go install github.com/mibk/dupl@latest

# Find duplicate code blocks (>15 lines)
dupl -t 15 ./internal/...
```

---

## Timeline & Milestones

### Week 1: Foundation & Quick Wins (Days 1-5)

**Goals**: Achieve 60% of refactoring value with 40% of effort

| Day | Task | Effort | Owner | Deliverable |
|-----|------|--------|-------|-------------|
| Mon AM | Add test command examples to all large test files | 0.5h | @dev1 | PR #1 |
| Mon PM | Enable `t.Parallel()` in existing tests | 1h | @dev1 | PR #2 |
| Tue-Wed | Split `coverage_boost_test.go` into 5 component files | 6h | @dev2 | PR #3 (highest ROI) |
| Thu | Extract validation from `mtls.go` to `mtls_validation.go` | 2h | @dev1 | PR #4 |
| Fri | Team review, merge PRs, measure improvements | 2h | Team | Week 1 retrospective |

**Milestone**: 🎯 **70% of oversized files addressed**, test speedup achieved

---

### Week 2: HTTP API & Config (Days 6-10)

**Goals**: Complete config module refactoring, optimize HTTP tests

| Day | Task | Effort | Owner | Deliverable |
|-----|------|--------|-------|-------------|
| Mon-Tue | Complete `mtls.go` full split (6-file structure) | 3h | @dev1 | PR #5 |
| Wed | Split `mtls_extended_test.go` to match source structure | 2h | @dev1 | PR #6 |
| Thu-Fri | Refactor `identity_test.go` by HTTP method | 4h | @dev2 | PR #7 |

**Milestone**: 🎯 **Config module fully modular**, HTTP API tests organized

---

### Week 3: Workload API Tests (Days 11-15)

**Goals**: Optimize client/server tests, establish patterns

| Day | Task | Effort | Owner | Deliverable |
|-----|------|--------|-------|-------------|
| Mon-Tue | Split `workloadapi/client_test.go` (happy/errors/fixtures) | 3h | @dev2 | PR #8 |
| Wed-Thu | Split `workloadapi/server_test.go` (happy/errors/fixtures) | 4h | @dev2 | PR #9 |
| Fri | Document refactoring patterns for future files | 2h | @dev1 | PATTERNS.md |

**Milestone**: 🎯 **All test files <400 lines**, patterns documented

---

### Week 4: DevOps & Monitoring (Days 16-20)

**Goals**: Refactor DevOps scripts, establish long-term monitoring

| Day | Task | Effort | Owner | Deliverable |
|-----|------|--------|-------|-------------|
| Mon-Wed | Split `install_dev.go` by environment (with shadow deploy) | 5h | @devops | PR #10 |
| Thu | Set up GitHub Actions file size monitoring | 1h | @dev1 | Workflow PR |
| Fri | Final metrics comparison, team presentation | 2h | Team | Refactoring report |

**Milestone**: 🎯 **All refactoring complete**, monitoring established

---

### Post-Week 4: Maintenance & Monitoring

**Ongoing Activities:**
- **Quarterly Reviews**: Check for files >300 lines (calendar reminder)
- **PR Checks**: GitHub Actions enforces <500 line limit
- **Code Reviews**: New files reviewed for size/complexity
- **Documentation**: Update refactoring patterns as team learns

---

## Team Assignments

| Team Member | Primary Focus | Files Assigned | Hours |
|-------------|---------------|----------------|-------|
| **@dev1** | Config Module | `mtls.go`, `mtls_extended_test.go`, test command docs | 10h |
| **@dev2** | Test Files | `coverage_boost_test.go`, `identity_test.go`, workload tests | 14h |
| **@devops** | DevOps | `install_dev.go`, CI/CD monitoring | 6h |
| **@team-lead** | Reviews & Coordination | PR reviews, metrics tracking | 5h |

**Total Team Effort**: 35 hours over 4 weeks (~9 hours/week, ~2 hours/day)

---

## Success Criteria

### Quantitative Metrics (Must Achieve)

| Metric | Baseline | Target | Success Threshold |
|--------|----------|--------|-------------------|
| **Files >500 lines** | 5 | 0 | ✅ 0 files |
| **Files >400 lines** | 12 | ≤3 | ✅ ≤4 files |
| **Average file size** | 115 lines | 95 lines | ✅ ≤100 lines |
| **Max cyclomatic complexity** | 26 | <15 | ✅ <18 |
| **Test coverage** | 85% | ≥85% | ✅ No decrease |
| **Test suite time** | 2m 30s | 2m 00s | ✅ ≤2m 10s |

### Qualitative Outcomes (Team Feedback)

- [ ] **Developer Onboarding**: New devs can find tests 50% faster (survey)
- [ ] **Code Reviews**: Reviewers spend 30% less time navigating large files
- [ ] **CI/CD**: Faster feedback loop on test failures (component isolation)
- [ ] **Maintainability**: Team agrees refactored files are easier to modify

### Risk Mitigation (Zero Tolerance)

- [ ] **No Regressions**: All tests pass before/after (100% pass rate)
- [ ] **No Coverage Loss**: Coverage unchanged or improved
- [ ] **No Breaking Changes**: APIs remain stable
- [ ] **No Production Impact**: Deployment scripts tested in staging first

### Definition of Done

✅ Refactoring is **COMPLETE** when:

1. All files in priority matrix addressed (Weeks 1-4 complete)
2. All quantitative metrics meet success thresholds
3. Full test suite passes with 100% success rate
4. GitHub Actions monitoring active and green
5. Documentation updated (README, PATTERNS.md, CHANGELOG.md)
6. Team retrospective completed with lessons learned
7. Monitoring dashboard established for ongoing tracking

---

## Appendix A: Rollback Plan

If any refactoring introduces issues:

1. **Immediate Rollback**:
   ```bash
   git revert <commit-hash>
   git push origin main
   ```

2. **Partial Rollback** (keep good changes):
   ```bash
   git checkout <last-good-commit> -- path/to/file.go
   git commit -m "rollback: revert problematic changes to file.go"
   ```

3. **Emergency Branch**:
   - Keep `_legacy.go` files for 1 sprint as backup
   - Tag commits: `git tag refactor-checkpoint-week1`

4. **Communication**:
   - Notify team immediately via Slack/email
   - Create incident ticket with root cause
   - Schedule post-mortem within 24 hours

---

## Appendix B: Change Log

| Version | Date | Changes | Author |
|---------|------|---------|--------|
| 1.0 | Oct 10, 2025 | Initial draft based on file size analysis | @dev1 |
| 2.0 | Oct 10, 2025 | Enhanced with metrics, automation, team review feedback | @dev1 |

---

## Appendix C: References

- [Go Code Review Comments](https://github.com/golang/go/wiki/CodeReviewComments)
- [Effective Go](https://go.dev/doc/effective_go)
- [Test Best Practices](https://go.dev/doc/effective_go#testing)
- [Cyclomatic Complexity Guidelines](https://en.wikipedia.org/wiki/Cyclomatic_complexity)
- Internal: Previous refactoring - `trust_domain_invariants_test.go` (274→162 lines, -41%)

---

**Next Steps**: Present this plan in sprint planning, assign owners, create tracking issues, and begin Week 1 Quick Wins.
