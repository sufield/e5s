// Package workloadapi implements a production-ready inbound adapter that exposes
// a Workload API over a Unix domain socket using kernel-verified credential extraction.
//
// This implementation uses HTTP over a Unix socket with SO_PEERCRED (Linux) for
// kernel-level workload attestation. The server extracts caller credentials from
// the socket using SO_PEERCRED, which provides security equivalent to production
// SPIRE deployments.
//
// Security:
//   - Kernel-verified process credentials (PID, UID, GID) via SO_PEERCRED
//   - Credentials cannot be forged by the calling process
//   - Socket permissions default to 0700 (owner-only access)
//   - Socket directory automatically created with secure permissions
//
// Platform Support:
//   - Linux: Production-ready with SO_PEERCRED
//   - Other platforms: Returns explicit error (requires platform-specific implementation)
//
// The package exposes an HTTP-based server with a `/svid/x509` endpoint
// that delegates to the application's IdentityClientService to issue SVIDs.
//
// For production SPIRE, consider migrating to gRPC with the official go-spiffe SDK.
package workloadapi
