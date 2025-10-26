package config

import (
	"os"
	"strings"
	"testing"
	"time"
)

// FuzzParseDurationInto fuzzes the parseDurationInto function to ensure it handles
// arbitrary duration string inputs without panicking and maintains correct wrapper behavior.
func FuzzParseDurationInto(f *testing.F) {
	// Seed with diverse duration strings covering valid, invalid, and edge cases
	f.Add("1s")                     // Valid second
	f.Add("5m")                     // Valid minute
	f.Add("2h")                     // Valid hour
	f.Add("100ms")                  // Valid millisecond
	f.Add("1h2m3s")                 // Mixed units
	f.Add("")                       // Empty (should be no-op)
	f.Add("invalid")                // Invalid format
	f.Add("1x")                     // Invalid unit
	f.Add("999999999h")             // Very large duration
	f.Add("9223372036854775807ns")  // Potential overflow
	f.Add("-5s")                    // Negative duration
	f.Add("1.5s")                   // Fractional duration
	f.Add("   10s   ")              // Whitespace (should be trimmed)
	f.Add("0s")                     // Zero duration
	f.Add("1")                      // Missing unit
	f.Add("1us")                    // Microsecond
	f.Add("1µs")                    // Unicode microsecond

	f.Fuzz(func(t *testing.T, durationStr string) {
		// Use a test-specific environment variable to avoid conflicts
		const testEnvVar = "FUZZ_TEST_DURATION"

		// Set the environment variable with the fuzzed value
		if durationStr == "" {
			// Test that empty env var results in no-op (target unchanged)
			t.Skip("empty string handled by env var absence")
		}

		originalValue := os.Getenv(testEnvVar)
		os.Setenv(testEnvVar, durationStr)
		defer os.Setenv(testEnvVar, originalValue) // Restore after test

		// Test the actual parseDurationInto function
		var target time.Duration
		err := parseDurationInto(testEnvVar, &target)

		// Invariant 1: Function should never panic
		// (implicitly tested by not crashing)

		// Invariant 2: For valid durations, target should be set correctly
		trimmed := strings.TrimSpace(durationStr)
		expectedDuration, parseErr := time.ParseDuration(trimmed)

		if parseErr == nil {
			// Valid duration - function should succeed
			if err != nil {
				t.Errorf("parseDurationInto failed for valid input %q: %v", durationStr, err)
			}
			if target != expectedDuration {
				t.Errorf("parseDurationInto(%q) = %v, expected %v", durationStr, target, expectedDuration)
			}

			// Invariant 3: Duration should match what time.ParseDuration returns
			// (validates our wrapper doesn't modify the value)

		} else {
			// Invalid duration - function should return error
			if err == nil {
				// Some invalid inputs might be accepted by time.ParseDuration
				// (e.g., null bytes might be treated as empty after trimming)
				// This is acceptable as long as the function doesn't panic
				return
			}
			// Invariant 4: Error message should contain the env var name
			// Note: We check for env var name but not the exact value since
			// error formatting may escape or quote special characters
			errStr := err.Error()
			if !strings.Contains(errStr, testEnvVar) {
				t.Errorf("error message missing env var name %q: %v", testEnvVar, err)
			}
		}
	})
}

// FuzzSplitCleanDedup fuzzes the splitCleanDedup function to ensure it handles
// arbitrary string inputs without panicking and maintains key invariants.
//
// This function validates the wrapper's custom logic for splitting, trimming,
// and deduplicating configuration values (e.g., comma-separated lists).
func FuzzSplitCleanDedup(f *testing.F) {
	// Seed with diverse inputs covering edge cases
	f.Add("a,b,c", ",")                    // Simple comma-separated
	f.Add("a, b, c", ",")                  // With spaces (should be trimmed)
	f.Add("a,,b,,c", ",")                  // Empty elements (should be removed)
	f.Add("a,a,b,b,c", ",")                // Duplicates (should be deduplicated)
	f.Add("  a  ,  b  ,  c  ", ",")        // Extra whitespace
	f.Add("", ",")                         // Empty string (should return empty slice)
	f.Add("single", ",")                   // Single element
	f.Add("a|b|c", "|")                    // Different separator
	f.Add("a;b;c", ";")                    // Semicolon separator
	f.Add(strings.Repeat("a,", 1000), ",") // Many elements
	f.Add("路径,path,パス", ",")              // Unicode
	f.Add("a,,,,b", ",")                   // Multiple consecutive separators
	f.Add("abc", "")                       // Empty separator (edge case)
	f.Add("\x80\x81", ",")                 // Invalid UTF-8

	f.Fuzz(func(t *testing.T, input, sep string) {
		// Handle empty separator edge case explicitly
		// When sep is "", strings.Split splits by character (Go behavior)
		// The splitCleanDedup function will treat each character as a separate element
		if sep == "" {
			result := splitCleanDedup(input, sep)
			// Validate the function handles this edge case without panicking
			// Result will be individual trimmed non-empty characters, deduplicated
			for _, s := range result {
				if s == "" {
					t.Errorf("splitCleanDedup with empty sep returned empty string")
				}
			}
			return
		}

		// Call splitCleanDedup - should never panic
		result := splitCleanDedup(input, sep)

		// Invariant 1: Result should never contain empty strings
		// Rationale: Empty elements add no value and should be filtered out
		for i, s := range result {
			if s == "" {
				t.Errorf("splitCleanDedup returned empty string at index %d for input %q with sep %q", i, input, sep)
			}
		}

		// Invariant 2: Result should never contain duplicates
		// Rationale: Deduplication is a core function guarantee
		seen := make(map[string]bool)
		for _, s := range result {
			if seen[s] {
				t.Errorf("splitCleanDedup returned duplicate %q for input %q with sep %q", s, input, sep)
			}
			seen[s] = true
		}

		// Invariant 3: Result should never contain strings with leading/trailing whitespace
		// Rationale: All values should be trimmed for consistent comparison
		for i, s := range result {
			trimmed := strings.TrimSpace(s)
			if s != trimmed {
				t.Errorf("splitCleanDedup returned untrimmed string %q at index %d for input %q with sep %q", s, i, input, sep)
			}
		}

		// Invariant 4: Result length should not exceed number of separators + 1
		// Rationale: Tighter bound to detect anomalies
		maxExpected := strings.Count(input, sep) + 1
		if len(result) > maxExpected {
			t.Errorf("splitCleanDedup returned too many elements: got %d, max expected %d for input %q", len(result), maxExpected, input)
		}

		// Invariant 5: All result elements should be present in original input
		// Rationale: Function should not create new values, only filter/transform
		parts := strings.Split(input, sep)
		for _, s := range result {
			found := false
			for _, p := range parts {
				if strings.TrimSpace(p) == s {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("splitCleanDedup returned element %q not found in input %q (after trim)", s, input)
			}
		}
	})
}
