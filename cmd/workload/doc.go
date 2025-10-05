// Package main contains a small example CLI that demonstrates how a workload
// can fetch an X.509 SVID from the local SPIRE agent via the Workload API.
//
// This example is minimal and intended for local development
// and tests. It performs the following steps:
//   - Reads the `SPIRE_AGENT_SOCKET` environment variable to determine the
//     UNIX domain socket path to connect to (defaults to
//     `/tmp/spire-agent/public/api.sock`).
//   - Creates a Workload API client and requests an X.509 SVID from the
//     agent.
//   - Prints basic information about the fetched SVID (SPIFFE ID and expiry).
//
// This example uses the project's in-memory adapters and is not
// suitable for production use. It's useful for integration tests and
// developer experimentation.
package main
