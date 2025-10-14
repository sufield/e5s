package domain

import (
	"fmt"
	"path/filepath"
)

// Workload represents a software process or service requesting an identity.
// It is the primary entity that undergoes attestation and receives an SVID.
//
// Immutable after construction - all fields are unexported and read-only via accessors.
type Workload struct {
	pid  int    // Process ID (>= 0)
	uid  int    // User ID (>= 0)
	gid  int    // Group ID (>= 0)
	path string // Executable path (cleaned via filepath.Clean)
}

// NewWorkloadValidated creates a workload with validation and path normalization.
// This is the recommended constructor for production code.
//
// Validations:
//   - pid, uid, gid must be >= 0
//   - path must not be empty
//   - path is normalized via filepath.Clean
//
// Returns ErrWorkloadInvalid if validation fails.
func NewWorkloadValidated(pid, uid, gid int, path string) (*Workload, error) {
	if pid < 0 {
		return nil, fmt.Errorf("%w: pid must be >= 0 (got %d)", ErrWorkloadInvalid, pid)
	}
	if uid < 0 {
		return nil, fmt.Errorf("%w: uid must be >= 0 (got %d)", ErrWorkloadInvalid, uid)
	}
	if gid < 0 {
		return nil, fmt.Errorf("%w: gid must be >= 0 (got %d)", ErrWorkloadInvalid, gid)
	}
	if path == "" {
		return nil, fmt.Errorf("%w: path cannot be empty", ErrWorkloadInvalid)
	}

	// Normalize path (no syscalls, pure string operation)
	p := filepath.Clean(path)

	return &Workload{
		pid:  pid,
		uid:  uid,
		gid:  gid,
		path: p,
	}, nil
}

// NewWorkload creates a workload without validation (for backward compatibility).
// Path is normalized via filepath.Clean if non-empty.
//
// Prefer NewWorkloadValidated in new code. This constructor exists to maintain
// API compatibility with existing code that may rely on lenient construction.
func NewWorkload(pid, uid, gid int, path string) *Workload {
	if path == "" {
		// Preserve prior behavior: allow empty; caller can Validate() later
		return &Workload{pid: pid, uid: uid, gid: gid, path: ""}
	}
	return &Workload{
		pid:  pid,
		uid:  uid,
		gid:  gid,
		path: filepath.Clean(path),
	}
}

// MustNewWorkload creates a workload or panics on validation error.
// Convenient for tests and scenarios where invalid input is a programming error.
//
// Example:
//
//	w := domain.MustNewWorkload(1234, 1000, 1000, "/usr/bin/app")
func MustNewWorkload(pid, uid, gid int, path string) *Workload {
	w, err := NewWorkloadValidated(pid, uid, gid, path)
	if err != nil {
		panic(err)
	}
	return w
}

// Validate checks basic invariants.
// Returns ErrWorkloadInvalid if any check fails.
//
// This allows callers to re-validate workloads created via NewWorkload
// or received from untrusted sources.
func (w *Workload) Validate() error {
	if w == nil {
		return fmt.Errorf("%w: nil workload", ErrWorkloadInvalid)
	}
	if w.pid < 0 || w.uid < 0 || w.gid < 0 {
		return fmt.Errorf("%w: negative pid/uid/gid", ErrWorkloadInvalid)
	}
	if w.path == "" {
		return fmt.Errorf("%w: empty path", ErrWorkloadInvalid)
	}
	return nil
}

// IsZero reports whether the workload is uninitialized or zero-valued.
// Safe on nil receiver.
//
// A workload is considered zero if it's nil or all fields are zero.
// Note: pid=0, uid=0, gid=0 with empty path is ambiguous (could be root's init),
// but we treat it as uninitialized for safety.
func (w *Workload) IsZero() bool {
	return w == nil || (w.pid == 0 && w.uid == 0 && w.gid == 0 && w.path == "")
}

// String returns a string representation suitable for logging.
// Safe on nil receiver.
//
// Format: workload{pid=1234,uid=1000,gid=1000,path="/usr/bin/app"}
//
// Note: Path may contain PII depending on your environment.
// Consider redacting in production logs if needed.
func (w *Workload) String() string {
	if w == nil {
		return "workload<nil>"
	}
	return fmt.Sprintf("workload{pid=%d,uid=%d,gid=%d,path=%q}", w.pid, w.uid, w.gid, w.path)
}

// PID returns the process ID.
// Safe on nil receiver (returns 0).
func (w *Workload) PID() int {
	if w == nil {
		return 0
	}
	return w.pid
}

// UID returns the user ID.
// Safe on nil receiver (returns 0).
func (w *Workload) UID() int {
	if w == nil {
		return 0
	}
	return w.uid
}

// GID returns the group ID.
// Safe on nil receiver (returns 0).
func (w *Workload) GID() int {
	if w == nil {
		return 0
	}
	return w.gid
}

// Path returns the executable path (normalized via filepath.Clean during construction).
// Safe on nil receiver (returns empty string).
func (w *Workload) Path() string {
	if w == nil {
		return ""
	}
	return w.path
}
