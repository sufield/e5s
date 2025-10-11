//go:build !linux

package workloadapi

import (
	"fmt"
	"log/slog"
	"net"
	"runtime"

	"github.com/pocket/hexagon/spire/internal/ports"
)

// extractCredentials is a fallback implementation for non-Linux platforms.
//
// Platform Support:
//   - macOS/BSD: Could use getpeereid() via cgo
//   - Windows: Named pipes with GetNamedPipeClientProcessId()
//   - Solaris: getpeerucred() via cgo
//
// Current Implementation:
// This fallback returns an error indicating the platform is unsupported.
// For production use on non-Linux platforms, implement platform-specific credential extraction.
//
// Security Note:
// DO NOT fall back to header-based attestation on unsupported platforms, as this would
// create a security vulnerability where credentials can be forged by the caller.
func extractCredentials(conn net.Conn, logger *slog.Logger) (ports.ProcessIdentity, error) {
	return ports.ProcessIdentity{}, fmt.Errorf(
		"kernel-verified credential extraction is not implemented for %s/%s; "+
			"SO_PEERCRED equivalent required for production use",
		runtime.GOOS, runtime.GOARCH,
	)
}
