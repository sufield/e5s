// Package app contains the application's composition root and coordinating
// services. It wires adapters and domain logic together and exposes small,
// focused service types used by inbound adapters.
//
// Responsibilities
//   - Bootstrap the application's dependencies and perform seeding (configuration)
//     via `Bootstrap` (see bootstrap_dev.go / bootstrap_prod.go). Bootstrap is
//     infrastructure code that constructs adapters provided by the `ports.AdapterFactory`
//     and uses a `ports.ConfigLoader` to seed the registry with identity mappers and
//     other configuration fixtures (dev mode only).
//   - Provide orchestrating services used by server and adapter code, notably
//     `IdentityClientService` (server-side SVID issuance) and `IdentityService`
//     (core, pure business logic for identity-based message exchange).
//
// Files
// - application_dev.go (//go:build dev) / application_prod.go (//go:build !dev)
//   - Application: holds references to the bootstrapped components (Config,
//     Service, IdentityClientService, Agent). Dev mode includes Registry field.
//
// - bootstrap_dev.go (//go:build dev) / bootstrap_prod.go (//go:build !dev)
//   - Bootstrap(ctx, configLoader, factory): performs the full composition
//     and seeding flow. Typical steps include loading config, creating the
//     registry and server (dev only), seeding the registry with identity mappers (dev only),
//     sealing it (dev only), creating the agent, and constructing service adapters.
//
// - service.go
//   - IdentityService: implements a use-case for authenticated
//     message exchange. This is domain logic using ports types and does not
//     perform any I/O or adapter work. Public API: `ExchangeMessage`.
//
// - workload_api.go (//go:build dev) / workload_api_prod.go (//go:build !dev)
//   - IdentityClientService: the server-side service responsible for
//     issuing SVIDs to callers after their credentials have been extracted
//     by an inbound adapter. The server adapter calls
//     `IssueIdentity` to perform attestation → match → issue flow
//     via the configured Agent. Dev version uses identityconv package,
//     prod version has inline implementation.
//
// Architectural notes
//   - The `app` package is a composition root and
//     application-layer wiring. It keeps adapter construction and seeding
//     centralized and returns a small `Application` object that adapters can
//     use to access services and the agent.
//   - Keep adapter-specific I/O out of this package; adapters should be thin
//     layers that call into the services provided by `app`.
package app
