// Package app contains the application's composition root.
// It wires adapters and domain logic together and exposes a minimal
// Application type used by inbound adapters.
//
// Responsibilities
//   - Bootstrap the application's dependencies via `Bootstrap` (see bootstrap.go).
//     Bootstrap loads configuration, constructs the SPIRE agent adapter, and
//     returns a wired Application.
//   - Provide the Application type that holds references to configuration and
//     the SPIRE agent for workload identity operations.
//
// Files
// - application.go
//   - Application: holds references to the bootstrapped components (Config, Agent).
//   - Provides accessors for config and agent.
//   - Manages resource cleanup via Close().
//
// - bootstrap.go
//   - Bootstrap(ctx, configLoader, factory): performs the full composition flow.
//     Loads config, validates inputs, creates the SPIRE agent via Workload API,
//     and constructs the Application with validated dependencies.
//
// Architectural notes
//   - The `app` package is a composition root and application-layer wiring.
//   - It keeps adapter construction centralized and returns a small
//     `Application` object that adapters can use to access the agent.
//   - Keep adapter-specific I/O out of this package; adapters should be thin
//     layers that call into the agent provided by `app`.
//   - All identity operations go through SPIRE Workload API - there is no
//     in-memory simulation or fake identity provider.
package app
