//go:build linux

package workloadapi

import (
	"errors"
	"fmt"
	"log/slog"
	"net"
	"os"
	"syscall"
	"time"

	"github.com/pocket/hexagon/spire/internal/ports"
)

const (
	// maxPathRetries is the maximum number of retry attempts for path resolution
	// to handle kernel races where process exits between SO_PEERCRED and readlink
	maxPathRetries = 2

	// initialRetryDelay is the starting delay for exponential backoff (1ms -> 2ms -> 4ms)
	initialRetryDelay = 1 * time.Millisecond

	// retryBackoffMultiplier is the multiplier for exponential backoff
	retryBackoffMultiplier = 2

	// maxRetryDelay caps the delay to prevent excessive waiting
	maxRetryDelay = 10 * time.Millisecond
)

// extractCredentials extracts kernel-verified process credentials from a Unix socket connection.
// This uses SO_PEERCRED on Linux to obtain credentials that cannot be forged by the caller.
//
// Security: This mechanism provides strong security guarantees because the kernel verifies
// the credentials. Unlike header-based attestation, the calling process cannot spoof these values.
//
// Platform: Linux-only via SO_PEERCRED. Other platforms need equivalent mechanisms:
//   - macOS/BSD: getpeereid() or LOCAL_PEERCRED
//   - Solaris: getpeerucred()
func extractCredentials(conn net.Conn, logger *slog.Logger) (ports.ProcessIdentity, error) {
	// Get underlying Unix connection
	unixConn, ok := conn.(*net.UnixConn)
	if !ok {
		return ports.ProcessIdentity{}, fmt.Errorf("connection is not a Unix socket connection")
	}

	// Get raw file descriptor
	rawConn, err := unixConn.SyscallConn()
	if err != nil {
		return ports.ProcessIdentity{}, fmt.Errorf("failed to get raw connection: %w", err)
	}

	var (
		ucred *syscall.Ucred
		credErr error
	)

	// Extract peer credentials using SO_PEERCRED
	controlErr := rawConn.Control(func(fd uintptr) {
		ucred, credErr = syscall.GetsockoptUcred(int(fd), syscall.SOL_SOCKET, syscall.SO_PEERCRED)
	})

	if controlErr != nil {
		return ports.ProcessIdentity{}, fmt.Errorf("failed to access socket file descriptor: %w", controlErr)
	}
	if credErr != nil {
		return ports.ProcessIdentity{}, fmt.Errorf("failed to get peer credentials: %w", credErr)
	}
	if ucred == nil {
		return ports.ProcessIdentity{}, fmt.Errorf("peer credentials are nil")
	}

	// Validate credential values (syscalls can return -1 on failure or invalid states)
	// PID must be > 0 (PID 0 is kernel/reserved, negative values are errors)
	// Note: UID/GID are uint32, so no need to check for negative values
	if ucred.Pid <= 0 {
		logger.Error("invalid peer credentials",
			"pid", ucred.Pid,
			"uid", ucred.Uid,
			"gid", ucred.Gid)
		return ports.ProcessIdentity{}, fmt.Errorf(
			"invalid peer credentials: PID=%d (must be >0)",
			ucred.Pid,
		)
	}

	// Extract executable path from /proc filesystem with exponential backoff retry
	// This is REQUIRED for SPIRE's unix:path selector matching
	//
	// Kernel Race Condition: Between SO_PEERCRED (gets PID) and readlink,
	// the process could exit, causing ENOENT. We use exponential backoff retries
	// (1ms -> 2ms -> 4ms, capped at 10ms) to handle variable timing in high-load/container environments.
	procPath := fmt.Sprintf("/proc/%d/exe", ucred.Pid)
	var path string
	delay := initialRetryDelay

	for attempt := 0; attempt <= maxPathRetries; attempt++ {
		path, err = os.Readlink(procPath)
		if err == nil {
			break // Success
		}
		if !errors.Is(err, os.ErrNotExist) {
			break // Non-retryable error (e.g., EACCES, not race condition)
		}
		if attempt < maxPathRetries {
			logger.Debug("retrying path resolution due to kernel race",
				"attempt", attempt+1,
				"pid", ucred.Pid,
				"delay_ms", delay.Milliseconds())
			time.Sleep(delay)
			delay *= retryBackoffMultiplier // Exponential backoff: 1ms -> 2ms -> 4ms
			if delay > maxRetryDelay {
				delay = maxRetryDelay // Cap at 10ms
			}
		}
	}

	if err != nil {
		// Path resolution can fail if:
		// - Process exited between SO_PEERCRED and readlink (kernel race)
		// - Kernel denies access to /proc (rare in containers)
		// - Process is a kernel thread (no executable)
		return ports.ProcessIdentity{}, fmt.Errorf(
			"failed to resolve executable path for PID %d (UID=%d, GID=%d) after %d retries: %w; "+
				"workload attestation requires executable path for unix:path selector matching",
			ucred.Pid, ucred.Uid, ucred.Gid, maxPathRetries, err,
		)
	}

	// Return kernel-verified credentials with executable path
	return ports.ProcessIdentity{
		PID:  int(ucred.Pid),
		UID:  int(ucred.Uid),
		GID:  int(ucred.Gid),
		Path: path,
	}, nil
}
