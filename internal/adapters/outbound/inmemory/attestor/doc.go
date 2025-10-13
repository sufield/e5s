//go:build dev

// Package attestor contains in-memory workload attestation implementation
// used by the walking-skeleton and examples in this repository.
//
// UnixWorkloadAttestor (unix.go)
//   - Purpose: Demonstrates workload attestation based on Unix process
//     attributes (UID/GID/PID) for local demos. It maps a process UID to a
//     selector string and returns Unix-style selectors during attestation.
//   - API:
//   - NewUnixWorkloadAttestor()
//   - RegisterUID(uid int, selector string)
//   - Attest(ctx, workload ports.ProcessIdentity) ([]string, error)
//   - Notes: This adapter uses UID-based attestation only for demo purposes.
//     Real workload attestation should use stronger platform mechanisms and
//     not rely on client-provided attributes.
//
// Notes
//
//	--------------------------
//	 - This adapter is an outbound adapter: it lives at the edge of the
//	   system and translates platform-specific attestation data into domain
//	   concepts (selectors). The core domain remains agnostic to platform details.
//	 - The implementation intentionally avoids network or cloud APIs and
//	   therefore must not be used in production. It is useful for integration
//	   tests, examples, and developer experimentation.
package attestor
