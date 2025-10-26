package domain

import (
	"strings"
	"testing"
	"unicode"
)

// shouldPanic determines if normalizePath should panic for the given input.
// This helper ensures consistency between test expectations and actual implementation.
//
// Panic conditions (strict validation):
//  1. Leading whitespace (any Unicode space character)
//  2. Trailing whitespace (any Unicode space character)
//  3. Internal whitespace (spaces, tabs, newlines, etc.)
//  4. Consecutive slashes ("//") - indicates non-normalized input
//  5. Trailing slash (except root "/") - indicates non-normalized input
//  6. Dot segments (".") - path traversal
//  7. Dotdot segments ("..") - path traversal
//
// Non-panic conditions:
//  1. Empty string → "/" (root identity)
//  2. Single slash "/" → "/" (root identity)
//  3. Valid path without leading slash → adds "/" (e.g., "foo" → "/foo")
//  4. Valid path with leading slash → unchanged
func shouldPanic(path string) bool {
	// Empty and root are valid
	if path == "" || path == "/" {
		return false
	}

	// Check whitespace BEFORE adding leading slash (matches implementation order)
	// This ensures we catch inputs like "\xa0" correctly
	if idx := strings.IndexFunc(path, unicode.IsSpace); idx >= 0 {
		return true
	}

	// Add leading slash for checking (convenience behavior)
	p := path
	if !strings.HasPrefix(p, "/") {
		p = "/" + p
	}

	// Check consecutive slashes (after adding leading slash if needed)
	if strings.Contains(p, "//") {
		return true
	}

	// Check trailing slash (except root)
	if len(p) > 1 && strings.HasSuffix(p, "/") {
		return true
	}

	// Check dot/dotdot segments
	if hasTraversalSegments(p) {
		return true
	}

	return false
}

// hasTraversalSegments checks if a path contains "." or ".." segments.
// Note: Matches exact behavior of normalizePath which checks segments without trimming
func hasTraversalSegments(path string) bool {
	segments := strings.Split(path, "/")
	for _, seg := range segments {
		// Check exact match - normalizePath doesn't trim individual segments
		if seg == "." || seg == ".." {
			return true
		}
	}
	return false
}

// FuzzNormalizePath fuzzes the normalizePath function to ensure it handles
// arbitrary string inputs with strict validation and maintains key invariants.
//
// Design Change (Strict Validation):
// Previously, normalizePath silently normalized invalid inputs (trimming whitespace,
// collapsing slashes, etc.). This created problems:
//  1. Duplicated go-spiffe SDK validation logic
//  2. Silent data corruption (e.g., "path " → "path")
//  3. Complex Unicode handling needed for idempotency
//
// New approach:
//  - Strict validation: PANIC on invalid inputs (whitespace, //, trailing /, etc.)
//  - Minimal normalization: Only add leading slash and handle root case
//  - Trust SDK: Production adapters use go-spiffe SDK for comprehensive validation
//  - Fail fast: Catch programmer errors in tests/domain logic before reaching SDK
//
// This tests the wrapper's strict path validation logic, which ensures paths
// are already in canonical form before constructing SPIFFE IDs. According to
// the SPIFFE spec (v1.0):
// - Paths must start with "/" (https://github.com/spiffe/spiffe/blob/main/standards/SPIFFE-ID.md#22-path)
// - Path segments cannot be "." or ".." (no traversal)
// - Paths must not contain whitespace (RFC 3986 compliance)
// - Paths should be in canonical form (no //, no trailing /)
//
// Spec reference: SPIFFE ID Standard v1.0
// https://github.com/spiffe/spiffe/blob/main/standards/SPIFFE-ID.md
func FuzzNormalizePath(f *testing.F) {
	// Seed with diverse inputs covering valid, invalid (panics), and edge cases

	// ===== VALID PATHS (no panic) =====
	f.Add("")                              // Empty path → "/" (root identity)
	f.Add("/")                             // Root path
	f.Add("/workload")                     // Valid simple path
	f.Add("/service/account")              // Valid nested path
	f.Add("/ns/spire/sa/default")          // Kubernetes-style path
	f.Add("/colon:allowed")                // Colons are valid per RFC 3986
	f.Add("no-leading-slash")              // Missing leading slash (convenience: adds /)
	f.Add("workload/server")               // Relative path (convenience: adds /)
	f.Add("/db:rw/user")                   // Colons in segments

	// ===== INVALID PATHS (should panic) =====
	// Whitespace variations (STRICT: no silent trimming)
	f.Add("   /whitespace   ")             // Leading/trailing whitespace → PANIC
	f.Add(" /path")                        // Leading space → PANIC
	f.Add("/path ")                        // Trailing space → PANIC
	f.Add("/path with spaces")             // Internal spaces → PANIC
	f.Add("/tab\t/path")                   // Tab characters → PANIC
	f.Add("/newline\n/path")               // Newline characters → PANIC
	f.Add("/formfeed\f")                   // Form feed → PANIC
	f.Add("/path / space")                 // Space in segment → PANIC

	// Slash normalization (STRICT: must be pre-normalized)
	f.Add("//invalid//path")               // Multiple slashes → PANIC
	f.Add("/path/")                        // Trailing slash → PANIC
	f.Add("//foo")                         // Leading double slash → PANIC
	f.Add("/foo//bar")                     // Internal double slash → PANIC

	// Traversal attempts (STRICT: reject dot segments)
	f.Add("/path/../traversal")            // Path traversal attempt → PANIC
	f.Add(".")                             // Single dot → PANIC (after adding /)
	f.Add("..")                            // Double dot → PANIC (after adding /)
	f.Add("./path")                        // Dot prefix → PANIC (becomes /./path)
	f.Add("/./path")                       // Dot segment → PANIC
	f.Add("/../foo")                       // Dotdot at start → PANIC

	// Special characters (some valid, some invalid per SPIFFE)
	f.Add("/path?query")                   // Query string (? allowed but unusual)
	f.Add("/path#fragment")                // Fragment (# allowed but unusual)
	f.Add("/special%20chars")              // URL-encoded (valid)

	// Unicode and encoding
	f.Add("/路径")                           // Unicode Chinese (valid UTF-8)
	f.Add("/パス")                           // Unicode Japanese (valid UTF-8)
	f.Add("\x80/invalid-utf8")             // Invalid UTF-8 (may panic on whitespace check)

	// Size variations
	f.Add("/" + strings.Repeat("a", 1024)) // Long path (valid)
	f.Add("/" + strings.Repeat("seg/", 100)) // Many segments (valid)

	f.Fuzz(func(t *testing.T, path string) {
		// normalizePath panics on invalid inputs (strict validation)
		// Catch panics and validate they're expected
		defer func() {
			if r := recover(); r != nil {
				// Use helper to determine if panic was expected
				if !shouldPanic(path) {
					t.Errorf("unexpected panic for input %q: %v", path, r)
				}
				// Expected panic - test passes
			}
		}()

		// Call normalizePath (may panic if input is invalid)
		normalized := normalizePath(path)

		// If we reach here, no panic occurred - verify input was valid
		if shouldPanic(path) {
			t.Errorf("normalizePath should have panicked for invalid input %q but returned %q", path, normalized)
		}

		// Invariant 1: Result must never be empty
		// Rationale: SPIFFE paths must be at least "/" (root identity)
		if normalized == "" {
			t.Errorf("normalizePath returned empty string for input %q", path)
		}

		// Invariant 2: Result must start with "/"
		// Rationale: SPIFFE spec requires paths to start with "/"
		// (https://github.com/spiffe/spiffe/blob/main/standards/SPIFFE-ID.md#22-path)
		if !strings.HasPrefix(normalized, "/") {
			t.Errorf("normalizePath result %q does not start with / for input %q", normalized, path)
		}

		// Invariant 3: No consecutive slashes
		// Rationale: Strict validation - inputs must be pre-normalized
		if strings.Contains(normalized, "//") {
			t.Errorf("normalizePath result %q contains consecutive slashes for input %q", normalized, path)
		}

		// Invariant 4: No trailing slash unless it's the root "/"
		// Rationale: Strict validation - inputs must be pre-normalized
		if len(normalized) > 1 && strings.HasSuffix(normalized, "/") {
			t.Errorf("normalizePath result %q has trailing slash for input %q", normalized, path)
		}

		// Invariant 5: Result length should not exceed input + 1
		// Rationale: Only adds at most one leading slash, no other expansion
		if len(normalized) > len(path)+1 {
			t.Errorf("normalizePath result too long: got len=%d from input len=%d (expected <= %d)",
				len(normalized), len(path), len(path)+1)
		}

		// Invariant 6: No traversal segments in normalized output
		// Rationale: Strict validation rejects dot/dotdot segments
		if hasTraversalSegments(normalized) {
			t.Errorf("normalizePath result %q contains traversal segments (. or ..) for input %q",
				normalized, path)
		}

		// Invariant 7: No whitespace in normalized output
		// Rationale: Strict validation rejects whitespace (SPIFFE spec compliance)
		for _, r := range normalized {
			if unicode.IsSpace(r) {
				t.Errorf("normalizePath result %q contains whitespace for input %q", normalized, path)
			}
		}

		// Invariant 8: Calling normalizePath again should be idempotent
		// Rationale: Normalized output should already be in canonical form,
		// so re-normalizing should produce identical output
		renormalized := normalizePath(normalized)
		if renormalized != normalized {
			t.Errorf("normalizePath is not idempotent: %q -> %q -> %q", path, normalized, renormalized)
		}
	})
}
