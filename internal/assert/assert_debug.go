//go:build debug

package assert

import "fmt"

// Invariant checks an invariant condition and panics if violated in debug builds.
// Invariants represent conditions that must always be true for the system to be correct.
// This includes postconditions (properties that must hold after function execution).
// Use this for internal sanity checks, not for validating external input.
//
// Examples:
//
//	// Structural invariant (always true for valid state)
//	assert.Invariant(user.ID != "", "user ID must never be empty after construction")
//
//	// Postcondition (property established by this function)
//	assert.Invariant(len(result) > 0, "normalization must not produce empty result")
func Invariant(ok bool, msg string) {
	if !ok {
		panic(fmt.Sprintf("INVARIANT VIOLATION: %s", msg))
	}
}
