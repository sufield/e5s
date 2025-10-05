// Package main provides the CLI entry point for the sample SPIRE agent used
// in this repository's examples and tests.
//
// The agent bootstraps the application composition root, configures in-memory
// adapters, and starts a Workload API server which exposes a UNIX domain
// socket for workloads to request SVIDs. It's designed for local development,
// testing, and documentation purposes.
//
// Summary:
//   - Loads configuration from an in-memory config loader (for examples/tests).
//   - Composes outbound/inbound adapters using the in-memory factory.
//   - Bootstraps the application services and starts the Workload API server.
//   - Reads the `SPIRE_AGENT_SOCKET` environment variable to determine the
//     Workload API UNIX socket path; if unset, it falls back to
//     `/tmp/spire-agent/public/api.sock`.
//
// This file contains only package-level documentation and is
// separate from `main.go` so `godoc` and other tooling can display a concise
// description of the command.
package main
