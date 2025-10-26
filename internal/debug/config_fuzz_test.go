package debug

import (
	"strconv"
	"strings"
	"testing"
)

// FuzzParseBool fuzzes the parseBool function to ensure it handles
// arbitrary string inputs without panicking and returns consistent results.
//
// This tests the wrapper's custom boolean parsing with default fallback logic.
// The function wraps strconv.ParseBool and returns a default value for invalid
// inputs or empty strings, making it safer for environment variable parsing.
func FuzzParseBool(f *testing.F) {
	// Seed with diverse boolean strings and edge cases
	// Standard Go bool formats
	f.Add("true", true)
	f.Add("false", false)
	f.Add("TRUE", true)
	f.Add("FALSE", false)
	f.Add("1", true)
	f.Add("0", false)
	f.Add("t", true)
	f.Add("f", false)
	f.Add("T", true)
	f.Add("F", false)

	// Edge cases
	f.Add("", true)                         // Empty string
	f.Add("", false)                        // Empty string with different default
	f.Add("invalid", true)                  // Invalid string
	f.Add("yes", false)                     // Not standard (should return default)
	f.Add("no", true)                       // Not standard (should return default)
	f.Add("   true   ", false)              // Whitespace (strconv.ParseBool doesn't trim)
	f.Add("2", true)                        // Number other than 0/1

	// Additional diverse cases
	f.Add("TrUe", true)                     // Mixed case
	f.Add("vérifié", false)                 // Non-ASCII
	f.Add(strings.Repeat("true", 100), true) // Long string
	f.Add("TRUE\n", false)                  // Newline
	f.Add("\ttrue", true)                   // Tab prefix
	f.Add("true\x00", false)                // Null byte suffix

	f.Fuzz(func(t *testing.T, input string, defaultVal bool) {
		// Handle empty string case explicitly for clarity
		if input == "" {
			result := parseBool(input, defaultVal)
			// Invariant 1: Empty string must return default
			// Rationale: parseBool treats empty as "not set", returning default
			if result != defaultVal {
				t.Errorf("parseBool(%q, %v) = %v, expected default %v", input, defaultVal, result, defaultVal)
			}
			return
		}

		// Call parseBool - should never panic
		result := parseBool(input, defaultVal)

		// Invariant 2: For inputs parseable by strconv.ParseBool, return parsed value
		// Rationale: Wrapper should preserve standard Go bool parsing behavior
		// Note: strconv.ParseBool doesn't trim whitespace, so "  true  " is invalid
		stdResult, stdErr := strconv.ParseBool(input)
		if stdErr == nil {
			// Input is a valid bool string, our function should return the parsed value
			if result != stdResult {
				t.Errorf("parseBool(%q, %v) = %v, but strconv.ParseBool gives %v", input, defaultVal, result, stdResult)
			}
		} else {
			// Invariant 3: For inputs NOT parseable by strconv.ParseBool, return default
			// Rationale: Wrapper provides safe fallback for invalid inputs
			// This includes non-standard strings like "yes", "no", whitespace-wrapped, etc.
			if result != defaultVal {
				t.Errorf("parseBool(%q, %v) = %v, expected default %v for invalid input (strconv error: %v)",
					input, defaultVal, result, defaultVal, stdErr)
			}
		}

		// Note: Removed determinism check as parseBool is a pure function with no
		// side effects, making the check redundant and adding no testing value
	})
}
