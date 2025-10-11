//go:build !linux

package workloadapi

import (
	"fmt"
	"log/slog"
	"runtime"
)

// logPlatformWarning logs a warning on non-Linux platforms where SO_PEERCRED is not available
// Returns an error to fail fast on unsupported platforms
func logPlatformWarning(logger *slog.Logger) error {
	platform := runtime.GOOS + "/" + runtime.GOARCH
	logger.Warn("workload API server starting on non-Linux platform",
		"platform", platform,
		"note", "kernel-verified credential extraction not available - requests will fail",
		"recommendation", "use Linux for production deployments with SO_PEERCRED support")
	return fmt.Errorf("unsupported platform %s: workload attestation requires Linux with SO_PEERCRED support", platform)
}
