// Package workloadapi implements an inbound adapter that exposes a simplified
// Workload API over a Unix domain socket for the purposes of this repository's
// examples and tests.
//
// This walking skeleton uses HTTP over a Unix socket and a
// header-based credential fallback for demonstration. In a real
// SPIRE agent implementation the Workload API is typically gRPC over a Unix
// socket and the agent extracts caller credentials from the socket using
// SO_PEERCRED (or an equivalent OS mechanism) to perform attestation.
//
// Important security note:
//   - The current implementation extracts caller identity from request
//     headers when SO_PEERCRED access is not available. This is ONLY suitable
//     for local demos and tests; it must not be used in production.
//   - The repository includes an example function `extractCallerCredentials`
//     showing the intended pattern for obtaining peer credentials from a
//     Unix socket connection.
//
// The package exposes a HTTP-based server with a `/svid/x509` endpoint
// that delegates to the application's IdentityClientService to issue SVIDs.
package workloadapi
