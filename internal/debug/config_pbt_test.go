package debug

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	mrand "math/rand"
	"os"
	"reflect"
	"strconv"
	"testing"
	"testing/quick"
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

// ValidBoolString generates valid boolean strings for focused testing
type ValidBoolString string

func (ValidBoolString) Generate(r *mrand.Rand, size int) reflect.Value {
	// Valid bool strings per strconv.ParseBool
	validStrings := []string{
		"1", "t", "T", "TRUE", "true", "True",
		"0", "f", "F", "FALSE", "false", "False",
	}
	idx := r.Intn(len(validStrings))
	return reflect.ValueOf(ValidBoolString(validStrings[idx]))
}

// TestParseBool_Properties tests algebraic properties of parseBool
// using property-based testing. These tests verify boolean algebra properties.
func TestParseBool_Properties(t *testing.T) {
	t.Parallel()

	t.Run("default_fallback", func(t *testing.T) {
		t.Parallel()

		// Property: For all inputs, parseBool returns default on error, correct value on success
		// Verifies both error handling AND correct parsing
		property := func(s string, defaultVal bool) bool {
			result := parseBool(s, defaultVal)

			// Check if valid boolean string
			stdResult, err := strconv.ParseBool(s)
			if err != nil {
				// Invalid - should return default
				return result == defaultVal
			}
			// Valid - should return parsed value
			return result == stdResult
		}

		if err := quick.Check(property, defaultPBTConfig()); err != nil {
			t.Error(err)
		}
	})

	t.Run("empty_string_returns_default", func(t *testing.T) {
		t.Parallel()

		// Property: parseBool("", d) always returns d
		// Verifies special handling of empty string
		property := func(defaultVal bool) bool {
			result := parseBool("", defaultVal)
			return result == defaultVal
		}

		if err := quick.Check(property, defaultPBTConfig()); err != nil {
			t.Error(err)
		}
	})

	t.Run("true_equivalents_return_true", func(t *testing.T) {
		t.Parallel()

		// Property: All true equivalents return true regardless of default
		// Tests: "1", "t", "T", "TRUE", "true", "True"
		property := func(defaultVal bool) bool {
			trueStrings := []string{"1", "t", "T", "TRUE", "true", "True"}
			for _, s := range trueStrings {
				result := parseBool(s, defaultVal)
				if result != true {
					return false
				}
			}
			return true
		}

		if err := quick.Check(property, defaultPBTConfig()); err != nil {
			t.Error(err)
		}
	})

	t.Run("false_equivalents_return_false", func(t *testing.T) {
		t.Parallel()

		// Property: All false equivalents return false regardless of default
		// Tests: "0", "f", "F", "FALSE", "false", "False"
		property := func(defaultVal bool) bool {
			falseStrings := []string{"0", "f", "F", "FALSE", "false", "False"}
			for _, s := range falseStrings {
				result := parseBool(s, defaultVal)
				if result != false {
					return false
				}
			}
			return true
		}

		if err := quick.Check(property, defaultPBTConfig()); err != nil {
			t.Error(err)
		}
	})

	t.Run("valid_inputs_ignore_default", func(t *testing.T) {
		t.Parallel()

		// Property: For valid bool strings, result doesn't depend on default
		// Uses custom generator to focus only on valid inputs
		property := func(vbs ValidBoolString) bool {
			s := string(vbs)

			// Parse with both defaults
			resultTrue := parseBool(s, true)
			resultFalse := parseBool(s, false)

			// Should get same result regardless of default
			if resultTrue != resultFalse {
				return false
			}

			// Also verify it matches stdlib
			expected, err := strconv.ParseBool(s)
			if err != nil {
				// Should never happen with ValidBoolString
				return false
			}

			return resultTrue == expected
		}

		if err := quick.Check(property, defaultPBTConfig()); err != nil {
			t.Error(err)
		}
	})
}

// TestGetEnvOrDefault_Properties tests algebraic properties of getEnvOrDefault
func TestGetEnvOrDefault_Properties(t *testing.T) {
	t.Parallel()

	t.Run("determinism", func(t *testing.T) {
		t.Parallel()

		// Property: Calling getEnvOrDefault twice with same args produces same result
		// Uses unique keys per iteration to avoid env interference
		property := func(r uint64, defaultVal string) bool {
			// Generate unique key for isolation
			key := fmt.Sprintf("TEST_PBT_%d", r)

			result1 := getEnvOrDefault(key, defaultVal)
			result2 := getEnvOrDefault(key, defaultVal)
			return result1 == result2
		}

		if err := quick.Check(property, defaultPBTConfig()); err != nil {
			t.Error(err)
		}
	})

	t.Run("non_empty_result", func(t *testing.T) {
		t.Parallel()

		// Property: If default is non-empty AND env unset or non-empty, result is non-empty
		// Verifies we never lose the default value
		property := func(r uint64, defaultVal string) bool {
			if defaultVal == "" {
				return true // Skip empty defaults
			}

			// Use unique key to ensure it's not set
			key := fmt.Sprintf("TEST_PBT_NONEMPTY_%d", r)

			result := getEnvOrDefault(key, defaultVal)

			// If env is set to empty, result could be empty
			// Otherwise, with non-empty default, result should be non-empty
			if os.Getenv(key) == "" && defaultVal != "" {
				return result != ""
			}
			return true
		}

		if err := quick.Check(property, defaultPBTConfig()); err != nil {
			t.Error(err)
		}
	})

	t.Run("default_fallback_when_unset", func(t *testing.T) {
		t.Parallel()

		// Property: For unset keys, returns default
		// Uses dynamic unlikely keys to minimize collision risk
		property := func(defaultVal string) bool {
			// Generate random bytes for unique key
			randBytes := make([]byte, 8)
			if _, err := rand.Read(randBytes); err != nil {
				return true // Skip on rand error
			}
			unlikelyKey := fmt.Sprintf("SPIRE_PBT_UNSET_%s", hex.EncodeToString(randBytes))

			result := getEnvOrDefault(unlikelyKey, defaultVal)
			return result == defaultVal
		}

		if err := quick.Check(property, defaultPBTConfig()); err != nil {
			t.Error(err)
		}
	})

	t.Run("env_override", func(t *testing.T) {
		// Note: Not parallel - modifies environment
		// Property: When env is set to non-empty value, returns env value not default
		property := func(r uint64, envVal, defaultVal string) bool {
			// Skip empty env values - getEnvOrDefault returns default for empty
			if envVal == "" {
				return true
			}

			// Use unique key for isolation
			key := fmt.Sprintf("TEST_PBT_OVERRIDE_%d", r)

			// Set env var
			os.Setenv(key, envVal)
			defer os.Unsetenv(key)

			result := getEnvOrDefault(key, defaultVal)
			return result == envVal
		}

		if err := quick.Check(property, defaultPBTConfig()); err != nil {
			t.Error(err)
		}
	})
}

// TestInit_ModeNormalization tests that unknown mode strings are normalized to "debug"
func TestInit_ModeNormalization(t *testing.T) {
	tests := []struct {
		name         string
		envMode      string
		expectedMode string
	}{
		{"valid debug", "debug", "debug"},
		{"valid staging", "staging", "staging"},
		{"valid production", "production", "production"},
		{"invalid typo", "prodution", "debug"},
		{"invalid garbage", "wat", "debug"},
		{"empty string", "", "debug"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Save old env to restore after subtest
			oldDebug := os.Getenv("SPIRE_DEBUG")
			oldServer := os.Getenv("SPIRE_DEBUG_SERVER")
			oldMode := os.Getenv("SPIRE_DEBUG_MODE")

			// Restore after subtest
			defer func() {
				if oldDebug == "" {
					os.Unsetenv("SPIRE_DEBUG")
				} else {
					os.Setenv("SPIRE_DEBUG", oldDebug)
				}

				if oldServer == "" {
					os.Unsetenv("SPIRE_DEBUG_SERVER")
				} else {
					os.Setenv("SPIRE_DEBUG_SERVER", oldServer)
				}

				if oldMode == "" {
					os.Unsetenv("SPIRE_DEBUG_MODE")
				} else {
					os.Setenv("SPIRE_DEBUG_MODE", oldMode)
				}
			}()

			// Apply this subtest's env
			if tt.envMode == "" {
				os.Unsetenv("SPIRE_DEBUG_MODE")
			} else {
				os.Setenv("SPIRE_DEBUG_MODE", tt.envMode)
			}

			os.Unsetenv("SPIRE_DEBUG")
			os.Unsetenv("SPIRE_DEBUG_SERVER")

			Init()

			if Active.Mode != tt.expectedMode {
				t.Errorf("expected mode %q, got %q", tt.expectedMode, Active.Mode)
			}
		})
	}
}
