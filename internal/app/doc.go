// Package app contains the application's composition root and use-case orchestration.
//
// This package is the APPLICATION LAYER in hexagonal architecture - it orchestrates
// domain objects and port interfaces to implement business use cases. It NEVER
// depends on concrete adapters, only on port interfaces.
//
// Hexagonal Architecture Boundaries:
//   - App imports from: internal/domain, internal/ports (interfaces only)
//   - App NEVER imports from: internal/adapters/* (concrete implementations)
//   - App receives: port interfaces (via dependency injection)
//   - App coordinates: domain logic, use case workflows
//   - Concrete adapter wiring: Done in cmd/main.go or examples/
//
// Responsibilities
//   - Bootstrap the application's dependencies via `Bootstrap` (see bootstrap.go).
//     Bootstrap loads configuration via ports.ConfigLoader and constructs components
//     via ports.AdapterFactory (both are interfaces).
//   - Provide the Application type that holds references to configuration and
//     port interfaces for workload identity operations.
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
