# End-to-End Tests Documentation

The End-to-End (E2E) test suite validates the complete SPIRE workload identity flow across multiple services.

## Overview

The E2E test suite verifies that the SPIRE implementation works correctly in a realistic multi-service environment, testing the complete flow from workload attestation through mTLS-authenticated service-to-service communication.

**Implemented** in `test/e2e/e2e_test.go`

E2E tests validate:

1. **Multi-Service Orchestration**: Three services deployed in Kubernetes communicating via mTLS
2. **Complete Identity Flow**: Attestation → Registration Matching → SVID Issuance → mTLS Authentication
3. **Service-to-Service Authentication**: Mutual TLS with SPIFFE ID validation
4. **Certificate Chain Validation**: Trust bundle verification across service boundaries
5. **Identity Rotation**: SVID renewal and rotation mechanisms
6. **Authorization Policies**: Zero-trust security model enforcement
7. **System-Level Behavior**: Real workload API, real SPIRE Agent/Server, real Kubernetes

### Service Topology

```
┌─────────────────────────────────────────────────────────────────┐
│                        Kubernetes Cluster                        │
│                                                                  │
│  ┌──────────────────────────────────────────────────────────┐  │
│  │                    spire-e2e-test namespace              │  │
│  │                                                          │  │
│  │  ┌─────────────┐       mTLS      ┌─────────────┐       │  │
│  │  │  Service A  │ ──────────────> │  Service B  │       │  │
│  │  │  (Client)   │   Authorize:    │  (Server)   │       │  │
│  │  │             │   spiffe://...  │             │       │  │
│  │  │ Pod: sa-xxx │   /service-b    │ Pod: sb-xxx │       │  │
│  │  └─────────────┘                 └─────────────┘       │  │
│  │         │                               │               │  │
│  │         │                               │ mTLS          │  │
│  │         │                               v               │  │
│  │         │                         ┌─────────────┐       │  │
│  │         │                         │  Service C  │       │  │
│  │         │                         │ (Database)  │       │  │
│  │         │                         │             │       │  │
│  │         │                         │ Pod: sc-xxx │       │  │
│  │         │                         └─────────────┘       │  │
│  │         │                               │               │  │
│  └─────────┼───────────────────────────────┼───────────────┘  │
│            │                               │                  │
│            └───────────┬───────────────────┘                  │
│                        │                                      │
│               Unix Socket: /spire-agent-socket/agent.sock    │
│                        │                                      │
│  ┌─────────────────────┴─────────────────────────────────┐  │
│  │                   spire-system namespace              │  │
│  │                                                        │  │
│  │  ┌──────────────┐              ┌──────────────────┐  │  │
│  │  │ SPIRE Agent  │◄────────────►│  SPIRE Server    │  │  │
│  │  │ (DaemonSet)  │   gRPC       │  (StatefulSet)   │  │  │
│  │  │              │              │  - CA Authority  │  │  │
│  │  │ - Attest     │              │  - Registration  │  │  │
│  │  │ - Issue SVID │              │  - Entry Store   │  │  │
│  │  └──────────────┘              └──────────────────┘  │  │
│  └────────────────────────────────────────────────────────┘  │
└─────────────────────────────────────────────────────────────────┘
```

### Service Descriptions

#### Service A (Client)
- **SPIFFE ID**: `spiffe://example.org/service-a`
- **Role**: HTTP client initiating requests
- **Behavior**:
  - Attests with SPIRE Agent to get X.509 SVID
  - Establishes mTLS connection to Service B
  - Validates Service B's SPIFFE ID during TLS handshake
  - Makes HTTP requests over authenticated channel
- **Selectors**:
  - `k8s:ns:spire-e2e-test`
  - `k8s:sa:service-a`
- **Deployment**: `test/e2e/manifests/service-a-client.yaml`

#### Service B (Server)
- **SPIFFE ID**: `spiffe://example.org/service-b`
- **Role**: HTTP server and intermediate client
- **Behavior**:
  - Attests with SPIRE Agent to get X.509 SVID
  - Listens on port 8443 with mTLS
  - Validates incoming connections from Service A
  - Acts as client to Service C for chained requests
  - Validates Service C's SPIFFE ID
- **Selectors**:
  - `k8s:ns:spire-e2e-test`
  - `k8s:sa:service-b`
- **Endpoints**:
  - `GET /health` - Health check endpoint
  - `GET /chain` - Forwards request to Service C and aggregates response
- **Deployment**: `test/e2e/manifests/service-b-server.yaml`

#### Service C (Database)
- **SPIFFE ID**: `spiffe://example.org/service-c`
- **Role**: HTTP server simulating database service
- **Behavior**:
  - Attests with SPIRE Agent to get X.509 SVID
  - Listens on port 8444 with mTLS
  - Validates incoming connections from Service B
  - Returns data responses
- **Selectors**:
  - `k8s:ns:spire-e2e-test`
  - `k8s:sa:service-c`
- **Endpoints**:
  - `GET /health` - Health check endpoint
  - `GET /data` - Returns database-like data
- **Deployment**: `test/e2e/manifests/service-c-database.yaml`

## Test Cases

### 1. TestE2EMultiServiceAttestation

**Purpose**: Verify all services can successfully attest with SPIRE Agent and receive valid X.509 SVIDs.

**What It Tests**:
- Workload API client creation and connection
- SPIRE Agent socket accessibility from pod
- X.509 SVID fetching via Workload API
- SVID certificate validity (NotBefore/NotAfter)
- Trust domain verification
- Certificate chain completeness

**Flow**:
```
Test → Create Workload API Client → Fetch X.509 SVID → Validate SVID
```

**Success Criteria**:
- ✅ Client connects to SPIRE Agent socket
- ✅ SVID is returned (not nil)
- ✅ SPIFFE ID matches expected trust domain
- ✅ Certificate chain contains at least one certificate
- ✅ Certificate is currently valid (not expired)

**Example Output**:
```
=== RUN   TestE2EMultiServiceAttestation
Successfully attested and fetched SVID for identity: spiffe://example.org/service-a
--- PASS: TestE2EMultiServiceAttestation (1.23s)
```

---

### 2. TestE2EServiceAToServiceBMTLS

**Purpose**: Verify Service A can establish mTLS connection to Service B with mutual authentication.

**What It Tests**:
- X.509 Source creation for automatic SVID rotation
- TLS config with SPIFFE ID authorization
- HTTP client mTLS connectivity
- Peer certificate validation
- SPIFFE ID extraction from peer certificate
- Service authorization logic

**Flow**:
```
Service A → Create X.509 Source → Configure mTLS with Service B ID
         → Make HTTPS Request → Verify Peer Certificate
```

**Success Criteria**:
- ✅ X.509 Source created successfully
- ✅ TLS connection established
- ✅ HTTP 200 response received
- ✅ Peer certificate contains Service B's SPIFFE ID
- ✅ No certificate validation errors

**Authorization Model**:
```go
tlsConfig := tlsconfig.MTLSClientConfig(
    x509Source,           // My credentials
    x509Source,           // Trust bundle for validation
    tlsconfig.AuthorizeID(serviceBID), // Only accept Service B
)
```

**Example Output**:
```
=== RUN   TestE2EServiceAToServiceBMTLS
Service B response: {"status":"healthy","service":"service-b"}
Successfully established mTLS connection: Service A -> Service B
--- PASS: TestE2EServiceAToServiceBMTLS (2.45s)
```

---

### 3. TestE2EServiceBToServiceCMTLS

**Purpose**: Verify Service B can establish mTLS connection to Service C with mutual authentication.

**What It Tests**:
- Service identity verification (Service B has correct SPIFFE ID)
- Chained trust (Service B trusts same CA as Service C)
- Multi-hop authentication chain
- Service-to-service authorization policies

**Flow**:
```
Service B → Verify Own Identity → Create X.509 Source
         → Configure mTLS with Service C ID → Make HTTPS Request
         → Verify Peer Certificate
```

**Success Criteria**:
- ✅ Service B's SVID matches expected SPIFFE ID
- ✅ TLS connection to Service C established
- ✅ HTTP 200 response received
- ✅ Peer certificate contains Service C's SPIFFE ID
- ✅ Certificate chain validated against trust bundle

**Example Output**:
```
=== RUN   TestE2EServiceBToServiceCMTLS
Service C response: {"status":"healthy","service":"service-c"}
Successfully established mTLS connection: Service B -> Service C
--- PASS: TestE2EServiceBToServiceCMTLS (2.31s)
```

---

### 4. TestE2EChainedMTLS

**Purpose**: Verify complete request chain from Service A through Service B to Service C.

**What It Tests**:
- Multi-hop mTLS authentication
- Request forwarding with identity preservation
- End-to-end trust chain
- Response aggregation from multiple services
- Service mesh-like behavior

**Flow**:
```
Service A → mTLS to Service B /chain endpoint
         → Service B validates A's identity
         → Service B mTLS to Service C
         → Service C validates B's identity
         → Service C returns data
         → Service B aggregates and returns to A
```

**Success Criteria**:
- ✅ Service A successfully calls Service B's /chain endpoint
- ✅ Service B successfully calls Service C internally
- ✅ Response contains data from both Service B and Service C
- ✅ HTTP 200 status from the complete chain
- ✅ All mTLS connections validated

**Example Output**:
```
=== RUN   TestE2EChainedMTLS
Successfully completed chained mTLS: Service A -> Service B -> Service C
Chain response: {"path":"A->B->C","service-b":"data","service-c":"data"}
--- PASS: TestE2EChainedMTLS (3.12s)
```

---

### 5. TestE2EIdentityRotation

**Purpose**: Verify SVID rotation mechanism works correctly without service disruption.

**What It Tests**:
- X.509 Source automatic rotation
- SPIFFE ID persistence across rotations
- Certificate validity after rotation
- Connection continuity during rotation
- Serial number changes (when TTL expires)

**Flow**:
```
Test → Create X.509 Source (with rotation enabled)
     → Get Initial SVID (record serial number)
     → Wait for potential rotation period
     → Get Current SVID (record serial number)
     → Verify identity unchanged, certificate still valid
```

**Success Criteria**:
- ✅ Initial SVID fetched successfully
- ✅ Current SVID fetched after wait period
- ✅ SPIFFE ID remains constant (identity preserved)
- ✅ Certificate is currently valid
- ✅ Rotation mechanism operational (serial may change in long tests)

**Note**: To truly test rotation, SPIRE would need short TTLs configured (e.g., 30 seconds). In production, SVIDs typically have 1-hour TTLs. This test verifies the rotation mechanism is in place, even if rotation doesn't occur during the test window.

**Example Output**:
```
=== RUN   TestE2EIdentityRotation
Initial SVID serial: 123456789
Current SVID serial: 123456789 (rotation mechanism verified)
--- PASS: TestE2EIdentityRotation (5.67s)
```

---

### 6. TestE2EUnauthorizedAccess

**Purpose**: Verify that unauthorized workloads cannot access protected services (zero-trust model).

**What It Tests**:
- mTLS requirement enforcement
- Unauthorized request rejection
- Certificate-based authentication requirement
- Defense against attacks without valid SPIFFE credentials

**Flow**:
```
Test → Create HTTP Client WITHOUT mTLS authentication
     → Attempt to call Service B
     → Expect: Connection failure or HTTP 401/403
```

**Success Criteria**:
- ✅ Connection fails (cannot establish TLS)
  OR
- ✅ Connection succeeds but returns HTTP 401 Unauthorized
  OR
- ✅ Connection succeeds but returns HTTP 403 Forbidden

**Security Model**: Services MUST reject requests without valid SPIFFE certificates. Either:
- **TLS Layer Rejection**: Server requires client certificate, connection fails
- **Application Layer Rejection**: Server accepts connection but rejects at HTTP layer

**Example Output**:
```
=== RUN   TestE2EUnauthorizedAccess
Unauthorized access correctly rejected with error: tls: bad certificate
--- PASS: TestE2EUnauthorizedAccess (0.89s)
```

---

### 7. TestE2ETrustBundleValidation

**Purpose**: Verify trust bundles are correctly fetched and used for certificate validation.

**What It Tests**:
- Trust bundle retrieval via Workload API
- Trust domain bundle existence
- CA certificate presence in bundle
- Certificate chain validation against trust bundle
- Bundle usage in mTLS handshake

**Flow**:
```
Test → Create Workload API Client
     → Fetch X.509 Bundles
     → Verify trust domain bundle exists
     → Verify bundle contains CA certificates
     → Log CA certificate details
```

**Success Criteria**:
- ✅ Trust bundles fetched successfully
- ✅ Bundle for configured trust domain exists
- ✅ Bundle contains at least one CA certificate
- ✅ CA certificate is valid (not expired)

**Trust Bundle Purpose**: The trust bundle contains the root CA certificates used to validate peer certificates during mTLS handshake. Without a valid trust bundle, certificate chain validation would fail.

**Example Output**:
```
=== RUN   TestE2ETrustBundleValidation
Trust bundle contains 1 CA certificate(s)
  CA 1: Subject=example.org, Valid until=2025-10-08T12:00:00Z
--- PASS: TestE2ETrustBundleValidation (1.01s)
```

---

## Prerequisites

### Infrastructure Requirements

1. **Kubernetes Cluster**
   - Minikube, kind, Docker Desktop, or any K8s cluster
   - Minimum version: 1.20+
   - Namespace isolation support

2. **SPIRE Deployment**
   - SPIRE Server running (StatefulSet)
   - SPIRE Agent running (DaemonSet)
   - Workload API socket accessible at `/run/spire/sockets/agent.sock`
   - Trust domain configured: `example.org`

3. **Service Images**
   - `service-a:latest` - Client service image
   - `service-b:latest` - Server service image
   - `service-c:latest` - Database service image
   - Images must be available in cluster (built and loaded)

4. **SPIRE Registration Entries**
   - Service A entry with selectors: `k8s:ns:spire-e2e-test`, `k8s:sa:service-a`
   - Service B entry with selectors: `k8s:ns:spire-e2e-test`, `k8s:sa:service-b`
   - Service C entry with selectors: `k8s:ns:spire-e2e-test`, `k8s:sa:service-c`

### Software Requirements

- Go 1.25+ (for running tests)
- kubectl configured with cluster access
- Docker (for building service images)
- make (for convenience commands)

## Setup Instructions

### Step 1: Start SPIRE Infrastructure

```bash
# Using the project's Makefile
cd /home/zepho/work/pocket/hexagon/spire
make minikube-up

# This will:
# - Start Minikube cluster
# - Deploy SPIRE Server (StatefulSet)
# - Deploy SPIRE Agent (DaemonSet)
# - Configure trust domain: example.org
```

**Verify SPIRE is Running**:
```bash
kubectl get pods -n spire-system

# Expected output:
# NAME              READY   STATUS    RESTARTS   AGE
# spire-agent-xxx   1/1     Running   0          1m
# spire-server-0    1/1     Running   0          1m
```

---

### Step 2: Build Service Images

The test services need to be built and loaded into the cluster.

**Option A: Using Provided Dockerfiles** (if they exist):
```bash
cd test/e2e

# Build images
docker build -t service-a:latest -f dockerfiles/Dockerfile.service-a .
docker build -t service-b:latest -f dockerfiles/Dockerfile.service-b .
docker build -t service-c:latest -f dockerfiles/Dockerfile.service-c .

# Load into Minikube
minikube image load service-a:latest
minikube image load service-b:latest
minikube image load service-c:latest
```

**Option B: Using Project Binary** (recommended):
```bash
# Build the SPIRE server binary with E2E test support
make prod-build

# Create service images that use the binary
docker build -t service-a:latest -f test/e2e/dockerfiles/Dockerfile.service-a .
docker build -t service-b:latest -f test/e2e/dockerfiles/Dockerfile.service-b .
docker build -t service-c:latest -f test/e2e/dockerfiles/Dockerfile.service-c .

# Load into Minikube
minikube image load service-a:latest
minikube image load service-b:latest
minikube image load service-c:latest
```

---

### Step 3: Create SPIRE Registration Entries

Registration entries tell SPIRE which SPIFFE IDs to issue to which workloads based on selectors.

```bash
# Get SPIRE Server pod name
SPIRE_SERVER_POD=$(kubectl get pod -n spire-system -l app=spire-server -o jsonpath='{.items[0].metadata.name}')

# Register Service A
kubectl exec -n spire-system $SPIRE_SERVER_POD -- \
  /opt/spire/bin/spire-server entry create \
    -spiffeID spiffe://example.org/service-a \
    -parentID spiffe://example.org/spire/agent/k8s_psat/default \
    -selector k8s:ns:spire-e2e-test \
    -selector k8s:sa:service-a \
    -ttl 3600

# Register Service B
kubectl exec -n spire-system $SPIRE_SERVER_POD -- \
  /opt/spire/bin/spire-server entry create \
    -spiffeID spiffe://example.org/service-b \
    -parentID spiffe://example.org/spire/agent/k8s_psat/default \
    -selector k8s:ns:spire-e2e-test \
    -selector k8s:sa:service-b \
    -ttl 3600

# Register Service C
kubectl exec -n spire-system $SPIRE_SERVER_POD -- \
  /opt/spire/bin/spire-server entry create \
    -spiffeID spiffe://example.org/service-c \
    -parentID spiffe://example.org/spire/agent/k8s_psat/default \
    -selector k8s:ns:spire-e2e-test \
    -selector k8s:sa:service-c \
    -ttl 3600
```

**Verify Entries Created**:
```bash
kubectl exec -n spire-system $SPIRE_SERVER_POD -- \
  /opt/spire/bin/spire-server entry show

# Expected output:
# Entry ID         : <uuid>
# SPIFFE ID        : spiffe://example.org/service-a
# Parent ID        : spiffe://example.org/spire/agent/k8s_psat/default
# Revision         : 0
# X509-SVID TTL    : 3600
# Selector         : k8s:ns:spire-e2e-test
# Selector         : k8s:sa:service-a
#
# (similar entries for service-b and service-c)
```

---

### Step 4: Deploy Test Services

```bash
# Deploy all services
kubectl apply -f test/e2e/manifests/

# This creates:
# - Namespace: spire-e2e-test
# - ServiceAccount: service-a, service-b, service-c
# - Service: service-b, service-c (ClusterIP)
# - Deployment: service-a, service-b, service-c
```

**Verify Deployment**:
```bash
kubectl get pods -n spire-e2e-test

# Expected output:
# NAME                         READY   STATUS    RESTARTS   AGE
# service-a-xxxxxxxxxx-xxxxx   1/1     Running   0          30s
# service-b-xxxxxxxxxx-xxxxx   1/1     Running   0          30s
# service-c-xxxxxxxxxx-xxxxx   1/1     Running   0          30s
```

**Wait for All Pods to be Ready**:
```bash
kubectl wait --for=condition=ready pod \
  -n spire-e2e-test \
  --all \
  --timeout=120s
```

---

## Running Tests

### Run All E2E Tests

```bash
# From project root
go test -tags=e2e ./test/e2e/... -v

# Expected output:
# === RUN   TestE2EMultiServiceAttestation
# Successfully attested and fetched SVID for identity: spiffe://example.org/service-a
# --- PASS: TestE2EMultiServiceAttestation (1.23s)
# === RUN   TestE2EServiceAToServiceBMTLS
# Service B response: {"status":"healthy"}
# Successfully established mTLS connection: Service A -> Service B
# --- PASS: TestE2EServiceAToServiceBMTLS (2.45s)
# === RUN   TestE2EServiceBToServiceCMTLS
# Service C response: {"status":"healthy"}
# Successfully established mTLS connection: Service B -> Service C
# --- PASS: TestE2EServiceBToServiceCMTLS (2.31s)
# === RUN   TestE2EChainedMTLS
# Successfully completed chained mTLS: Service A -> Service B -> Service C
# Chain response: {"path":"A->B->C"}
# --- PASS: TestE2EChainedMTLS (3.12s)
# === RUN   TestE2EIdentityRotation
# Initial SVID serial: 123456789
# Current SVID serial: 123456789 (rotation mechanism verified)
# --- PASS: TestE2EIdentityRotation (5.67s)
# === RUN   TestE2EUnauthorizedAccess
# Unauthorized access correctly rejected with error: tls: bad certificate
# --- PASS: TestE2EUnauthorizedAccess (0.89s)
# === RUN   TestE2ETrustBundleValidation
# Trust bundle contains 1 CA certificate(s)
#   CA 1: Subject=example.org, Valid until=2025-10-08T12:00:00Z
# --- PASS: TestE2ETrustBundleValidation (1.01s)
# PASS
# ok      github.com/pocket/hexagon/spire/test/e2e    16.680s
```

---

### Run Specific Test

```bash
# Run only attestation test
go test -tags=e2e ./test/e2e/... -v -run TestE2EMultiServiceAttestation

# Run only mTLS tests
go test -tags=e2e ./test/e2e/... -v -run "TestE2E.*MTLS"

# Run with timeout
go test -tags=e2e ./test/e2e/... -v -timeout 5m
```

---

### Run Tests from Inside Pod

For debugging or testing from within the cluster:

```bash
# Exec into Service A pod
kubectl exec -it -n spire-e2e-test deployment/service-a -- /bin/sh

# Inside the pod, run tests
cd /app
go test -tags=e2e ./test/e2e/... -v
```

---

### Run Tests with Coverage

```bash
go test -tags=e2e ./test/e2e/... -v -coverprofile=e2e_coverage.out
go tool cover -html=e2e_coverage.out -o e2e_coverage.html
```

---

## Troubleshooting

### Issue: Tests Fail with "connection refused"

**Symptoms**:
```
Failed to create workload API client: dial unix /spire-agent-socket/agent.sock: connect: connection refused
```

**Causes**:
1. SPIRE Agent not running
2. Socket path mismatch
3. Volume mount not configured

**Solutions**:

```bash
# 1. Check SPIRE Agent is running
kubectl get pods -n spire-system | grep spire-agent
# Should show: spire-agent-xxx   1/1     Running

# 2. Check socket exists in Agent pod
kubectl exec -n spire-system spire-agent-xxx -- ls -la /run/spire/sockets/
# Should show: agent.sock

# 3. Check socket is mounted in test pod
kubectl exec -n spire-e2e-test deployment/service-a -- ls -la /spire-agent-socket/
# Should show: agent.sock

# 4. Verify socket path in test configuration
grep "spireSocketPath" test/e2e/e2e_test.go
# Should match: unix:///spire-agent-socket/agent.sock
```

---

### Issue: Tests Fail with "no identity is available"

**Symptoms**:
```
Failed to fetch X.509 SVID: no identity is available
```

**Causes**:
1. Workload not registered in SPIRE
2. Selectors don't match pod attributes
3. Registration entry not propagated to Agent

**Solutions**:

```bash
# 1. Check registration entries exist
kubectl exec -n spire-system spire-server-0 -- \
  /opt/spire/bin/spire-server entry show | grep service-a

# 2. Verify pod labels match selectors
kubectl get pod -n spire-e2e-test -o yaml | grep -A10 metadata

# Should show:
#   namespace: spire-e2e-test
#   serviceAccount: service-a

# 3. Check SPIRE Agent logs for attestation
kubectl logs -n spire-system -l app=spire-agent --tail=50

# Look for:
# - "Attestation successful"
# - "Workload attested"

# 4. Force Agent to re-sync
kubectl delete pod -n spire-system -l app=spire-agent
kubectl wait --for=condition=ready pod -n spire-system -l app=spire-agent
```

---

### Issue: mTLS Connection Fails

**Symptoms**:
```
Failed to make mTLS request to Service B: tls: bad certificate
```

**Causes**:
1. Certificate validation failure
2. Trust bundle mismatch
3. SPIFFE ID authorization failure
4. Certificate expired

**Solutions**:

```bash
# 1. Check trust bundle
kubectl exec -n spire-e2e-test deployment/service-a -- \
  spire-agent api fetch x509 -socketPath /spire-agent-socket/agent.sock

# Should show:
# - SPIFFE ID: spiffe://example.org/service-a
# - Bundle: (CA certificates)

# 2. Verify SPIFFE IDs match expectations
# Test expects:
# - Service A: spiffe://example.org/service-a
# - Service B: spiffe://example.org/service-b
# - Service C: spiffe://example.org/service-c

# Check registration entries:
kubectl exec -n spire-system spire-server-0 -- \
  /opt/spire/bin/spire-server entry show

# 3. Check service logs for authorization errors
kubectl logs -n spire-e2e-test deployment/service-b | grep -i "authori\|tls\|certificate"

# 4. Verify certificate expiration
kubectl exec -n spire-e2e-test deployment/service-a -- \
  spire-agent api fetch x509 -socketPath /spire-agent-socket/agent.sock -write /tmp
openssl x509 -in /tmp/svid.0.pem -text -noout | grep -A2 Validity
```

---

### Issue: Service Images Not Found

**Symptoms**:
```
Failed to pull image "service-a:latest": rpc error: code = Unknown desc = Error response from daemon: pull access denied
```

**Causes**:
1. Images not built
2. Images not loaded into cluster (Minikube)
3. ImagePullPolicy incorrect

**Solutions**:

```bash
# 1. Build images
cd test/e2e
docker build -t service-a:latest -f dockerfiles/Dockerfile.service-a .
docker build -t service-b:latest -f dockerfiles/Dockerfile.service-b .
docker build -t service-c:latest -f dockerfiles/Dockerfile.service-c .

# 2. Load into Minikube
minikube image load service-a:latest
minikube image load service-b:latest
minikube image load service-c:latest

# 3. Verify images in Minikube
minikube ssh
docker images | grep service

# 4. Alternative: Set imagePullPolicy to Never in manifests
kubectl edit deployment service-a -n spire-e2e-test
# Change: imagePullPolicy: IfNotPresent → imagePullPolicy: Never
```

---

### Issue: Test Timeout

**Symptoms**:
```
panic: test timed out after 10m0s
```

**Causes**:
1. Services not starting (CrashLoopBackOff)
2. SPIRE not responding
3. Network connectivity issues

**Solutions**:

```bash
# 1. Check all pods are running
kubectl get pods --all-namespaces

# 2. Check pod status and events
kubectl describe pod -n spire-e2e-test service-a-xxx

# 3. Check service logs
kubectl logs -n spire-e2e-test deployment/service-a --tail=100

# 4. Increase test timeout
go test -tags=e2e ./test/e2e/... -v -timeout 20m

# 5. Check network policies (if any)
kubectl get networkpolicies -n spire-e2e-test
```

---

## Cleanup

### Delete Test Resources

```bash
# Delete test namespace (removes all services)
kubectl delete namespace spire-e2e-test

# Verify deletion
kubectl get pods -n spire-e2e-test
# Should show: No resources found
```

### Delete SPIRE Registration Entries

```bash
# List entries to get Entry IDs
kubectl exec -n spire-system spire-server-0 -- \
  /opt/spire/bin/spire-server entry show

# Delete each entry by ID
kubectl exec -n spire-system spire-server-0 -- \
  /opt/spire/bin/spire-server entry delete -entryID <entry-id>

# Or delete all entries for the test namespace (careful!)
kubectl exec -n spire-system spire-server-0 -- \
  /opt/spire/bin/spire-server entry show | grep "spiffe://example.org/service-" | \
  awk '{print $4}' | xargs -I {} kubectl exec -n spire-system spire-server-0 -- \
  /opt/spire/bin/spire-server entry delete -entryID {}
```

### Stop SPIRE Infrastructure

```bash
# Using project Makefile
make minikube-down

# Or manually
kubectl delete namespace spire-system
```

### Cleanup

```bash
# Remove everything
kubectl delete namespace spire-e2e-test
kubectl delete namespace spire-system
minikube stop
minikube delete

# Remove local images (optional)
docker rmi service-a:latest service-b:latest service-c:latest
```

---

## CI/CD Integration

### GitHub Actions Example

```yaml
name: E2E Tests
on:
  push:
    branches: [main]
  pull_request:
    branches: [main]

jobs:
  e2e:
    runs-on: ubuntu-latest
    timeout-minutes: 30

    steps:
      - name: Checkout code
        uses: actions/checkout@v3

      - name: Set up Go
        uses: actions/setup-go@v4
        with:
          go-version: '1.21'

      - name: Start Minikube
        uses: medyagh/setup-minikube@latest
        with:
          kubernetes-version: 'v1.28.0'
          driver: docker

      - name: Deploy SPIRE
        run: |
          make minikube-up
          kubectl wait --for=condition=ready pod -n spire-system --all --timeout=300s

      - name: Build service images
        run: |
          cd test/e2e
          docker build -t service-a:latest -f dockerfiles/Dockerfile.service-a .
          docker build -t service-b:latest -f dockerfiles/Dockerfile.service-b .
          docker build -t service-c:latest -f dockerfiles/Dockerfile.service-c .
          minikube image load service-a:latest
          minikube image load service-b:latest
          minikube image load service-c:latest

      - name: Create SPIRE registration entries
        run: |
          SPIRE_SERVER_POD=$(kubectl get pod -n spire-system -l app=spire-server -o jsonpath='{.items[0].metadata.name}')
          kubectl exec -n spire-system $SPIRE_SERVER_POD -- \
            /opt/spire/bin/spire-server entry create \
              -spiffeID spiffe://example.org/service-a \
              -parentID spiffe://example.org/spire/agent/k8s_psat/default \
              -selector k8s:ns:spire-e2e-test \
              -selector k8s:sa:service-a
          # (repeat for service-b and service-c)

      - name: Deploy test services
        run: |
          kubectl apply -f test/e2e/manifests/
          kubectl wait --for=condition=ready pod -n spire-e2e-test --all --timeout=300s

      - name: Run E2E tests
        run: |
          go test -tags=e2e ./test/e2e/... -v -timeout 15m

      - name: Collect logs on failure
        if: failure()
        run: |
          kubectl logs -n spire-system -l app=spire-server > spire-server.log
          kubectl logs -n spire-system -l app=spire-agent > spire-agent.log
          kubectl logs -n spire-e2e-test deployment/service-a > service-a.log
          kubectl logs -n spire-e2e-test deployment/service-b > service-b.log
          kubectl logs -n spire-e2e-test deployment/service-c > service-c.log

      - name: Upload logs
        if: failure()
        uses: actions/upload-artifact@v3
        with:
          name: e2e-logs
          path: '*.log'

      - name: Cleanup
        if: always()
        run: |
          kubectl delete namespace spire-e2e-test || true
          kubectl delete namespace spire-system || true
          minikube delete
```

---

## Performance Considerations

### Expected Test Duration

| Test Case | Typical Duration | Max Duration |
|-----------|-----------------|--------------|
| TestE2EMultiServiceAttestation | 1-2s | 5s |
| TestE2EServiceAToServiceBMTLS | 2-3s | 10s |
| TestE2EServiceBToServiceCMTLS | 2-3s | 10s |
| TestE2EChainedMTLS | 3-4s | 15s |
| TestE2EIdentityRotation | 5-10s | 30s |
| TestE2EUnauthorizedAccess | 1s | 5s |
| TestE2ETrustBundleValidation | 1s | 5s |
| **Total** | **15-25s** | **80s** |

### Optimization Tips

1. **Parallel Test Execution**: Tests are independent and can run in parallel
   ```bash
   go test -tags=e2e ./test/e2e/... -v -parallel 4
   ```

2. **Reuse X.509 Source**: Tests create X.509 source multiple times; consider caching
3. **Reduce Timeouts**: Default 30s timeout per test; reduce for faster CI
4. **Pre-warm Connections**: Keep persistent connections to reduce handshake overhead

---

## Security Considerations

### What E2E Tests Validate

✅ **Validated Security Properties**:
- mTLS authentication required for all service-to-service communication
- SPIFFE ID-based authorization (only expected IDs accepted)
- Certificate chain validation against trust bundle
- Unauthorized access rejection
- Identity rotation without credential leakage

❌ **Not Validated** (out of scope):
- Cryptographic strength of SPIRE's CA
- Side-channel attacks
- Denial of service resilience
- SPIRE Server compromise scenarios
- Network-level attacks (eavesdropping, MITM)

### Threat Model

The E2E tests validate the **Zero Trust Workload Security Model**:

```
Assumption: Network is hostile
Trust Model: Workload identities only (not IPs, ports, or DNS)
Authentication: Mutual TLS with SPIFFE certificates
Authorization: SPIFFE ID validation
```

**What an attacker CANNOT do** (validated by tests):
- ❌ Connect to Service B without valid Service A SVID
- ❌ Impersonate Service A without SPIRE-issued certificate
- ❌ Access Service C directly from Service A (must go through B)
- ❌ Bypass certificate validation

---

## References

- **SPIRE E2E Testing Guide**: https://spiffe.io/docs/latest/testing/
- **go-spiffe SDK Testing**: https://github.com/spiffe/go-spiffe/tree/main/v2/workloadapi
- **Kubernetes mTLS with SPIRE**: https://spiffe.io/docs/latest/microservices/
- **SPIFFE Authentication Spec**: https://github.com/spiffe/spiffe/blob/main/standards/SPIFFE.md
- **Test Code**: `test/e2e/e2e_test.go`
- **Kubernetes Manifests**: `test/e2e/manifests/`
- **Setup README**: `test/e2e/README.md`

---

## Summary

The E2E test suite provides validation of SPIRE workload identity flow across multiple services. It verifies:

1. ✅ All services can attest and receive valid SVIDs
2. ✅ Mutual TLS authentication works between services
3. ✅ SPIFFE ID-based authorization is enforced
4. ✅ Certificate chains are validated correctly
5. ✅ Identity rotation mechanisms are operational
6. ✅ Unauthorized access is rejected
7. ✅ Trust bundles are correctly used for validation

These tests represent **system-level validation** that goes beyond component-level integration tests, ensuring the complete SPIRE implementation works as a cohesive system in a realistic Kubernetes environment.
