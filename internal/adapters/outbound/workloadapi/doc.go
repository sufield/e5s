// Package workloadapi provides a production-ready Workload API client used by
// workloads to fetch X.509 SVIDs from the SPIRE Agent over a Unix domain socket
// with kernel-verified security.
//
// This is an outbound adapter from the workload's perspective. The client
// connects to the agent's Workload API endpoint over HTTP/Unix socket and
// requests an X.509 SVID. The server uses SO_PEERCRED (Linux) to extract
// kernel-verified process credentials for workload attestation.
//
// Security:
//   - Workload attestation uses SO_PEERCRED (kernel-verified credentials)
//   - No client-provided headers or data are needed for attestation
//   - Credentials cannot be forged by the calling process
//   - Production-grade security equivalent to SPIRE deployments
//
// Features:
//   - Basic SVID fetch via `FetchX509SVID`
//   - Optional mTLS support via `FetchX509SVIDWithConfig`
//   - Configurable timeout and endpoint
//
// Usage:
//
//	client, err := workloadapi.NewClient(socketPath, nil)
//	if err != nil {
//	    log.Fatalf("failed to create client: %v", err)
//	}
//	svid, err := client.FetchX509SVID(ctx)
//	if err != nil {
//	    log.Fatalf("failed to fetch SVID: %v", err)
//	}
//
// The adapter exposes `FetchX509SVID` and `FetchX509SVIDWithConfig` along with
// a simple `X509SVIDResponse` type that implements the `ports.X509SVIDResponse`
// interface used by the rest of the application.
package workloadapi
