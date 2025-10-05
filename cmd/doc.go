// Package main provides the top-level CLI entrypoint for the example SPIRE
// application contained in this repository.
//
// It bootstraps the application's
// composition root using in-memory adapters (useful for local development and
// tests), constructs an inbound CLI adapter, and runs the application via
// that adapter. Subcommands/examples such as `agent` and `workload` live in
// their respective subpackages under `cmd/` and demonstrate specific runtime
// behaviors (starting a Workload API server or fetching an SVID).
package main
