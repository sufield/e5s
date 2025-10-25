//go:build debug

package assert

import (
	"testing"
)

// assertPanic is a test helper that verifies a function panics with expected message
func assertPanic(t *testing.T, f func(), expectedMsg string) {
	t.Helper()

	defer func() {
		r := recover()
		if r == nil {
			t.Fatalf("Expected panic with message %q, but function did not panic", expectedMsg)
		}

		msg, ok := r.(string)
		if !ok {
			t.Fatalf("Expected string panic, got %T: %v", r, r)
		}

		if msg != expectedMsg {
			t.Fatalf("Expected panic message %q, got %q", expectedMsg, msg)
		}
	}()

	f()
}

// assertNoPanic is a test helper that verifies a function does not panic
func assertNoPanic(t *testing.T, f func()) {
	t.Helper()

	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("Expected no panic, but got: %v", r)
		}
	}()

	f()
}

func TestInvariant(t *testing.T) {
	t.Run("should not panic when condition is true", func(t *testing.T) {
		t.Parallel()
		assertNoPanic(t, func() {
			Invariant(true, "this should not panic")
		})
	})

	t.Run("should panic when condition is false", func(t *testing.T) {
		t.Parallel()
		assertPanic(t, func() {
			Invariant(false, "test invariant")
		}, "INVARIANT VIOLATION: test invariant")
	})

	t.Run("should include custom message in panic", func(t *testing.T) {
		t.Parallel()
		customMsg := "custom error message with details"
		assertPanic(t, func() {
			Invariant(false, customMsg)
		}, "INVARIANT VIOLATION: "+customMsg)
	})
}

func TestInvariant_EdgeCases(t *testing.T) {
	t.Run("empty message", func(t *testing.T) {
		t.Parallel()
		assertPanic(t, func() {
			Invariant(false, "")
		}, "INVARIANT VIOLATION: ")
	})

	t.Run("message with special characters", func(t *testing.T) {
		t.Parallel()
		msg := "error: field 'name' must not be empty (got \"\")"
		assertPanic(t, func() {
			Invariant(false, msg)
		}, "INVARIANT VIOLATION: "+msg)
	})

	t.Run("multiline message", func(t *testing.T) {
		t.Parallel()
		msg := "first line\nsecond line"
		assertPanic(t, func() {
			Invariant(false, msg)
		}, "INVARIANT VIOLATION: "+msg)
	})
}
