# Kubernetes Deployment Guide

This guide covers deploying the mTLS server and client examples to Kubernetes with SPIRE.

## Table of Contents

- [Prerequisites](#prerequisites)
- [Quick Start](#quick-start)
- [Deployment Manifests](#deployment-manifests)
- [Building Images](#building-images)
- [Deploying to Kubernetes](#deploying-to-kubernetes)
- [Verification](#verification)
- [Workload Registration](#workload-registration)

---

## Prerequisites

1. **Kubernetes cluster** - Minikube, kind, or any Kubernetes cluster
2. **SPIRE installed** - Server and agent running in the cluster
3. **kubectl** - Configured to access your cluster
4. **Docker** - For building container images

### Using Minikube

```bash
# Start Minikube with SPIRE
make minikube-up

# Register mTLS workloads
make register-mtls-workloads
```

---

## Quick Start

```bash
# 1. Build images (from repository root)
eval $(minikube docker-env)  # If using Minikube
make build-mtls-images

# 2. Deploy to Kubernetes
kubectl apply -f examples/mtls-adapters/k8s/

# 3. View logs
kubectl logs -l app=mtls-server -f
kubectl logs job/mtls-client

# 4. Check status
kubectl get pods -l app=mtls-server
kubectl get jobs
```

---

## Deployment Manifests

### Server Deployment

`k8s/server-deployment.yaml`:

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: mtls-server
  namespace: default
  labels:
    app: mtls-server
spec:
  replicas: 1
  selector:
    matchLabels:
      app: mtls-server
  template:
    metadata:
      labels:
        app: mtls-server
    spec:
      containers:
      - name: server
        image: mtls-server:latest
        imagePullPolicy: IfNotPresent
        ports:
        - containerPort: 8443
          name: https
          protocol: TCP
        env:
        - name: SPIRE_AGENT_SOCKET
          value: "unix:///spire-agent-socket/api.sock"
        - name: SERVER_ADDRESS
          value: ":8443"
        - name: ALLOWED_CLIENT_ID
          value: ""  # Allow any client from trust domain
        volumeMounts:
        - name: spire-agent-socket
          mountPath: /spire-agent-socket
          readOnly: true
        livenessProbe:
          httpGet:
            path: /health
            port: 8443
            scheme: HTTPS
          initialDelaySeconds: 10
          periodSeconds: 30
        readinessProbe:
          httpGet:
            path: /health
            port: 8443
            scheme: HTTPS
          initialDelaySeconds: 5
          periodSeconds: 10
      volumes:
      - name: spire-agent-socket
        hostPath:
          path: /run/spire/sockets
          type: Directory
---
apiVersion: v1
kind: Service
metadata:
  name: mtls-server
  namespace: default
  labels:
    app: mtls-server
spec:
  type: ClusterIP
  selector:
    app: mtls-server
  ports:
  - port: 8443
    targetPort: 8443
    protocol: TCP
    name: https
```

**Key Configuration**:
- **Socket Mount**: SPIRE agent socket mounted from host at `/run/spire/sockets`
- **Environment**: `SPIRE_AGENT_SOCKET` points to mounted socket
- **Health Checks**: Liveness and readiness probes on `/health`
- **Service**: Exposes server on port 8443 within cluster

### Client Job

`k8s/client-job.yaml`:

```yaml
apiVersion: batch/v1
kind: Job
metadata:
  name: mtls-client
  namespace: default
  labels:
    app: mtls-client
spec:
  template:
    metadata:
      labels:
        app: mtls-client
    spec:
      containers:
      - name: client
        image: mtls-client:latest
        imagePullPolicy: IfNotPresent
        env:
        - name: SPIRE_AGENT_SOCKET
          value: "unix:///spire-agent-socket/api.sock"
        - name: SERVER_URL
          value: "https://mtls-server:8443"
        - name: EXPECTED_SERVER_ID
          value: ""  # Accept any server from trust domain
        volumeMounts:
        - name: spire-agent-socket
          mountPath: /spire-agent-socket
          readOnly: true
      volumes:
      - name: spire-agent-socket
        hostPath:
          path: /run/spire/sockets
          type: Directory
      restartPolicy: OnFailure
  backoffLimit: 3
```

**Key Configuration**:
- **Socket Mount**: Same SPIRE agent socket as server
- **Service Discovery**: Uses Kubernetes DNS (`mtls-server:8443`)
- **Restart Policy**: Retries up to 3 times on failure
- **Job**: Runs once and completes

---

## Building Images

### Dockerfiles

#### Server Dockerfile

`server/Dockerfile`:

```dockerfile
FROM golang:1.21-alpine AS builder

WORKDIR /app
COPY . .

# Build server binary
RUN go build -o /bin/mtls-server ./examples/mtls-adapters/server

FROM alpine:latest
RUN apk --no-cache add ca-certificates

COPY --from=builder /bin/mtls-server /bin/mtls-server

EXPOSE 8443

ENTRYPOINT ["/bin/mtls-server"]
```

#### Client Dockerfile

`client/Dockerfile`:

```dockerfile
FROM golang:1.21-alpine AS builder

WORKDIR /app
COPY . .

# Build client binary
RUN go build -o /bin/mtls-client ./examples/mtls-adapters/client

FROM alpine:latest
RUN apk --no-cache add ca-certificates

COPY --from=builder /bin/mtls-client /bin/mtls-client

ENTRYPOINT ["/bin/mtls-client"]
```

### Building for Minikube

```bash
# Use Minikube's Docker daemon
eval $(minikube docker-env)

# Build server image
docker build -t mtls-server:latest \
  -f examples/mtls-adapters/server/Dockerfile .

# Build client image
docker build -t mtls-client:latest \
  -f examples/mtls-adapters/client/Dockerfile .

# Verify images
docker images | grep mtls
```

### Building for Remote Registry

```bash
# Build and tag for your registry
docker build -t your-registry.io/mtls-server:v1.0 \
  -f examples/mtls-adapters/server/Dockerfile .

docker build -t your-registry.io/mtls-client:v1.0 \
  -f examples/mtls-adapters/client/Dockerfile .

# Push to registry
docker push your-registry.io/mtls-server:v1.0
docker push your-registry.io/mtls-client:v1.0

# Update manifests to use registry images
sed -i 's|mtls-server:latest|your-registry.io/mtls-server:v1.0|g' \
  k8s/server-deployment.yaml
```

---

## Deploying to Kubernetes

### Step 1: Ensure SPIRE is Running

```bash
# Check SPIRE server
kubectl get pods -n spire-system -l app=spire-server

# Check SPIRE agent
kubectl get pods -n spire-system -l app=spire-agent

# Verify agent socket on nodes
kubectl exec -n spire-system spire-agent-xxxxx -- \
  ls -la /run/spire/sockets/
```

### Step 2: Register Workloads

```bash
# Register server workload
kubectl exec -n spire-system spire-server-0 -- \
  spire-server entry create \
    -spiffeID spiffe://example.org/mtls-server \
    -parentID spiffe://example.org/spire-agent \
    -selector k8s:ns:default \
    -selector k8s:pod-label:app:mtls-server

# Register client workload
kubectl exec -n spire-system spire-server-0 -- \
  spire-server entry create \
    -spiffeID spiffe://example.org/mtls-client \
    -parentID spiffe://example.org/spire-agent \
    -selector k8s:ns:default \
    -selector k8s:pod-label:app:mtls-client

# Verify registrations
kubectl exec -n spire-system spire-server-0 -- \
  spire-server entry show
```

### Step 3: Deploy Applications

```bash
# Deploy server
kubectl apply -f examples/mtls-adapters/k8s/server-deployment.yaml

# Wait for server to be ready
kubectl wait --for=condition=ready pod -l app=mtls-server --timeout=60s

# Deploy client
kubectl apply -f examples/mtls-adapters/k8s/client-job.yaml

# Wait for job to complete
kubectl wait --for=condition=complete job/mtls-client --timeout=60s
```

---

## Verification

### Check Server Logs

```bash
# View server startup and requests
kubectl logs -l app=mtls-server -f

# Expected output:
# Starting mTLS server with configuration:
#   Socket: unix:///spire-agent-socket/api.sock
#   Address: :8443
# ✓ Server started successfully on :8443
# Waiting for requests...
# [INFO] Request: GET /api/hello from spiffe://example.org/mtls-client
```

### Check Client Logs

```bash
# View client requests
kubectl logs job/mtls-client

# Expected output:
# Creating mTLS client...
# ✓ Client created successfully
# === Making GET request to /api/hello ===
# Status: 200 OK
# Response: Hello from mTLS server!
# ✓ All requests succeeded
```

### Test Connectivity

```bash
# Port-forward to server
kubectl port-forward svc/mtls-server 8443:8443 &

# Run client locally
SPIRE_AGENT_SOCKET="unix:///tmp/spire-agent/public/api.sock" \
SERVER_URL="https://localhost:8443" \
go run ./examples/mtls-adapters/client
```

### Check Service

```bash
# Get service details
kubectl get svc mtls-server

# Describe service
kubectl describe svc mtls-server

# Test from within cluster
kubectl run -it --rm debug --image=alpine --restart=Never -- sh
# Inside pod:
apk add curl
curl -k https://mtls-server:8443/health
```

---

## Workload Registration

### Selector Types

SPIRE supports various Kubernetes selectors:

```bash
# By namespace
-selector k8s:ns:default

# By pod label
-selector k8s:pod-label:app:mtls-server
-selector k8s:pod-label:version:v1.0

# By service account
-selector k8s:sa:mtls-server

# By node name
-selector k8s:node-name:minikube
```

### Multiple Selectors (AND logic)

```bash
# Match pods in 'default' namespace with label 'app=mtls-server'
spire-server entry create \
  -spiffeID spiffe://example.org/mtls-server \
  -parentID spiffe://example.org/spire-agent \
  -selector k8s:ns:default \
  -selector k8s:pod-label:app:mtls-server
```

### Wildcard Registrations

```bash
# Allow all pods in a namespace
spire-server entry create \
  -spiffeID spiffe://example.org/namespace/default \
  -parentID spiffe://example.org/spire-agent \
  -selector k8s:ns:default
```

### View and Delete Entries

```bash
# List all entries
kubectl exec -n spire-system spire-server-0 -- \
  spire-server entry show

# Show specific entry
kubectl exec -n spire-system spire-server-0 -- \
  spire-server entry show -spiffeID spiffe://example.org/mtls-server

# Delete entry
kubectl exec -n spire-system spire-server-0 -- \
  spire-server entry delete -entryID <entry-id>
```

---

## Advanced Configuration

### Using ConfigMaps for Configuration

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: mtls-server-config
data:
  server.yaml: |
    spire:
      socket_path: unix:///spire-agent-socket/api.sock
      trust_domain: example.org
    http:
      enabled: true
      address: :8443
      authentication:
        peer_verification: trust-domain
```

Mount in deployment:
```yaml
volumeMounts:
- name: config
  mountPath: /etc/mtls
volumes:
- name: config
  configMap:
    name: mtls-server-config
```

### Using Secrets for Sensitive Config

```bash
# Create secret with allowed client ID
kubectl create secret generic mtls-server-secrets \
  --from-literal=allowed-client-id="spiffe://example.org/specific-client"
```

Reference in deployment:
```yaml
env:
- name: ALLOWED_CLIENT_ID
  valueFrom:
    secretKeyRef:
      name: mtls-server-secrets
      key: allowed-client-id
```

### Multi-Replica Deployment

```yaml
spec:
  replicas: 3
  strategy:
    type: RollingUpdate
    rollingUpdate:
      maxUnavailable: 1
      maxSurge: 1
```

### Resource Limits

```yaml
resources:
  requests:
    memory: "64Mi"
    cpu: "100m"
  limits:
    memory: "128Mi"
    cpu: "200m"
```

---

## Cleanup

```bash
# Delete deployments
kubectl delete -f examples/mtls-adapters/k8s/

# Or individually
kubectl delete deployment mtls-server
kubectl delete job mtls-client
kubectl delete svc mtls-server

# Remove SPIRE entries
kubectl exec -n spire-system spire-server-0 -- \
  spire-server entry delete -spiffeID spiffe://example.org/mtls-server

kubectl exec -n spire-system spire-server-0 -- \
  spire-server entry delete -spiffeID spiffe://example.org/mtls-client
```

---

## Troubleshooting

For common issues and solutions, see [TROUBLESHOOTING.md](TROUBLESHOOTING.md).

## References

- [Kubernetes Documentation](https://kubernetes.io/docs/)
- [SPIRE Kubernetes Quickstart](https://spiffe.io/docs/latest/spire/installing/)
- [Main README](README.md)
