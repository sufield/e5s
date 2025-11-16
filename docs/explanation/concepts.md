# Core Concepts

Understanding e5s architecture and when to use each API.

## Two-Layer Design

e5s provides two APIs for different use cases:

| | High-Level (`e5s` package) | Low-Level (`spire` + `spiffehttp` packages) |
|---|---|---|
| **For** | Application developers | Library/framework authors |
| **Config** | YAML file | Go code |
| **Code** | ~10 lines | ~75+ lines |
| **Example** | `e5s.Serve("e5s.yaml", handler)` | Manual TLS config + `http.Server` setup |
| **Shutdown** | Automatic | Manual |
| **Use when** | Building applications | Need fine-grained control |

## High-Level API

**Goal:** Make mTLS work with minimal code.

**Benefits:**
- No hardcoded values (config-driven)
- Built-in graceful shutdown
- Production-ready defaults
- Works with any HTTP framework

**Example:**
```go
func main() {
    http.HandleFunc("/api", handler)
    e5s.Serve("e5s.yaml", http.DefaultServeMux)
}
```

## Low-Level API

**Goal:** Provide building blocks for custom integrations.

**Use when:**
- Building libraries/frameworks on top of e5s
- Need programmatic (not config-driven) setup
- Integrating with custom identity providers
- Building non-HTTP services

**Packages:**
- `spire` - SPIRE Workload API client
- `spiffehttp` - Provider-agnostic mTLS primitives

## SPIFFE and SPIRE

**SPIFFE** (Secure Production Identity Framework For Everyone):
- Standard for workload identity
- SPIFFE ID format: `spiffe://trust-domain/path`
- X.509 certificates (SVIDs) for authentication

**SPIRE** (SPIFFE Runtime Environment):
- Implementation of SPIFFE standard
- Workload API for fetching identities
- Automatic certificate rotation

**e5s role:**
- High-level API: Handles SPIRE integration automatically
- Low-level API: Provides SPIRE client (`spire` package)

## Choosing an API

**Use high-level API if:**
- ✅ You're building an HTTP service
- ✅ You want config-driven setup
- ✅ You want production defaults

**Use low-level API if:**
- ⚠️ You're building a library/framework
- ⚠️ You need non-HTTP mTLS
- ⚠️ You need custom TLS config

**Default:** Start with high-level API. Switch to low-level only when needed.
