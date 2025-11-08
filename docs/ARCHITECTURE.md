# e5s Architecture

This document defines the internal layering of e5s, which enforces modularity and prevents circular dependencies.

## Layer Model

e5s is organized in **5 layers**, from low-level primitives to high-level conveniences:

```
┌─────────────────────────────────────────────────────────┐
│  Layer 4: cmd/ + examples/                              │
│  ├─ cmd/example-server, cmd/example-client              │
│  ├─ cmd/e5s (CLI tool)                                  │
│  └─ Executable programs built on e5s API                │
└────────────────┬────────────────────────────────────────┘
                 │
┌────────────────▼────────────────────────────────────────┐
│  Layer 3: e5s (config-driven façade)                    │
│  ├─ Start(), StartSingleThread(), Client(), Serve()     │
│  ├─ Config file loading (e5s.yaml)                      │
│  ├─ PeerInfo(), PeerID() - request context extraction   │
│  └─ High-level API that combines layers 1 & 2           │
└────────────────┬────────────────────────────────────────┘
                 │
      ┌──────────┴──────────┐
      │                     │
┌─────▼──────┐    ┌─────────▼────────┐
│  Layer 2:  │    │  Layer 1:        │
│  spiffehttp│    │  spire           │
│            │    │                  │
│  TLS config│    │  Identity source │
│  + peer    │    │  (X509Source)    │
│  extraction│    │  + lifecycle     │
└─────┬──────┘    └─────────┬────────┘
      │                     │
      └──────────┬──────────┘
                 │
┌────────────────▼────────────────────────────────────────┐
│  Layer 0: go-spiffe (external SDK)                      │
│  ├─ workloadapi.X509Source                              │
│  ├─ spiffetls.tlsconfig                                 │
│  ├─ spiffeid.ID                                         │
│  └─ SPIFFE/SPIRE protocol implementation                │
└─────────────────────────────────────────────────────────┘
```

## Layer Descriptions

### Layer 0: go-spiffe (external)

**Location**: External dependency (`github.com/spiffe/go-spiffe/v2/...`)

**Purpose**: Official SPIFFE SDK providing:
- SPIRE Workload API client (`workloadapi`)
- X.509 SVID management with automatic rotation (`svid/x509svid`)
- TLS configuration helpers (`spiffetls/tlsconfig`)
- SPIFFE ID types and validation (`spiffeid`)

**Key Types**:
- `workloadapi.X509Source` - Source of X.509-SVIDs and trust bundles
- `spiffeid.ID` - Strongly-typed SPIFFE ID
- `tlsconfig.Authorizer` - TLS verification policy

This layer is maintained by the SPIFFE community and used as-is.

### Layer 1: spire (identity source)

**Location**: `/spire/`

**Purpose**: Simplified wrapper around go-spiffe's `X509Source` with:
- Lifecycle management (connect, auto-rotate, shutdown)
- Sensible defaults for SPIRE Workload API
- Clear error messages for common issues

**Key Types**:
- `spire.IdentitySource` - Managed X.509 source with cleanup

**Depends on**: Layer 0 (go-spiffe)

**Used by**: Layer 3 (e5s)

**Rules**:
- ✅ MAY import go-spiffe packages
- ❌ MUST NOT import spiffehttp or e5s
- ✅ Should focus on identity lifecycle only

### Layer 2: spiffehttp (TLS + peer extraction)

**Location**: `/spiffehttp/`

**Purpose**: mTLS configuration and peer identity extraction for HTTP:
- TLS config builders for servers and clients
- Peer identity extraction from request context
- Authorization policy configuration

**Key Types**:
- `spiffehttp.ServerConfig` - mTLS server verification policy
- `spiffehttp.ClientConfig` - mTLS client verification policy
- `spiffehttp.Peer` - Extracted peer identity
- `spiffehttp.NewServerTLSConfig()` - Build server TLS config
- `spiffehttp.NewClientTLSConfig()` - Build client TLS config
- `spiffehttp.PeerFromRequest()` - Extract peer from HTTP request

**Depends on**: Layer 0 (go-spiffe)

**Used by**: Layer 3 (e5s)

**Rules**:
- ✅ MAY import go-spiffe packages
- ❌ MUST NOT import spire or e5s
- ✅ Should be protocol-specific (HTTP/TLS) but config-agnostic

### Layer 3: e5s (config-driven façade)

**Location**: `/e5s.go`, `/internal/config/`

**Purpose**: High-level, config-file-driven API that combines layers 1 & 2:
- Load configuration from YAML files
- Start servers with single function calls
- Create clients with single function calls
- Extract peer identity from requests

**Key Functions**:
- `e5s.Start()` - Start mTLS server (background goroutine)
- `e5s.StartSingleThread()` - Start mTLS server (foreground, blocking)
- `e5s.Client()` - Create mTLS HTTP client
- `e5s.Serve()` - Serve mTLS with custom listener
- `e5s.PeerInfo()` - Extract full peer from request
- `e5s.PeerID()` - Extract SPIFFE ID from request

**Depends on**: Layer 0 (go-spiffe), Layer 1 (spire), Layer 2 (spiffehttp), internal/config

**Used by**: Layer 4 (cmd/, examples/)

**Rules**:
- ✅ MAY import spire, spiffehttp, go-spiffe, internal packages
- ❌ MUST NOT be imported by spire or spiffehttp
- ✅ Should handle all config file parsing and defaults

### Layer 4: cmd/ + examples/

**Location**: `/cmd/`, `/examples/`

**Purpose**: Executable programs and examples:
- Example servers and clients showing e5s usage
- CLI tool (`cmd/e5s`) for validation and discovery
- Integration test programs

**Depends on**: All layers (primarily Layer 3)

**Rules**:
- ✅ MAY import any public package in e5s
- ✅ Should demonstrate real-world usage patterns

## Internal Packages

**Location**: `/internal/`

**Purpose**: Implementation details not part of public API:
- `internal/config` - YAML parsing and validation
- `internal/bg` - Background execution abstraction
- `internal/testhelpers` - Test utilities

**Rules**:
- ✅ MAY be imported by any e5s package
- ❌ CANNOT be imported by external projects (enforced by Go)
- ✅ Should contain no business logic - only helpers

## Import Rules Summary

| Package | May Import | Must NOT Import |
|---------|-----------|-----------------|
| `spire` | go-spiffe | e5s, spiffehttp |
| `spiffehttp` | go-spiffe | e5s, spire |
| `e5s` | spire, spiffehttp, go-spiffe, internal/* | (none - top layer) |
| `cmd/*` | e5s, spire, spiffehttp, go-spiffe | (none - top layer) |
| `internal/*` | Depends on helper type | e5s (except config), spire, spiffehttp |

## Dependency Graph

```
cmd/*, examples/
    ↓
   e5s  ←──┐
  ↙   ↘    │
spire  spiffehttp
  ↘   ↙
go-spiffe (external)
```

**Key Property**: No cycles. Each layer only depends on layers below it.

## Rationale

### Why Layer 1 (spire) and Layer 2 (spiffehttp) Don't Import Each Other

- **spire** manages identity sources (connection to SPIRE agent)
- **spiffehttp** configures TLS and extracts peer information
- They have different responsibilities and no overlap
- Both can be used independently or combined by Layer 3 (e5s)

### Why Layer 3 (e5s) Exists

Without e5s, users would need to:
1. Parse config files manually
2. Call `spire.NewIdentitySource()` with correct parameters
3. Call `spiffehttp.NewServerTLSConfig()` with correct policies
4. Wire up HTTP server with TLS config
5. Handle shutdown sequence correctly

e5s does all of this in one `Start("e5s.yaml", handler)` call.

### Why Layers 1 & 2 Are Separate from e5s

- Allows advanced users to use `spire` or `spiffehttp` directly without config files
- Enables different config formats in the future (environment variables, programmatic)
- Keeps protocol-specific logic (HTTP/TLS) separate from identity management
- Makes testing easier (can test TLS config independently of identity source)

## Verification

To verify layer compliance:

```bash
# Check that spire doesn't import e5s or spiffehttp
go list -f '{{.ImportPath}}: {{.Imports}}' ./spire | grep -E 'e5s|spiffehttp'
# Should return nothing

# Check that spiffehttp doesn't import e5s or spire
go list -f '{{.ImportPath}}: {{.Imports}}' ./spiffehttp | grep -E 'github.com/sufield/e5s[^/]|spire'
# Should return nothing

# Check that e5s imports both spire and spiffehttp
go list -f '{{.ImportPath}}: {{.Imports}}' . | grep -E 'spire|spiffehttp'
# Should show: github.com/sufield/e5s/spire github.com/sufield/e5s/spiffehttp
```

## Evolution

Future changes must respect layering:

- **Adding features to spire**: Should not require spiffehttp or e5s
- **Adding features to spiffehttp**: Should not require spire or e5s
- **Adding features to e5s**: May use spire and spiffehttp
- **New protocol support** (e.g., gRPC): Create sibling to spiffehttp (e.g., `spiffegrpc`), both used by e5s

This layering enables modular growth without creating dependency tangles.
