//go:build linux && dev

package localpeer

import (
	"context"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"syscall"
)

// Cred represents Unix domain socket peer credentials obtained via SO_PEERCRED
type Cred struct {
	PID int32  `json:"pid"`
	UID uint32 `json:"uid"`
	GID uint32 `json:"gid"`
}

// ctxKey is the context key for storing peer credentials
const ctxKey = "localpeer.Cred"

// WithCred stores peer credentials in the context
func WithCred(ctx context.Context, c Cred) context.Context {
	return context.WithValue(ctx, ctxKey, c)
}

// FromCtx retrieves peer credentials from the context
func FromCtx(ctx context.Context) (Cred, error) {
	v := ctx.Value(ctxKey)
	if v == nil {
		return Cred{}, fmt.Errorf("no local peer cred in context")
	}
	c, ok := v.(Cred)
	if !ok {
		return Cred{}, fmt.Errorf("invalid type in context: expected Cred, got %T", v)
	}
	return c, nil
}

// GetPeerCred extracts SO_PEERCRED from a Unix domain socket connection
// This provides kernel-backed attestation of the peer process
func GetPeerCred(conn *net.UnixConn) (Cred, error) {
	// Validate connection is not nil
	if conn == nil {
		return Cred{}, fmt.Errorf("nil connection")
	}

	// Get the underlying file descriptor
	rawConn, err := conn.SyscallConn()
	if err != nil {
		return Cred{}, fmt.Errorf("failed to get raw connection: %w", err)
	}

	var cred *syscall.Ucred
	var syscallErr error

	// Control function to extract credentials
	controlErr := rawConn.Control(func(fd uintptr) {
		cred, syscallErr = syscall.GetsockoptUcred(int(fd), syscall.SOL_SOCKET, syscall.SO_PEERCRED)
	})

	if controlErr != nil {
		return Cred{}, fmt.Errorf("failed to control raw connection: %w", controlErr)
	}
	if syscallErr != nil {
		return Cred{}, fmt.Errorf("failed to get SO_PEERCRED: %w", syscallErr)
	}
	if cred == nil {
		return Cred{}, fmt.Errorf("SO_PEERCRED returned nil")
	}

	// Validate credential ranges
	if cred.Pid <= 0 {
		return Cred{}, fmt.Errorf("invalid PID from SO_PEERCRED: %d", cred.Pid)
	}

	return Cred{
		PID: cred.Pid,
		UID: cred.Uid,
		GID: cred.Gid,
	}, nil
}

// GetExecutablePath reads /proc/{pid}/exe to determine the peer's executable path
// Returns the base name of the executable (e.g., "client-demo")
// Returns empty string and error on failure
func GetExecutablePath(pid int32) (string, error) {
	// Validate PID early
	if pid <= 0 {
		return "", fmt.Errorf("invalid PID %d", pid)
	}

	procPath := fmt.Sprintf("/proc/%d/exe", pid)

	// readlink on /proc/{pid}/exe gives us the full executable path
	exe, err := os.Readlink(procPath)
	if err != nil {
		// PID might have exited, or we don't have permissions
		return "", fmt.Errorf("failed to read %s: %w", procPath, err)
	}

	// Return just the base name (e.g., "/usr/bin/client" -> "client")
	return filepath.Base(exe), nil
}

// FormatSyntheticSPIFFEID creates a synthetic SPIFFE ID from peer credentials
// Format: spiffe://dev.local/uid-{uid}/{executable-name}
// This matches the design in docs/roadmap/1.md
func FormatSyntheticSPIFFEID(cred Cred, trustDomain string) (string, error) {
	// Validate trust domain is not empty
	if trustDomain == "" {
		return "", fmt.Errorf("empty trust domain")
	}

	exe, err := GetExecutablePath(cred.PID)
	if err != nil {
		// Fall back to unknown if we can't determine executable
		exe = "unknown"
	}

	// Escape slashes in executable name to avoid invalid SPIFFE IDs
	// Replace "/" with "-" to create valid URI path component
	exeSafe := strings.ReplaceAll(exe, "/", "-")

	// Create synthetic SPIFFE ID
	// Example: spiffe://dev.local/uid-1000/client-demo
	spiffeID := fmt.Sprintf("spiffe://%s/uid-%d/%s", trustDomain, cred.UID, exeSafe)

	return spiffeID, nil
}
