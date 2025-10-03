package domain

// Workload represents a software process or service requesting an identity
// It is the primary entity that undergoes attestation and receives an SVID
type Workload struct {
	pid  int    // Process ID
	uid  int    // User ID
	gid  int    // Group ID
	path string // Executable path
}

// NewWorkload creates a new workload
func NewWorkload(pid, uid, gid int, path string) *Workload {
	return &Workload{
		pid:  pid,
		uid:  uid,
		gid:  gid,
		path: path,
	}
}

// PID returns the process ID
func (w *Workload) PID() int {
	return w.pid
}

// UID returns the user ID
func (w *Workload) UID() int {
	return w.uid
}

// GID returns the group ID
func (w *Workload) GID() int {
	return w.gid
}

// Path returns the executable path
func (w *Workload) Path() string {
	return w.path
}
