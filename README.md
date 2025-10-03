# Identity Based Authentication using Hexagonal Architecture

A complete in-memory implementation of SPIRE (SPIFFE Runtime Environment) demonstrating hexagonal architecture. This application runs entirely in a single process without requiring external servers, network connections, or actual SPIRE infrastructure.

## Overview

This project demonstrates how to build a walking skeleton for identity-based authentication system using hexagonal architecture. The business logic is isolated from infrastructure concerns through well-defined ports (interfaces). All SPIRE server and agent functionality is implemented as in-memory adapters.

## Architecture

### Hexagonal Architecture

The application follows hexagonal (ports and adapters) architecture:

```
┌─────────────────────────────────────────────────────────────┐
│                         Adapters                             │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐      │
│  │ SPIRE Server │  │ SPIRE Agent  │  │ Unix Attestor│      │
│  └──────┬───────┘  └──────┬───────┘  └──────┬───────┘      │
│         │                  │                  │              │
│         └──────────────────┼──────────────────┘              │
│                            │                                 │
│  ┌─────────────────────────▼──────────────────────────┐     │
│  │              Ports (Interfaces)                     │     │
│  │  • SPIREServer    • IdentityStore                   │     │
│  │  • SPIREAgent     • WorkloadAttestor                │     │
│  └─────────────────────────┬──────────────────────────┘     │
│                            │                                 │
│  ┌─────────────────────────▼──────────────────────────┐     │
│  │           Core Business Logic                       │     │
│  │        (Identity-based messaging)                   │     │
│  └─────────────────────────────────────────────────────┘     │
└─────────────────────────────────────────────────────────────┘
```

### Directory Structure

```
.
├── cmd/
│   └── console/
│       └── main.go                  # Composition root - wires everything
├── internal/
│   ├── adapters/
│   │   ├── inbound/                 # Inbound adapters (driving)
│   │   │   └── cli/
│   │   │       └── cli.go           # CLI adapter - I/O presentation only
│   │   └── outbound/                # Outbound adapters (driven)
│   │       ├── attestor/
│   │       │   └── unix.go          # Unix workload attestor
│   │       ├── compose/
│   │       │   └── inmemory.go      # Dependency factory
│   │       ├── config/
│   │       │   └── inmemory.go      # Configuration loader
│   │       └── spire/
│   │           ├── agent.go         # In-memory SPIRE agent
│   │           ├── server.go        # In-memory SPIRE server
│   │           └── store.go         # In-memory identity store
│   └── app/                         # Core domain
│       ├── application.go           # Bootstrap/composition logic
│       ├── ports.go                 # Port definitions (interfaces)
│       └── service.go               # Business logic
├── go.mod
└── README.md
```

**Separation of Concerns:**
- **`cmd/console/main.go`** - Composition root: loads config, bootstraps app, runs CLI
- **`inbound/cli/`** - CLI adapter: handles ONLY I/O presentation and orchestration
- **`outbound/config/`** - Configuration adapter: implements ConfigLoader port
- **`outbound/compose/`** - Dependency factory: creates concrete adapter instances
- **`app/application.go`** - Bootstrap logic: wires all dependencies together
- **`app/ports.go`** - Port interfaces: defines contracts between core and adapters
- **`app/service.go`** - Core business logic: pure domain logic

## Core Domain

Core domain owns the interfaces defined in ports.go.

### Defined Ports

**Driving Port:**
- `Service` - Core business logic for authenticated message exchange

**Driven Ports:**
- `ConfigLoader` - Loads application configuration
- `SPIREServer` - SPIRE server functionality (CA management, SVID issuance)
- `SPIREAgent` - SPIRE agent functionality (workload attestation, SVID fetching)
- `IdentityStore` - Identity registration and retrieval
- `WorkloadAttestor` - Platform-specific workload attestation

### Domain Types

- `Config` - Application configuration (trust domain, agent ID, workloads)
- `WorkloadEntry` - Workload registration configuration
- `Identity` - Represents a SPIFFE identity with identity document
- `IdentityDocument` - X.509 certificate or JWT token (formerly SVID)
- `ProcessIdentity` - Process attributes (PID, UID, GID, path)
- `Message` - Authenticated message between identities

### Application Bootstrap

**`Application`** - Composition root that wires all dependencies:
```go
type Application struct {
    Config  *Config
    Service Service
    Agent   SPIREAgent
    Store   IdentityStore
}
```

**`Bootstrap()`** - Creates and wires components:
1. Loads configuration via ConfigLoader port
2. Creates all outbound adapters using ApplicationDeps factory
3. Registers workload identities
4. Initializes SPIRE server and agent
5. Returns fully-wired Application

## Outbound Adapters (Driven Ports)

All outbound adapters are in `internal/adapters/outbound/` and implement the driven port interfaces defined in `internal/app/ports.go`.

### Configuration Loader (`internal/adapters/outbound/config/inmemory.go`)

**Responsibilities:**
- Provides application configuration
- Implements ConfigLoader port
- Can be swapped for YAML/ENV/remote config

**Configuration Provided:**
```go
Config{
    TrustDomain:   "example.org"
    AgentSpiffeID: "spiffe://example.org/host"
    Workloads: []WorkloadEntry{
        {SpiffeID: "spiffe://example.org/server-workload", Selector: "unix:user:server-workload", UID: 1001}
        {SpiffeID: "spiffe://example.org/client-workload", Selector: "unix:user:client-workload", UID: 1002}
    }
}
```

### SPIRE Server (`internal/adapters/outbound/spire/server.go`)

**Responsibilities:**
- Generates root CA certificate for the trust domain
- Issues X.509 SVIDs for registered workloads
- Signs certificates with SPIFFE ID in URI SAN field
- Maintains trust domain configuration

**Server API:**
```go
IssueIdentity(ctx, identityNamespace) (*IdentityDocument, error)  // Issues identity document
GetTrustDomain() *TrustDomain                                      // Returns trust domain
GetCA() *x509.Certificate                                          // Returns CA cert
```

**Implementation Details:**
- RSA 2048-bit key pairs
- 24-hour certificate validity
- SPIFFE ID (IdentityNamespace) embedded as URI SAN
- Certificates signed by in-memory CA

### SPIRE Agent (`internal/adapters/outbound/spire/agent.go`)

**Responsibilities:**
- Bootstraps with its own identity (`spiffe://example.org/host`)
- Attests workloads using the workload attestor
- Fetches SVIDs from the server for attested workloads
- Provides workload API functionality

**SPIRE Agent API:**
```go
GetIdentity(ctx) (*Identity, error)                             // Agent's identity
FetchIdentityDocument(ctx, workload ProcessIdentity) (*Identity, error) // Get workload identity document
```

**Attestation Flow:**
1. Receives workload information (PID, UID, GID)
2. Calls attestor to generate selectors
3. Looks up registered identity by selector
4. Requests SVID from server
5. Returns complete identity with SVID

### Identity Store (`internal/adapters/outbound/spire/store.go`)

**Responsibilities:**
- Maintains registration of workload identities
- Maps selectors to SPIFFE IDs
- Thread-safe operations with mutex

**Identity Store API:**
```go
Register(ctx, identityNamespace, selector) error              // Register workload
GetIdentity(ctx, identityNamespace) (*Identity, error)        // Lookup by IdentityNamespace
GetIdentityBySelector(ctx, selector) (*Identity, error)       // Lookup by selector
ListIdentities(ctx) ([]*Identity, error)                      // List all
```

**Data Model:**
- In-memory maps for fast lookup
- `identities`: IdentityNamespace → registered identity
- `selectors`: selector → IdentityNamespace

### Unix Workload Attestor (`internal/adapters/outbound/attestor/unix.go`)

**Responsibilities:**
- Attests workloads based on Unix process attributes
- Maps UIDs to workload selectors
- Generates Unix-style selector strings

**Workload Attestor API:**
```go
RegisterUID(uid, selector)                               // Register UID mapping
Attest(ctx, workload ProcessIdentity) ([]string, error)  // Attest workload
```

**Selector Format:**
```
unix:user:server-workload
unix:uid:1001
unix:gid:1001
```

## Inbound Adapters (Driving Ports)

### CLI Adapter (`internal/adapters/inbound/cli/cli.go`)

**Responsibilities (I/O Presentation ONLY):**
- Displays configuration and execution flow
- Orchestrates use case calls (fetch SVIDs, exchange messages)
- Formats and presents output to user
- **Does NOT** configure, wire dependencies, or load config

**Principle:**
The CLI adapter receives a fully-wired `Application` instance. It focuses purely on:
1. Reading from the application state
2. Calling application services
3. Presenting results to the user

**Example:**
```go
cli := cli.New(application)  // Receives pre-wired app
cli.Run(ctx)                 // Only handles I/O
```

## Composition Root

### `cmd/console/main.go`

The entry point demonstrates proper separation:

```go
func main() {
    ctx := context.Background()

    // 1. Load configuration (via ConfigLoader port)
    configLoader := config.NewInMemoryConfig()

    // 2. Create dependency factory
    deps := compose.NewInMemoryDeps()

    // 3. Bootstrap application (wires everything)
    application, _ := app.Bootstrap(ctx, configLoader, deps)

    // 4. Create CLI adapter (only for I/O)
    cliAdapter := cli.New(application)

    // 5. Run
    cliAdapter.Run(ctx)
}
```

**This ensures:**
- Configuration loading is pluggable
- Dependency wiring is centralized
- CLI focuses on presentation
- Easy to swap adapters (HTTP, gRPC, etc.)

## Configuration

### Trust Domain
```
example.org
```

### Agent Identity
```
spiffe://example.org/host
```

### Registered Workloads

**Server Workload:**
- SPIFFE ID: `spiffe://example.org/server-workload`
- Selector: `unix:user:server-workload`
- UID: 1001

**Client Workload:**
- SPIFFE ID: `spiffe://example.org/client-workload`
- Selector: `unix:user:client-workload`
- UID: 1002

## Running the Skeleton

This walking skeleton runs entirely in-memory without external dependencies.

### Prerequisites
- Go 1.25.1 or higher

### Quick Start
```bash
# Run directly
go run cmd/console/main.go

# Or build and run
go build -o console cmd/console/main.go
./console
```

### Expected Output
```
=== In-Memory SPIRE System with Hexagonal Architecture ===

Configuration:
  Trust Domain: example.org
  Agent SPIFFE ID: spiffe://example.org/host
  Registered Workloads: 2
    - spiffe://example.org/server-workload (UID: 1001)
    - spiffe://example.org/client-workload (UID: 1002)

Attesting and fetching identity documents for workloads...
  ✓ Server workload identity document issued: spiffe://example.org/server-workload
  ✓ Client workload identity document issued: spiffe://example.org/client-workload

Performing authenticated message exchange...
  [client-workload → server-workload]: Hello server
  [server-workload → client-workload]: Hello client

=== Summary ===
✓ Success! Hexagonal architecture with separated concerns:
  - ConfigLoader port: loads configuration
  - Application composer: wires all dependencies
  - CLI adapter: handles ONLY I/O presentation
  - Core domain: pure business logic
  - Current process UID: 1000
```

### What Just Happened?

1. **Configuration loaded** - In-memory config with trust domain and workload registrations
2. **SPIRE server bootstrapped** - Generated CA certificate for `example.org` trust domain
3. **SPIRE agent initialized** - Created agent identity and connected to server
4. **Workloads attested** - Used Unix UID attestation (1001, 1002) to verify workloads
5. **Identity documents issued** - Generated X.509 certificates with SPIFFE IDs
6. **Messages exchanged** - Demonstrated authenticated communication using identities

## System Flow

### 1. Initialization Phase

```
┌──────────────┐
│ Create Store │
└──────┬───────┘
       │
       ▼
┌──────────────────┐
│ Create Server    │
│ (Generate CA)    │
└──────┬───────────┘
       │
       ▼
┌──────────────────┐
│ Create Attestor  │
│ (Register UIDs)  │
└──────┬───────────┘
       │
       ▼
┌──────────────────┐
│ Register         │
│ Workloads        │
└──────┬───────────┘
       │
       ▼
┌──────────────────┐
│ Create Agent     │
│ (Bootstrap ID)   │
└──────────────────┘
```

### 2. Workload Attestation Flow

```
┌──────────────┐
│ Workload     │
│ (UID 1001)   │
└──────┬───────┘
       │
       ▼
┌─────────────────────────┐
│ Agent.FetchIdentity     │
│ Document                │
└──────┬──────────────────┘
       │
       ▼
┌──────────────────┐
│ Attestor.Attest  │
│ Returns selector │
└──────┬───────────┘
       │
       ▼
┌──────────────────┐
│ Store lookup     │
│ by selector      │
└──────┬───────────┘
       │
       ▼
┌──────────────────┐
│ Server.Issue     │
│ SVID             │
└──────┬───────────┘
       │
       ▼
┌──────────────────┐
│ Return Identity  │
│ with SVID        │
└──────────────────┘
```

### 3. Message Exchange Flow

```
┌──────────────────┐
│ Client Identity  │
│ (with SVID)      │
└──────┬───────────┘
       │
       ▼
┌──────────────────┐
│ Service.Exchange │
│ Message          │
└──────┬───────────┘
       │
       ├─▶ Verify sender IdentityNamespace exists
       ├─▶ Verify receiver IdentityNamespace exists
       ├─▶ Verify identity documents are valid
       │
       ▼
┌──────────────────┐
│ Create Message   │
│ (authenticated)  │
└──────────────────┘
```

## Benefits

### 1. **Pure In-Memory Execution**
- No external dependencies
- No network sockets
- No file system access (for SPIRE operations)
- Perfect for testing and development

### 2. **Hexagonal Architecture**
- Core logic independent of infrastructure
- Easy to swap implementations (in-memory → real SPIRE)
- Clear separation of concerns
- Testable without mocks

### 3. **Complete SPIRE Simulation**
- Full attestation flow
- X.509 identity document generation
- Certificate chain validation
- Selector-based identity resolution

### 4. **Separated Concerns (SRP)**
- **ConfigLoader**: Loads configuration only
- **Application.Bootstrap()**: Wires dependencies only
- **CLI Adapter**: Handles I/O presentation only
- **Core Service**: Business logic only
- Each component has a single, well-defined responsibility

### 5. **Testability**
- CLI can be tested with mock Application
- ConfigLoader can be swapped for test fixtures
- Core logic testable without any adapters
- Each adapter testable in isolation

## Walking Skeleton

A walking skeleton is a minimal implementation of a system that:

  1. ✅ Connects all architectural components end-to-end - Your system wires together SPIRE server, agent, attestor, identity store, and core service
  2. ✅ Demonstrates the complete flow - Full attestation → SVID issuance → authenticated message exchange
  3. ✅ Uses the production architecture - Hexagonal architecture with ports and adapters that can be swapped for real implementations
  4. ✅ Is deployable and runnable - go run ./cmd/console executes the entire flow
  5. ✅ Proves the architecture works - Validates that all components integrate correctly

  What makes it a walking skeleton (not just a prototype):

  - Real architecture: Uses actual hexagonal design with proper port/adapter separation
  - Thin but complete: In-memory implementations are simple but functionally complete (CA generation, certificate signing, attestation, etc.)
  - Incrementally expandable: You can replace in-memory adapters with real SPIRE, HTTP servers, gRPC clients without changing core logic
  - Tests the integration: Proves that attestation → identity lookup → identity document issuance → message exchange all work together

  Next steps to grow the skeleton:

  1. Add real SPIRE server/agent adapters alongside in-memory ones
  2. Add HTTP/gRPC transport adapters for networking
  3. Add persistent storage adapters
  4. Add more comprehensive error handling and validation
  5. Add automated tests using the in-memory implementations

The in-memory implementations serve as both the walking skeleton AND excellent test doubles for future development!

## Extending to Real SPIRE

The hexagonal architecture makes it easy to replace in-memory implementations with real SPIRE SDK integrations.

### Migration Strategy

All in-memory implementations are in `internal/adapters/outbound/inmemory/`. The `internal/adapters/outbound/spire/` directory is reserved for real SPIRE SDK implementations.

**Step 1: Implement Real SPIRE Adapters**

See `internal/adapters/outbound/spire/README.md` for detailed guidance on implementing:

- **Server Adapter** - Connect to real SPIRE server via Workload API
- **Agent Adapter** - Use go-spiffe SDK's `workloadapi.Client`
- **Attestors** - Platform-specific attestation (AWS, GCP, Azure, Kubernetes)
- **Store Adapter** - Connect to SPIRE server registration API

**Step 2: Create New Dependency Factory**

```go
// internal/adapters/outbound/compose/realspire.go
type RealSpireDeps struct {
    serverAddr string
    agentSocket string
}

func (d *RealSpireDeps) CreateAgent(...) (ports.Agent, error) {
    // Use go-spiffe SDK
    source, err := workloadapi.NewX509Source(ctx,
        workloadapi.WithClientOptions(
            workloadapi.WithAddr(d.agentSocket),
        ),
    )
    return spire.NewRealAgent(source), nil
}
```

**Step 3: Switch in Main**

```go
// cmd/console/main.go
func main() {
    // Old: In-memory
    // deps := compose.NewInMemoryDeps()

    // New: Real SPIRE
    deps := compose.NewRealSpireDeps(
        serverAddr: "unix:///tmp/spire-server/api.sock",
        agentSocket: "unix:///tmp/spire-agent/api.sock",
    )

    // Everything else stays the same!
    application, _ := app.Bootstrap(ctx, configLoader, deps)
    // ...
}
```

**Benefits:**

- ✅ **Zero changes to ports** - All `ports.Agent`, `ports.Server` interfaces remain unchanged
- ✅ **Zero changes to domain** - Business logic is completely isolated
- ✅ **Zero changes to services** - `IdentityService` works with any implementation
- ✅ **Gradual migration** - Replace adapters one at a time, test incrementally
- ✅ **Keep in-memory for tests** - Use `InMemoryDeps` for unit/integration tests

### Quick Reference

| Component | In-Memory Location | Real SPIRE Location |
|-----------|-------------------|---------------------|
| Agent | `inmemory/agent.go` | `spire/agent.go` |
| Server | `inmemory/server.go` | `spire/server.go` |
| Store | `inmemory/store.go` | `spire/store.go` |
| Attestor | `inmemory/attestor/unix.go` | `spire/attestor/unix.go` |
| Parser | `inmemory/identity_namespace_parser.go` | `spire/identity_namespace_parser.go` |
| Factory | `compose/inmemory.go` | `compose/realspire.go` |

See `internal/adapters/outbound/spire/README.md` for implementation details and SDK usage examples.

## Extending the System

### Adding New Workloads

```go
// Register in the store
store.Register(ctx, "spiffe://example.org/my-workload", "unix:user:my-workload")

// Register UID mapping
unixAttestor.RegisterUID(1003, "unix:user:my-workload")

// Fetch identity document
workload := ports.ProcessIdentity{
    PID: 12347,
    UID: 1003,
    GID: 1003,
    Path: "/usr/bin/my-app",
}
identity, err := agent.FetchIdentityDocument(ctx, workload)
```

### Implementing Different Attestors

Create a new attestor implementing the `WorkloadAttestor` interface:

```go
type CustomAttestor struct {
    // Your implementation
}

func (a *CustomAttestor) Attest(ctx context.Context, workload ports.ProcessIdentity) ([]string, error) {
    // Custom attestation logic
    return selectors, nil
}
```

### Adding Network Adapters

The hexagonal architecture makes it easy to add HTTP/gRPC adapters:

1. Keep the core `Service` interface unchanged
2. Create new driving adapters (HTTP handlers, gRPC servers)
3. Wire them to the core in the main function
4. Use the in-memory SPIRE components for identity

## SPIFFE/SPIRE Concepts Demonstrated

### SPIFFE ID (IdentityNamespace in Code)
Unique identifier for workloads in the format:
```
spiffe://<trust-domain>/<workload-path>
```

In this codebase, SPIFFE IDs are represented by the `IdentityNamespace` domain type. This name emphasizes the structured, namespaced nature of the unique identifier (scheme + trust domain + path).

Examples:
- Agent: `spiffe://example.org/host`
- Server workload: `spiffe://example.org/server-workload`
- Client workload: `spiffe://example.org/client-workload`

### Identity Document
X.509 certificate or JWT token containing:
- SPIFFE ID (IdentityNamespace) in URI SAN field
- Certificate chain to trust root
- Private key for signing/encryption

In this codebase, identity documents are represented by the `IdentityDocument` domain type, which encompasses both X.509 and JWT formats.

### Trust Domain
Administrative boundary for identities:
```
example.org
```

### Workload Attestation
Process of verifying workload identity using platform-specific attributes (UID, PID, etc.)

### Selectors
Attributes used to match workloads to identities:
```
unix:uid:1001
unix:user:server-workload
```

## Testing

The in-memory implementation is ideal for testing:

```go
// Create test components
store := inmemory.NewInMemoryStore()
trustDomainParser := inmemory.NewInMemoryTrustDomainParser()
docProvider := inmemory.NewInMemoryIdentityDocumentProvider()
server, _ := inmemory.NewInMemoryServer(ctx, "test.org", store, trustDomainParser, docProvider)
attestor := attestor.NewUnixWorkloadAttestor()
parser := inmemory.NewInMemoryIdentityNamespaceParser()
agent, _ := inmemory.NewInMemoryAgent(ctx, "spiffe://test.org/agent", server, store, attestor, parser, docProvider)

// Test workload registration
selector, _ := domain.ParseSelectorFromString("unix:user:test-workload")
identityNamespace, _ := parser.ParseFromString(ctx, "spiffe://test.org/workload")
store.Register(ctx, identityNamespace, selector)

// Test identity document issuance
doc, err := server.IssueIdentity(ctx, identityNamespace)

// Test attestation
workload := ports.ProcessIdentity{UID: 1001, PID: 123, GID: 1001, Path: "/usr/bin/test"}
identity, err := agent.FetchIdentityDocument(ctx, workload)
```

## References

- [SPIFFE Specification](https://github.com/spiffe/spiffe)
- [SPIRE Documentation](https://spiffe.io/docs/latest/spire/)
- [Hexagonal Architecture](https://alistair.cockburn.us/hexagonal-architecture/)
- [SPIFFE X.509 SVID Spec](https://github.com/spiffe/spiffe/blob/main/standards/X509-SVID.md)
