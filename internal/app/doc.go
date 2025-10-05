// Package app contains the application's composition root and coordinating
// services. It wires adapters and domain logic together and exposes small,
// focused service types used by inbound adapters.
//
// Responsibilities
//   - Bootstrap the application's dependencies and perform seeding (configuration)
//     via `Bootstrap` (see `application.go`). Bootstrap is infrastructure code
//     that constructs adapters provided by the `ports.AdapterFactory` and uses a
//     `ports.ConfigLoader` to seed the registry with identity mappers and other
//     configuration fixtures.
//   - Provide orchestrating services used by server and adapter code, notably
//     `IdentityClientService` (server-side SVID issuance) and `IdentityService`
//     (core, pure business logic for identity-based message exchange).
//
// Files
// - application.go
//   - Application: holds references to the bootstrapped components (Config,
//     Service, IdentityClientService, Agent, Registry).
//   - Bootstrap(ctx, configLoader, factory): performs the full composition
//     and seeding flow. Typical steps include loading config, creating the
//     registry and server, seeding the registry with identity mappers,
//     sealing it, creating the agent, and constructing service adapters.
//
// - service.go
//   - IdentityService: implements a use-case for authenticated
//     message exchange. This is domain logic using ports types and does not
//     perform any I/O or adapter work. Public API: `ExchangeMessage`.
//
// - workload_api.go
//   - IdentityClientService: the server-side service responsible for
//     issuing SVIDs to callers after their credentials have been extracted
//     by an inbound adapter. The server adapter calls
//     `FetchX509SVIDForCaller` to perform attestation → match → issue flow
//     via the configured Agent.
//
// Architectural notes
//   - The `app` package is a composition root and
//     application-layer wiring. It keeps adapter construction and seeding
//     centralized and returns a small `Application` object that adapters can
//     use to access services and the agent.
//   - Keep adapter-specific I/O out of this package; adapters should be thin
//     layers that call into the services provided by `app`.
package app
