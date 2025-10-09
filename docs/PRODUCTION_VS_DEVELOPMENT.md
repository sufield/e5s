# Production vs Development Architecture

This document clarifies the architectural differences between production and development deployments.

## Summary

| Aspect | Development (In-Memory) | Production (SPIRE) |
|--------|------------------------|-------------------|
| **Registry** | In-memory registry seeded at startup | SPIRE Server manages registration entries |
| **Workload Attestation** | Local Unix attestor | SPIRE Agent performs attestation |
| **Selector Matching** | Local domain logic | SPIRE Server matches against entries |
| **SVID Issuance** | In-memory CA | SPIRE Server CA |
| **Agent Type** | `InMemoryAgent` or hybrid `Agent` | `ProductionAgent` |
| **Selector Domain Logic** | Required | NOT required (delegated to SPIRE) |

## Production Architecture

### Components

In production, the system **fully delegates** to external SPIRE infrastructure:

```
┌─────────────────────────────────────────────────────────┐
│                  Your Application                        │
│                                                          │
│  ┌──────────────────────────────────────────────────┐  │
│  │  ProductionAgent (spire.ProductionAgent)         │  │
│  │  - Fetches SVIDs from SPIRE Workload API         │  │
│  │  - No local registry                              │  │
│  │  - No local attestation                           │  │
│  │  - No selector matching                           │  │
│  └───────────────────┬──────────────────────────────┘  │
│                      │                                   │
└──────────────────────┼───────────────────────────────────┘
                       │
                       │ Workload API (Unix Socket)
                       │
              ┌────────▼────────┐
              │  SPIRE Agent    │
              │  (External)     │
              └────────┬────────┘
                       │
                       │ gRPC
                       │
              ┌────────▼────────┐
              │  SPIRE Server   │
              │  (External)     │
              │                 │
              │  - Registry     │
              │  - Attestation  │
              │  - Matching     │
              │  - CA           │
              └─────────────────┘
```

### What Production Does NOT Use

❌ **Selector domain logic** (`domain/selector.go`, `domain/identity_mapper.go`)
❌ **In-memory registry** (`inmemory.InMemoryRegistry`)
❌ **Unix attestor** (`inmemory/attestor/unix.go`)
❌ **Local selector matching**
❌ **In-memory CA**

### What Production DOES Use

✅ **ProductionAgent** (`spire.ProductionAgent`) - Delegates to SPIRE Workload API
✅ **SPIRE Client** (`spire.SPIREClient`) - go-spiffe SDK wrapper
✅ **Translation layer** (`spire/translation.go`) - Domain ↔ SDK type conversion
✅ **Trust domain parser** (wraps go-spiffe SDK)
✅ **Identity credential parser** (wraps go-spiffe SDK)

### Production Flow

```
1. Workload calls: FetchIdentityDocument(ctx, workload)
   │
2. ProductionAgent → SPIRE Workload API: FetchX509SVID()
   │
3. SPIRE Agent:
   - Extracts calling process credentials (UID/PID from Unix socket)
   - Sends attestation request to SPIRE Server
   │
4. SPIRE Server:
   - Attests workload (generates selectors)
   - Matches selectors against registration entries
   - Issues SVID for matched SPIFFE ID
   │
5. SPIRE Agent → ProductionAgent: Returns SVID
   │
6. ProductionAgent → Workload: Returns Identity with SVID
```

**Key Point**: Steps 3-4 happen **entirely in SPIRE**. Your application doesn't do attestation or matching.

### Production Configuration

```go
// Production factory
factory, err := compose.NewSPIREAdapterFactory(ctx, &spire.Config{
    SocketPath:  "unix:///var/run/spire/sockets/agent.sock",
    TrustDomain: "example.org",
    Timeout:     30 * time.Second,
})

// Registry is nil in production
registry := factory.CreateRegistry() // Returns nil

// Attestor is nil in production
attestor := factory.CreateAttestor() // Returns nil

// Agent delegates to SPIRE
agent, err := factory.CreateAgent(ctx, agentSpiffeID, server, registry, attestor, parser, docProvider)
// Returns ProductionAgent
```

### Production Registration

Registration entries are created via SPIRE Server CLI:

```bash
# Create entry for a workload
spire-server entry create \
  -spiffeID spiffe://example.org/webapp \
  -parentID spiffe://example.org/spire/agent/k8s_psat/default \
  -selector k8s:ns:production \
  -selector k8s:sa:webapp \
  -ttl 3600
```

NOT in application code. The `SeedRegistry()` method is a no-op in production.

---

## Development Architecture (In-Memory)

### Components

Development mode uses in-memory implementations for everything:

```
┌─────────────────────────────────────────────────────────┐
│               Your Application (Single Process)          │
│                                                          │
│  ┌──────────────────────────────────────────────────┐  │
│  │  InMemoryAgent                                    │  │
│  │  ┌────────────────────────────────────────────┐  │  │
│  │  │ 1. Unix Attestor                          │  │  │
│  │  │    - Extracts UID/PID                     │  │  │
│  │  │    - Generates selectors                  │  │  │
│  │  └────────────────────────────────────────────┘  │  │
│  │                    │                              │  │
│  │  ┌─────────────────▼──────────────────────────┐  │  │
│  │  │ 2. In-Memory Registry                     │  │  │
│  │  │    - Selector → SPIFFE ID mappings        │  │  │
│  │  │    - AND matching logic                   │  │  │
│  │  └────────────────────────────────────────────┘  │  │
│  │                    │                              │  │
│  │  ┌─────────────────▼──────────────────────────┐  │  │
│  │  │ 3. In-Memory Server                       │  │  │
│  │  │    - Self-signed CA                       │  │  │
│  │  │    - Certificate generation               │  │  │
│  │  └────────────────────────────────────────────┘  │  │
│  └──────────────────────────────────────────────────┘  │
└─────────────────────────────────────────────────────────┘
```

### What Development Uses

✅ **Selector domain logic** - Required for in-process matching
✅ **In-memory registry** - Seeded at application startup
✅ **Unix attestor** - Local UID-based attestation
✅ **Identity mapper** - Selector matching with AND logic
✅ **In-memory CA** - Self-signed certificate generation

### Development Flow

```
1. Workload calls: FetchIdentityDocument(ctx, workload)
   │
2. InMemoryAgent → UnixAttestor: Attest(workload)
   │  Returns: ["unix:uid:1000", "unix:gid:1000"]
   │
3. InMemoryAgent: Parse selectors into SelectorSet
   │
4. InMemoryAgent → Registry: FindBySelectors(selectorSet)
   │  Matches against in-memory mappers
   │  Returns: IdentityMapper for spiffe://example.org/webapp
   │
5. InMemoryAgent → Server: IssueIdentity(identityCredential)
   │  Generates certificate signed by in-memory CA
   │
6. InMemoryAgent → Workload: Returns Identity with SVID
```

**Key Point**: All steps happen **in-process**. No external SPIRE needed.

### Development Configuration

```go
// Development factory
factory := compose.NewInMemoryAdapterFactory()

// Create and seed registry
registry := factory.CreateRegistry() // Returns InMemoryRegistry

// Seed with mappers
mapper, _ := domain.NewIdentityMapper(
    credential,
    selectors, // ["unix:uid:1000", "unix:gid:1000"]
)
factory.SeedRegistry(registry, ctx, mapper)
factory.SealRegistry(registry) // Prevent further mutations

// Attestor for local UID-based attestation
attestor := factory.CreateAttestor() // Returns UnixWorkloadAttestor
factory.RegisterWorkloadUID(attestor, 1000, "unix:uid:1000")

// Agent uses local components
agent, err := factory.CreateAgent(ctx, agentSpiffeID, server, registry, attestor, parser, docProvider)
// Returns InMemoryAgent
```

---

## Hybrid Mode (Deprecated for Production)

The old `spire.Agent` (not `ProductionAgent`) was a hybrid that:
- Used SPIRE for SVID fetching
- But did local selector matching

This is **deprecated for production** because:
❌ Duplicates SPIRE Server's selector matching logic
❌ Requires selector domain logic in production builds
❌ Adds unnecessary complexity
❌ Can lead to mismatches between local registry and SPIRE Server

**Use Cases** (development only):
- Testing selector matching logic
- Validating local registry against SPIRE
- Understanding SPIRE's internals

---

## Build Considerations

### What Should Be Excluded from Production Builds

To minimize production binary size and complexity, the following should be excluded:

**In-Memory Implementations**:
- `internal/adapters/outbound/inmemory/registry.go`
- `internal/adapters/outbound/inmemory/attestor/unix.go`
- `internal/adapters/outbound/inmemory/server.go`
- `internal/adapters/outbound/compose/inmemory.go`

**Selector Domain Logic** (if not using hybrid agent):
- `internal/domain/selector.go`
- `internal/domain/identity_mapper.go`
- `internal/domain/attestation.go`

**Note**: If using `ProductionAgent`, selector domain logic is NOT needed.

### What Must Be Included in Production Builds

**SPIRE Adapters**:
- `internal/adapters/outbound/spire/production_agent.go` ✅
- `internal/adapters/outbound/spire/client.go`
- `internal/adapters/outbound/spire/server.go`
- `internal/adapters/outbound/spire/translation.go`
- `internal/adapters/outbound/compose/spire.go`

**Domain Core** (used by translation layer):
- `internal/domain/identity_credential.go`
- `internal/domain/identity_document.go`
- `internal/domain/trust_domain.go`
- `internal/domain/errors.go`

**Ports** (interfaces):
- `internal/ports/outbound.go`
- `internal/ports/types.go`

---

## Migration Path

### From Development to Production

1. **Stop using in-memory factory**:
```go
// Before (development)
factory := compose.NewInMemoryAdapterFactory()

// After (production)
factory, err := compose.NewSPIREAdapterFactory(ctx, spireConfig)
```

2. **Remove registry seeding**:
```go
// Before (development)
mapper, _ := domain.NewIdentityMapper(credential, selectors)
factory.SeedRegistry(registry, ctx, mapper)

// After (production)
// No code - use SPIRE Server CLI instead:
// spire-server entry create -spiffeID ... -selector ...
```

3. **Deploy SPIRE infrastructure**:
```bash
# Deploy SPIRE Server
kubectl apply -f spire-server.yaml

# Deploy SPIRE Agent (DaemonSet)
kubectl apply -f spire-agent.yaml

# Create registration entries
spire-server entry create ...
```

4. **Update configuration**:
```bash
# Development
export IDP_MODE=inmem

# Production
export SPIRE_AGENT_SOCKET=unix:///var/run/spire/sockets/agent.sock
export SPIRE_TRUST_DOMAIN=example.org
```

---

## Decision Matrix

**Use In-Memory (Development) When**:
- ✅ Local development without SPIRE infrastructure
- ✅ Unit testing
- ✅ Learning SPIRE concepts
- ✅ Prototyping
- ✅ CI/CD testing without K8s

**Use Production (SPIRE) When**:
- ✅ Deploying to production
- ✅ Deploying to staging
- ✅ Integration testing with real SPIRE
- ✅ Multi-service deployments
- ✅ Distributed workloads across nodes

---

## Summary

**Production**: `ProductionAgent` → SPIRE does everything → No selector logic needed

**Development**: `InMemoryAgent` → Local components do everything → Selector logic required

The selector domain logic (`selector.go`, `identity_mapper.go`) is **only needed for development/testing** when using in-memory implementations. Production deployments with `ProductionAgent` delegate all selector matching to SPIRE Server and can exclude this code from builds.
