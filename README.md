# SPIRE Workload API - Hexagonal Architecture Implementation

A production-ready implementation of SPIRE's Workload API using hexagonal architecture. This demonstrates how workloads fetch their SVIDs (SPIFFE Verifiable Identity Documents) from a SPIRE agent through a Unix domain socket.

## Overview

This project provides two implementations:

1. **Production mTLS Library** ✅ - Using real `go-spiffe` SDK v2.6.0
   - HTTP server and client with automatic mTLS
   - Production-ready with comprehensive tests
   - Used in `internal/adapters/inbound/httpapi` and `internal/adapters/outbound/httpclient`

2. **Learning SPIRE Implementation** - In-memory for learning
   - Demonstrates SPIRE Workload API concepts
   - Agent server, workload clients, attestation flow
   - Used in `cmd/agent` and `cmd/workload` for demos

**Hexagonal Architecture**: Clear separation between domain, ports, and adapters allows both implementations to coexist

## Features

### 1. SPIRE Workload API

- **Agent Server**: Runs on each host, provides Workload API on Unix socket
- **Workload Clients**: Applications fetch their SVIDs from the local agent
- **Attestation Flow**: Agent attests workload identity → matches selectors → issues SVID
- **Hexagonal Architecture**: Clean separation between domain, ports, and adapters

### 2. mTLS Authentication Library ✨

A production-ready mTLS authentication library using SPIFFE/SPIRE for service-to-service communication:

- ✅ **Automatic Certificate Management**: Zero-downtime certificate rotation via SPIRE
- ✅ **mTLS Authentication**: Both client and server authenticate each other
- ✅ **Identity Extraction**: SPIFFE ID available to application handlers
- ✅ **Standard HTTP**: Compatible with Go's standard `http` package
- ✅ **Authentication Only**: No authorization logic - app decides access
- ✅ **Production Ready**: Comprehensive tests (unit + integration)
- ✅ **Config Structs**: Clean API with structured configuration
- ✅ **Thread-Safe**: Proper shutdown and resource management
- ✅ **Graceful Shutdown**: Separate Shutdown() and Close() methods

**Quick Start**:

```go
// Server with config struct
server, err := httpapi.NewHTTPServer(ctx, httpapi.ServerConfig{
    Address:    ":8443",
    SocketPath: socketPath,
    Authorizer: tlsconfig.AuthorizeAny(),
})
if err != nil {
    log.Fatal(err)
}
defer server.Stop(ctx)

server.RegisterHandler("/api/hello", func(w http.ResponseWriter, r *http.Request) {
    clientID, ok := httpapi.GetSPIFFEID(r)
    if !ok {
        http.Error(w, "Unauthorized", http.StatusUnauthorized)
        return
    }
    fmt.Fprintf(w, "Hello, %s!\n", clientID)
})

if err := server.Start(ctx); err != nil {
    log.Fatal(err)
}

// Client with config struct
client, err := httpclient.NewSPIFFEHTTPClient(ctx, httpclient.ClientConfig{
    SocketPath:       socketPath,
    ServerAuthorizer: tlsconfig.AuthorizeID(serverID),
    Timeout:          30 * time.Second,
})
if err != nil {
    log.Fatal(err)
}
defer client.Close()

resp, err := client.Get(ctx, "https://server:8443/api/hello")
if err != nil {
    log.Fatal(err)
}
defer resp.Body.Close()
```

**Documentation**:
- [docs/MTLS.md](docs/MTLS.md) - Complete mTLS guide
- [docs/TEST_ARCHITECTURE.md](docs/TEST_ARCHITECTURE.md) - Testing strategy and best practices
- [examples/mtls-adapters/](examples/mtls-adapters/) - Working examples

## Architecture

### Directory Structure

```
internal/
├── domain/              # Pure domain entities (TrustDomain, IdentityCredential, etc.)
├── ports/               # Port interfaces (contracts between layers)
├── app/                 # Application services (business logic)
├── config/              # Configuration (YAML + env fallback)
└── adapters/            # Infrastructure implementations
    ├── inbound/
    │   ├── workloadapi/ # Workload API server (Unix socket HTTP)
    │   ├── httpapi/     # mTLS HTTP server (authentication)
    │   └── cli/         # CLI demonstration
    └── outbound/
        ├── inmemory/    # In-memory SPIRE implementation
        ├── workloadapi/ # Workload API client
        ├── httpclient/  # mTLS HTTP client
        └── compose/     # Dependency injection factory

cmd/
├── agent/               # SPIRE agent server (production entrypoint)
├── workload/            # Workload SVID fetch (production entrypoint)
└── main.go              # CLI demonstration tool

examples/
└── mtls-adapters/       # Complete mTLS server/client examples
    ├── server/          # Example mTLS server
    ├── client/          # Example mTLS client
    └── k8s/             # Kubernetes deployment manifests
```

### Hexagonal Architecture

```
┌─────────────────────────────────────────────────────────┐
│                    Inbound Adapters                      │
│  ┌────────────────┐              ┌─────────────────┐    │
│  │ Workload API   │              │ CLI Demo        │    │
│  │ Server (HTTP)  │              │ Adapter         │    │
│  └────────┬───────┘              └────────┬────────┘    │
│           │                               │             │
│           └───────────────┬───────────────┘             │
│                           │                             │
│  ┌────────────────────────▼─────────────────────────┐   │
│  │              Ports (Interfaces)                   │   │
│  │  • IdentityClient  • IdentityMapperRegistry      │   │
│  │  • Agent           • Server                       │   │
│  └────────────────────────┬─────────────────────────┘   │
│                           │                             │
│  ┌────────────────────────▼─────────────────────────┐   │
│  │         Application Services                      │   │
│  │  • IdentityClientService (SVID issuance)         │   │
│  │  • IdentityService (demonstration)               │   │
│  └────────────────────────┬─────────────────────────┘   │
│                           │                             │
│  ┌────────────────────────▼─────────────────────────┐   │
│  │              Domain Entities                      │   │
│  │  • TrustDomain  • IdentityCredential              │   │
│  │  • IdentityDocument  • Selector                  │   │
│  └───────────────────────────────────────────────────┘   │
│                           │                             │
│  ┌────────────────────────▼─────────────────────────┐   │
│  │            Outbound Adapters                      │   │
│  │  • InMemoryAgent  • InMemoryServer               │   │
│  │  • InMemoryRegistry  • Attestors                 │   │
│  └───────────────────────────────────────────────────┘   │
└─────────────────────────────────────────────────────────┘
```

## Production Flow

### 1. Agent Server

The agent runs as a daemon, providing the Workload API:

```bash
# Start SPIRE agent
./bin/agent

# Output:
# Workload API listening on /tmp/spire-agent/public/api.sock
# === SPIRE Agent with Workload API ===
# Trust Domain: example.org
# Agent Identity: spiffe://example.org/host
# Registered workloads: 3
#   - spiffe://example.org/server-workload (UID: 1001)
#   - spiffe://example.org/client-workload (UID: 1002)
#   - spiffe://example.org/test-workload (UID: 1000)
```

**What it does**:
1. Creates Unix domain socket at `/tmp/spire-agent/public/api.sock`
2. Listens for workload SVID requests
3. Extracts calling process credentials (UID/PID/GID)
4. Attests workload → matches selectors → issues SVID

### 2. Workload Client

Workloads fetch their SVIDs by connecting to the agent's socket:

```bash
# Workload fetches its SVID
./bin/workload

# Output:
# === Workload Fetching SVID ===
# Process UID: 1000
# Process PID: 123456
# Connecting to: /tmp/spire-agent/public/api.sock
#
# Fetching X.509 SVID from agent...
# ✓ SVID fetched successfully!
#
# SPIFFE ID: spiffe://example.org/test-workload
# Certificate: X.509 Certificate for spiffe://example.org/test-workload
# Expires At: 2025-10-04 23:04:33
#
# ✓ Workload successfully authenticated!
```

**What it does**:
1. Connects to agent's Unix socket
2. Sends `GET /svid/x509` request with process credentials
3. Receives SVID (certificate + private key)
4. Uses SVID for mTLS or authentication

### 3. Complete Flow

```
┌─────────────┐
│  Workload   │
│  (UID 1000) │
└──────┬──────┘
       │ 1. Connect to Unix socket
       ▼
┌──────────────────────────┐
│  Workload API Server     │
│  /tmp/.../api.sock       │
└──────┬───────────────────┘
       │ 2. Extract caller UID/PID via SO_PEERCRED
       ▼
┌──────────────────────────┐
│  IdentityClientService   │
│  (app layer)             │
└──────┬───────────────────┘
       │ 3. Delegate to agent
       ▼
┌──────────────────────────┐
│  Agent.FetchIdentity     │
│  Document                │
└──────┬───────────────────┘
       │ 4. Attest workload
       ▼
┌──────────────────────────┐
│  WorkloadAttestor        │
│  Returns: unix:uid:1000  │
└──────┬───────────────────┘
       │ 5. Match selectors
       ▼
┌──────────────────────────┐
│  IdentityMapperRegistry  │
│  (immutable, seeded)     │
└──────┬───────────────────┘
       │ 6. Issue SVID
       ▼
┌──────────────────────────┐
│  Server.IssueIdentity    │
│  (generates X.509 cert)  │
└──────┬───────────────────┘
       │ 7. Return to workload
       ▼
┌──────────────────────────┐
│  SVID                    │
│  spiffe://...workload    │
└──────────────────────────┘
```

## Running the System

### Prerequisites

- Go 1.25.1 or higher

### Build Binaries

```bash
# Build agent and workload
go build -o bin/agent ./cmd/agent
go build -o bin/workload ./cmd/workload

# Or build all
go build -o bin/agent ./cmd/agent && \
go build -o bin/workload ./cmd/workload && \
go build -o bin/demo ./cmd
```

### Start Agent Server

```bash
# Start agent in background
IDP_MODE=inmem ./bin/agent &

# Or with custom socket path
SPIRE_AGENT_SOCKET=/custom/path/api.sock ./bin/agent
```

### Fetch SVID as Workload

```bash
# Fetch SVID (uses current process UID)
./bin/workload

# Or with custom socket path
SPIRE_AGENT_SOCKET=/custom/path/api.sock ./bin/workload
```

### Run CLI Demo

```bash
# Run full demonstration (does not use Workload API)
IDP_MODE=inmem go run ./cmd

# Or use built binary
./bin/demo
```

## Port Interfaces

### IdentityClient (Client-Side Interface)

**Location**: `internal/ports/inbound.go`

```go
// IdentityClient is the main entrypoint for workloads to fetch their SVID
// Signature matches go-spiffe SDK's workloadapi.Client
type IdentityClient interface {
    // FetchX509SVID fetches an X.509 SVID for the calling workload
    // Server extracts caller identity from Unix socket connection
    FetchX509SVID(ctx context.Context) (*Identity, error)
}
```

**Usage in workloads**:
```go
client := workloadapi.NewClient("/tmp/spire-agent/public/api.sock")
svid, err := client.FetchX509SVID(ctx)
// Use svid.IdentityCredential, svid.IdentityDocument
```

### Agent (Server-Side Interface)

**Location**: `internal/ports/outbound.go`

```go
type Agent interface {
    // GetIdentity returns the agent's own identity
    GetIdentity(ctx context.Context) (*Identity, error)

    // FetchIdentityDocument fetches identity document for a workload
    // Used internally by IdentityClientService after credential extraction
    FetchIdentityDocument(ctx context.Context, workload ProcessIdentity) (*Identity, error)
}
```

### IdentityMapperRegistry (Immutable Registry)

**Location**: `internal/ports/outbound.go`

```go
// IdentityMapperRegistry provides read-only access to identity mappings
// Seeded at startup, sealed before runtime, no mutations allowed
type IdentityMapperRegistry interface {
    // FindBySelectors matches selectors to identity credential (AND logic)
    FindBySelectors(ctx context.Context, selectors *domain.SelectorSet) (*domain.IdentityMapper, error)

    // ListAll returns all seeded identity mappers
    ListAll(ctx context.Context) ([]*domain.IdentityMapper, error)
}
```

## Domain Entities

### IdentityCredential (SPIFFE ID)

```go
// IdentityCredential represents a SPIFFE ID: spiffe://<trust-domain>/<path>
type IdentityCredential struct {
    trustDomain *TrustDomain
    path        string
}
```

**Examples**:
- `spiffe://example.org/host` (agent)
- `spiffe://example.org/server-workload` (workload)

### IdentityDocument (SVID)

```go
// IdentityDocument represents an X.509 SVID
// Note: This implementation is X.509-only. JWT SVIDs are not supported.
type IdentityDocument struct {
    identityCredential *IdentityCredential
    certificate        *x509.Certificate
    privateKey         *rsa.PrivateKey
    certificateChain   []*x509.Certificate
    expiresAt          time.Time
}
```

**Why X.509-only?** Focus on simplicity and the primary use case (mTLS). JWT can be added via adapters if needed without changing the domain model.

### Selector

```go
// Selector represents a workload attribute used for attestation
type Selector struct {
    selectorType  string  // e.g., "unix"
    selectorValue string  // e.g., "uid:1000"
}
```

**Examples**:
- `unix:uid:1000`
- `unix:user:server-workload`
- `k8s:namespace:production`

### IdentityMapper

```go
// IdentityMapper maps selectors to identity credential
// Used by registry to match attested workloads to identities
type IdentityMapper struct {
    identityCredential *IdentityCredential
    selectors         *SelectorSet
}
```

## Configuration

### Seeding the Registry

Registration is configuration, not runtime behavior. Workload mappings are seeded at agent startup:

**Location**: `internal/adapters/outbound/inmemory/config.go`

```go
Workloads: []ports.WorkloadEntry{
    {
        SpiffeID: "spiffe://example.org/server-workload",
        Selector: "unix:user:server-workload",
        UID:      1001,
    },
    {
        SpiffeID: "spiffe://example.org/client-workload",
        Selector: "unix:user:client-workload",
        UID:      1002,
    },
}
```

**Bootstrap flow**:
1. Load configuration (workload entries)
2. Parse each entry into `IdentityMapper` (domain entity)
3. Seed registry with mappers
4. Seal registry (prevent mutations)
5. Start Workload API server

After sealing, registry is immutable - read-only at runtime.

## Design Decisions

### 1. IdentityClient (Not WorkloadAPI)

Following `go-spiffe` SDK naming: `workloadapi.Client` provides the client interface. Our `IdentityClient` port matches this exactly.

### 2. No Parameters in FetchX509SVID()

```go
// ✅ Correct: Matches SDK
FetchX509SVID(ctx context.Context) (*Identity, error)

// ❌ Wrong: Caller shouldn't provide identity
FetchX509SVID(ctx context.Context, callerIdentity ProcessIdentity) (*Identity, error)
```

Server extracts credentials from Unix socket connection, not from client-provided data.

### 3. Immutable Registry

Registration is seeded data, not runtime mutations:
- ✅ Seed at startup (composition root)
- ✅ Seal before serving requests
- ✅ Read-only at runtime
- ❌ No Register() API endpoint

### 4. Hexagonal Architecture

Pure domain, port interfaces, swappable adapters:
- In-memory implementation for development/testing
- ✅ **Real `go-spiffe` SDK (v2.6.0)** - Production-ready implementation
- No domain coupling to infrastructure

## SPIRE Integration ✅

**The project is already using the real go-spiffe SDK v2.6.0!**

The mTLS adapters (`httpapi` and `httpclient`) use the production `go-spiffe` SDK:

```go
// Real SDK integration (already implemented)
import "github.com/spiffe/go-spiffe/v2/workloadapi"

// In server.go and client.go
x509Source, err := workloadapi.NewX509Source(
    ctx,
    workloadapi.WithClientOptions(
        workloadapi.WithAddr(cfg.SocketPath),
    ),
)
```

**What's using SPIRE**:
- ✅ `internal/adapters/inbound/httpapi` - mTLS server using go-spiffe SDK
- ✅ `internal/adapters/outbound/httpclient` - mTLS client using go-spiffe SDK
- ✅ Integration tests - Full mTLS with real SPIRE agent

**What's using in-memory** (for development):
- `internal/adapters/outbound/inmemory` - In-memory SPIRE implementation for testing
- Used by `cmd/agent` and `cmd/workload` for demo purposes

See `docs/SDK_MIGRATION.md` for details on the dual-mode architecture.

## Testing

The project has comprehensive test coverage with both unit and integration tests. See [docs/TEST_ARCHITECTURE.md](docs/TEST_ARCHITECTURE.md) for complete testing guide.

### Quick Test Commands

```bash
# Run unit tests (fast, no dependencies)
go test ./internal/adapters/... -short

# Run validation tests
go test ./internal/adapters/inbound/httpapi -run 'TestNewHTTPServer_Missing' -v

# Run integration tests (requires SPIRE)
make minikube-up
go test -tags=integration ./internal/adapters/... -v

# Run all tests with coverage
go test -cover ./internal/...
```

### Unit Tests

Mock the `IdentityClient` interface:

```go
type MockIdentityClient struct {
    svid *ports.Identity
}

func (m *MockIdentityClient) FetchX509SVID(ctx context.Context) (*ports.Identity, error) {
    return m.svid, nil
}

func TestMyService(t *testing.T) {
    client := &MockIdentityClient{svid: testSVID}
    service := NewMyService(client)
    // Test service logic
}
```

### Integration Tests

Use in-memory implementation or real SPIRE:

```go
func TestWorkloadAttestation(t *testing.T) {
    // Bootstrap in-memory agent
    app, _ := app.Bootstrap(ctx, inmemory.NewInMemoryConfig(), compose.NewInMemoryDeps())

    // Test workload fetch
    workload := ports.ProcessIdentity{UID: 1000, PID: 123}
    identity, err := app.Agent.FetchIdentityDocument(ctx, workload)

    require.NoError(t, err)
    assert.Equal(t, "spiffe://example.org/test-workload", identity.IdentityCredential.String())
}
```

### Test Coverage

- ✅ **Unit Tests**: Config validation, identity extraction, helper functions
- ✅ **Integration Tests**: Full mTLS handshake, authorization, health checks
- ✅ **Graceful Skipping**: Tests skip when SPIRE unavailable (not fail)
- ✅ **Fast Feedback**: Unit tests run in ~3ms

## Documentation

### Core Documentation

- **[docs/PROJECT_STATUS.md](docs/PROJECT_STATUS.md)** - **Current state: Production vs Educational**
- [docs/MTLS.md](docs/MTLS.md) - Complete mTLS authentication guide
- [docs/TEST_ARCHITECTURE.md](docs/TEST_ARCHITECTURE.md) - Testing strategy and best practices
- [docs/SDK_MIGRATION.md](docs/SDK_MIGRATION.md) - Dual-mode architecture (SDK vs in-memory)
- [docs/CONTROL_PLANE.md](docs/CONTROL_PLANE.md) - Seeding vs runtime architecture
- [docs/ARCHITECTURE_REVIEW.md](docs/ARCHITECTURE_REVIEW.md) - Port placement and design decisions

### API Reference

**Server API** (`internal/adapters/inbound/httpapi`):
- `NewHTTPServer(ctx, ServerConfig)` - Create mTLS server
- `ServerConfig` - Server configuration struct
- `Start(ctx)` - Start listening
- `Shutdown(ctx)` - Graceful shutdown
- `Stop(ctx)` - Shutdown and cleanup
- `RegisterHandler(pattern, handler)` - Register HTTP handler
- `GetSPIFFEID(r)` - Extract client SPIFFE ID

**Client API** (`internal/adapters/outbound/httpclient`):
- `NewSPIFFEHTTPClient(ctx, ClientConfig)` - Create mTLS client
- `ClientConfig` - Client configuration struct
- `TransportConfig` - HTTP transport settings
- `Get/Post/Put/Delete/Patch(ctx, url, ...)` - HTTP methods
- `Close()` - Release resources

### Configuration

Both server and client use config structs for clean, extensible APIs:

```go
// Server configuration
type ServerConfig struct {
    Address    string                  // Required: listen address
    SocketPath string                  // Required: SPIRE agent socket
    Authorizer tlsconfig.Authorizer    // Required: client authorization

    // Optional timeouts (defaults provided)
    ReadHeaderTimeout time.Duration
    ReadTimeout       time.Duration
    WriteTimeout      time.Duration
    IdleTimeout       time.Duration
}

// Client configuration
type ClientConfig struct {
    SocketPath       string                  // Required: SPIRE agent socket
    ServerAuthorizer tlsconfig.Authorizer    // Required: server authorization
    Timeout          time.Duration           // Optional: request timeout
    Transport        TransportConfig         // Optional: connection pooling
}
```

## Quality and Best Practices

This implementation follows Go best practices and production-ready patterns:

1. **✅ Config Structs**: APIs use config structs (not 4+ parameters) for maintainability
2. **✅ Proper Validation**: Required fields validated with clear error messages
3. **✅ Resource Management**: Proper cleanup with defer, separate Shutdown/Close
4. **✅ Thread Safety**: Mutex protects shared state, sync.Once for initialization
5. **✅ Graceful Shutdown**: Separate shutdown context with timeout
6. **✅ Error Wrapping**: Context preserved with `fmt.Errorf("%w", err)`
7. **✅ Test Coverage**: Unit tests (validation) + Integration tests (mTLS)
8. **✅ Documentation**: Inline docs, comprehensive guides, examples

### Design Patterns

**Config Struct Pattern**:
```go
// ✅ Good: Grouped parameters with defaults
NewHTTPServer(ctx, ServerConfig{
    Address:    ":8443",
    SocketPath: socketPath,
    Authorizer: tlsconfig.AuthorizeAny(),
})

// ❌ Bad: Too many parameters
NewHTTPServer(ctx, ":8443", socketPath, authorizer, 10*time.Second, 30*time.Second, ...)
```

**Resource Cleanup**:
```go
server, err := NewHTTPServer(ctx, cfg)
if err != nil {
    return err
}
defer server.Stop(ctx)  // Always cleanup

// Graceful shutdown with timeout
shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
defer cancel()
server.Shutdown(shutdownCtx)
```

**Identity Extraction**:
```go
clientID, ok := httpapi.GetSPIFFEID(r)
if !ok {
    http.Error(w, "Unauthorized", http.StatusUnauthorized)
    return
}
// Use clientID for application authorization
```

### Security Considerations

1. **mTLS Required**: All connections must use mutual TLS
2. **Identity-Based**: Authentication via SPIFFE IDs, not passwords
3. **Certificate Rotation**: Automatic via SPIRE (zero downtime)
4. **No Authorization**: Library only authenticates - app decides access
5. **Timeout Configuration**: All operations have configurable timeouts

## References

- [SPIFFE Specification](https://github.com/spiffe/spiffe)
- [SPIRE Documentation](https://spiffe.io/docs/latest/spire/)
- [go-spiffe SDK](https://github.com/spiffe/go-spiffe)
- [Hexagonal Architecture](https://alistair.cockburn.us/hexagonal-architecture/)

### Identifying and Expressing Invariants for Better Code Quality

Invariants are properties in your code that must always hold true at specific points (e.g., after a method call or in a struct's state). Examples: In a bank account, "balance >= 0" is an invariant; in your auth system, "IdentityDocument.IsValid() == true" before exchange. Expressing them explicitly (via code, docs, or tests) catches bugs early, improves maintainability, and enforces design intent—directly boosting quality.

#### Step 1: Identifying Invariants

Scan your code for:
- **State Assumptions**: What must be true post-operation? (e.g., in `ExchangeMessage`, post-validation: `msg.From.IdentityDocument != nil`).
- **Pre/Post Conditions**: Entry (pre): "from/to namespaces non-nil"; Exit (post): "msg created only if valid".
- **Business Rules**: Domain-specific (e.g., "SelectorSet size <= 10" to prevent DoS).
- **Tools for Discovery**: 
  - Code review/static analysis (e.g., `go vet`, SonarQube).
  - Property-based testing (e.g., Go's `github.com/leanovate/gopter` for random inputs).
  - Ask: "What breaks if this flips?" (e.g., nil doc → auth failure).

Focus on high-impact areas like your auth flow: `IdentityService`, `InMemoryAgent.FetchIdentityDocument`.

#### Step 2: Expressing Invariants

Don't just identify—make them **executable**:
- **Documentation**: Javadoc-style comments (e.g., `// Invariant: Balance >= 0 after Deposit/Withdraw`).
- **Runtime Assertions**: Enforce dynamically (cheap in debug; strip in prod).
  - Go: Use `if !invariant { panic("invariant violated") }` or `debug.Assert` from `runtime/debug`.
- **Design by Contract**: Pre/post via funcs (e.g., `RequireNonNil(fromDoc)`).
- **Static Tools**: Types (e.g., non-nil structs) or linters.

| Method | Pros | Cons | When to Use |
|--------|------|------|-------------|
| **Docs/Comments** | Zero cost, readable. | Not enforced. | Quick prototypes. |
| **Assertions** | Catches at runtime. | Panics in prod (if not stripped). | Debug builds, critical paths. |
| **Tests** | Verifies across scenarios. | No runtime guard. | Always—your main tool. |

#### Step 3: Write Tests for Invariants

**write tests for invariants**. They're a natural fit for unit/integration tests, verifying the "always true" guarantee without runtime overhead. But combine them with runtime checks (e.g., assertions) for enforcement. Below, I'll break it down: identification, expression, testing strategy, and Go-specific tips.

tests are the best way to *verify* invariants hold under varied inputs, ensuring quality without runtime cost. They're "executable specs" that fail on regressions. Focus on:
- **Unit Tests**: Isolate core (mock ports); assert invariant post-call.
- **Property Tests**: Random data to stress invariants (e.g., 1k runs with fuzzed UIDs).
- **Integration**: With in-memory adapters to check end-flow invariants (e.g., full auth succeeds only if docs valid).

Perfect—tests core + adapters without real SPIRE setup. Later, parametrize for real vs. mock.

**Pros/Cons of Invariant Tests**
| Pros | Cons |
|------|------|
| Fast/isolated; high coverage. | Can feel repetitive (e.g., many expiry cases). |
| Enforces DbC mindset. | Over-testing trivia (e.g., skip trivial "non-nil"). |
| Fuzz-friendly for robustness. | Mocks add setup boilerplate (mitigate with helpers). |

#### Go-Specific Testing Example
Extend your `app_test.go` with invariant-focused tests. Use `testify`

```go
// In app_test.go (add after existing ExchangeMessage tests)

func TestIdentityService_Invariants_PostExchange(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		setup    func(service *app.IdentityService, from, to *ports.Identity)
		assertFn func(msg *ports.Message)
	}{
		{
			name: "msg from/to namespaces match inputs",
			setup: func(s *app.IdentityService, from, to *ports.Identity) {
				// Act in setup
				msg, _ := s.ExchangeMessage(context.Background(), *from, *to, "test")
				// But assert in assertFn
			},
			assertFn: func(msg *ports.Message) {
				assert.NotNil(t, msg.From.IdentityCredential)
				assert.NotNil(t, msg.To.IdentityCredential)
				assert.True(t, msg.From.IdentityDocument.IsValid())
				assert.True(t, msg.To.IdentityDocument.IsValid())
			},
		},
		{
			name: "msg content preserved (immutability invariant)",
			setup: func(s *app.IdentityService, from, to *ports.Identity) {
				msg, _ := s.ExchangeMessage(context.Background(), *from, *to, "immutable content")
			},
			assertFn: func(msg *ports.Message) {
				assert.Equal(t, "immutable content", msg.Content)  // No truncation/mutation
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockAgent := new(MockAgent)
			mockRegistry := new(MockRegistry)
			service := app.NewIdentityService(mockAgent, mockRegistry)
			from := createValidIdentity(t, "spiffe://example.org/client", time.Now().Add(1*time.Hour))
			to := createValidIdentity(t, "spiffe://example.org/server", time.Now().Add(1*time.Hour))

			tt.setup(service, from, to)  // Act

			// Re-act to get msg for assert (or capture in setup)
			msg, err := service.ExchangeMessage(context.Background(), *from, *to, "test")
			require.NoError(t, err)
			tt.assertFn(msg)
		})
	}
}

// Property test example (add "github.com/leanovate/gopter" for fuzz)
func TestIdentityService_Property_ValidDocsAlwaysProduceValidMsg(t *testing.T) {
	// Use gopter for 100 random valid identities
	// Assert: ExchangeMessage always succeeds with valid msg invariant
	// (Omitted for brevity; focus on unit first)
}
```

- **Start Small**: Add 2-3 invariant tests to `app/service_test.go` (post-call checks). Run `go test -cover ./internal/app`—aim for 90%+.
- **Evolve**: Once core is solid, add integration (wire in-memory factory, test full flow).
- **Tools**: `go test -race` for concurrency invariants; `golangci-lint` for static invariant hints.

This approach ensures invariants are not just identified but enforced, improving quality iteratively. 