# SPIRE mTLS Examples - Ubuntu 24.04 Setup Guide

Step-by-step instructions for running the mTLS server examples on Ubuntu 24.04.

## Table of Contents

1. [Prerequisites](#prerequisites)
2. [Install SPIRE](#install-spire)
3. [Start SPIRE Server](#start-spire-server)
4. [Start SPIRE Agent](#start-spire-agent)
5. [Create Registration Entries](#create-registration-entries)
6. [Run the Example Server](#run-the-example-server)
7. [Test the Server](#test-the-server)
8. [Troubleshooting](#troubleshooting)

---

## Prerequisites

### 1. Install Go 1.25+

```bash
# Download Go 1.25 (or later)
wget https://go.dev/dl/go1.25.0.linux-amd64.tar.gz

# Remove old Go installation (if exists)
sudo rm -rf /usr/local/go

# Extract and install
sudo tar -C /usr/local -xzf go1.25.0.linux-amd64.tar.gz

# Add to PATH (add to ~/.bashrc for persistence)
export PATH=$PATH:/usr/local/go/bin
export GOPATH=$HOME/go
export PATH=$PATH:$GOPATH/bin

# Verify installation
go version
# Expected output: go version go1.25.0 linux/amd64
```

If you already have Go 1.25+ installed, skip this step and verify with `go version`.

### 2. Clone this Repository

```bash
cd ~
git clone https://github.com/pocket/hexagon.git
cd hexagon/spire
```

---

## Install SPIRE

### Download SPIRE

```bash
# Create SPIRE directory
mkdir -p ~/spire
cd ~/spire

# Download SPIRE 1.13.2 (latest stable)
wget https://github.com/spiffe/spire/releases/download/v1.13.2/spire-1.13.2-linux-amd64-musl.tar.gz

# Extract
tar -xzf spire-1.13.2-linux-amd64-musl.tar.gz
cd spire-1.13.2

# Make binaries executable
chmod +x bin/spire-server bin/spire-agent
```

### Verify Installation

```bash
./bin/spire-server --version
./bin/spire-agent --version
# Expected output: spire-server 1.13.2 / spire-agent 1.13.2
```

---

## Start SPIRE Server

### 1. Create Server Configuration

```bash
mkdir -p ~/spire/config
cat > ~/spire/config/server.conf <<'EOF'
server {
    bind_address = "127.0.0.1"
    bind_port = "8081"
    trust_domain = "example.org"
    data_dir = "/tmp/spire-server/data"
    log_level = "DEBUG"
}

plugins {
    DataStore "sql" {
        plugin_data {
            database_type = "sqlite3"
            connection_string = "/tmp/spire-server/data/datastore.sqlite3"
        }
    }

    NodeAttestor "join_token" {
        plugin_data {}
    }

    KeyManager "disk" {
        plugin_data {
            keys_path = "/tmp/spire-server/data/keys.json"
        }
    }
}
EOF
```

### 2. Start the Server

```bash
# Create data directory
mkdir -p /tmp/spire-server/data

# Start server (run in a separate terminal or use nohup)
cd ~/spire/spire-1.13.2
./bin/spire-server run -config ~/spire/config/server.conf &

# Or run in foreground (new terminal):
./bin/spire-server run -config ~/spire/config/server.conf
```

### 3. Verify Server is Running

```bash
# Check server health
./bin/spire-server healthcheck

# Expected output:
# Server is healthy.
```

---

## Start SPIRE Agent

### 1. Generate Join Token

In a new terminal:

```bash
cd ~/spire/spire-1.13.2

# Generate a join token for the agent
JOIN_TOKEN=$(./bin/spire-server token generate -spiffeID spiffe://example.org/host -output json | jq -r '.value')

echo "Join token: $JOIN_TOKEN"
```

### 2. Create Agent Configuration

```bash
cat > ~/spire/config/agent.conf <<'EOF'
agent {
    trust_domain = "example.org"
    data_dir = "/tmp/spire-agent/data"
    log_level = "DEBUG"
    server_address = "127.0.0.1"
    server_port = "8081"
    socket_path = "/tmp/spire-agent/public/api.sock"
}

plugins {
    NodeAttestor "join_token" {
        plugin_data {}
    }

    KeyManager "disk" {
        plugin_data {
            directory = "/tmp/spire-agent/data"
        }
    }

    WorkloadAttestor "unix" {
        plugin_data {}
    }
}
EOF
```

### 3. Start the Agent

```bash
# Create data and socket directories
mkdir -p /tmp/spire-agent/data
mkdir -p /tmp/spire-agent/public

# Start agent with join token (new terminal or background)
cd ~/spire/spire-1.13.2
./bin/spire-agent run -config ~/spire/config/agent.conf -joinToken $JOIN_TOKEN &

# Or run in foreground (new terminal):
./bin/spire-agent run -config ~/spire/config/agent.conf -joinToken $JOIN_TOKEN
```

### 4. Verify Agent is Running

```bash
# Check socket exists
ls -la /tmp/spire-agent/public/api.sock
# Expected: srwxr-xr-x 1 user user 0 Oct 16 12:34 /tmp/spire-agent/public/api.sock

# Check agent health
./bin/spire-agent healthcheck -socketPath /tmp/spire-agent/public/api.sock
# Expected: Agent is healthy.
```

---

## Create Registration Entries

Registration entries map workload identities to SPIFFE IDs.

### 1. Register the Example Server

```bash
cd ~/spire/spire-1.13.2

# Get your user UID
USER_UID=$(id -u)

# Create server registration entry
./bin/spire-server entry create \
    -spiffeID spiffe://example.org/server \
    -parentID spiffe://example.org/host \
    -selector unix:uid:$USER_UID \
    -dns localhost \
    -dns 127.0.0.1

# Expected output:
# Entry ID         : <uuid>
# SPIFFE ID        : spiffe://example.org/server
# Parent ID        : spiffe://example.org/host
# ...
```

### 2. Register a Client (for testing)

```bash
# Create client registration entry
./bin/spire-server entry create \
    -spiffeID spiffe://example.org/client \
    -parentID spiffe://example.org/host \
    -selector unix:uid:$USER_UID

# Expected output:
# Entry ID         : <uuid>
# SPIFFE ID        : spiffe://example.org/client
# ...
```

### 3. Verify Registration Entries

```bash
# List all entries
./bin/spire-server entry show

# You should see both server and client entries
```

---

## Run the Example Server

### 1. Build the Example

```bash
cd ~/hexagon/spire
go build -o /tmp/mtls-server ./examples/identityserver-example
```

### 2. Configure Environment Variables

The server supports these environment variables:

| Variable | Description | Default |
|----------|-------------|---------|
| `SPIFFE_ENDPOINT_SOCKET` | SPIRE agent socket path (SPIFFE standard) | - |
| `SPIRE_AGENT_SOCKET` | SPIRE agent socket path (fallback) | `unix:///tmp/spire-agent/public/api.sock` |
| `ALLOWED_CLIENT_ID` | Specific SPIFFE ID to allow | `spiffe://example.org/client` |
| `ALLOWED_TRUST_DOMAIN` | Trust domain to allow (any ID) | - |
| `SERVER_ADDRESS` | Server bind address | `:8443` |

### 3. Start the Server

**Option A: Default Configuration** (allows `spiffe://example.org/client`)

```bash
# Use default socket path and allowed client
/tmp/mtls-server
```

**Option B: Custom Configuration**

```bash
# Custom socket path and allow entire trust domain
export SPIFFE_ENDPOINT_SOCKET="unix:///tmp/spire-agent/public/api.sock"
export ALLOWED_TRUST_DOMAIN="example.org"
export SERVER_ADDRESS=":8443"

/tmp/mtls-server
```

### 4. Verify Server Started

```
Creating mTLS server with configuration:
  Socket: unix:///tmp/spire-agent/public/api.sock
  Address: :8443
  Allowed peer: spiffe://example.org/client
✓ Server created and handlers registered successfully
Listening on :8443 with mTLS authentication
Press Ctrl+C to stop
```

---

## Test the Server

### Option 1: Create a Simple Client

Create a test client to verify mTLS:

```bash
cd ~/hexagon/spire
cat > /tmp/test-client.go <<'EOF'
package main

import (
    "context"
    "fmt"
    "io"
    "log"
    "net/http"
    "time"

    "github.com/pocket/hexagon/spire/internal/adapters/outbound/httpclient"
    "github.com/pocket/hexagon/spire/internal/ports"
)

func main() {
    ctx := context.Background()

    // Configure mTLS client
    var cfg ports.MTLSConfig
    cfg.WorkloadAPI.SocketPath = "unix:///tmp/spire-agent/public/api.sock"
    cfg.SPIFFE.AllowedTrustDomain = "example.org"
    cfg.HTTP.ReadTimeout = 10 * time.Second

    // Create client
    client, err := httpclient.New(ctx, cfg)
    if err != nil {
        log.Fatalf("Failed to create client: %v", err)
    }
    defer client.Close()

    // Test endpoints
    endpoints := []string{
        "https://localhost:8443/",
        "https://localhost:8443/api/hello",
        "https://localhost:8443/api/identity",
        "https://localhost:8443/health",
    }

    for _, url := range endpoints {
        fmt.Printf("\n=== Testing %s ===\n", url)
        req, _ := http.NewRequestWithContext(ctx, "GET", url, nil)
        resp, err := client.Do(ctx, req)
        if err != nil {
            log.Printf("Request failed: %v", err)
            continue
        }

        body, _ := io.ReadAll(resp.Body)
        resp.Body.Close()

        fmt.Printf("Status: %d\n", resp.StatusCode)
        fmt.Printf("Body: %s\n", string(body))
    }
}
EOF

# Build and run the client
go run /tmp/test-client.go
```

**Expected Output:**

```
=== Testing https://localhost:8443/ ===
Status: 200
Body: Success! Authenticated as: spiffe://example.org/client

=== Testing https://localhost:8443/api/hello ===
Status: 200
Body: {"identity":"spiffe://example.org/client","message":"Hello from mTLS server!"}

=== Testing https://localhost:8443/api/identity ===
Status: 200
Body: {"identity":{"path":"/client","spiffe_id":"spiffe://example.org/client","trust_domain":"example.org"},"request":{"method":"GET","remote_addr":"127.0.0.1:xxxxx","url":"/api/identity"}}

=== Testing https://localhost:8443/health ===
Status: 200
Body: {"status":"healthy"}
```

### Option 2: Use spire-agent api fetch (for debugging)

```bash
# Fetch workload SVID (verifies agent is working)
cd ~/spire/spire-1.13.2
./bin/spire-agent api fetch x509 -socketPath /tmp/spire-agent/public/api.sock

# Expected: Shows X.509 SVID details with SPIFFE ID spiffe://example.org/server
```

---

## Available Endpoints

The example server exposes these endpoints:

### 1. `GET /` - Root endpoint
Returns plain text greeting with authenticated SPIFFE ID.

```bash
# Response: Success! Authenticated as: spiffe://example.org/client
```

### 2. `GET /api/hello` - Hello endpoint
Returns JSON greeting with identity.

```json
{
  "message": "Hello from mTLS server!",
  "identity": "spiffe://example.org/client"
}
```

### 3. `GET /api/identity` - Identity details
Returns detailed identity and request information.

```json
{
  "identity": {
    "spiffe_id": "spiffe://example.org/client",
    "trust_domain": "example.org",
    "path": "/client"
  },
  "request": {
    "method": "GET",
    "url": "/api/identity",
    "remote_addr": "127.0.0.1:xxxxx"
  }
}
```

### 4. `GET /health` - Health check
Returns server health status (no authentication).

```json
{
  "status": "healthy"
}
```

---

## Troubleshooting

### 1. "Failed to create server: failed to create X509Source"

**Problem**: Cannot connect to SPIRE agent.

**Solution**:
```bash
# Check agent is running
ps aux | grep spire-agent

# Check socket exists
ls -la /tmp/spire-agent/public/api.sock

# Check socket permissions
sudo chmod 777 /tmp/spire-agent/public/api.sock

# Restart agent if needed
pkill spire-agent
./bin/spire-agent run -config ~/spire/config/agent.conf -joinToken $JOIN_TOKEN &
```

### 2. "Failed to create server: failed to parse allowed peer ID"

**Problem**: Invalid SPIFFE ID format.

**Solution**:
```bash
# Ensure SPIFFE ID has correct format: spiffe://trust-domain/path
export ALLOWED_CLIENT_ID="spiffe://example.org/client"

# Or use trust domain (allows any ID in domain)
unset ALLOWED_CLIENT_ID
export ALLOWED_TRUST_DOMAIN="example.org"
```

### 3. "Server error: bind: address already in use"

**Problem**: Port 8443 is already in use.

**Solution**:
```bash
# Check what's using port 8443
sudo lsof -i :8443

# Kill the process or use a different port
export SERVER_ADDRESS=":9443"
/tmp/mtls-server
```

### 4. "No workload SVID found"

**Problem**: No registration entry matches your workload.

**Solution**:
```bash
# Check your UID
id -u

# List registration entries
cd ~/spire/spire-1.13.2
./bin/spire-server entry show

# Create entry with correct UID
./bin/spire-server entry create \
    -spiffeID spiffe://example.org/server \
    -parentID spiffe://example.org/host \
    -selector unix:uid:$(id -u)

# Wait a few seconds for agent to fetch new SVID
sleep 5
```

### 5. Client connection fails with TLS handshake error

**Problem**: Client cannot verify server certificate.

**Solution**:
```bash
# Ensure client has valid registration entry
./bin/spire-server entry show | grep client

# Create client entry if missing
./bin/spire-server entry create \
    -spiffeID spiffe://example.org/client \
    -parentID spiffe://example.org/host \
    -selector unix:uid:$(id -u)

# Verify both server and client can fetch SVIDs
./bin/spire-agent api fetch x509 -socketPath /tmp/spire-agent/public/api.sock
```

### 6. Check Logs

**Server logs:**
```bash
# Server logs are printed to stdout
# Increase verbosity by checking SPIRE agent logs
```

**SPIRE Server logs:**
```bash
# Server logs (if running in background)
tail -f /tmp/spire-server/spire-server.log

# Or check stdout if running in foreground
```

**SPIRE Agent logs:**
```bash
# Agent logs (if running in background)
tail -f /tmp/spire-agent/spire-agent.log

# Or check stdout if running in foreground
```

---

## Clean Up

To stop all services and clean up:

```bash
# Stop server (Ctrl+C or)
pkill -f mtls-server

# Stop SPIRE agent
pkill spire-agent

# Stop SPIRE server
pkill spire-server

# Clean up data directories
rm -rf /tmp/spire-server /tmp/spire-agent

# Remove built binary
rm /tmp/mtls-server
```

---

## Next Steps

- **Read the documentation**: Check `../docs/` for architecture details
- **Customize handlers**: Modify `identityserver-example/main.go` to add your own endpoints
- **Add authorization**: Implement application-level access control based on SPIFFE IDs
- **Production deployment**: See `docs/CONTROL_PLANE.md` for Kubernetes deployment with Minikube

---

## References

- [SPIRE Documentation](https://spiffe.io/docs/latest/spire/)
- [SPIFFE Specification](https://github.com/spiffe/spiffe)
- [go-spiffe SDK](https://github.com/spiffe/go-spiffe)
- [Project Documentation](../docs/ARCHITECTURE.md)
