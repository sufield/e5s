# mTLS Examples Comparison Guide

This document explains the differences between the two mTLS example implementations in this repository and helps you choose the right one for your use case.

## Table of Contents

- [Overview](#overview)
- [Quick Comparison](#quick-comparison)
- [Detailed Differences](#detailed-differences)
- [When to Use Which](#when-to-use-which)
- [Migration Path](#migration-path)

---

## Overview

This repository contains two mTLS examples demonstrating service-to-service communication with SPIRE/SPIFFE:

1. **`examples/mtls/`** - Basic implementation for learning
2. **`examples/mtls-adapters/`** - Production-ready implementation with adapter pattern

Both examples achieve the same goal (mTLS authentication between services), but differ significantly in architecture, features, and documentation depth.

---

## Quick Comparison

### At a Glance

| Aspect | examples/mtls | examples/mtls-adapters |
|--------|---------------|------------------------|
| **Purpose** | Learning & POC | Production deployment |
| **Architecture** | Direct implementation | Hexagonal (adapter pattern) |
| **Documentation** | 330 lines (1 file) | 1,625 lines (3 files) |
| **Server Package** | `identityserver.Server` | `httpapi.NewHTTPServer` |
| **Client Package** | `httpclient.Client` | `httpclient.NewSPIFFEHTTPClient` |
| **Identity Utilities** | Basic extraction | 5+ helper functions |
| **Kubernetes** | Simple Pod | Deployment + Service |
| **Troubleshooting** | 3 issues | 20+ issues |
| **Complexity** | Low | Medium-High |
| **Best For** | Beginners | Production users |

---

## Detailed Differences

### 1. Implementation Architecture

#### examples/mtls/
```
Direct Implementation:
┌─────────────────────────────────────┐
│  Application Code                    │
│  ↓                                   │
│  identityserver.Server (low-level)  │
│  ↓                                   │
│  SPIRE Workload API                 │
└─────────────────────────────────────┘
```

**Characteristics**:
- Direct use of SPIRE libraries
- Minimal abstraction layers
- Good for understanding internals
- Less separation of concerns

#### examples/mtls-adapters/
```
Hexagonal Architecture (Adapter Pattern):
┌─────────────────────────────────────┐
│  Application Core (Business Logic)  │
│  ↓                                   │
│  Ports (Interfaces)                 │
│  ↓                                   │
│  Adapters                           │
│  ├─ Inbound: httpapi                │
│  └─ Outbound: httpclient            │
│  ↓                                   │
│  SPIRE Workload API                 │
└─────────────────────────────────────┘
```

**Characteristics**:
- Clear separation of concerns
- Easier to test (mock adapters)
- Better maintainability
- Production-ready patterns

---

### 2. Server Implementation

#### examples/mtls/server/main.go
```go
import "github.com/your-org/identityserver"

// Lower-level server creation
server := identityserver.Server{
    Config: config,
}

// Basic identity extraction
clientID, _ := spiffetls.PeerIDFromConnectionState(*r.TLS)
```

**Features**:
- Basic HTTP server with mTLS
- Simple identity verification
- Manual TLS connection state handling

#### examples/mtls-adapters/server/main.go
```go
import "github.com/your-org/internal/adapters/inbound/httpapi"

// Adapter pattern server
server := httpapi.NewHTTPServer(ctx, httpapi.ServerConfig{
    SPIRE:  spireConfig,
    HTTP:   httpConfig,
})

// Advanced identity utilities
clientID, ok := httpapi.GetSPIFFEID(r)
if httpapi.MatchesTrustDomain(r, "example.org") {
    // Trust domain verified
}
if httpapi.HasPathPrefix(r, "/admin") {
    // Admin path detected
}
```

**Features**:
- Adapter pattern (inbound adapter)
- Rich identity extraction utilities
- Built-in middleware support
- Configuration via structs

---

### 3. Client Implementation

#### examples/mtls/client/main.go
```go
import "github.com/your-org/httpclient"

// Basic client
client := httpclient.Client{
    Config: config,
}

resp, err := client.Get(ctx, url)
```

**Features**:
- Simple HTTP client
- Basic request/response
- Manual configuration

#### examples/mtls-adapters/client/main.go
```go
import "github.com/your-org/internal/adapters/outbound/httpclient"

// Adapter pattern client
client := httpclient.NewSPIFFEHTTPClient(ctx, httpclient.ClientConfig{
    SPIRE:            spireConfig,
    ServerURL:        serverURL,
    ExpectedServerID: expectedID,
})

resp, err := client.Get(ctx, "/api/hello")
```

**Features**:
- Adapter pattern (outbound adapter)
- Enhanced configuration options
- Better error handling
- Server identity verification

---

### 4. Identity Features

#### examples/mtls/

**Basic Identity Extraction**:
```go
// Manual extraction from TLS connection state
func handler(w http.ResponseWriter, r *http.Request) {
    clientID, err := spiffetls.PeerIDFromConnectionState(*r.TLS)
    if err != nil {
        http.Error(w, "Unauthorized", 401)
        return
    }

    // Use clientID...
}
```

**Capabilities**:
- Extract peer SPIFFE ID
- Manual trust domain checking
- Basic authorization logic

#### examples/mtls-adapters/

**Advanced Identity Utilities**:
```go
import "github.com/your-org/internal/adapters/inbound/httpapi"

func handler(w http.ResponseWriter, r *http.Request) {
    // 1. Extract SPIFFE ID
    clientID, ok := httpapi.GetSPIFFEID(r)

    // 2. Verify trust domain
    if !httpapi.MatchesTrustDomain(r, "example.org") {
        http.Error(w, "Wrong trust domain", 403)
        return
    }

    // 3. Check path prefix (service identification)
    if httpapi.HasPathPrefix(r, "/service/") {
        // Handle service-to-service request
    }

    // 4. Check path suffix (role-based)
    if httpapi.HasPathSuffix(r, "/admin") {
        // Handle admin request
    }

    // 5. Parse path segments
    segments, ok := httpapi.GetPathSegments(r)
    // For spiffe://example.org/ns/prod/svc/api
    // segments = ["ns", "prod", "svc", "api"]
    namespace := segments[0]  // "ns"
    environment := segments[1] // "prod"
}
```

**Capabilities**:
- 5+ helper functions for identity extraction
- Trust domain verification
- Path-based routing and authorization
- Namespace/environment extraction
- Dedicated `/api/identity` demo endpoint

---

### 5. Documentation Scope

#### examples/mtls/README.md (330 lines)

**Structure**:
- Single file
- Basic setup instructions
- 3 troubleshooting scenarios
- Inline Kubernetes examples

**Sections**:
1. Architecture diagram
2. Running locally
3. Running in Kubernetes
4. Configuration
5. Security notes
6. Basic troubleshooting

#### examples/mtls-adapters/ (1,625 lines total)

**Structure**:
- **README.md** (394 lines) - Overview and local development
- **KUBERNETES.md** (566 lines) - Complete Kubernetes guide
- **TROUBLESHOOTING.md** (665 lines) - Comprehensive problem-solving

**README.md Sections**:
1. Detailed architecture diagram
2. Files reference
3. Prerequisites (local + Kubernetes)
4. Running examples
5. Configuration (env vars)
6. API endpoints
7. Identity extraction examples
8. Testing identity mismatch
9. Security considerations
10. References

**KUBERNETES.md Sections**:
1. Prerequisites
2. Quick start
3. Deployment manifests (complete YAML)
4. Building images (Minikube + remote registry)
5. Deploying to Kubernetes (step-by-step)
6. Verification procedures
7. Workload registration (selectors, wildcards)
8. Advanced configuration (ConfigMaps, Secrets, replicas)
9. Cleanup

**TROUBLESHOOTING.md Sections**:
1. Connection issues (2 problems)
2. SPIRE agent issues (2 problems)
3. Workload registration (2 problems)
4. TLS handshake failures (2 problems)
5. Kubernetes-specific issues (3 problems)
6. Debugging tips and tools
7. Common error messages table

---

### 6. Kubernetes Deployment

#### examples/mtls/

**Simple Pod Deployment**:
```yaml
apiVersion: v1
kind: Pod
metadata:
  name: mtls-server
spec:
  containers:
  - name: server
    image: mtls-server:latest
    volumeMounts:
    - name: spire-agent-socket
      mountPath: /spire-agent-socket
  volumes:
  - name: spire-agent-socket
    hostPath:
      path: /run/spire/agent-sockets  # Note: different path
```

**Features**:
- Basic Pod configuration
- No health checks
- No resource limits
- Simple socket mount

#### examples/mtls-adapters/

**Production Deployment**:
```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: mtls-server
spec:
  replicas: 1
  selector:
    matchLabels:
      app: mtls-server
  template:
    spec:
      containers:
      - name: server
        image: mtls-server:latest
        ports:
        - containerPort: 8443
        livenessProbe:
          httpGet:
            path: /health
            port: 8443
            scheme: HTTPS
        readinessProbe:
          httpGet:
            path: /health
            port: 8443
            scheme: HTTPS
        resources:
          requests:
            memory: "64Mi"
            cpu: "100m"
          limits:
            memory: "128Mi"
            cpu: "200m"
        volumeMounts:
        - name: spire-agent-socket
          mountPath: /spire-agent-socket
      volumes:
      - name: spire-agent-socket
        hostPath:
          path: /run/spire/sockets
---
apiVersion: v1
kind: Service
metadata:
  name: mtls-server
spec:
  selector:
    app: mtls-server
  ports:
  - port: 8443
    targetPort: 8443
```

**Features**:
- Deployment (not Pod)
- Liveness and readiness probes
- Resource requests/limits
- Service for discovery
- ConfigMap/Secret support
- Multi-replica capable

---

### 7. Troubleshooting Coverage

#### examples/mtls/ - 3 Issues

1. **"Failed to create X509Source"**
   - Brief description
   - Basic socket check

2. **"No identity issued"**
   - Registration entry missing
   - Simple verification command

3. **"TLS handshake failed"**
   - Trust domain mismatch
   - SVID verification

#### examples/mtls-adapters/TROUBLESHOOTING.md - 20+ Issues

**Connection Issues**:
1. "Failed to create X509Source: context deadline exceeded"
   - 4-step diagnostic process
   - Local and Kubernetes solutions
   - Permission checking
2. "Connection Refused"
   - Process verification
   - Port checking
   - Network diagnostics
   - Firewall rules

**SPIRE Agent Issues**:
3. "workload is not registered"
4. Socket permission denied
   - User/group management
   - Permission fixing

**Workload Registration**:
5. "No identity issued" or "no such registration entry"
   - Selector matching
   - Trust domain verification
6. Registration exists but identity not issued
   - Agent attestation
   - Selector debugging

**TLS Handshake**:
7. "TLS handshake failed" or "bad certificate"
   - SVID expiration
   - Authorizer config
   - Debug logging
8. "certificate signed by unknown authority"
   - Trust bundle verification

**Kubernetes-Specific**:
9. Pod can't access socket
   - Node-level debugging
   - Volume mount verification
10. ImagePullBackOff
    - Minikube Docker env
    - Registry configuration
11. CrashLoopBackOff
    - Log analysis
    - Event inspection

**Plus**: Debugging tools, network capture, certificate inspection, error message reference table

---

### 8. API Endpoints

#### examples/mtls/

**3 Endpoints**:
- `GET /api/hello` - Returns greeting
- `GET /api/echo` - Echoes request
- `GET /health` - Health check

#### examples/mtls-adapters/

**4 Endpoints**:
- `GET /api/hello` - Returns greeting with client identity
- `GET /api/echo` - Echoes request details with identity
- `GET /api/identity` - **NEW**: Demonstrates all identity utilities
  ```
  Identity Details
  ================
  Full ID: spiffe://example.org/client
  Trust Domain: example.org
  Path: /client
  Path Segments: [client]

  Path Checks:
    Has /service/ prefix: false
    Has /admin suffix: false
    Matches trust domain 'example.org': true
  ```
- `GET /health` - Health check

---

### 9. Configuration

#### examples/mtls/

**Environment Variables**:
```bash
# Server
SPIRE_AGENT_SOCKET=unix:///tmp/spire-agent/public/api.sock
SERVER_ADDRESS=:8443
ALLOWED_CLIENT_ID=spiffe://example.org/client  # optional

# Client
SPIRE_AGENT_SOCKET=unix:///tmp/spire-agent/public/api.sock
SERVER_URL=https://localhost:8443
EXPECTED_SERVER_ID=spiffe://example.org/server  # optional
```

**Features**:
- Basic env var configuration
- Simple examples

#### examples/mtls-adapters/

**Environment Variables + Advanced Options**:
```bash
# Same env vars as above, PLUS:

# ConfigMap support
kubectl create configmap mtls-server-config \
  --from-file=server.yaml

# Secret support for sensitive data
kubectl create secret generic mtls-server-secrets \
  --from-literal=allowed-client-id="spiffe://example.org/client"
```

**Features**:
- Environment variables
- ConfigMaps for configuration files
- Secrets for sensitive data
- Production security warnings
- Default value documentation
- YAML configuration support

---

### 10. Code Quality & Testability

#### examples/mtls/

**Testing**:
- Harder to mock (direct dependencies)
- Tests coupled to SPIRE
- Limited test utilities

**Maintenance**:
- Changes affect multiple concerns
- Less modular

#### examples/mtls-adapters/

**Testing**:
- Easy to mock adapters
- Unit tests without SPIRE
- Integration tests with SPIRE
- Test utilities provided

**Example Test**:
```go
// Mock the adapter
type mockHTTPAdapter struct {
    httpapi.HTTPServer
}

func TestHandler(t *testing.T) {
    mock := &mockHTTPAdapter{}
    // Test without real SPIRE
}
```

**Maintenance**:
- Changes isolated to adapters
- Business logic separate
- Easier refactoring

---

## When to Use Which

### Use `examples/mtls/` When:

✅ **Learning SPIRE/SPIFFE**
- Understanding basic concepts
- First time using X.509 SVIDs
- Learning mTLS fundamentals

✅ **Quick Proof of Concept**
- Rapid prototyping
- Demonstrating feasibility
- Internal experiments

✅ **Minimal Requirements**
- Simple use case
- No production deployment
- Basic authentication only

✅ **Understanding Internals**
- Learning how SPIRE works
- Debugging SPIRE issues
- Contributing to SPIRE

**Example Scenarios**:
- "I want to learn how SPIRE mTLS works"
- "I need a quick demo for a presentation"
- "I'm troubleshooting SPIRE agent issues"

---

### Use `examples/mtls-adapters/` When:

✅ **Production Deployment**
- Real services in production
- Need reliability and monitoring
- Require health checks

✅ **Kubernetes Environment**
- Deploying to Kubernetes
- Need Deployments and Services
- Want ConfigMaps/Secrets

✅ **Identity-Based Routing**
- Path-based authorization
- Namespace/environment extraction
- Trust domain verification

✅ **Team Collaboration**
- Multiple developers
- Need comprehensive docs
- Onboarding new team members

✅ **Hexagonal Architecture**
- Clean architecture required
- Need testable code
- Planning to extend/customize

✅ **Advanced Features**
- Multiple identity utilities
- Complex authorization
- Service mesh integration

**Example Scenarios**:
- "We're deploying microservices to Kubernetes"
- "We need identity-based routing across namespaces"
- "Our team needs comprehensive troubleshooting docs"
- "We want clean, testable architecture"

---

## Migration Path

### From examples/mtls/ to examples/mtls-adapters/

If you started with `examples/mtls/` and want to migrate:

#### Step 1: Understand Adapter Pattern
```
Current (Direct):
  App → identityserver.Server → SPIRE

Target (Adapter):
  App → httpapi Adapter → SPIRE
```

#### Step 2: Replace Server Implementation

**Before** (examples/mtls):
```go
import "github.com/your-org/identityserver"

server := identityserver.Server{
    Config: config,
}
```

**After** (examples/mtls-adapters):
```go
import "github.com/your-org/internal/adapters/inbound/httpapi"

server := httpapi.NewHTTPServer(ctx, httpapi.ServerConfig{
    SPIRE: spireConfig,
    HTTP:  httpConfig,
})
```

#### Step 3: Replace Client Implementation

**Before**:
```go
import "github.com/your-org/httpclient"

client := httpclient.Client{Config: config}
```

**After**:
```go
import "github.com/your-org/internal/adapters/outbound/httpclient"

client := httpclient.NewSPIFFEHTTPClient(ctx, clientConfig)
```

#### Step 4: Update Identity Extraction

**Before**:
```go
clientID, _ := spiffetls.PeerIDFromConnectionState(*r.TLS)
```

**After**:
```go
import "github.com/your-org/internal/adapters/inbound/httpapi"

clientID, ok := httpapi.GetSPIFFEID(r)
if !ok {
    http.Error(w, "Unauthorized", 401)
    return
}

// Use advanced utilities
if httpapi.MatchesTrustDomain(r, "example.org") {
    // Verified
}
```

#### Step 5: Update Kubernetes Manifests

**Before** (Pod):
```yaml
apiVersion: v1
kind: Pod
```

**After** (Deployment + Service):
```yaml
apiVersion: apps/v1
kind: Deployment
# ... add probes, resources, etc.
---
apiVersion: v1
kind: Service
```

#### Step 6: Adopt Documentation Structure

1. Split README into focused documents
2. Add comprehensive troubleshooting guide
3. Create Kubernetes deployment guide
4. Document identity utilities

---

## Architecture Benefits Comparison

### examples/mtls/ - Simple Direct Architecture

**Pros**:
- ✅ Easy to understand
- ✅ Quick to implement
- ✅ Fewer files to manage
- ✅ Good for learning

**Cons**:
- ❌ Harder to test
- ❌ Less separation of concerns
- ❌ Difficult to extend
- ❌ Coupled to SPIRE implementation

### examples/mtls-adapters/ - Hexagonal Architecture

**Pros**:
- ✅ Clean separation of concerns
- ✅ Easy to test (mock adapters)
- ✅ Easy to extend/customize
- ✅ Production-ready patterns
- ✅ Swap implementations easily
- ✅ Better maintainability

**Cons**:
- ❌ More complex initially
- ❌ More files to understand
- ❌ Requires architectural knowledge
- ❌ Steeper learning curve

---

## Summary

### Quick Decision Matrix

| Your Need | Recommended Example | Reason |
|-----------|-------------------|---------|
| Learning SPIRE | `examples/mtls/` | Simpler, fewer concepts |
| POC/Demo | `examples/mtls/` | Quick setup |
| Production | `examples/mtls-adapters/` | Production-ready features |
| Kubernetes | `examples/mtls-adapters/` | Complete K8s guide |
| Identity routing | `examples/mtls-adapters/` | Advanced utilities |
| Team project | `examples/mtls-adapters/` | Better docs |
| Clean architecture | `examples/mtls-adapters/` | Hexagonal pattern |
| Simple use case | `examples/mtls/` | Less overhead |

### Bottom Line

- **`examples/mtls/`** = Learning tool, POC, understanding fundamentals
- **`examples/mtls-adapters/`** = Production deployment, advanced features, team collaboration

Both examples are maintained and valid. Choose based on your specific needs, team experience, and deployment requirements.

---

## Additional Resources

### For examples/mtls/
- [README](mtls/README.md)
- [SPIFFE/SPIRE Docs](https://spiffe.io/docs/)

### For examples/mtls-adapters/
- [README](mtls-adapters/README.md) - Overview and local development
- [KUBERNETES.md](mtls-adapters/KUBERNETES.md) - Kubernetes deployment guide
- [TROUBLESHOOTING.md](mtls-adapters/TROUBLESHOOTING.md) - Comprehensive troubleshooting

### Architecture Documentation
- [MTLS_IMPLEMENTATION.md](../docs/MTLS_IMPLEMENTATION.md) - Implementation plan
- [ITERATION_1_COMPLETE.md](../docs/ITERATION_1_COMPLETE.md) - Server implementation
- [ITERATION_2_COMPLETE.md](../docs/ITERATION_2_COMPLETE.md) - Client implementation
- [ITERATION_3_COMPLETE.md](../docs/ITERATION_3_COMPLETE.md) - Identity utilities
