//go:build linux

package workloadapi

import "log/slog"

// logPlatformWarning logs platform compatibility information on Linux (no warning needed)
// Returns nil on Linux since SO_PEERCRED is fully supported
func logPlatformWarning(logger *slog.Logger) error {
	// No warning needed on Linux - SO_PEERCRED is fully supported
	return nil
}
