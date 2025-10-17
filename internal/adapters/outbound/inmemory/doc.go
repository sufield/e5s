//go:build dev

// Package inmemory contains concrete, in-memory implementations of outbound
// adapters used by the walking-skeleton in this repository.
//
// Purpose
// -------
// These adapters let the application run completely in-process without
// external dependencies. They are intended for local development, examples,
// and tests, and are not suitable for production use.
//
// Files and responsibilities
// --------------------------
// The package implements the following components (files) and their key
// responsibilities. Each entry points to the primary API surface in that
// file.
//
//   - agent.go
//     InMemoryAgent: implements the `ports.Agent` port. It performs workload
//     attestation (via a WorkloadAttestor), converts attestor selector strings
//     into `domain.Selector` objects, matches them against the
//     IdentityMapperRegistry, and requests identity documents from the
//     InMemoryServer. Key constructors:
//
//   - NewInMemoryAgent(...)
//     Runtime flow: Attest -> Match -> Issue -> Return.
//
//   - server.go
//     InMemoryServer: issues identity documents and holds a CA and private key
//     for the demo system. It exposes methods such as `IssueIdentity` and
//     `GetCA` used by other adapters and the compose factory.
//
//   - registry.go
//     InMemoryRegistry: stores IdentityMappers and supports read-only lookup
//     by selector sets (FindBySelectors). It also exposes `Seed` and `Seal`
//     to populate configuration during bootstrap.
//
//   - trust_domain_parser.go
//     InMemoryTrustDomainParser: parses trust domain strings into
//     `domain.TrustDomain` values. Used when bootstrapping server/agent
//     components.
//
//   - identity_namespace_parser.go
//     InMemoryIdentityCredentialParser: parses SPIFFE ID-like strings into
//     `domain.IdentityCredential` objects. Used when constructing agent and
//     mapper identities.
//
//   - identity_document_provider.go
//     Implements ports.IdentityDocumentProvider. Responsible for creating
//     identity documents (X.509-ish) for agents and workloads. The in-memory
//     provider returns simple identity documents suitable for demos and tests.
//
//   - trust_bundle_provider.go
//     InMemoryTrustBundleProvider: returns the CA bundle used for document
//     validation in the demo environment. Can be used to verify certificate
//     chains if needed by external components.
//
//   - config.go
//     Provides a simple in-memory ConfigLoader used by CLI and examples to
//     supply configuration (trust domain, agent SPIFFE ID, workload list,
//     etc.). This is not a dynamic config system — it's a static, in-memory
//     loader for examples.
//
//   - translation.go
//     Helper functions to translate between domain and port types when the
//     in-memory implementations require small adapters between layers.
//
//   - attestor/ (subdirectory)
//     Contains `InMemoryNodeAttestor` and `UnixWorkloadAttestor` which provide
//     node- and workload-level attestation adapters for demo scenarios. See
//     `internal/adapters/outbound/inmemory/attestor/doc.go` for details.
//
// Security note
//   - These implementations avoid network/cloud APIs and should never be used
//     in production. They are purposely simple to keep the example focused on
//     architecture and flow rather than platform integration.
//
// How to use in examples
// ----------------------
// The compose AdapterFactory creates these concrete implementations and is
// used by the application's bootstrapper. Typical usage in examples:
//
//   factory := compose.NewInMemoryAdapterFactory()
//
//   serverCfg := ports.DevelopmentServerConfig{
//       TrustDomain:       trustDomain,
//       TrustDomainParser: parser,
//       DocProvider:       docProvider,
//   }
//   server, _ := factory.CreateDevelopmentServer(ctx, serverCfg)
//
//   registry := factory.CreateRegistry()
//   attestor := factory.CreateAttestor()
//
//   agentCfg := ports.DevelopmentAgentConfig{
//       SPIFFEID:    spiffeID,
//       Server:      server,
//       Registry:    registry,
//       Attestor:    attestor,
//       Parser:      parser,
//       DocProvider: docProvider,
//   }
//   agent, _ := factory.CreateDevelopmentAgent(ctx, agentCfg)
//
// Keep adapter-specific logic out of the core domain; these are bridge
// implementations that translate platform details into domain objects.
package inmemory
