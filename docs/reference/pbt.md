---
type: reference
audience: intermediate
---

# Property-Based Testing Guide

**Audience**: Contributors implementing and maintaining tests
**Last Updated**: 2025-10-26

## Overview

This guide provides suggestions for adding property-based testing to the SPIRE wrapper library. Property-based testing complements our existing fuzz tests and unit tests by checking algebraic properties and relationships between functions.

**Current Testing Stack**:
- **Unit Tests**: Example-based tests for specific inputs/outputs
- **Fuzz Tests**: Native Go fuzzing (1.18+) for finding edge cases and crashes
- **Property-Based Tests** (this guide): Testing algebraic properties and invariants

## Why Property-Based Testing?

PBT differs from fuzzing in important ways:

| Aspect | Fuzzing | Property-Based Testing |
|--------|---------|------------------------|
| **Goal** | Find crashes, panics, edge cases | Verify mathematical properties |
| **Input Generation** | Random mutation of seed corpus | Structured generation with shrinking |
| **Output** | Pass/fail (did it crash?) | Property assertions (is relationship preserved?) |
| **Shrinking** | Limited (coverage-guided corpus reduction) | Automatic minimal counterexample (testing/quick uses binary search-like reduction) |
| **Examples** | "Does this input panic?" | "Does f(f(x)) == f(x)?" (idempotency) |

**Example**: For `normalizePath()`, fuzzing finds inputs that crash. PBT verifies properties like:
- Idempotency: `normalize(normalize(p)) == normalize(p)`
- Monotonicity: `len(normalize(p)) <= len(p) + 1`
- Inverse relationship: `parse(normalize(p))` succeeds if p is valid

## Recommended Library: testing/quick

**Rationale**: Use Go's built-in `testing/quick` package because:
1. No external dependencies (part of standard library)
2. Integrates seamlessly with existing `testing` infrastructure
3. Automatic shrinking of counterexamples
4. Sufficient for our wrapper library's needs

**Alternative**: Consider `gopter` if you need:
- More sophisticated generators (e.g., generating valid SPIFFE IDs)
- Better shrinking strategies
- Property combinators

**Not Recommended**: `rapid` - requires Go 1.20+ and provides limited benefits over testing/quick for this library.

## Target Functions for PBT

Based on the codebase analysis, these functions are excellent PBT candidates:

### 1. normalizePath() - Path Normalization (HIGHEST PRIORITY)

**Location**: `internal/domain/identity_credential.go:normalizePath()`

**Why PBT?**: Already has comprehensive fuzz tests, PBT adds verification of mathematical properties.

**Properties to Test**:

#### Property 1: Idempotency
```go
// For all valid paths p: normalize(normalize(p)) == normalize(p)
func TestNormalizePath_Idempotency(t *testing.T) {
    property := func(p string) bool {
        // Skip invalid inputs that should panic
        if shouldPanic(p) {
            return true
        }

        normalized := normalizePath(p)
        renormalized := normalizePath(normalized)
        return normalized == renormalized
    }

    if err := quick.Check(property, nil); err != nil {
        t.Error(err)
    }
}
```

#### Property 2: Leading Slash Invariant
```go
// For all inputs p: normalize(p) starts with "/"
func TestNormalizePath_LeadingSlash(t *testing.T) {
    property := func(p string) bool {
        if shouldPanic(p) {
            return true
        }

        normalized := normalizePath(p)
        return strings.HasPrefix(normalized, "/")
    }

    if err := quick.Check(property, nil); err != nil {
        t.Error(err)
    }
}
```

#### Property 3: Length Bound
```go
// For all inputs p: len(normalize(p)) <= len(p) + 1
// (Only adds leading slash, never expands further)
func TestNormalizePath_LengthBound(t *testing.T) {
    property := func(p string) bool {
        if shouldPanic(p) {
            return true
        }

        normalized := normalizePath(p)
        return len(normalized) <= len(p) + 1
    }

    if err := quick.Check(property, nil); err != nil {
        t.Error(err)
    }
}
```

#### Property 4: Whitespace Rejection
```go
// For all inputs p: if p contains whitespace, normalize(p) panics
func TestNormalizePath_WhitespaceRejection(t *testing.T) {
    property := func(p string) bool {
        hasWhitespace := strings.IndexFunc(p, unicode.IsSpace) >= 0

        // Handle root cases (valid even if empty)
        if p == "" || p == "/" {
            return true
        }

        if hasWhitespace {
            // Should panic
            defer func() {
                if recover() == nil {
                    t.Errorf("Expected panic for whitespace in %q", p)
                }
            }()
            normalizePath(p)
            return false // Should not reach here
        }

        return true
    }

    if err := quick.Check(property, nil); err != nil {
        t.Error(err)
    }
}
```

### 2. parseDurationInto() - Duration Parsing with Defaults

**Location**: `internal/config/mtls_env.go:parseDurationInto()`

**Why PBT?**: Tests relationship between parsing and formatting.

**Properties to Test**:

#### Property 1: Parse-Format Roundtrip
```go
// For valid duration strings: parse(s) -> d -> format(d) produces equivalent duration
func TestParseDurationInto_Roundtrip(t *testing.T) {
    property := func(d time.Duration) bool {
        // Format duration
        formatted := d.String()

        // Parse it back
        var parsed time.Duration
        parseDurationInto(&parsed, formatted, "test")

        // Compare using String() to handle formatting differences
        // (e.g., "1h0m0s" vs "1h" both represent same duration)
        return parsed.String() == d.String()
    }

    if err := quick.Check(property, nil); err != nil {
        t.Error(err)
    }
}
```

#### Property 2: Default Fallback
```go
// For all inputs: if parsing fails, result equals default
func TestParseDurationInto_DefaultFallback(t *testing.T) {
    property := func(s string, defaultDur time.Duration) bool {
        result := defaultDur
        parseDurationInto(&result, s, "test")

        // Either successfully parsed or kept default
        _, parseErr := time.ParseDuration(s)
        if parseErr != nil {
            return result == defaultDur
        }
        return true
    }

    if err := quick.Check(property, nil); err != nil {
        t.Error(err)
    }
}
```

### 3. splitCleanDedup() - String List Processing

**Location**: `internal/config/mtls_env.go:splitCleanDedup()`

**Why PBT?**: Tests set properties (uniqueness, ordering).

**Properties to Test**:

#### Property 1: No Duplicates
```go
// For all inputs s: splitCleanDedup(s) contains no duplicates
func TestSplitCleanDedup_NoDuplicates(t *testing.T) {
    property := func(s string) bool {
        result := splitCleanDedup(s, ",")

        seen := make(map[string]bool)
        for _, item := range result {
            if seen[item] {
                return false // Found duplicate
            }
            seen[item] = true
        }
        return true
    }

    if err := quick.Check(property, nil); err != nil {
        t.Error(err)
    }
}
```

#### Property 2: Idempotency
```go
// For all inputs s: splitCleanDedup(join(splitCleanDedup(s))) == splitCleanDedup(s)
func TestSplitCleanDedup_Idempotency(t *testing.T) {
    property := func(s string) bool {
        first := splitCleanDedup(s, ",")
        rejoined := strings.Join(first, ",")
        second := splitCleanDedup(rejoined, ",")

        // Should be equal
        if len(first) != len(second) {
            return false
        }
        for i := range first {
            if first[i] != second[i] {
                return false
            }
        }
        return true
    }

    if err := quick.Check(property, nil); err != nil {
        t.Error(err)
    }
}
```

#### Property 3: Subset Preservation
```go
// For all inputs s: every element in result was in original (after cleaning)
func TestSplitCleanDedup_SubsetPreservation(t *testing.T) {
    property := func(s string) bool {
        result := splitCleanDedup(s, ",")

        // Build set of original elements (cleaned)
        original := make(map[string]bool)
        for _, item := range strings.Split(s, ",") {
            cleaned := strings.TrimSpace(item)
            if cleaned != "" {
                original[cleaned] = true
            }
        }

        // Every result element must be in original
        for _, item := range result {
            if !original[item] {
                return false
            }
        }
        return true
    }

    if err := quick.Check(property, nil); err != nil {
        t.Error(err)
    }
}
```

### 4. parseBool() - Boolean Parsing with Defaults

**Location**: `internal/debug/config.go:parseBool()`

**Why PBT?**: Tests boolean algebra properties.

**Properties to Test**:

#### Property 1: Negation Symmetry
```go
// For all valid bool strings s: parseBool("true") != parseBool("false")
func TestParseBool_NegationSymmetry(t *testing.T) {
    trueVal := parseBool("true", false, "test")
    falseVal := parseBool("false", true, "test")

    if trueVal == falseVal {
        t.Errorf("parseBool should distinguish true from false")
    }
}
```

#### Property 2: Default Fallback
```go
// For all invalid inputs: parseBool(invalid, d) == d
func TestParseBool_DefaultFallback(t *testing.T) {
    property := func(s string, defaultVal bool) bool {
        result := parseBool(s, defaultVal, "test")

        // Check if valid boolean string
        _, err := strconv.ParseBool(s)
        if err != nil {
            // Invalid - should return default
            return result == defaultVal
        }
        return true
    }

    if err := quick.Check(property, nil); err != nil {
        t.Error(err)
    }
}
```

### 5. IdentityCredential Operations - Domain Model

**Location**: `internal/domain/identity_credential.go`

**Why PBT?**: Tests domain invariants and value object properties.

**Properties to Test**:

#### Property 1: String Roundtrip (with Parser)
```go
// For all valid IDs: parse(id.String()) == id
// Note: Requires IdentityCredentialParser adapter
// This property should be tested in internal/adapters/outbound/spire/*_test.go
// to avoid coupling domain layer to adapter implementations.
// Documented here for completeness - implementation example:
//
// func TestIdentityCredentialParser_StringRoundtrip(t *testing.T) {
//     parser := NewIdentityCredentialParser()
//     property := func(td, path string) bool {
//         if td == "" || shouldPanic(path) {
//             return true
//         }
//         domain := NewTrustDomainFromName(td)
//         original := NewIdentityCredentialFromComponents(domain, path)
//         parsed, err := parser.Parse(original.String())
//         if err != nil {
//             t.Errorf("Failed to parse valid ID: %v", err)
//             return false
//         }
//         return parsed.String() == original.String()
//     }
//     if err := quick.Check(property, nil); err != nil {
//         t.Error(err)
//     }
// }
```

#### Property 2: Key Uniqueness
```go
// For all IDs: id1 == id2 <=> id1.Key() == id2.Key()
func TestIdentityCredential_KeyUniqueness(t *testing.T) {
    property := func(td1, td2, path1, path2 string) bool {
        // Skip invalid inputs
        if td1 == "" || td2 == "" {
            return true
        }
        if shouldPanic(path1) || shouldPanic(path2) {
            return true
        }

        domain1 := NewTrustDomainFromName(td1)
        domain2 := NewTrustDomainFromName(td2)
        id1 := NewIdentityCredentialFromComponents(domain1, path1)
        id2 := NewIdentityCredentialFromComponents(domain2, path2)

        // If keys equal, String() should equal
        if id1.Key() == id2.Key() {
            return id1.String() == id2.String()
        }
        return true
    }

    if err := quick.Check(property, nil); err != nil {
        t.Error(err)
    }
}
```

#### Property 3: Zero Value Safety
```go
// For all IDs: !id.IsZero() => id.String() is valid SPIFFE URI
func TestIdentityCredential_ZeroValueSafety(t *testing.T) {
    property := func(td, path string) bool {
        // Skip invalid inputs
        if td == "" {
            return true
        }
        if shouldPanic(path) {
            return true
        }

        domain := NewTrustDomainFromName(td)
        id := NewIdentityCredentialFromComponents(domain, path)

        if !id.IsZero() {
            uri := id.String()
            return strings.HasPrefix(uri, "spiffe://")
        }
        return true
    }

    if err := quick.Check(property, nil); err != nil {
        t.Error(err)
    }
}
```

## Implementation Steps

### Step 1: Create Property-Based Test Files

Create new test files alongside existing fuzz tests:

```bash
# For normalizePath properties
touch internal/domain/identity_credential_pbt_test.go

# For config parsing properties
touch internal/config/mtls_env_pbt_test.go

# For debug config properties
touch internal/debug/config_pbt_test.go
```

### Step 2: Import testing/quick

```go
import (
    "testing"
    "testing/quick"
)
```

### Step 3: Implement Properties Incrementally

Start with highest-value properties:

1. **normalizePath idempotency** - Critical invariant already tested by fuzz, PBT adds clarity
2. **splitCleanDedup no duplicates** - Core correctness property
3. **parseDurationInto roundtrip** - Ensures parse/format consistency

### Step 4: Configure testing/quick

Use `quick.Config` for more control:

```go
config := &quick.Config{
    MaxCount:      1000,  // Number of test cases (default: 100)
    MaxCountScale: 0,     // No scaling
    Rand:          nil,   // Use default random source (can set custom for reproducibility)
    Values:        nil,   // Custom generator function (nil = use default generators)
}

if err := quick.Check(property, config); err != nil {
    t.Error(err)
}
```

### Step 5: Handle Panicking Functions

For functions that panic on invalid input (strict validation), use this pattern:

```go
property := func(input string) bool {
    // Skip inputs that should panic
    if shouldPanic(input) {
        return true  // Property vacuously true for invalid inputs
    }

    result := functionUnderTest(input)

    // Check property on result
    return checkProperty(result)
}
```

**Rationale**: PBT is for testing properties of valid outputs, not testing crash safety (that's fuzzing's job).

### Step 6: Integrate with CI

Add PBT to existing test suite:

```bash
# Run all tests (unit + fuzz + PBT)
go test ./...

# Run only PBT tests
go test -run TestPBT ./...

# Run with verbose output
go test -v -run TestPBT ./...
```

No separate CI step needed - PBT tests are regular Go tests.

## Custom Generators (Advanced)

For complex types, implement `quick.Generator`:

```go
// Generate valid trust domain names
type ValidTrustDomain string

func (ValidTrustDomain) Generate(rand *rand.Rand, size int) reflect.Value {
    // Generate lowercase alphanumeric with dots
    const chars = "abcdefghijklmnopqrstuvwxyz0123456789."

    length := rand.Intn(size) + 1
    b := make([]byte, length)
    for i := range b {
        b[i] = chars[rand.Intn(len(chars))]
    }

    return reflect.ValueOf(ValidTrustDomain(string(b)))
}

// Use in property test
func TestWithCustomGenerator(t *testing.T) {
    property := func(td ValidTrustDomain) bool {
        domain := NewTrustDomainFromName(string(td))
        return domain != nil
    }

    if err := quick.Check(property, nil); err != nil {
        t.Error(err)
    }
}
```

## Relationship to Existing Tests

### PBT vs Fuzz Testing

**Use Fuzz Testing for**:
- Finding crashes, panics, and edge cases
- Testing with malformed/malicious input
- Exploring large input space (millions of executions)
- Finding bugs you didn't anticipate

**Use Property-Based Testing for**:
- Verifying mathematical properties (idempotency, associativity, etc.)
- Testing relationships between functions
- Documenting invariants as executable tests
- Getting minimal counterexamples when properties fail

**Example**: For `normalizePath()`:
- **Fuzz test**: "Does this random string cause a panic or infinite loop?"
- **PBT**: "Does normalize(normalize(x)) == normalize(x) for all valid x?"

Both are valuable and complementary.

### PBT vs Unit Testing

**Use Unit Testing for**:
- Concrete examples (documentation)
- Regression tests (specific bugs)
- Boundary conditions (empty string, nil, etc.)
- Happy path verification

**Use Property-Based Testing for**:
- General correctness (works for ALL inputs)
- Discovering edge cases
- Verifying algebraic laws
- Reducing test maintenance (one property = many examples)

**Example**: For `splitCleanDedup()`:
- **Unit test**: `splitCleanDedup("a,b,a", ",") == []string{"a", "b"}`
- **PBT**: "Result contains no duplicates for ANY input"

## Common Pitfalls

### Pitfall 1: Testing Implementation Details

**Bad**:
```go
// Tests implementation detail (how it works internally)
property := func(s string) bool {
    result := normalizePath(s)
    // Assumes implementation uses strings.ReplaceAll for normalization
    return strings.Count(result, "//") == 0
}
```

**Good**:
```go
// Tests observable behavior (mathematical property)
property := func(s string) bool {
    if shouldPanic(s) {
        return true
    }
    result := normalizePath(s)
    return normalizePath(result) == result  // Idempotency
}
```

### Pitfall 2: Tautological Properties

**Bad**:
```go
// Always true, tests nothing
property := func(s string) bool {
    result := normalizePath(s)
    return result == result
}
```

**Good**:
```go
// Non-trivial property
property := func(s string) bool {
    result := normalizePath(s)
    return len(result) >= 1  // Always at least "/"
}
```

### Pitfall 3: Ignoring Shrinking

When a property fails, `testing/quick` provides a minimal counterexample:

```
--- FAIL: TestNormalizePath_Idempotency (0.01s)
    quick: #42: failed on input: "\x00/"
```

**Don't ignore this!** The shrunk input "\x00/" is minimal and helps debug.

### Pitfall 4: Over-Constraining Generators

**Bad**:
```go
// Too constrained, misses edge cases
property := func(p string) bool {
    if len(p) > 10 || !isAlphanumeric(p) {
        return true  // Skip
    }
    // ... test
}
```

**Good**:
```go
// Let quick.Check generate anything, use shouldPanic helper
property := func(p string) bool {
    if shouldPanic(p) {
        return true
    }
    // ... test
}
```

## File Organization

Recommended structure:

```
internal/domain/
├── identity_credential.go           # Implementation
├── identity_credential_test.go      # Unit tests (examples)
├── identity_credential_fuzz_test.go # Fuzz tests (crash safety)
└── identity_credential_pbt_test.go  # Property tests (invariants)

internal/config/
├── mtls_env.go
├── mtls_env_test.go
├── mtls_env_fuzz_test.go
└── mtls_env_pbt_test.go
```

**Rationale**: Separate files by testing approach for clarity and maintainability.

## Example: Complete PBT Test File

```go
package domain

import (
    "strings"
    "testing"
    "testing/quick"
    "unicode"
)

// TestNormalizePath_Properties tests algebraic properties of normalizePath
func TestNormalizePath_Properties(t *testing.T) {
    t.Run("idempotency", func(t *testing.T) {
        property := func(p string) bool {
            if shouldPanic(p) {
                return true
            }

            normalized := normalizePath(p)
            renormalized := normalizePath(normalized)
            return normalized == renormalized
        }

        config := &quick.Config{MaxCount: 1000}
        if err := quick.Check(property, config); err != nil {
            t.Error(err)
        }
    })

    t.Run("leading_slash", func(t *testing.T) {
        property := func(p string) bool {
            if shouldPanic(p) {
                return true
            }

            normalized := normalizePath(p)
            return strings.HasPrefix(normalized, "/")
        }

        config := &quick.Config{MaxCount: 1000}
        if err := quick.Check(property, config); err != nil {
            t.Error(err)
        }
    })

    t.Run("length_bound", func(t *testing.T) {
        property := func(p string) bool {
            if shouldPanic(p) {
                return true
            }

            normalized := normalizePath(p)
            return len(normalized) <= len(p) + 1
        }

        config := &quick.Config{MaxCount: 1000}
        if err := quick.Check(property, config); err != nil {
            t.Error(err)
        }
    })

    t.Run("no_consecutive_slashes", func(t *testing.T) {
        property := func(p string) bool {
            if shouldPanic(p) {
                return true
            }

            normalized := normalizePath(p)
            return !strings.Contains(normalized, "//")
        }

        config := &quick.Config{MaxCount: 1000}
        if err := quick.Check(property, config); err != nil {
            t.Error(err)
        }
    })
}

// Helper function - reuse from fuzz test or extract to shared test util
func shouldPanic(path string) bool {
    if path == "" || path == "/" {
        return false
    }
    if idx := strings.IndexFunc(path, unicode.IsSpace); idx >= 0 {
        return true
    }
    p := path
    if !strings.HasPrefix(p, "/") {
        p = "/" + p
    }
    if strings.Contains(p, "//") {
        return true
    }
    if len(p) > 1 && strings.HasSuffix(p, "/") {
        return true
    }
    return hasTraversalSegments(p)
}

func hasTraversalSegments(path string) bool {
    segments := strings.Split(path, "/")
    for _, seg := range segments {
        if seg == "." || seg == ".." {
            return true
        }
    }
    return false
}
```

## Success Metrics

Track these metrics to measure PBT effectiveness:

1. **Property Count**: Aim for 2-5 properties per critical function
2. **Coverage**: Properties should cover all major invariants documented in INVARIANTS.md
3. **Shrinking Quality**: When failures occur, counterexamples should be minimal
4. **Execution Time**: PBT suite should complete in < 5 seconds (1000 cases/property)
5. **Defect Detection**: Track bugs found by PBT vs fuzz vs unit tests
6. **Failure Rate**: Monitor bugs found per quarter to track long-term value and effectiveness

## References

- [Go testing/quick documentation](https://pkg.go.dev/testing/quick)
- [QuickCheck: A Lightweight Tool for Random Testing of Haskell Programs](https://www.cse.chalmers.se/~rjmh/QuickCheck/manual.html) - Original PBT paper
- [Gopter (alternative Go PBT library)](https://pkg.go.dev/github.com/leanovate/gopter)
- [SPIFFE spec requirements](https://github.com/spiffe/spiffe/blob/main/standards/SPIFFE-ID.md)
- Internal: [INVARIANTS.md](../architecture/INVARIANTS.md) - System guarantees
- Internal: [TESTING.md](TESTING.md) - Overall testing strategy
