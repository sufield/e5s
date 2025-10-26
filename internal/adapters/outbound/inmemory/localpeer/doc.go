// Package localpeer provides kernel-backed workload attestation via SO_PEERCRED
// for Unix domain socket connections in development mode.
//
// This package implements the design described in docs/roadmap/1.md for dev mode
// that still provides real attestation (kernel-backed via SO_PEERCRED) without
// requiring SPIRE server/agent infrastructure.
//
// # Design Goals
//
// 1. No forged headers - attestation comes from the kernel via SO_PEERCRED
// 2. Same port interface as production - domain code doesn't change
// 3. Synthetic SPIFFE IDs that look like production IDs
// 4. Deterministic mapping from (UID, executable) to identity
//
// # Usage
//
// On the server side (accepting Unix domain socket connections):
//
//	conn, err := listener.AcceptUnix()
//	if err != nil {
//		return err
//	}
//
//	// Extract peer credentials from kernel
//	cred, err := localpeer.GetPeerCred(conn)
//	if err != nil {
//		return err
//	}
//
//	// Store in context for handlers
//	ctx := localpeer.WithCred(r.Context(), cred)
//
// On the handler side:
//
//	func myHandler(w http.ResponseWriter, r *http.Request) {
//		cred, err := localpeer.FromCtx(r.Context())
//		if err != nil {
//			http.Error(w, "Unauthorized", http.StatusUnauthorized)
//			return
//		}
//
//		// Use cred.UID, cred.PID, cred.GID for authorization
//		spiffeID, _ := localpeer.FormatSyntheticSPIFFEID(cred, "dev.local")
//		fmt.Fprintf(w, "Authenticated as: %s\n", spiffeID)
//	}
//
// # Synthetic SPIFFE IDs
//
// Format: spiffe://{trust-domain}/uid-{uid}/{executable-name}
//
// Examples:
//   - spiffe://dev.local/uid-1000/client-demo
//   - spiffe://dev.local/uid-1001/server-app
//   - spiffe://dev.local/uid-0/root-admin
//
// This format ensures:
//   - Unique identity per (UID, executable) pair
//   - Looks like a real SPIFFE ID to application code
//   - Deterministic (same binary + same UID = same ID)
//   - Human-readable for debugging
//
// # Security Properties
//
// SO_PEERCRED is kernel-backed attestation:
//   - Cannot be forged by the peer process
//   - Represents the peer at connection time
//   - Includes PID (can be used to read /proc/{pid}/exe)
//   - Includes UID/GID (can be used for Unix permission checks)
//
// This provides similar security to SPIRE's node attestation, but:
//   - Only works on the same Linux host (not across network)
//   - Requires Unix domain sockets (not TCP)
//   - Requires Linux kernel (not portable to other OSes)
//
// # Build Tags
//
// This package is only compiled with:
//   //go:build linux && dev
//
// Production builds use the real SPIRE adapter instead.
package localpeer
