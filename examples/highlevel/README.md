# High-Level API Example

**Application Developer Example** - Production behavior, simplest API.

This example demonstrates the high-level e5s API for building mTLS services with SPIRE. This is the recommended starting point for most users.

## What's Here

- `cmd/server/` - mTLS server using chi router
- `cmd/client/` - mTLS client that calls the server
- `e5s.yaml` - Configuration file (used by both)

## Features Demonstrated

- **Simple API**: `e5s.Start()` and `e5s.Client()` - no TLS code needed
- **Config-driven**: All SPIRE and mTLS settings in `e5s.yaml`
- **Identity extraction**: `e5s.PeerID()` in handlers
- **Automatic rotation**: SPIRE handles certificate renewal
- **Chi integration**: Works with any HTTP framework

## Prerequisites

You need a SPIRE deployment with registered workloads. See the [minikube-lowlevel example](../minikube-lowlevel/) for a complete SPIRE setup.

For this example to work:

1. SPIRE Agent must be running and reachable at the socket path in `e5s.yaml`
2. The server workload must be registered with a SPIFFE ID in the trust domain allowed by `server.allowed_client_*`
3. The client workload must be registered with a SPIFFE ID in the trust domain (or specific ID) expected by `client.expected_server_*`

## Environment Variables

Both server and client support these environment variables:

- `E5S_CONFIG` - Path to config file (default: `e5s.yaml`)
- `SERVER_ADDR` - Server URL for client (default: `https://localhost:8443`)

Examples:
```bash
# Use a different config file
E5S_CONFIG=/etc/e5s/prod.yaml ./bin/highlevel-server

# Connect to a remote server
SERVER_ADDR=https://api.example.org:8443 ./bin/highlevel-client

# Combine both
E5S_CONFIG=./custom.yaml SERVER_ADDR=https://remote:8443 ./bin/highlevel-client
```

## Configuration

The `e5s.yaml` file configures both server and client:

```yaml
spire:
  workload_socket: unix:///tmp/spire-agent/public/api.sock

server:
  listen_addr: ":8443"
  # Accept any client in this trust domain
  allowed_client_trust_domain: "example.org"

client:
  # Connect to any server in this trust domain
  expected_server_trust_domain: "example.org"
```

For production, use specific SPIFFE IDs instead of trust domains:

```yaml
server:
  allowed_client_spiffe_id: "spiffe://example.org/frontend"

client:
  expected_server_spiffe_id: "spiffe://example.org/api-server"
```

## Running the Example

### Build

```bash
# From examples/highlevel/
go build -o bin/highlevel-server ./cmd/server
go build -o bin/highlevel-client ./cmd/client
```

### Run Server

```bash
./bin/highlevel-server
```

Output:
```
Starting e5s mTLS server (config: e5s.yaml)...
✓ Server listening on :8443
Health checks: /healthz, /healthz/ready
Press Ctrl+C for graceful shutdown
→ GET /hello from 127.0.0.1:54321
← GET /hello → 200
```

The server provides:
- **Graceful shutdown** - Handles SIGINT/SIGTERM and drains connections
- **Health checks** - Lightweight health endpoints intended for readiness/liveness. In this example they're served on the same listener as the authenticated routes. In production you can expose them on a separate unauthenticated port if your platform requires that.
- **Structured logging** - Redacts sensitive query params, skips health check noise
- **Dynamic config** - Reads listen address from config (no hardcoded values)

### Run Client

In another terminal:

```bash
./bin/highlevel-client
```

(Example output; SPIFFE IDs and expiry will reflect your environment)

Output:
```
Creating mTLS client (config: e5s.yaml)...
Client created successfully
Testing GET https://localhost:8443/

=== Response from https://localhost:8443/ ===
Status: 200 OK
Server Identity: spiffe://example.org/server
Body:
Welcome to the e5s mTLS server

Testing GET https://localhost:8443/hello
=== Response from https://localhost:8443/hello ===
Status: 200 OK
Server Identity: spiffe://example.org/server
Body:
Hello, spiffe://example.org/client!

Testing GET https://localhost:8443/api/status
=== Response from https://localhost:8443/api/status ===
Status: 200 OK
Server Identity: spiffe://example.org/server
Body:
{"status":"ok","authenticated_as":"spiffe://example.org/client","trust_domain":"example.org","cert_expires":"2024-10-28T12:30:00Z"}

✓ All requests completed successfully!
```

Note how the client logs the **Server Identity** - this demonstrates mutual TLS verification. Both client and server authenticate each other.

## Health Check Endpoints

The server provides health check endpoints for Kubernetes liveness/readiness probes:

```bash
# Liveness probe
curl -k https://localhost:8443/healthz
ok

# Readiness probe
curl -k https://localhost:8443/healthz/ready
ready
```

These endpoints:
- Are lightweight and fast for probes
- Are excluded from noisy request logging
- Can be moved to a separate unauthenticated port in production if needed

In this example, all endpoints (including `/healthz`) are served on the same mTLS listener. If your platform requires truly unauthenticated health checks, you'll need to run a separate HTTP listener on a different port.

## How It Works

### Server Code

```go
r := chi.NewRouter()

r.Get("/hello", func(w http.ResponseWriter, req *http.Request) {
    id, ok := e5s.PeerID(req)
    if !ok {
        http.Error(w, "unauthorized", http.StatusUnauthorized)
        return
    }
    fmt.Fprintf(w, "Hello, %s!\n", id)
})

shutdown, err := e5s.Start("e5s.yaml", r)
if err != nil {
    log.Fatal(err)
}
defer shutdown()
```

The server:
1. Loads config from `e5s.yaml`
2. Connects to SPIRE Agent
3. Starts mTLS server with automatic cert rotation
4. Injects peer identity into request context
5. Handlers use `e5s.PeerID()` to get authenticated caller

### Client Code

```go
client, shutdown, err := e5s.Client("e5s.yaml")
if err != nil {
    log.Fatal(err)
}
defer shutdown()

resp, err := client.Get("https://localhost:8443/hello")
```

The client:
1. Loads config from `e5s.yaml`
2. Connects to SPIRE Agent
3. Returns standard `*http.Client` with mTLS
4. Automatically presents SPIFFE ID to servers
5. Verifies server identity per config policy

## What You Don't See

All of this is handled internally by e5s:

- SPIRE Workload API connection
- Certificate fetching and rotation
- TLS 1.3 configuration
- mTLS handshake setup
- Trust bundle management
- SPIFFE ID verification
- Shutdown sequencing

You just use `e5s.Start()`, `e5s.Client()`, and `e5s.PeerID()`.

## Next Steps

- See [minikube-lowlevel example](../minikube-lowlevel/) for complete SPIRE cluster setup
- Read [API documentation](../../docs/QUICKSTART_LIBRARY.md) for lower-level usage
- Check [security documentation](../../security/) for production hardening
