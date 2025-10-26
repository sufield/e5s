package config

import (
	"math/rand"
	"os"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"testing/quick"
	"time"
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

// PositiveDuration generates only positive durations for focused testing
type PositiveDuration time.Duration

func (PositiveDuration) Generate(r *rand.Rand, size int) reflect.Value {
	// Generate positive duration up to 24 hours
	maxNanos := int64(time.Hour * 24)
	d := time.Duration(r.Int63n(maxNanos))
	return reflect.ValueOf(PositiveDuration(d))
}

// TestSplitCleanDedup_Properties tests algebraic properties of splitCleanDedup
// using property-based testing. These tests verify set-theoretic properties.
func TestSplitCleanDedup_Properties(t *testing.T) {
	t.Parallel()

	t.Run("no_duplicates", func(t *testing.T) {
		t.Parallel()

		// Property: For all inputs s, splitCleanDedup(s) contains no duplicate elements
		// This is the core correctness property - verifies deduplication works
		property := func(s string) bool {
			result := splitCleanDedup(s, ",")

			// Check for empty result when input is all empty/whitespace
			allEmptyOrWhitespace := true
			for _, item := range strings.Split(s, ",") {
				if strings.TrimSpace(item) != "" {
					allEmptyOrWhitespace = false
					break
				}
			}
			if allEmptyOrWhitespace && len(result) != 0 {
				return false
			}

			seen := make(map[string]bool)
			for _, item := range result {
				if seen[item] {
					return false // Found duplicate
				}
				seen[item] = true
			}
			return true
		}

		if err := quick.Check(property, defaultPBTConfig()); err != nil {
			t.Error(err)
		}
	})

	t.Run("idempotency", func(t *testing.T) {
		t.Parallel()

		// Property: For all inputs s, splitCleanDedup(join(splitCleanDedup(s))) == splitCleanDedup(s)
		// Verifies that processing twice produces same result
		property := func(s string) bool {
			first := splitCleanDedup(s, ",")
			rejoined := strings.Join(first, ",")
			second := splitCleanDedup(rejoined, ",")

			// Use reflect.DeepEqual for slice comparison
			return reflect.DeepEqual(first, second)
		}

		if err := quick.Check(property, defaultPBTConfig()); err != nil {
			t.Error(err)
		}
	})

	t.Run("subset_preservation", func(t *testing.T) {
		t.Parallel()

		// Property: For all inputs s, every element in result was in original (after cleaning)
		// Verifies no elements are added, only removed or deduped
		property := func(s string) bool {
			// Skip if input is empty or all whitespace (trivial case)
			if strings.TrimSpace(s) == "" {
				return true
			}

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

		if err := quick.Check(property, defaultPBTConfig()); err != nil {
			t.Error(err)
		}
	})

	t.Run("no_invalid_elements", func(t *testing.T) {
		t.Parallel()

		// Property: For all inputs s, splitCleanDedup(s) contains no empty or whitespace-only strings
		// Combines verification of empty removal and whitespace trimming
		property := func(s string) bool {
			result := splitCleanDedup(s, ",")

			for _, item := range result {
				// Check both empty and whitespace-only
				if item == "" || strings.TrimSpace(item) == "" {
					return false
				}
				// Verify no leading/trailing whitespace (proper trimming)
				if item != strings.TrimSpace(item) {
					return false
				}
			}
			return true
		}

		if err := quick.Check(property, defaultPBTConfig()); err != nil {
			t.Error(err)
		}
	})

	t.Run("order_preservation", func(t *testing.T) {
		t.Parallel()

		// Property: For all inputs s, splitCleanDedup preserves first occurrence order
		// Verifies deduplication keeps first appearance, not last
		property := func(s string) bool {
			// Build expected order from first occurrences
			seen := make(map[string]bool)
			expected := make([]string, 0)
			for _, item := range strings.Split(s, ",") {
				cleaned := strings.TrimSpace(item)
				if cleaned == "" {
					continue
				}
				if !seen[cleaned] {
					seen[cleaned] = true
					expected = append(expected, cleaned)
				}
			}

			result := splitCleanDedup(s, ",")

			// Assert result length <= expected (dedup reduces size)
			if len(result) > len(expected) {
				return false
			}

			// Should match expected order using DeepEqual
			return reflect.DeepEqual(result, expected)
		}

		if err := quick.Check(property, defaultPBTConfig()); err != nil {
			t.Error(err)
		}
	})
}

// TestParseDurationInto_Properties tests algebraic properties of duration parsing
func TestParseDurationInto_Properties(t *testing.T) {
	t.Parallel()

	t.Run("roundtrip", func(t *testing.T) {
		t.Parallel()

		// Property: For valid durations d, format(d) can be parsed back to equivalent duration
		// Tests the actual parseDurationInto wrapper function
		property := func(d time.Duration) bool {
			// Format duration
			formatted := d.String()

			// Parse it back using parseDurationInto
			// Set env var temporarily for the test
			testEnvVar := "TEST_DURATION_ROUNDTRIP"
			oldVal := os.Getenv(testEnvVar)
			os.Setenv(testEnvVar, formatted)
			defer func() {
				if oldVal == "" {
					os.Unsetenv(testEnvVar)
				} else {
					os.Setenv(testEnvVar, oldVal)
				}
			}()

			var parsed time.Duration
			if err := parseDurationInto(testEnvVar, &parsed); err != nil {
				// Should never happen for valid duration strings
				return false
			}

			// Compare using String() to handle formatting differences
			return parsed.String() == d.String()
		}

		if err := quick.Check(property, defaultPBTConfig()); err != nil {
			t.Error(err)
		}
	})

	t.Run("parse_equivalence", func(t *testing.T) {
		t.Parallel()

		// Property: Calling parseDurationInto twice with same input produces same result
		// Tests the actual wrapper function for determinism
		property := func(s string) bool {
			testEnvVar := "TEST_DURATION_EQUIV"

			// Set env var
			os.Setenv(testEnvVar, s)
			defer os.Unsetenv(testEnvVar)

			var parsed1, parsed2 time.Duration
			defaultDur := time.Second * 30 // Use a non-zero default

			parsed1 = defaultDur
			err1 := parseDurationInto(testEnvVar, &parsed1)

			parsed2 = defaultDur
			err2 := parseDurationInto(testEnvVar, &parsed2)

			// Both should succeed or both should fail
			if (err1 == nil) != (err2 == nil) {
				return false
			}

			// Results should match
			return parsed1 == parsed2
		}

		if err := quick.Check(property, defaultPBTConfig()); err != nil {
			t.Error(err)
		}
	})

	t.Run("non_negative_for_positive_durations", func(t *testing.T) {
		t.Parallel()

		// Property: Parsing positive duration strings produces non-negative durations
		// Uses custom generator to focus only on positive durations
		property := func(pd PositiveDuration) bool {
			d := time.Duration(pd)

			formatted := d.String()

			// Use parseDurationInto
			testEnvVar := "TEST_POSITIVE_DURATION"
			os.Setenv(testEnvVar, formatted)
			defer os.Unsetenv(testEnvVar)

			var parsed time.Duration
			if err := parseDurationInto(testEnvVar, &parsed); err != nil {
				return false
			}

			return parsed >= 0
		}

		if err := quick.Check(property, defaultPBTConfig()); err != nil {
			t.Error(err)
		}
	})
}
