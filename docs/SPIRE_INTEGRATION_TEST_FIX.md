# SPIRE Integration Test Fix Guide

## Problem Summary

Integration tests fail with "context deadline exceeded" when calling `workloadapi.NewX509Source()` because the SPIRE Workload API doesn't return an X.509 SVID within the timeout period (30s).

**Root Cause**: The workload (integration test pod) likely has no matching registration entry in SPIRE server, so the agent cannot issue an SVID.

## Solution Steps (Prioritized by Likelihood)

### 1. Verify and Create Workload Registration Entries ⭐ **Most Likely Fix**

#### Why This Matters
SPIRE requires explicit registration entries that map workload selectors (Kubernetes namespace, service account, pod labels) to SPIFFE IDs. Without a matching entry, the Workload API won't respond with an SVID.

#### Diagnostic Steps

1. **List existing entries** (if server has shell access):
   ```bash
   kubectl exec -it -n spire-system <spire-server-pod> -- \
     /opt/spire/bin/spire-server entry list
   ```

2. **Check for entries matching your test pod**:
   - Look for selectors like `k8s:ns:spire-system` and `k8s:sa:default`
   - If none exist, that's the problem!

3. **Identify your pod's metadata**:
   ```bash
   kubectl describe pod integration-test -n spire-system
   ```
   Note the namespace, service account, and any labels.

#### Fix: Create Registration Entry

Replace `<domain>` with your trust domain (e.g., `example.org`):

```bash
kubectl exec -it -n spire-system <spire-server-pod> -- \
  /opt/spire/bin/spire-server entry create \
    -spiffeID spiffe://<domain>/integration-test \
    -parentID spiffe://<domain>/spire-agent \
    -selector k8s:ns:spire-system \
    -selector k8s:sa:default
```

**With pod labels** (more specific matching):
```bash
kubectl exec -it -n spire-system <spire-server-pod> -- \
  /opt/spire/bin/spire-server entry create \
    -spiffeID spiffe://<domain>/integration-test \
    -parentID spiffe://<domain>/spire-agent \
    -selector k8s:ns:spire-system \
    -selector k8s:sa:default \
    -selector k8s:pod-label:app:integration-test
```

#### If Server is Distroless

Temporarily switch to a non-distroless image for debugging:

```yaml
# In your SPIRE server deployment
spec:
  containers:
  - name: spire-server
    image: ghcr.io/spiffe/spire-server:1.9.0  # Non-distroless
    # ... rest of config
```

Or use the SPIRE Server API via a tool pod (requires API enabled in server config).

---

### 2. Confirm Agent Attestation and Node Registration

#### Why This Matters
If the agent itself isn't properly attested, it can't handle workload requests.

#### Diagnostic Steps

1. **List attested agents**:
   ```bash
   kubectl exec -it -n spire-system <spire-server-pod> -- \
     /opt/spire/bin/spire-server agent list
   ```

2. **Expected output**:
   ```
   Found 1 attested agent:
   SPIFFE ID         : spiffe://example.org/spire-agent
   Attestation type  : k8s_psat
   Expiration time   : ...
   Serial number     : ...
   ```

3. **Check agent logs**:
   ```bash
   kubectl logs -n spire-system <spire-agent-pod> | grep -i attest
   ```
   Look for errors like "failed to attest" or "no selectors found".

#### Fix: Create Node Registration Entry

If agent is missing:

```bash
kubectl exec -it -n spire-system <spire-server-pod> -- \
  /opt/spire/bin/spire-server entry create \
    -spiffeID spiffe://<domain>/spire-agent \
    -selector k8s_psat:cluster:minikube \
    -selector k8s_psat:agent_ns:spire-system \
    -selector k8s_psat:agent_sa:spire-agent \
    -node
```

**Adjust selectors** based on your attestor configuration:
- For `k8s_sat`: Use `k8s_sat:` prefix instead of `k8s_psat:`
- For custom cluster name: Replace `minikube` with actual cluster identifier

---

### 3. Test Socket Connectivity Directly

#### Why This Matters
Container access to the socket might fail due to SELinux/AppArmor, mount issues, or network policies.

#### Create Debug Pod

```yaml
apiVersion: v1
kind: Pod
metadata:
  name: socket-debug
  namespace: spire-system
spec:
  containers:
  - name: debug
    image: alpine:latest
    command: ["/bin/sh", "-c", "sleep infinity"]
    volumeMounts:
    - name: spire-agent-socket
      mountPath: /spire-socket
      readOnly: true
  volumes:
  - name: spire-agent-socket
    hostPath:
      path: /tmp/spire-agent/public
      type: Directory
```

#### Test Socket Access

```bash
# Deploy debug pod
kubectl apply -f debug-pod.yaml

# Exec into it
kubectl exec -it -n spire-system socket-debug -- /bin/sh

# Check socket exists and is accessible
ls -la /spire-socket/
# Should show: api.sock

# Test basic connectivity (if nc is available)
nc -U /spire-socket/api.sock
# Should connect (Ctrl+C to exit)

# Install grpcurl for full test
apk add --no-cache curl tar
curl -L https://github.com/fullstorydev/grpcurl/releases/download/v1.8.7/grpcurl_1.8.7_linux_x86_64.tar.gz | tar xz
mv grpcurl /usr/local/bin/

# Test Workload API gRPC call
grpcurl -unix -plaintext \
  -authority spiffe://example.org \
  /spire-socket/api.sock \
  spiffe.workload.Workload/FetchX509SVID
```

#### Troubleshooting Socket Issues

If socket isn't accessible:

1. **Check SELinux** (on Minikube node):
   ```bash
   minikube ssh -- getenforce
   # If "Enforcing", try: setenforce 0 (temporarily)
   ```

2. **Try different hostPath type**:
   ```yaml
   hostPath:
     path: /tmp/spire-agent/public
     type: DirectoryOrCreate  # Instead of Directory
   ```

3. **Add privileged mode** (temporary debugging only):
   ```yaml
   securityContext:
     privileged: true
   ```

---

### 4. Add Retry Logic and Readiness Checks

#### Update Integration Test Code

Add retries for transient timing issues:

```go
import (
    "github.com/avast/retry-go/v4"
    "github.com/spiffe/go-spiffe/v2/workloadapi"
)

func NewClient(ctx context.Context, cfg *ports.WorkloadAPIConfig) (*Client, error) {
    if cfg.SocketPath == "" {
        return nil, fmt.Errorf("socket path is required")
    }

    clientOpts := workloadapi.WithClientOptions(
        workloadapi.WithAddr(cfg.SocketPath),
    )

    var source *workloadapi.X509Source
    err := retry.Do(
        func() error {
            var err error
            source, err = workloadapi.NewX509Source(ctx, clientOpts)
            return err
        },
        retry.Attempts(5),
        retry.Delay(5*time.Second),
        retry.DelayType(retry.FixedDelay),
        retry.LastErrorOnly(true),
        retry.OnRetry(func(n uint, err error) {
            log.Printf("Retry %d/5: %v", n+1, err)
        }),
    )
    if err != nil {
        return nil, fmt.Errorf("failed to create X509 source after retries: %w", err)
    }

    return &Client{source: source}, nil
}
```

#### Add Init Container to Test Pod

Wait for socket before running tests:

```yaml
apiVersion: v1
kind: Pod
metadata:
  name: integration-test
  namespace: spire-system
spec:
  initContainers:
  - name: wait-for-socket
    image: busybox:latest
    command:
    - sh
    - -c
    - |
      echo "Waiting for SPIRE agent socket..."
      until [ -S /spire-socket/api.sock ]; do
        echo "Socket not found, retrying in 2s..."
        sleep 2
      done
      echo "Socket found! Waiting 5s for agent initialization..."
      sleep 5
      echo "Ready to run tests"
    volumeMounts:
    - name: spire-agent-socket
      mountPath: /spire-socket
      readOnly: true
  containers:
  - name: test
    image: integration-test:latest
    env:
    - name: SPIRE_AGENT_SOCKET
      value: "unix:///spire-socket/api.sock"
    volumeMounts:
    - name: spire-agent-socket
      mountPath: /spire-socket
      readOnly: true
  volumes:
  - name: spire-agent-socket
    hostPath:
      path: /tmp/spire-agent/public
      type: Directory
```

---

### 5. Enable Debug Logging

#### Enable gRPC Debug Logs in Test Pod

```yaml
env:
- name: GRPC_GO_LOG_VERBOSITY_LEVEL
  value: "99"
- name: GRPC_GO_LOG_SEVERITY_LEVEL
  value: "info"
- name: SPIRE_AGENT_SOCKET
  value: "unix:///spire-socket/api.sock"
```

#### Enable SPIRE Agent Debug Logging

Update agent ConfigMap:

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: spire-agent
  namespace: spire-system
data:
  agent.conf: |
    agent {
      data_dir = "/run/spire"
      log_level = "DEBUG"  # Changed from INFO
      server_address = "spire-server"
      server_port = "8081"
      socket_path = "/tmp/spire-agent/public/api.sock"
      trust_domain = "example.org"
    }
    # ... rest of config
```

Then restart agent:
```bash
kubectl rollout restart daemonset/spire-agent -n spire-system
```

#### View Enhanced Logs

```bash
# Agent logs
kubectl logs -n spire-system <spire-agent-pod> -f | grep -E "(Workload|attest|SVID)"

# Test pod logs
kubectl logs -n spire-system integration-test
```

---

### 6. Additional Quick Checks

#### Network Policies

If you have NetworkPolicies, ensure they allow local traffic:

```bash
kubectl get networkpolicies -n spire-system
```

Temporarily delete restrictive policies for testing:
```bash
kubectl delete networkpolicy <policy-name> -n spire-system
```

#### Timing: Add Delay in CI Script

In `scripts/run-integration-tests-ci.sh`:

```bash
# Wait for pod ready
kubectl wait --for=condition=Ready pod/integration-test -n spire-system --timeout=60s

# Additional grace period for agent initialization
echo "Waiting 30s for SPIRE agent to fully initialize..."
sleep 30

# Now run tests
kubectl exec integration-test -n spire-system -- ./integration.test -test.v
```

#### Switch to Non-Distroless Agent Temporarily

In agent DaemonSet:

```yaml
spec:
  template:
    spec:
      containers:
      - name: spire-agent
        image: ghcr.io/spiffe/spire-agent:1.9.0  # Non-distroless
        # ... rest stays the same
```

This enables:
```bash
kubectl exec -n spire-system <spire-agent-pod> -- \
  /opt/spire/bin/spire-agent api fetch x509 \
  -socketPath /tmp/spire-agent/public/api.sock
```

---

## Complete Troubleshooting Workflow

### Step-by-Step Diagnostic Process

1. **Check Server is Running**
   ```bash
   kubectl get pods -n spire-system -l app=spire-server
   ```

2. **List Registration Entries**
   ```bash
   kubectl exec -it -n spire-system <spire-server-pod> -- \
     /opt/spire/bin/spire-server entry list
   ```

3. **Check Agent is Attested**
   ```bash
   kubectl exec -it -n spire-system <spire-server-pod> -- \
     /opt/spire/bin/spire-server agent list
   ```

4. **Create Workload Entry** (if missing)
   ```bash
   kubectl exec -it -n spire-system <spire-server-pod> -- \
     /opt/spire/bin/spire-server entry create \
       -spiffeID spiffe://example.org/integration-test \
       -parentID spiffe://example.org/spire-agent \
       -selector k8s:ns:spire-system \
       -selector k8s:sa:default
   ```

5. **Verify Socket Access** (debug pod)
   ```bash
   kubectl apply -f debug-pod.yaml
   kubectl exec -it socket-debug -n spire-system -- ls -la /spire-socket/
   ```

6. **Re-run Tests**
   ```bash
   kubectl delete pod integration-test -n spire-system
   kubectl apply -f integration-test-pod.yaml
   kubectl logs -f integration-test -n spire-system
   ```

---

## Expected Success Indicators

### Successful Registration
```
Entry ID         : ...
SPIFFE ID        : spiffe://example.org/integration-test
Parent ID        : spiffe://example.org/spire-agent
Revision         : 0
X509-SVID TTL    : default
JWT-SVID TTL     : default
Selector         : k8s:ns:spire-system
Selector         : k8s:sa:default
```

### Successful Test Output
```
=== RUN   TestClientConnection
--- PASS: TestClientConnection (0.15s)
=== RUN   TestFetchX509SVID
--- PASS: TestFetchX509SVID (0.08s)
```

### Agent Logs (Success)
```
level=debug msg="Received workload API request" method=FetchX509SVID
level=debug msg="Fetched X.509 SVID" spiffe_id=spiffe://example.org/integration-test
```

---

## Common Pitfalls

1. **Wrong parentID**: Use the agent's SPIFFE ID, not the server's
2. **Selector typos**: `k8s:ns:` not `k8s:namespace:`
3. **Trust domain mismatch**: Must match server config
4. **Service account**: Ensure pod uses the SA specified in selectors
5. **Agent not ready**: Wait for agent to fully initialize (30s after pod Ready)

---

## References

- [SPIRE Kubernetes Quickstart](https://spiffe.io/docs/latest/try/getting-started-k8s/)
- [SPIRE Server CLI Reference](https://spiffe.io/docs/latest/deploying/spire_server/)
- [Workload Registration](https://spiffe.io/docs/latest/deploying/registering/)
- [Kubernetes Workload Attestor](https://github.com/spiffe/spire/blob/main/doc/plugin_agent_workloadattestor_k8s.md)
