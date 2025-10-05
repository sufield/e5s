// Package ports defines the inbound and outbound ports (interfaces and types)
// used to decouple the core domain and application logic from adapters.
//
// Purpose
// -------
// Ports are the boundary between the domain/application and the
// infrastructure (adapters). Interfaces represent the contracts that
// adapters must satisfy. Keep these interfaces stable and focused; adapters
// implement concrete behavior (in-memory, SDK-backed, etc.).
//
// Files and responsibilities
// --------------------------
//   - inbound.go
//   - Defines inbound ports implemented by adapters that drive the app or
//     expose services, e.g. `CLI`, `IdentityClientService`, and the
//     Workload API server/client abstractions. Also declares `Service` used
//     by application use-cases.
//   - outbound.go
//   - Defines outbound ports used by the application to talk to
//     infrastructure or SDKs: `ConfigLoader`, `IdentityMapperRegistry`,
//     `WorkloadAttestor`, `NodeAttestor`, `Server`, `Agent`,
//     `IdentityDocumentProvider`, `IdentityDocumentValidator`, `AdapterFactory`,
//     and related parsing/translation ports.
//   - Each interface includes an "Error Contract" in comments describing
//     sentinel errors returned by implementations (e.g., domain.ErrNoMatchingMapper).
//   - types.go
//   - Shared data types used across ports and adapters: `Identity`,
//     `ProcessIdentity`, `Message`, and `Config`/`WorkloadEntry` used by the
//     bootstrap loader.
//
// notes
// ------------
//   - Ports should remain small and well-documented. They define the application's
//     expectations of adapters and are the primary place to record error contracts
//     and compatibility notes (e.g., SDK function signatures).
//   - The project uses an AdapterFactory to keep bootstrap and testing simple: a
//     single factory can produce in-memory or SDK-backed adapters depending on
//     the environment.
//   - Keep domain and application logic free of adapter concerns. Use the ports
//     to pass pure domain types (defined under `internal/domain`) and plain data
//     structures where appropriate.
package ports
