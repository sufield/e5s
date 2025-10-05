// Package compose provides an AdapterFactory implementation that composes the
// concrete, in-memory outbound adapters used by the walking-skeleton and
// examples in this repository.
//
// The factory exposes methods to create all required outbound adapters (server,
// registry, parsers, attestor, trust bundle provider, etc.) and handles any
// necessary conversions between the public ports and the concrete in-memory
// implementations.
//
// This package is used by the CLI and agent examples to bootstrap a fully
// wired application without external dependencies, simplifying local testing
// and developer workflows.
package compose
