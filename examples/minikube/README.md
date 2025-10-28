# Minikube mTLS Example

This example demonstrates production-like mTLS communication using e5s with SPIRE in Minikube.

## Prerequisites

- Minikube installed and running
- kubectl configured
- Helmfile installed (for SPIRE deployment)

## Architecture

```
┌─────────────┐           mTLS            ┌─────────────┐
│   Client    │ ◄──────────────────────► │   Server    │
│  (Go app)   │   SPIFFE IDs verified    │  (Go app)   │
└──────┬──────┘                           └──────┬──────┘
       │                                          │
       │  Workload API                            │  Workload API
       │  (Unix Socket)                           │  (Unix Socket)
       ▼                                          ▼
┌────────────────────────────────────────────────────┐
│              SPIRE Agent (DaemonSet)               │
│    /tmp/spire-agent/public/api.sock                │
└──────────────┬─────────────────────────────────────┘
               │ Node Attestation + Workload Attestation
               ▼
     ┌─────────────────┐
     │  SPIRE Server   │
     │   (StatefulSet) │
     └─────────────────┘
```

## Quick Start

### 1. Start Minikube and deploy SPIRE

```bash
# From repository root
cd examples/minikube

# Start cluster and deploy SPIRE
./scripts/cluster-up.sh

# Wait for SPIRE to be ready
kubectl wait --for=condition=ready pod -l app=spire-server -n spire --timeout=300s
kubectl wait --for=condition=ready pod -l app=spire-agent -n spire --timeout=300s
```

### 2. Register workloads with SPIRE

```bash
# Register server and client workload entries
./scripts/setup-spire-registrations.sh
```

This creates SPIFFE IDs:
- Server: `spiffe://example.org/server`
- Client: `spiffe://example.org/client`

### 3. Build and run the server

```bash
# Build server binary
go build -o bin/server ./cmd/server

# Deploy to Minikube (or run locally with socket access)
kubectl create configmap server-binary --from-file=bin/server -n spire
kubectl apply -f manifests/server-deployment.yaml

# Check server logs
kubectl logs -l app=mtls-server -n spire -f
```

### 4. Run the client

```bash
# Build client binary
go build -o bin/client ./cmd/client

# Run client (replace SERVER_ADDR with actual service address)
export SERVER_ADDR=https://mtls-server.spire.svc.cluster.local:8443
./bin/client
```

Expected output:
```
Response from server:
Hello, spiffe://example.org/client!
Trust Domain: example.org
Cert Expires: 2024-10-28T10:30:00Z
```

## Running Integration Tests

```bash
# Run full integration test suite
./scripts/run-integration-tests.sh

# Run tests in CI mode (with verbose output)
./scripts/run-integration-tests-ci.sh
```

## What's Happening

1. **SPIRE Agent** runs as a DaemonSet on each node, exposing the Workload API via Unix socket
2. **Server app** connects to the local SPIRE Agent socket and fetches its SVID (X.509 certificate with SPIFFE ID)
3. **Client app** also fetches its SVID from SPIRE Agent
4. **mTLS handshake**: Both present certificates, both verify the peer's SPIFFE ID against policy
5. **Certificate rotation**: SPIRE automatically rotates SVIDs before expiry, apps pick up new certs without restart

## Troubleshooting

### Server can't connect to SPIRE socket

```bash
# Check SPIRE Agent is running
kubectl get pods -n spire -l app=spire-agent

# Verify socket exists in pod
kubectl exec -n spire spire-agent-xxxxx -- ls -la /tmp/spire-agent/public/api.sock

# Check socket permissions
kubectl exec -n spire spire-agent-xxxxx -- stat /tmp/spire-agent/public/api.sock
```

### Workload not registered

```bash
# List all registered workload entries
kubectl exec -n spire spire-server-0 -- \
  /opt/spire/bin/spire-server entry show

# Re-run registration script
./scripts/setup-spire-registrations.sh
```

### mTLS handshake failures

```bash
# Enable debug logging in server/client
export SPIFFE_ENDPOINT_SOCKET=/tmp/spire-agent/public/api.sock
export LOG_LEVEL=debug

# Check server logs for verification errors
kubectl logs -l app=mtls-server -n spire --tail=50

# Verify trust domains match
kubectl exec -n spire spire-server-0 -- \
  /opt/spire/bin/spire-server bundle show
```

## Configuration

The examples use environment variables for configuration:

### Server Configuration

- `SPIFFE_ENDPOINT_SOCKET` - Path to SPIRE Agent socket (default: auto-detect)
- `PORT` - Server listen port (default: 8443)

### Client Configuration

- `SPIFFE_ENDPOINT_SOCKET` - Path to SPIRE Agent socket (default: auto-detect)
- `SERVER_ADDR` - Server URL to connect to (default: https://localhost:8443)
- `TRUST_DOMAIN` - Expected server trust domain (set in code: example.org)

### SPIRE Trust Domain

The examples use trust domain `example.org`. To change it:

1. Update `infra/values-minikube.yaml` (SPIRE server config)
2. Update `scripts/setup-spire-registrations.sh` (workload entries)
3. Update `cmd/client/main.go` (`ExpectedServerTrustDomain`)

## Cleanup

```bash
# Delete Minikube cluster
./scripts/cluster-down.sh

# Or just delete SPIRE namespace
kubectl delete namespace spire
```

## Directory Structure

```
examples/minikube/
├── cmd/
│   ├── server/main.go       # mTLS server application
│   └── client/main.go       # mTLS client application
├── infra/
│   ├── helmfile.yaml        # SPIRE deployment
│   └── values-minikube.yaml # Helm values
├── deploy/
│   └── values/              # Deployment-specific config
├── scripts/                 # All scripts (cluster + workload + tests)
│   ├── cluster-up.sh
│   ├── cluster-down.sh
│   ├── setup-spire-registrations.sh
│   ├── run-integration-tests.sh
│   └── ...
└── README.md                # This file
```

## Next Steps

- Modify `cmd/server/main.go` to add your API endpoints
- Update SPIRE registrations in `scripts/setup-spire-registrations.sh` for your workload selectors
- Customize server/client verification policies in `NewServerTLSConfig` / `NewClientTLSConfig`
- Deploy to production Kubernetes (see `deploy/values/values-prod.yaml`)
