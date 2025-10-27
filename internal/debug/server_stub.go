//go:build !debug

package debug

// Start is a no-op in production builds.
// The introspector parameter is accepted to match the debug build signature,
// but is never used since no debug endpoints are exposed in production.
func Start(introspector Introspector) {
	// Intentionally ignore introspector parameter
	_ = introspector
	// Debug server disabled in production build
}
