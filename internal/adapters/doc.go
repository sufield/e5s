// Package adapters contains infrastructure implementations of port interfaces.
//
// This package is the ADAPTER LAYER in hexagonal architecture - it implements
// the port interfaces defined in internal/ports using concrete technologies
// (SPIRE SDK, HTTP clients, gRPC, etc.). Adapters translate between the domain's
// business logic and external systems/frameworks.
//
// Hexagonal Architecture Boundaries:
//   - Adapters implement: internal/ports interfaces
//   - Adapters import from: internal/domain, internal/ports, external SDKs, standard library
//   - Adapters CAN depend on: go-spiffe SDK, HTTP libraries, crypto libraries, etc.
//   - Adapters are instantiated: by cmd/main.go or examples/ (composition root)
//   - Domain/App layers: NEVER import concrete adapters directly
//
// Adapter Organization
//
// Adapters are organized by data flow direction and composition:
//
//   - inbound/   - Adapters that receive external requests (HTTP servers, gRPC servers)
//   - outbound/  - Adapters that make external calls (SPIRE client, HTTP client, Helm)
//   - compose/   - Factory adapters that wire multiple outbound adapters together
//
// Inbound Adapters (Driving Adapters)
//
// Inbound adapters expose the application to external actors. They implement
// server-side port interfaces and delegate to application services.
//
// Example: identityserver (inbound/identityserver/)
//   - Implements: ports.MTLSServer
//   - Technology: net/http over Unix socket
//   - Purpose: Provides mTLS-authenticated HTTP endpoints for workloads
//   - Security: Enforces workload attestation via SO_PEERCRED (Linux)
//
// Outbound Adapters (Driven Adapters)
//
// Outbound adapters connect the application to external systems. They implement
// client-side port interfaces that the application depends on.
//
// Example: spire (outbound/spire/)
//   - Implements: ports.Agent, ports.TrustDomainParser, ports.IdentityCredentialParser
//   - Technology: go-spiffe SDK (workloadapi)
//   - Purpose: Fetches X.509 SVIDs and trust bundles from SPIRE Agent
//   - External dependency: SPIRE Agent via Unix socket
//
// Example: httpclient (outbound/httpclient/)
//   - Implements: ports.MTLSClient
//   - Technology: net/http with mTLS
//   - Purpose: Makes authenticated HTTP requests to mTLS-protected services
//   - Uses: X.509 credentials from SPIRE for mutual authentication
//
// Example: helm (outbound/helm/)
//   - Implements: Helm-based SPIRE installer for development
//   - Technology: helm.sh/helm/v3/pkg/action
//   - Purpose: Automates SPIRE control plane setup in Kubernetes (dev only)
//   - Build tag: dev (not included in production builds)
//
// Composition Adapters
//
// Composition adapters implement factory interfaces that coordinate multiple
// outbound adapters into a cohesive system.
//
// Example: compose (outbound/compose/)
//   - Implements: ports.AdapterFactory, ports.AgentFactory, ports.BaseAdapterFactory
//   - Purpose: Wires SPIRE client, identity parsers, and validators together
//   - Pattern: Abstract Factory
//   - Usage: Passed to internal/app.Bootstrap() for dependency injection
//
// Design Principles
//
// 1. **Interface-First**: Adapters implement port interfaces, not the other way around
// 2. **One-Way Dependencies**: Adapters depend on ports/domain; domain never depends on adapters
// 3. **Technology Isolation**: SDK-specific code stays in adapters (e.g., go-spiffe types)
// 4. **Testability**: Adapters can be swapped via ports.AdapterFactory for testing
// 5. **Composition Root**: Concrete adapter wiring happens in cmd/main.go or examples/
//
// Testing Adapters
//
// - **Unit Tests**: Mock port interfaces, test adapter logic in isolation
// - **Integration Tests**: Test real adapters against actual SPIRE infrastructure
// - **Fake Adapters**: Can implement ports for in-memory testing (not in this package)
//
// Build Tags
//
// Some adapters use build tags to separate development-only functionality:
//   - `dev` tag: Includes Helm installer adapter (outbound/helm/)
//   - Production builds: Exclude dev-tagged adapters
//
// Example Dependency Flow
//
//	cmd/main.go (composition root)
//	    ↓ creates
//	compose.SPIREAdapterFactory (implements ports.AdapterFactory)
//	    ↓ passed to
//	app.Bootstrap(factory ports.AdapterFactory)
//	    ↓ uses interface
//	Application.agent (ports.Agent)
//	    ↓ implemented by
//	spire.Client (outbound/spire/)
//
// See Also
//   - internal/ports/ - Port interface definitions
//   - internal/domain/ - Domain models and business logic
//   - internal/app/ - Application layer and use-case orchestration
//   - docs/architecture/INVARIANTS.md - Architectural constraints
package adapters
