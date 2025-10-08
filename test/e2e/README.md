# End-to-End Test Suite

This directory contains end-to-end tests that verify the complete workload identity flow across multiple services using SPIRE.

## Overview

The E2E tests deploy three services in Kubernetes and verify:
- Multi-service workload attestation
- Mutual TLS (mTLS) authentication between services
- Certificate chain validation
- Identity rotation mechanisms
- Authorization and access control

## Test Architecture

```
┌─────────────┐       mTLS        ┌─────────────┐       mTLS        ┌─────────────┐
│  Service A  │ ───────────────> │  Service B  │ ───────────────> │  Service C  │
│  (Client)   │    Authorize B    │  (Server)   │    Authorize C    │ (Database)  │
│  UID: 1000  │                   │  UID: 1001  │                   │  UID: 1002  │
└─────────────┘                   └─────────────┘                   └─────────────┘
       │                                 │                                 │
       └─────────────────────────────────┴─────────────────────────────────┘
                            Attest via SPIRE Agent
                                      │
                            ┌─────────┴──────────┐
                            │   SPIRE Server     │
                            │  (CA + Registry)   │
                            └────────────────────┘
```

## Services

### Service A (Client)
- **SPIFFE ID**: `spiffe://example.org/service-a`
- **Role**: HTTP client
- **Behavior**: Makes mTLS requests to Service B
- **Selector**: `k8s:sa:service-a`

### Service B (Server)
- **SPIFFE ID**: `spiffe://example.org/service-b`
- **Role**: HTTP server + client
- **Behavior**:
  - Accepts mTLS connections from Service A
  - Makes mTLS requests to Service C
- **Selector**: `k8s:sa:service-b`

### Service C (Database)
- **SPIFFE ID**: `spiffe://example.org/service-c`
- **Role**: HTTP server (simulates database)
- **Behavior**: Accepts mTLS connections from Service B
- **Selector**: `k8s:sa:service-c`

## Test Cases

### 1. TestE2EMultiServiceAttestation
Verifies all services can attest with SPIRE Agent and receive valid X.509 SVIDs.

**Validates**:
- Workload API connectivity
- Attestation success
- SVID certificate validity
- Trust domain verification

### 2. TestE2EServiceAToServiceBMTLS
Verifies Service A can establish mTLS connection to Service B.

**Validates**:
- mTLS handshake success
- Peer certificate validation
- SPIFFE ID authorization
- HTTP request/response over mTLS

### 3. TestE2EServiceBToServiceCMTLS
Verifies Service B can establish mTLS connection to Service C.

**Validates**:
- Chained trust (B trusts same CA as C)
- Service-to-service authentication
- Certificate chain validation

### 4. TestE2EChainedMTLS
Verifies complete request chain: Service A → Service B → Service C.

**Validates**:
- Multi-hop mTLS authentication
- Request forwarding with identity preservation
- End-to-end trust chain

### 5. TestE2EIdentityRotation
Verifies SVID rotation mechanism works correctly.

**Validates**:
- X.509 source automatic rotation
- SPIFFE ID persistence across rotations
- Certificate validity after rotation

### 6. TestE2EUnauthorizedAccess
Verifies unauthorized workloads cannot access protected services.

**Validates**:
- mTLS requirement enforcement
- Unauthorized request rejection
- Zero-trust security model

### 7. TestE2ETrustBundleValidation
Verifies trust bundles are correctly fetched and used for validation.

**Validates**:
- Trust bundle retrieval
- CA certificate presence
- Bundle-based certificate validation

## Prerequisites

1. **Kubernetes Cluster**: Minikube, kind, or any K8s cluster
2. **SPIRE Deployed**: SPIRE Server and Agent running
3. **Service Images**: Build and load service images (see below)
4. **Registration Entries**: Create SPIRE entries for all services

## Setup

### 1. Start SPIRE Infrastructure

```bash
# Using Minikube
make minikube-up

# Verify SPIRE is running
kubectl get pods -n spire-system
# Expected:
# NAME              READY   STATUS    RESTARTS   AGE
# spire-agent-xxx   1/1     Running   0          1m
# spire-server-0    1/1     Running   0          1m
```

### 2. Build Service Images

```bash
# Build test service images
cd test/e2e
docker build -t service-a:latest -f dockerfiles/Dockerfile.service-a .
docker build -t service-b:latest -f dockerfiles/Dockerfile.service-b .
docker build -t service-c:latest -f dockerfiles/Dockerfile.service-c .

# Load into Minikube (if using Minikube)
minikube image load service-a:latest
minikube image load service-b:latest
minikube image load service-c:latest
```

### 3. Create SPIRE Registration Entries

```bash
# Register Service A
kubectl exec -n spire-system spire-server-0 -- \
  /opt/spire/bin/spire-server entry create \
    -spiffeID spiffe://example.org/service-a \
    -parentID spiffe://example.org/spire/agent/k8s_psat/default \
    -selector k8s:ns:spire-e2e-test \
    -selector k8s:sa:service-a

# Register Service B
kubectl exec -n spire-system spire-server-0 -- \
  /opt/spire/bin/spire-server entry create \
    -spiffeID spiffe://example.org/service-b \
    -parentID spiffe://example.org/spire/agent/k8s_psat/default \
    -selector k8s:ns:spire-e2e-test \
    -selector k8s:sa:service-b

# Register Service C
kubectl exec -n spire-system spire-server-0 -- \
  /opt/spire/bin/spire-server entry create \
    -spiffeID spiffe://example.org/service-c \
    -parentID spiffe://example.org/spire/agent/k8s_psat/default \
    -selector k8s:ns:spire-e2e-test \
    -selector k8s:sa:service-c

# Verify entries
kubectl exec -n spire-system spire-server-0 -- \
  /opt/spire/bin/spire-server entry show
```

### 4. Deploy Test Services

```bash
# Deploy all services
kubectl apply -f test/e2e/manifests/

# Verify deployment
kubectl get pods -n spire-e2e-test
# Expected:
# NAME                         READY   STATUS    RESTARTS   AGE
# service-a-xxxxxxxxxx-xxxxx   1/1     Running   0          30s
# service-b-xxxxxxxxxx-xxxxx   1/1     Running   0          30s
# service-c-xxxxxxxxxx-xxxxx   1/1     Running   0          30s
```

## Running Tests

### Run All E2E Tests

```bash
# From project root
go test -tags=e2e ./test/e2e/... -v

# Expected output:
# === RUN   TestE2EMultiServiceAttestation
# --- PASS: TestE2EMultiServiceAttestation (1.23s)
# === RUN   TestE2EServiceAToServiceBMTLS
# --- PASS: TestE2EServiceAToServiceBMTLS (2.45s)
# === RUN   TestE2EServiceBToServiceCMTLS
# --- PASS: TestE2EServiceBToServiceCMTLS (2.31s)
# === RUN   TestE2EChainedMTLS
# --- PASS: TestE2EChainedMTLS (3.12s)
# === RUN   TestE2EIdentityRotation
# --- PASS: TestE2EIdentityRotation (5.67s)
# === RUN   TestE2EUnauthorizedAccess
# --- PASS: TestE2EUnauthorizedAccess (0.89s)
# === RUN   TestE2ETrustBundleValidation
# --- PASS: TestE2ETrustBundleValidation (1.01s)
# PASS
# ok      github.com/pocket/hexagon/spire/test/e2e    16.680s
```

### Run Specific Test

```bash
# Run only attestation test
go test -tags=e2e ./test/e2e/... -v -run TestE2EMultiServiceAttestation

# Run only mTLS tests
go test -tags=e2e ./test/e2e/... -v -run TestE2E.*MTLS
```

### Run from Inside Pod

```bash
# Exec into Service A pod
kubectl exec -it -n spire-e2e-test deployment/service-a -- /bin/sh

# Run tests from inside pod
cd /app
go test -tags=e2e ./test/e2e/... -v
```

## Troubleshooting

### Tests Fail with "connection refused"

**Problem**: Cannot connect to SPIRE Agent socket

**Solution**:
```bash
# Check SPIRE Agent is running
kubectl get pods -n spire-system | grep spire-agent

# Check socket path in pod
kubectl exec -n spire-e2e-test deployment/service-a -- ls -la /spire-agent-socket/

# Verify socket path matches test configuration
# Default: unix:///spire-agent-socket/agent.sock
```

### Tests Fail with "no identity is available"

**Problem**: Workload not registered or selectors don't match

**Solution**:
```bash
# Check registration entries exist
kubectl exec -n spire-system spire-server-0 -- \
  /opt/spire/bin/spire-server entry show

# Verify selectors match pod labels
kubectl get pod -n spire-e2e-test -o yaml | grep -A5 labels

# Check SPIRE Agent logs
kubectl logs -n spire-system -l app=spire-agent
```

### mTLS Connection Fails

**Problem**: Certificate validation or authorization failure

**Solution**:
```bash
# Check trust bundle
kubectl exec -n spire-e2e-test deployment/service-a -- \
  spire-agent api fetch x509 -socketPath /spire-agent-socket/agent.sock

# Verify SPIFFE IDs in test match registration
# Test expects: spiffe://example.org/service-a, service-b, service-c

# Check service logs for authorization errors
kubectl logs -n spire-e2e-test deployment/service-b
```

### Service Images Not Found

**Problem**: Kubernetes cannot pull service images

**Solution**:
```bash
# If using Minikube, load images:
minikube image load service-a:latest
minikube image load service-b:latest
minikube image load service-c:latest

# Or set imagePullPolicy to Never in manifests
```

## Cleanup

```bash
# Delete test namespace and all resources
kubectl delete namespace spire-e2e-test

# Delete SPIRE entries
kubectl exec -n spire-system spire-server-0 -- \
  /opt/spire/bin/spire-server entry delete -entryID <entry-id>

# Stop Minikube (if used)
make minikube-down
```

## CI Integration

To integrate E2E tests into CI pipeline:

```yaml
# Example GitHub Actions workflow
name: E2E Tests
on: [push, pull_request]

jobs:
  e2e:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3

      - name: Start Minikube
        run: |
          minikube start
          make minikube-up

      - name: Build service images
        run: |
          cd test/e2e
          docker build -t service-a:latest -f dockerfiles/Dockerfile.service-a .
          docker build -t service-b:latest -f dockerfiles/Dockerfile.service-b .
          docker build -t service-c:latest -f dockerfiles/Dockerfile.service-c .
          minikube image load service-a:latest
          minikube image load service-b:latest
          minikube image load service-c:latest

      - name: Deploy services
        run: kubectl apply -f test/e2e/manifests/

      - name: Wait for services
        run: kubectl wait --for=condition=ready pod -n spire-e2e-test --all --timeout=120s

      - name: Run E2E tests
        run: go test -tags=e2e ./test/e2e/... -v

      - name: Cleanup
        if: always()
        run: |
          kubectl delete namespace spire-e2e-test
          minikube delete
```

## References

- [SPIRE E2E Testing Guide](https://spiffe.io/docs/latest/testing/)
- [go-spiffe SDK Testing](https://github.com/spiffe/go-spiffe/tree/main/v2/workloadapi)
- [Kubernetes mTLS with SPIRE](https://spiffe.io/docs/latest/microservices/)
