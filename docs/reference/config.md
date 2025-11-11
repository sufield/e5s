# e5s Configuration Reference

This document defines the e5s configuration file format (e5s.yaml).

## Overview

e5s uses a single YAML configuration file for both servers and clients. This design:
- Simplifies deployment (one config format to learn)
- Enables shared configuration (SPIRE socket, timeouts)
- Supports language-independent tooling (any YAML parser works)

The config format is **versioned** to support future evolution while maintaining backward compatibility.

## Version

**Current Version**: 1

The config format uses semantic versioning:
- Version 1: Initial format (all current fields)
- Future versions will add/change fields while maintaining compatibility where possible

## Configuration File Structure

```yaml
# Optional version field (defaults to 1 if omitted)
version: 1

# SPIRE connection settings (required for all modes)
spire:
  workload_socket: "unix:///tmp/spire-agent/public/api.sock"
  initial_fetch_timeout: "30s"

# Server settings (required for server mode)
server:
  listen_addr: ":8443"
  # Choose ONE of:
  allowed_client_spiffe_id: "spiffe://example.org/client"
  # OR
  allowed_client_trust_domain: "example.org"

# Client settings (required for client mode)
client:
  server_url: "https://localhost:8443/api"
  # Choose ONE of:
  expected_server_spiffe_id: "spiffe://example.org/server"
  # OR
  expected_server_trust_domain: "example.org"
```

## Top-Level Fields

### `version` (integer, optional)

The config file format version. Currently always `1`.

**Default**: 1 (if omitted)

**Purpose**: Enables future format changes while maintaining backward compatibility.

**Example**:
```yaml
version: 1
```

**Notes**:
- Version 0 (unspecified) is treated as version 1
- Future e5s versions may support higher version numbers
- Using an unsupported version will result in an error

---

## `spire` Section (required)

Configures the connection to the SPIRE Workload API.

### `workload_socket` (string, required)

Path to the SPIRE Agent's Workload API socket.

**Format**: `unix:///path/to/socket` or `/path/to/socket`

**Example**:

```yaml
spire:
  workload_socket: "unix:///tmp/spire-agent/public/api.sock"
```

**Common values**:

- Linux/Mac: `unix:///tmp/spire-agent/public/api.sock`
- Kubernetes: `unix:///run/spire/sockets/agent.sock`
- Custom deployment: Check your SPIRE agent configuration

**Notes**:

- The `unix://` prefix is optional but recommended for clarity
- Socket must be accessible by the e5s process
- SPIRE agent must be running and healthy

### `initial_fetch_timeout` (duration string, optional)

How long to wait for the first SVID/Bundle from the Workload API before giving up.

**Format**: Go duration string (`5s`, `30s`, `1m`, `1m30s`, etc.)

**Default**: `30s`

**Example**:

```yaml
spire:
  initial_fetch_timeout: "45s"
```

**Recommendations**:

- Development: `10s` - `30s` (fail fast for quick iteration)
- Production: `30s` - `60s` (tolerate network/load delays)
- CI/CD: `15s` - `30s` (balance speed vs reliability)

**Notes**:

- Only affects startup time, not runtime rotation
- After initial fetch, certificates rotate automatically
- Too short: May fail during heavy load or slow networks
- Too long: Delays error detection during startup

---

## `server` Section (required for server mode)

Configures mTLS server behavior and client authorization.

### `listen_addr` (string, required)

Address and port for the HTTPS server to listen on.

**Format**: `host:port` or `:port`

**Examples**:

```yaml
server:
  listen_addr: ":8443"              # Listen on all interfaces, port 8443
  listen_addr: "localhost:8443"     # Listen only on localhost
  listen_addr: "0.0.0.0:443"        # Listen on all IPv4 interfaces, port 443
  listen_addr: "[::]:8443"          # Listen on all IPv6 interfaces
```

**Common values**:

- `:8443` - Standard mTLS port (all interfaces)
- `:443` - HTTPS port (requires root/CAP_NET_BIND_SERVICE)
- `localhost:8443` - Local development only

**Notes**:

- Empty host means "all interfaces"
- Port must not be in use by another process
- Ports < 1024 require elevated privileges on Linux/Mac

### Client Authorization (mutually exclusive)

Choose **exactly one** of the following authorization modes:

#### `allowed_client_spiffe_id` (string, optional)

Allow connections from a **specific client SPIFFE ID only**.

**Format**: Full SPIFFE ID string

**Example**:

```yaml
server:
  allowed_client_spiffe_id: "spiffe://example.org/client/api-client"
```

**Use when**:

- You know the exact client SPIFFE ID
- You want maximum security (zero-trust, specific identity)
- Single client or known set of clients

**Security**: Strongest - only one specific identity allowed

#### `allowed_client_trust_domain` (string, optional)

Allow connections from **any client in the specified trust domain**.

**Format**: Trust domain only (no `spiffe://` prefix)

**Example**:

```yaml
server:
  allowed_client_trust_domain: "example.org"
```

**Use when**:

- Multiple clients from the same organization/environment
- Trust all workloads in your SPIRE deployment
- Development/testing environments

**Security**: Permissive - any workload in the trust domain can connect

**Notes**:

- More convenient but less secure than specific ID
- Suitable for internal microservices within same trust boundary
- Consider using specific ID for external-facing services

---

## `client` Section (required for client mode)

Configures mTLS client behavior and server verification.

### `server_url` (string, optional)

The HTTPS URL of the server to connect to.

**Format**: `https://host:port/path`

**Examples**:

```yaml
client:
  server_url: "https://localhost:8443/api"
  server_url: "https://e5s-server.prod.svc.cluster.local:8443/time"
  server_url: "https://api.example.org:443/v1/data"
```

**Notes**:

- Must use `https://` scheme (mTLS requires TLS)
- Port is required
- Path is optional but often needed

### Server Verification (mutually exclusive)

Choose **exactly one** of the following verification modes:

#### `expected_server_spiffe_id` (string, optional)

Verify the server has a **specific SPIFFE ID**.

**Format**: Full SPIFFE ID string

**Example**:

```yaml
client:
  expected_server_spiffe_id: "spiffe://example.org/server/api-server"
```

**Use when**:

- You know the exact server SPIFFE ID
- You want maximum security (prevent impersonation)
- Connecting to a specific known service

**Security**: Strongest - only one specific server identity accepted

#### `expected_server_trust_domain` (string, optional)

Accept **any server in the specified trust domain**.

**Format**: Trust domain only (no `spiffe://` prefix)

**Example**:

```yaml
client:
  expected_server_trust_domain: "example.org"
```

**Use when**:

- Connecting to multiple servers in the same organization
- Dynamic server discovery (load balancers, service mesh)
- Development/testing environments

**Security**: Permissive - any workload in the trust domain is trusted

**Notes**:

- More flexible but less secure than specific ID
- Suitable for internal microservices
- Consider using specific ID for sensitive connections

---

## Complete Examples

### Example 1: Production Server (Zero-Trust)

```yaml
version: 1

spire:
  workload_socket: "unix:///run/spire/sockets/agent.sock"
  initial_fetch_timeout: "45s"

server:
  listen_addr: ":8443"
  # Only allow connections from the specific API client
  allowed_client_spiffe_id: "spiffe://prod.example.org/ns/api/sa/api-client"
```

**Use case**: Production API server that only accepts connections from a specific client workload.

### Example 2: Development Server (Trust Domain)

```yaml
version: 1

spire:
  workload_socket: "unix:///tmp/spire-agent/public/api.sock"
  initial_fetch_timeout: "15s"

server:
  listen_addr: ":8443"
  # Accept any client in the development trust domain
  allowed_client_trust_domain: "dev.example.org"
```

**Use case**: Development server that accepts connections from any workload in the dev environment.

### Example 3: Production Client (Zero-Trust)

```yaml
version: 1

spire:
  workload_socket: "unix:///run/spire/sockets/agent.sock"
  initial_fetch_timeout: "30s"

client:
  server_url: "https://api-server.prod.svc.cluster.local:8443/v1"
  # Only connect if server has this exact SPIFFE ID
  expected_server_spiffe_id: "spiffe://prod.example.org/ns/api/sa/api-server"
```

**Use case**: Production client that verifies the exact identity of the API server.

### Example 4: Internal Microservice Client

```yaml
version: 1

spire:
  workload_socket: "unix:///run/spire/sockets/agent.sock"
  initial_fetch_timeout: "30s"

client:
  server_url: "https://backend-service:8443/data"
  # Accept any server in our trust domain
  expected_server_trust_domain: "prod.example.org"
```

**Use case**: Microservice client connecting to various backend services in the same trust domain.

### Example 5: Unified Config (Server + Client)

```yaml
version: 1

spire:
  workload_socket: "unix:///run/spire/sockets/agent.sock"
  initial_fetch_timeout: "30s"

server:
  listen_addr: ":8443"
  allowed_client_trust_domain: "example.org"

client:
  server_url: "https://upstream-service:8443/api"
  expected_server_spiffe_id: "spiffe://example.org/upstream"
```

**Use case**: Workload that acts as both server (receiving requests) and client (making requests to upstream service).

---

## Validation Rules

The config package enforces these rules:

### SPIRE Section

✅ **Valid**:
- `workload_socket` is set and non-empty
- `initial_fetch_timeout` is valid Go duration format (if specified)
- `initial_fetch_timeout` is positive (if specified)

❌ **Invalid**:
- Missing `workload_socket`
- Empty `workload_socket`
- Invalid duration format (e.g., `"30"`, `"xyz"`)
- Negative or zero timeout

### Server Section

✅ **Valid**:
- `listen_addr` is set and non-empty
- Exactly one of `allowed_client_spiffe_id` or `allowed_client_trust_domain` is set
- SPIFFE ID is well-formed (if using ID-based authz)
- Trust domain is well-formed (if using trust-domain-based authz)

❌ **Invalid**:
- Missing `listen_addr`
- Both `allowed_client_spiffe_id` AND `allowed_client_trust_domain` set
- Neither `allowed_client_spiffe_id` nor `allowed_client_trust_domain` set
- Malformed SPIFFE ID (e.g., missing `spiffe://`)
- Malformed trust domain

### Client Section

✅ **Valid**:
- Exactly one of `expected_server_spiffe_id` or `expected_server_trust_domain` is set
- SPIFFE ID is well-formed (if using ID-based verification)
- Trust domain is well-formed (if using trust-domain-based verification)

❌ **Invalid**:
- Both `expected_server_spiffe_id` AND `expected_server_trust_domain` set
- Neither `expected_server_spiffe_id` nor `expected_server_trust_domain` set
- Malformed SPIFFE ID
- Malformed trust domain

**Note**: `server_url` is optional in the client section because it can be specified programmatically via the API.

---

## Environment Variable Overrides

The config package itself does NOT handle environment variables. Environment variable substitution (if needed) happens in the application layer (`cmd/` or user code), NOT in the config parser.

This design keeps the config format language-independent and tool-friendly.

Example of environment variable handling in application code:

```go
// Application code (cmd/), not config package
cfg, _ := config.Load("e5s.yaml")

// Override from environment (application's responsibility)
if url := os.Getenv("SERVER_URL"); url != "" {
    cfg.Client.ServerURL = url
}
```

---

## Future Evolution

The versioned format enables future changes:

### Potential Version 2 Features

```yaml
version: 2

spire:
  workload_socket: "unix:///run/spire/agent.sock"
  # NEW: Support for multiple trust bundles (federation)
  trust_bundles:
    - trust_domain: "partner.org"
      bundle_path: "/etc/spire/bundles/partner.pem"

server:
  listen_addr: ":8443"
  # NEW: Rate limiting
  rate_limit:
    requests_per_second: 100
    burst: 200
  # NEW: Health check endpoint
  health_check_path: "/healthz"

  allowed_client_trust_domain: "example.org"

# NEW: Observability section
observability:
  metrics_addr: ":9090"
  tracing_endpoint: "jaeger:14268"
```

### Backward Compatibility Strategy

When introducing version 2:
1. Version 1 configs continue to work unchanged
2. New fields are optional (have sensible defaults)
3. Version 2 parser can read version 1 configs
4. Clear migration guide provided

---

## Configuration Best Practices

### Security

1. **Use Specific IDs in Production**

   ```yaml
   # Good: Zero-trust
   allowed_client_spiffe_id: "spiffe://prod.example.org/client"

   # Acceptable for development
   allowed_client_trust_domain: "dev.example.org"
   ```

2. **Protect Config Files**

   ```bash
   # Config files may contain sensitive paths/settings
   chmod 600 e5s.yaml
   chown app-user:app-group e5s.yaml
   ```

3. **Version Your Configs**

   ```yaml
   # Always include version for future-proofing
   version: 1
   ```

### Operations

1. **Use Appropriate Timeouts**

   ```yaml
   # Production: tolerate delays
   initial_fetch_timeout: "45s"

   # Development: fail fast
   initial_fetch_timeout: "15s"
   ```

2. **Document Your Choices**

   ```yaml
   # Why using trust domain instead of specific ID
   server:
      # Allow any workload in prod environment (internal services only)
     allowed_client_trust_domain: "prod.example.org"
   ```

### Deployment

1. **One Config Per Environment**

   ```
   config/
   ├── dev.yaml
   ├── staging.yaml
   └── prod.yaml
   ```

2. **Validate Before Deploying**

   ```bash
   e5s validate prod.yaml
   ```

3. **Use ConfigMaps in Kubernetes**

   ```bash
   kubectl create configmap e5s-config --from-file=e5s.yaml
   ```

---

## Troubleshooting

### "workload_socket must be set"

**Cause**: Missing or empty `spire.workload_socket`

**Fix**:
```yaml
spire:
  workload_socket: "unix:///tmp/spire-agent/public/api.sock"
```

### "must set exactly one of..."

**Cause**: Both ID and trust domain specified, or neither specified

**Fix** (choose one):
```yaml
# Option 1: Specific ID
allowed_client_spiffe_id: "spiffe://example.org/client"

# Option 2: Trust domain
allowed_client_trust_domain: "example.org"
```

### "invalid SPIFFE ID"

**Cause**: Malformed SPIFFE ID

**Fix**: Ensure format is `spiffe://trust-domain/path`
```yaml
# Good
allowed_client_spiffe_id: "spiffe://example.org/ns/default/sa/client"

# Bad (missing spiffe://)
allowed_client_spiffe_id: "example.org/client"
```

### "unsupported config version"

**Cause**: Config version is higher than what this e5s version supports

**Fix**: Either:
1. Upgrade e5s to a newer version
2. Downgrade config file to version 1
3. Remove `version:` field (defaults to 1)

---

## Config Format Stability

The e5s config format is a **stable protocol**:
- Version 1 will be supported indefinitely
- Future versions will maintain backward compatibility where possible
- Breaking changes will be clearly documented with migration guides
- The format is language-independent (any YAML parser works)

This makes e5s configs suitable for:
- Long-lived deployments
- Infrastructure-as-code (Terraform, Ansible)
- CI/CD pipelines
- Multi-language tooling
