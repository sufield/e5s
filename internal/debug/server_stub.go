//go:build !debug

package debug

// Start is a no-op in production builds
func Start() {
	// Debug server disabled in production build
}
