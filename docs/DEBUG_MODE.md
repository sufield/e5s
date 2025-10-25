# Debug Mode

Debug mode is a first-class operational mode that allows the system to help you debug itself by exposing internal state and enabling mutation of runtime behavior for testing purposes.

## ⚠️ WARNING

**Debug mode exposes internal state and allows mutation of runtime behavior. NEVER use in production.**

Debug features are compiled out of production builds using build tags, ensuring they cannot accidentally ship to production. Always verify debug code is absent in production binaries (see "Verifying Debug Mode is Disabled" section).

## Prerequisites

- Go 1.23+ (uses modern build tags)
- Familiarity with environment variables and HTTP tools like `curl`
- For Windows users: Use `set` instead of `export` for environment variables (e.g., `set SPIRE_DEBUG=true`)
- Access to localhost (127.0.0.1) for the debug server

## Enabling Debug Mode

### Build Time (Recommended)

Build with both `dev` and `debug` tags:

```bash
go build -tags=dev,debug -o bin/demo-debug ./cmd
```

### Runtime

Enable debug features via environment variables:

```bash
# Enable debug logging
export SPIRE_DEBUG=true

# Enable stress testing mode (tiny buffers, fast rotation)
export SPIRE_DEBUG_STRESS=true

# Enable single-threaded mode (no goroutines in auth path)
export SPIRE_DEBUG_SINGLE_THREAD=true

# Enable debug HTTP server
export SPIRE_DEBUG_SERVER=true

# Set debug server address (default: 127.0.0.1:6060)
export SPIRE_DEBUG_ADDR=127.0.0.1:6060
```

### Run with Debug Mode

```bash
# Build with debug tags
go build -tags=dev,debug -o bin/demo-debug ./cmd

# Run with debug server enabled
SPIRE_DEBUG=true SPIRE_DEBUG_SERVER=true ./bin/demo-debug
```

You should see:

```
⚠️  DEBUG SERVER RUNNING ON 127.0.0.1:6060
⚠️  WARNING: Debug mode is enabled. DO NOT USE IN PRODUCTION!
```

## Debug Capabilities

### 1. Debug HTTP Server

Access the debug interface at `http://127.0.0.1:6060/_debug/`

**Available Endpoints:**

- `GET /_debug/` - Debug interface index
- `GET /_debug/state` - View current runtime state
- `GET /_debug/config` - View debug configuration
- `GET /_debug/faults` - View current fault injection settings
- `POST /_debug/faults` - Inject faults for testing
- `POST /_debug/faults/reset` - Reset all fault injections

### 2. Fault Injection

Simulate failures to test error handling. **All faults are one-shot** (automatically consumed after use) to ensure predictable, isolated test behavior and prevent accidental long-term system corruption. Faults do not persist across restarts.

**Available Faults:**

```bash
# Drop next mTLS (mutual TLS) handshake
# Effect: Simulates network failure; expect connection timeout or TLS error in logs
curl -X POST http://localhost:6060/_debug/faults \
  -H "Content-Type: application/json" \
  -d '{"drop_next_handshake": true}'

# Corrupt next SPIFFE ID
# Effect: Simulates invalid identity; expect authentication rejection with "invalid SPIFFE ID" error
curl -X POST http://localhost:6060/_debug/faults \
  -d '{"corrupt_next_spiffe_id": true}'

# Delay next identity issuance by 5 seconds
# Effect: Tests timeout handling; expect delayed response and debug log entry
curl -X POST http://localhost:6060/_debug/faults \
  -d '{"delay_next_issue_seconds": 5}'

# Force trust domain mismatch
# Effect: Simulates configuration error; expect "trust domain mismatch" failure
curl -X POST http://localhost:6060/_debug/faults \
  -d '{"force_trust_domain_mismatch": true}'

# Force expired certificate
# Effect: Tests certificate expiry handling; expect certificate validation failure
curl -X POST http://localhost:6060/_debug/faults \
  -d '{"force_expired_cert": true}'

# Reject next workload lookup
# Effect: Simulates database/registry failure; expect "workload not found" error
curl -X POST http://localhost:6060/_debug/faults \
  -d '{"reject_next_workload_lookup": true}'

# Reset all faults
curl -X POST http://localhost:6060/_debug/faults/reset
```

**Note:** Faults are consumed after one use (one-shot behavior) to prevent accidental long-term effects.

### 3. Debug Logging

When `SPIRE_DEBUG=true`, detailed structured logs are printed to stdout for tracing operations:

```
[DEBUG] Starting authentication for identity: spiffe://example.org/workload
[DEBUG] Mapped client PID=1234 to SPIFFE ID=spiffe://example.org/workload
[DEBUG] Authentication successful
```

**Note:** Debug logs may contain sensitive information like SPIFFE IDs. In shared environments, consider rotating logs or disabling debug mode when not actively troubleshooting.

### 4. Stress Mode

Test rare code paths under high load:

```bash
export SPIRE_DEBUG_STRESS=true
go run -tags=dev,debug ./cmd
```

**Effects:**
- Tiny buffer sizes (1 entry) to force buffer overflows
- Frequent cache evictions to test cache miss handling
- Accelerated certificate rotation (every few seconds instead of hours)
- Forced identity re-attestation every minute to test re-attestation paths

### 5. Single-Threaded Mode

Simplify debugging by eliminating concurrency:

```bash
export SPIRE_DEBUG_SINGLE_THREAD=true
go run -tags=dev,debug ./cmd
```

**Effects:**
- No goroutines in authentication path (synchronous execution)
- Synchronous certificate rotation (blocking)
- Inline identity renewal (no background workers)
- Easier to reproduce race conditions or ordering issues (e.g., via debugger breakpoints)

## Verifying Debug Mode is Disabled

Verify production builds have no debug code:

```bash
# Build production binary
go build -o bin/spire-server ./cmd

# Verify no debug server code
strings bin/spire-server | grep "DEBUG SERVER"
# Expected: no output (exit code 1)

# Verify no debug endpoints
strings bin/spire-server | grep "_debug"
# Expected: no output (exit code 1)
```

## Architecture

Debug mode follows hexagonal architecture:

```
┌─────────────────────────────────────┐
│  Debug HTTP Server (Adapter)        │
│  - Only in //go:build debug         │
│  - Localhost only (127.0.0.1)       │
│  - Never compiled in production     │
└──────────────┬──────────────────────┘
               │
               v
┌─────────────────────────────────────┐
│  Debug Package (internal/debug)     │
│  - Config (runtime switches)        │
│  - Faults (injection profile)       │
│  - Logger (structured debug logs)   │
└──────────────┬──────────────────────┘
               │
               v
┌─────────────────────────────────────┐
│  Application Layer                  │
│  - Checks debug.Faults              │
│  - Uses debug.L.Debugf(...)         │
│  - Respects debug.Active settings   │
└─────────────────────────────────────┘
```

## Use Cases

### Reproduce Authentication Failures

```bash
# Start demo with debug mode
SPIRE_DEBUG_SERVER=true go run -tags=dev,debug ./cmd

# In another terminal, inject failure
curl -X POST http://localhost:6060/_debug/faults \
  -d '{"reject_next_workload_lookup": true}'

# Next authentication will fail
# Check debug logs for details
```

### Test Certificate Rotation

```bash
# Enable stress mode for fast rotation
SPIRE_DEBUG_STRESS=true go run -tags=dev,debug ./cmd

# Certificates will rotate every few seconds
# Test rotation logic without waiting hours
```

### Test Concurrency Issues

```bash
# Run in single-threaded mode
SPIRE_DEBUG_SINGLE_THREAD=true go run -tags=dev,debug ./cmd

# Auth flow runs synchronously
# Easier to reproduce ordering bugs
```

## Safety

Debug mode is designed with multiple safeguards:

1. **Build Tag Protection**: Debug server only compiles with `-tags=debug`
2. **Localhost Only**: Debug HTTP server binds to `127.0.0.1` (no remote access)
3. **Explicit Enable**: Requires `SPIRE_DEBUG_SERVER=true` environment variable
4. **Prominent Warnings**: Prints bold console warnings on startup
5. **No Secrets Exposed**: Debug endpoints avoid exposing private keys or credentials (state shows anonymized data)
6. **Audit Considerations**: Debug logs may contain PII or SPIFFE IDs; rotate logs or disable in shared environments
7. **No Authentication**: Debug server has no auth; rely on localhost binding for security

## Environment Variables Reference

| Variable | Description | Default |
|----------|-------------|---------|
| `SPIRE_DEBUG` | Enable debug mode globally | `false` |
| `SPIRE_DEBUG_STRESS` | Enable stress testing mode | `false` |
| `SPIRE_DEBUG_SINGLE_THREAD` | Disable goroutines | `false` |
| `SPIRE_DEBUG_SERVER` | Enable HTTP debug server | `false` |
| `SPIRE_DEBUG_ADDR` | Debug server address | `127.0.0.1:6060` |

## Example Session

```bash
# Terminal 1: Start with debug mode
$ SPIRE_DEBUG=true SPIRE_DEBUG_SERVER=true go run -tags=dev,debug ./cmd

⚠️  DEBUG SERVER RUNNING ON 127.0.0.1:6060
⚠️  WARNING: Debug mode is enabled. DO NOT USE IN PRODUCTION!

=== In-Memory SPIRE System Demo ===
...

# Terminal 2: View debug state
$ curl http://localhost:6060/_debug/state
{"debug_enabled":true,"stress_mode":false,"single_thread":false,...}

# Terminal 2: Inject a fault
$ curl -X POST http://localhost:6060/_debug/faults \
  -d '{"drop_next_handshake": true}'
{"drop_next_handshake":true,...}

# Terminal 2: Reset faults
$ curl -X POST http://localhost:6060/_debug/faults/reset
{"status":"reset"}
```
