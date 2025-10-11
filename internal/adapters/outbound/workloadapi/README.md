# Workload API Client

## Overview

This package provides a production-ready Workload API client that implements the `ports.WorkloadAPIClient` interface. Workloads use this client to fetch X.509 SVIDs (SPIFFE Verifiable Identity Documents) from the SPIRE Agent with kernel-verified security.

## Production Readiness

✅ **Production-Ready on Linux** - Uses SO_PEERCRED for kernel-verified workload attestation
⚠️ **Other Platforms** - Requires platform-specific implementation (see [Platform Support](#platform-support))

## Architecture

This is an outbound adapter from the workload's perspective:

```
┌─────────────┐
│  Workload   │
│   Process   │
└──────┬──────┘
       │
       ↓
┌─────────────────────────┐
│ WorkloadAPI Client      │ ← THIS PACKAGE
│ (ports.WorkloadAPIClient)│
└────────┬────────────────┘
         │ Unix Socket
         ↓
┌─────────────────────────┐
│ WorkloadAPI Server      │
│ (inbound adapter)       │
└─────────────────────────┘
```

The hexagonal architecture ensures the core application depends only on the `ports.WorkloadAPIClient` interface, not this specific implementation. This allows different implementations to be swapped based on deployment needs.

### Communication Protocol

- **Transport**: HTTP over Unix domain sockets
- **Format**: JSON request/response
- **Endpoint**: Configurable (default: `/svid/x509`)

### Workload Attestation (Kernel-Verified)

This implementation uses **SO_PEERCRED** on Linux for kernel-level workload attestation:

- **Security**: Credentials are extracted by the kernel and **cannot be forged** by the caller
- **Verification**: PID, UID, and GID are verified at the socket layer before HTTP processing
- **Zero Trust**: No client-provided data is trusted for attestation
- **Production Grade**: Same security level as production SPIRE deployments

**How It Works**:
1. Workload connects to Unix socket (client makes HTTP request)
2. Server's custom listener extracts peer credentials using `SO_PEERCRED` syscall
3. Kernel provides verified PID, UID, GID of the calling process
4. Server uses these credentials for workload identity resolution
5. No headers or client-provided data are needed or trusted

**Previous Implementation**:
Earlier versions used HTTP headers for attestation. This has been replaced with kernel-verified credentials for production security.

## Platform Support

### Linux (Production-Ready) ✅

Fully implemented using SO_PEERCRED with production-grade features:
- **Kernel-verified** process credentials (PID, UID, GID)
- **Cannot be spoofed** by malicious workloads
- **Exponential backoff** retry logic for /proc race conditions (1ms -> 5ms)
- **Credential validation** prevents invalid syscall values (PID must be >= 1)
- **Structured logging** with slog for observability (defaults to stderr with Info level)
- **Secure defaults**: Socket permissions default to 0700 (owner-only)
- **Error context injection**: Detects unwrapped connections to prevent silent failures

**Files**:
- `peercred_linux.go`: SO_PEERCRED implementation with retry logic
- `conn.go`: Custom listener wrapper with logging and error detection
- `middleware.go`: Context injection for request handling with error propagation

**Build Notes**:
```bash
# Build on Linux (auto-detected via build tags)
go build ./internal/adapters/inbound/workloadapi/...

# Run with structured logging for production observability
# Logging is enabled by default to stderr with Info level
# (See "Production Configuration" section below for customization)
```

**Logging** (Defaults to stderr with Info level):
Production deployments automatically include:
- [INFO] Server start with socket path and permissions
- [INFO] SVID issuance with SPIFFE ID
- [ERROR] Credential extraction failures
- [ERROR] Attestation failures with detailed context

For debugging, configure with LevelDebug:
- [DEBUG] Credential extraction (PID/UID/GID)
- [DEBUG] Workload authentication details

### Other Platforms ⚠️

Currently returns an explicit error with platform guidance. **Do not deploy on non-Linux without custom implementation.** To enable production use on other platforms:

**macOS/BSD**:
```go
// Use getpeereid() via cgo
import "C"
// ucred = C.getpeereid(...)
```

**Windows**:
```go
// Use GetNamedPipeClientProcessId() for named pipes
```

**Solaris**:
```go
// Use getpeerucred() via cgo
```

**Implementation**: See `peercred_other.go` for the fallback implementation template.

## Future Enhancements

### 1. Protocol Evolution (gRPC)

The real SPIRE Workload API uses gRPC:

**Consider migrating to**:
- gRPC with streaming support for watch operations
- Protobuf for efficient serialization
- Native support for credential extraction in gRPC interceptors

**Example**:
```go
// Client using official go-spiffe SDK
import "github.com/spiffe/go-spiffe/v2/workloadapi"

client, err := workloadapi.New(ctx,
    workloadapi.WithAddr("unix:///tmp/spire-agent/public/api.sock"))
```

### 2. Alternative: Adapter for Official SDK

Another evolution path is to create an adapter that wraps the official SPIRE go-spiffe client:

```go
// spire_workloadapi_client.go
type SPIREWorkloadAPIClient struct {
    client *workloadapi.Client
}

func (c *SPIREWorkloadAPIClient) FetchX509SVID(ctx context.Context) (ports.X509SVIDResponse, error) {
    x509Context, err := c.client.FetchX509Context(ctx)
    if err != nil {
        return nil, err
    }
    // Adapt go-spiffe response to ports.X509SVIDResponse
    return adaptX509Context(x509Context), nil
}
```

This approach lets you use the official SPIRE SDK while maintaining your hexagonal architecture.

## Security Comparison

| Mechanism | Security Level | Forgeable | Production |
|-----------|---------------|-----------|------------|
| **SO_PEERCRED (Current)** | ✅ Kernel-verified | ❌ No | ✅ Yes |
| HTTP Headers (Legacy) | ⚠️ User-space trust | ✅ Yes | ❌ No |
| Official SPIRE SDK | ✅ Kernel-verified (gRPC) | ❌ No | ✅ Yes |

## Production Configuration

### Secure Defaults

The server is production-ready out of the box with secure defaults:
- **Socket permissions**: 0700 (owner-only access)
- **Logging**: stderr with Info level (includes errors and SVID issuance)
- **Socket directory**: Created automatically with 0700 permissions
- **Error detection**: Unwrapped connections detected and rejected

### Server Setup (Minimal)

```go
package main

import (
    "github.com/pocket/hexagon/spire/internal/adapters/inbound/workloadapi"
)

func main() {
    // Production-ready with secure defaults
    server := workloadapi.NewServer(
        service,
        "/var/run/spire/api.sock",
    )

    if err := server.Start(ctx); err != nil {
        panic(err)  // Logs automatically written to stderr
    }
}
```

### Server Setup with Custom Options

```go
package main

import (
    "log/slog"
    "os"

    "github.com/pocket/hexagon/spire/internal/adapters/inbound/workloadapi"
)

func main() {
    // Custom configuration for specific requirements
    logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
        Level: slog.LevelDebug,  // Include debug logs for detailed credential tracking
    }))

    server := workloadapi.NewServer(
        service,
        "/var/run/spire/api.sock",
        workloadapi.WithLogger(logger),                // Custom logger (optional)
        workloadapi.WithSocketPermissions(0770),       // Group-access (optional)
    )

    if err := server.Start(ctx); err != nil {
        logger.Error("failed to start workload API server", "error", err)
        os.Exit(1)
    }

    // Server logs will include:
    // - [INFO] Server start with socket path and permissions
    // - [DEBUG] Credential extraction (PID/UID/GID)
    // - [DEBUG] Workload authentication
    // - [INFO] SVID issuance with SPIFFE ID
    // - [ERROR] Attestation failures with detailed context
}
```

### Development/Testing Setup

For development with multiple users accessing the socket:

```go
server := workloadapi.NewServer(
    service,
    "/tmp/spire-agent/api.sock",
    workloadapi.WithSocketPermissions(0777),  // Allow any user (dev only!)
)
```

## Usage

### Basic

```go
client, err := workloadapi.NewClient("/tmp/spire-agent/public/api.sock", nil)
if err != nil {
    return fmt.Errorf("failed to create client: %w", err)
}

resp, err := client.FetchX509SVID(ctx)
if err != nil {
    return fmt.Errorf("failed to fetch SVID: %w", err)
}

fmt.Printf("SPIFFE ID: %s\n", resp.GetSPIFFEID())
```

### With Custom Configuration

```go
opts := &workloadapi.ClientOpts{
    Timeout:  60 * time.Second,
    Endpoint: "http://unix/custom/svid",
}

client, err := workloadapi.NewClient(socketPath, opts)
```

### With mTLS

```go
tlsConfig := &tls.Config{
    RootCAs:      trustedCAs,
    Certificates: []tls.Certificate{clientCert},
}

resp, err := client.FetchX509SVIDWithConfig(ctx, tlsConfig)
```

## Testing

The package includes comprehensive tests:

```bash
# Run all tests
go test ./internal/adapters/outbound/workloadapi/...

# Run with coverage
go test ./internal/adapters/outbound/workloadapi/... -cover

# Run specific test
go test -run TestClient_FetchX509SVID_Success
```

## References

- [SPIFFE Workload API Specification](https://github.com/spiffe/spiffe/blob/main/standards/SPIFFE_Workload_API.md)
- [go-spiffe SDK](https://pkg.go.dev/github.com/spiffe/go-spiffe/v2/workloadapi)
- [SPIRE Documentation](https://spiffe.io/docs/latest/spire-about/)


