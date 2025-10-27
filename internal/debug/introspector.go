package debug

import "context"

// Introspector is implemented by components that can provide debug snapshots.
//
// This interface is safe to compile in all builds (no build tags) because
// it's just an interface definition. The implementation is only provided
// in debug builds.
//
// Typical implementation: The identity service or application core
// implements this in a debug-only file to provide sanitized runtime state.
type Introspector interface {
	// SnapshotData returns a sanitized view of current identity state.
	//
	// This should NEVER include secrets (private keys, tokens, passwords).
	// Only return information safe for debugging:
	//   - Current SPIFFE IDs
	//   - Certificate expiration times
	//   - Recent authentication decisions
	//   - Adapter type (inmemory vs spire)
	SnapshotData(ctx context.Context) Snapshot
}
