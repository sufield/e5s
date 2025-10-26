package domain

import (
	mrand "math/rand"
	"os"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"testing/quick"
	"unicode"
)

// defaultPBTConfig returns standard config for property-based tests
func defaultPBTConfig() *quick.Config {
	// Use 10000 for higher confidence, configurable via env
	maxCount := 10000
	if v := os.Getenv("PBT_MAX_COUNT"); v != "" {
		// Allow override for faster local runs
		if n, err := parseInt(v); err == nil && n > 0 {
			maxCount = n
		}
	}

	return &quick.Config{
		MaxCount:      maxCount,
		MaxCountScale: 0,
		Rand:          nil,
		Values:        nil,
	}
}

func parseInt(s string) (int, error) {
	return strconv.Atoi(s)
}

// ValidPath generates valid normalized paths for focused testing
type ValidPath string

func (ValidPath) Generate(r *mrand.Rand, size int) reflect.Value {
	// Generate path with alphanumeric + / + :, no spaces/dots
	const chars = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789:-_"

	// Build path segments
	numSegments := r.Intn(size) + 1
	if numSegments > 10 {
		numSegments = 10
	}

	var path strings.Builder
	for i := 0; i < numSegments; i++ {
		// Add leading slash
		path.WriteString("/")

		// Generate segment (1-20 chars)
		segLen := r.Intn(20) + 1
		for j := 0; j < segLen; j++ {
			path.WriteByte(chars[r.Intn(len(chars))])
		}
	}

	result := path.String()
	if result == "" {
		result = "/" // Ensure root if empty
	}

	return reflect.ValueOf(ValidPath(result))
}

// Note: shouldPanic and hasTraversalSegments are imported from identity_credential_fuzz_test.go

// TestNormalizePath_Properties tests algebraic properties of normalizePath
// using property-based testing. These tests complement fuzz tests by verifying
// mathematical invariants hold for all valid inputs.
func TestNormalizePath_Properties(t *testing.T) {
	t.Parallel()

	t.Run("idempotency", func(t *testing.T) {
		t.Parallel()

		// Property: For all valid paths p, normalize(normalize(p)) == normalize(p)
		// This is the most critical property - normalized output should already be canonical
		// Uses ValidPath generator to focus on valid input space (fuzz tests cover invalid)
		property := func(vp ValidPath) bool {
			p := string(vp)

			// Explicit root case check for coverage
			if p == "/" {
				normalized := normalizePath(p)
				return normalized == "/"
			}

			normalized := normalizePath(p)
			renormalized := normalizePath(normalized)
			return normalized == renormalized
		}

		if err := quick.Check(property, defaultPBTConfig()); err != nil {
			t.Error(err)
		}
	})

	t.Run("canonical_form", func(t *testing.T) {
		t.Parallel()

		// Property: For all valid paths p, normalize(p) has canonical form:
		// - Starts with "/" (leading slash)
		// - No trailing slash (except root "/")
		// Combines leading_slash_invariant and no_trailing_slash for efficiency
		property := func(vp ValidPath) bool {
			p := string(vp)
			normalized := normalizePath(p)

			// Must start with "/"
			if !strings.HasPrefix(normalized, "/") {
				return false
			}

			// No trailing slash for non-root paths
			if len(normalized) > 1 && strings.HasSuffix(normalized, "/") {
				return false
			}

			return true
		}

		if err := quick.Check(property, defaultPBTConfig()); err != nil {
			t.Error(err)
		}
	})

	t.Run("exact_length", func(t *testing.T) {
		t.Parallel()

		// Property: For all valid paths p, normalizePath adds leading "/" only when needed
		// - If p starts with "/", len(normalized) == len(p)
		// - Otherwise, len(normalized) == len(p) + 1
		// Strengthens length_bound to exact cases
		property := func(vp ValidPath) bool {
			p := string(vp)
			normalized := normalizePath(p)

			if strings.HasPrefix(p, "/") {
				return len(normalized) == len(p)
			}
			return len(normalized) == len(p)+1
		}

		if err := quick.Check(property, defaultPBTConfig()); err != nil {
			t.Error(err)
		}
	})

	t.Run("no_consecutive_slashes", func(t *testing.T) {
		t.Parallel()

		// Property: For all valid paths p, normalize(p) contains no "//"
		// Validates strict normalization - no consecutive slashes in output
		// Uses Count == 0 for potential future multi-slash checks
		property := func(vp ValidPath) bool {
			p := string(vp)
			normalized := normalizePath(p)
			return strings.Count(normalized, "//") == 0
		}

		if err := quick.Check(property, defaultPBTConfig()); err != nil {
			t.Error(err)
		}
	})

	t.Run("no_whitespace", func(t *testing.T) {
		t.Parallel()

		// Property: For all valid paths p, normalize(p) contains no whitespace
		// SPIFFE spec requires no whitespace (RFC 3986 compliance)
		// Uses IndexFunc for short-circuit efficiency
		property := func(vp ValidPath) bool {
			p := string(vp)
			normalized := normalizePath(p)
			return strings.IndexFunc(normalized, unicode.IsSpace) < 0
		}

		if err := quick.Check(property, defaultPBTConfig()); err != nil {
			t.Error(err)
		}
	})

	t.Run("no_traversal_segments", func(t *testing.T) {
		t.Parallel()

		// Property: For all valid paths p, normalize(p) has no dot/dotdot segments
		// SPIFFE spec forbids path traversal
		property := func(vp ValidPath) bool {
			p := string(vp)
			normalized := normalizePath(p)
			return !hasTraversalSegments(normalized)
		}

		if err := quick.Check(property, defaultPBTConfig()); err != nil {
			t.Error(err)
		}
	})
}

