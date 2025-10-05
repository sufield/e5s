// Package workloadapi provides a Workload API client used by example
// workloads to fetch X.509 SVIDs from the local agent over a Unix domain
// socket.
//
// This is an outbound adapter from the workload's perspective. The client
// connects to the agent's Workload API endpoint (the walking skeleton exposes
// a simple HTTP-over-unix endpoint) and requests an X.509 SVID for the
// calling process.
//
// Notes:
//   - For demonstration the client sends process credentials in
//     HTTP headers (UID/PID/GID/path). The corresponding demo server uses
//     these headers when SO_PEERCRED is not available. This is insecure and
//     should never be used in production.
//   - The client supports a basic mTLS flow via `FetchX509SVIDWithConfig` by
//     allowing a custom `tls.Config` to be supplied; however, in the demo the
//     connection is over a Unix socket and TLS is optional.
//
// Usage:
//
//	client := workloadapi.NewClient(socketPath)
//	svid, err := client.FetchX509SVID(ctx)
//
// The adapter exposes `FetchX509SVID` and `FetchX509SVIDWithConfig` along with
// a simple `X509SVIDResponse` type that implements the `ports.X509SVIDResponse`
// interface used by the rest of the application.
package workloadapi
