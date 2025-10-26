//go:build linux && dev

package attestor

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/google/uuid"
	"github.com/pocket/hexagon/spire/internal/domain"
	"github.com/pocket/hexagon/spire/internal/ports"
)

// UnixPeerCredAttestor provides real kernel-backed Unix attestation via SO_PEERCRED
// This implementation follows the design in docs/roadmap/1.md
//
// Security properties:
//   - Uses actual kernel-provided credentials (cannot be forged)
//   - Reads /proc/{pid}/exe for executable verification
//   - Creates synthetic SPIFFE IDs that look like production
//   - No HTTP headers or forged data
type UnixPeerCredAttestor struct {
	trustDomain string // e.g., "dev.local"
}

// NewUnixPeerCredAttestor creates a new SO_PEERCRED-based attestor
// Validates trust domain to ensure SPIFFE compliance
//
// Example usage:
//
//	attestor := NewUnixPeerCredAttestor("example.org")
//	selectors, err := attestor.Attest(ctx, workload)
//	if err != nil {
//	    log.Fatalf("attestation failed: %v", err)
//	}
func NewUnixPeerCredAttestor(trustDomain string) *UnixPeerCredAttestor {
	if trustDomain == "" {
		trustDomain = "dev.local"
	}

	// Validate trust domain format
	// Per SPIFFE spec, trust domain is just the authority component (no spiffe:// prefix)
	if strings.Contains(trustDomain, "://") {
		panic(fmt.Sprintf("invalid trust domain %q: must not include scheme (e.g., use 'dev.local' not 'spiffe://dev.local')", trustDomain))
	}
	if strings.Contains(trustDomain, "/") {
		panic(fmt.Sprintf("invalid trust domain %q: must not include path separators", trustDomain))
	}
	if trustDomain == "" {
		panic("trust domain must not be empty")
	}

	// Validate trust domain characters per SPIFFE spec
	// Trust domains must be lowercase alphanumeric with dots and hyphens
	validTrustDomain := regexp.MustCompile(`^[a-z0-9.-]+$`)
	if !validTrustDomain.MatchString(trustDomain) {
		panic(fmt.Sprintf("invalid trust domain %q: must be lowercase alphanumeric with dots/hyphens only", trustDomain))
	}

	// Validate no leading/trailing dots or hyphens
	if strings.HasPrefix(trustDomain, ".") || strings.HasSuffix(trustDomain, ".") ||
		strings.HasPrefix(trustDomain, "-") || strings.HasSuffix(trustDomain, "-") {
		panic(fmt.Sprintf("invalid trust domain %q: no leading/trailing dots/hyphens", trustDomain))
	}

	return &UnixPeerCredAttestor{
		trustDomain: trustDomain,
	}
}

// Attest returns selectors based on real kernel credentials (SO_PEERCRED)
// This provides real attestation without SPIRE infrastructure
//
// Errors include unique trace IDs for logging correlation
//
// Example usage:
//
//	attestor := NewUnixPeerCredAttestor("dev.local")
//	workload := &domain.Workload{/* ... */}
//	selectors, err := attestor.Attest(context.Background(), workload)
//	if err != nil {
//	    // Error includes [trace-id] for correlation
//	    log.Printf("attestation error: %v", err)
//	    return err
//	}
//	// Use selectors for authorization
//	for _, selector := range selectors {
//	    log.Printf("selector: %s", selector)
//	}
func (a *UnixPeerCredAttestor) Attest(ctx context.Context, workload *domain.Workload) ([]string, error) {
	// Generate unique trace ID for error correlation
	traceID := uuid.NewString()

	// Check for context cancellation before expensive operations
	select {
	case <-ctx.Done():
		return nil, fmt.Errorf("[%s] attestation cancelled: %w", traceID, ctx.Err())
	default:
	}

	// Validate workload
	if workload == nil {
		return nil, fmt.Errorf("[%s] %w: nil workload", traceID, domain.ErrInvalidProcessIdentity)
	}
	if workload.UID() < 0 {
		return nil, fmt.Errorf("[%s] %w: invalid UID %d", traceID, domain.ErrInvalidProcessIdentity, workload.UID())
	}
	if workload.GID() < 0 {
		return nil, fmt.Errorf("[%s] %w: invalid GID %d", traceID, domain.ErrInvalidProcessIdentity, workload.GID())
	}
	if workload.PID() <= 0 {
		return nil, fmt.Errorf("[%s] %w: invalid PID %d", traceID, domain.ErrInvalidProcessIdentity, workload.PID())
	}

	// Read executable path from /proc (kernel-backed)
	exe, err := a.getExecutablePath(ctx, workload.PID())
	if err != nil {
		// If we can't read the executable, we can't attest
		// This could happen if PID exited, we lack permissions, or binary was deleted
		return nil, fmt.Errorf("[%s] %w: failed to read executable for PID %d: %v",
			traceID, domain.ErrWorkloadAttestationFailed, workload.PID(), err)
	}

	// Build selectors similar to SPIRE's Unix selectors
	// Use %q for exe and path to safely escape special characters (spaces, quotes, etc.)
	selectors := []string{
		fmt.Sprintf("unix:uid:%d", workload.UID()),
		fmt.Sprintf("unix:gid:%d", workload.GID()),
		fmt.Sprintf("unix:pid:%d", workload.PID()),
		fmt.Sprintf("unix:exe:%q", exe),
		fmt.Sprintf("unix:path:%q", filepath.Dir(exe)),
	}

	return selectors, nil
}

// getExecutablePath reads /proc/{pid}/exe to get the full executable path
// This is kernel-backed and cannot be forged by the peer process
// Context parameter enables timeout support for long syscalls
func (a *UnixPeerCredAttestor) getExecutablePath(ctx context.Context, pid int) (string, error) {
	// Validate PID early for consistency with Attest validation
	if pid <= 0 {
		return "", fmt.Errorf("invalid PID %d", pid)
	}

	procPath := fmt.Sprintf("/proc/%d/exe", pid)

	// Use context-aware readlink with timeout support
	type result struct {
		exe string
		err error
	}
	resultCh := make(chan result, 1)

	go func() {
		exe, err := os.Readlink(procPath)
		resultCh <- result{exe: exe, err: err}
	}()

	select {
	case <-ctx.Done():
		return "", ctx.Err()
	case res := <-resultCh:
		if res.err != nil {
			return "", fmt.Errorf("failed to read %s: %w", procPath, res.err)
		}
		exe := res.exe

		// Handle deleted executables (exe would be "/path/to/binary (deleted)")
		// This can happen during binary upgrades or if the executable was removed
		if strings.HasSuffix(exe, " (deleted)") {
			return "", fmt.Errorf("executable deleted for PID %d: %s", pid, exe)
		}

		// Canonicalize path to resolve any symlinks
		// This ensures consistent selectors even if executable is accessed via symlink
		absPath, err := filepath.Abs(exe)
		if err != nil {
			// If Abs fails, fall back to original path
			// This shouldn't happen since exe is already absolute from /proc
			return exe, nil
		}

		// Evaluate symlinks to get the real path
		realPath, err := filepath.EvalSymlinks(absPath)
		if err != nil {
			// If EvalSymlinks fails, use the absolute path
			// This could fail if intermediate directories were removed
			return absPath, nil
		}

		// Verify realPath exists and is executable
		info, err := os.Stat(realPath)
		if err != nil {
			// If the resolved path doesn't exist, fall back to absPath
			// This can happen if file was removed after readlink but before EvalSymlinks
			return absPath, nil
		}

		// Check if file is executable (has any execute bit set)
		if info.Mode()&0111 == 0 {
			// Not executable, fall back to absPath
			// This handles cases where symlink points to non-executable file
			return absPath, nil
		}

		return realPath, nil
	}
}

// Verify that UnixPeerCredAttestor implements ports.WorkloadAttestor
var _ ports.WorkloadAttestor = (*UnixPeerCredAttestor)(nil)
