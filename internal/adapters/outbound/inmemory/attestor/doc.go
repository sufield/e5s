// Package attestor contains in-memory implementations of node and workload
// attestation used by the walking-skeleton and examples in this repository.
//
// This package provides two primary adapters implemented in this folder:
//
// 1) InMemoryNodeAttestor (node.go)
//   - Purpose: Demonstrates node-level attestation in-memory for the demo
//     server. It maps a node's IdentityNamespace (SPIFFE ID) to a set of
//     platform selectors and returns a `domain.Node` marked as attested.
//   - API:
//   - NewInMemoryNodeAttestor(trustDomain string)
//   - RegisterNodeSelectors(spiffeID string, selectors []*domain.Selector)
//   - AttestNode(ctx, identityNamespace) (*domain.Node, error)
//   - Notes: In production, node attestation is platform-specific (EC2 IID,
//     GCP tokens, TPM, etc.). This in-memory attestor is only suitable for
//     local development and tests.
//
// 2) UnixWorkloadAttestor (unix.go)
//   - Purpose: Demonstrates workload attestation based on Unix process
//     attributes (UID/GID/PID) for local demos. It maps a process UID to a
//     selector string and returns Unix-style selectors during attestation.
//   - API:
//   - NewUnixWorkloadAttestor()
//   - RegisterUID(uid int, selector string)
//   - Attest(ctx, workload ports.ProcessIdentity) ([]string, error)
//   - Notes: This adapter uses UID-based attestation only for demo purposes.
//     Real workload attestation should use stronger platform mechanisms and
//     not rely on client-provided attributes.
//
// Notes
//
//	--------------------------
//	 - These adapters are outbound adapters: they live at the edge of the
//	   system and translate platform-specific attestation data into domain
//	   concepts (selectors, Nodes). The core domain should remain agnostic to
//	   platform details.
//	 - The implementations in this package intentionally avoid network or cloud
//	   APIs and therefore must not be used in production. They are useful for
//	   integration tests, examples, and developer experimentation.
//
// Keep attestation logic out of the core domain; these adapters implement the
// `ports.NodeAttestor` and related interfaces and translate platform data into
// domain models used by the rest of the application.
package attestor
