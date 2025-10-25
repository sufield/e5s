//go:build !debug

package assert

// Invariant is a no-op in production builds.
// Invariant checks are stripped to avoid runtime overhead.
func Invariant(ok bool, msg string) {
	// No-op in production
}
