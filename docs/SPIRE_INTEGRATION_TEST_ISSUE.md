# SPIRE Integration Test Timeout Issue

## Problem Statement

Integration tests for SPIRE Workload API client are failing with "context deadline exceeded" when attempting to connect to the SPIRE agent's Workload API socket.

## Stack Overflow Question Format

---

### Title

SPIRE Workload API integration tests timeout with "context deadline exceeded" - socket exists but connection fails

### Tags

`go` `spire` `spiffe` `integration-testing` `grpc`

### Question Body

I'm writing integration tests for a Go application that uses the SPIRE Workload API (via the `go-spiffe/v2` SDK). The tests are failing with timeout errors even though the SPIRE agent is running and the socket file exists.

#### Environment

- **SPIRE Version**: Latest (using distroless agent image)
- **go-spiffe SDK**: v2.6.0
- **Go Version**: 1.25
- **Deployment**: Kubernetes (Minikube) with hostPath volume for socket
- **Test Framework**: Go testing with `-tags=integration`

#### Test Code

```go
//go:build integration
// +build integration

package spire_test

func TestClientConnection(t *testing.T) {
    ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
    defer cancel()

    socketPath := os.Getenv("SPIRE_AGENT_SOCKET")
    if socketPath == "" {
        socketPath = "unix:///tmp/spire-agent/public/api.sock"
    }

    cfg := &ports.WorkloadAPIConfig{
        SocketPath: socketPath,
    }

    client, err := spire.NewClient(ctx, cfg)
    require.NoError(t, err, "Failed to create SPIRE client")
    defer client.Close()
}
```

#### Client Implementation

```go
func NewClient(ctx context.Context, cfg *ports.WorkloadAPIConfig) (*Client, error) {
    if cfg.SocketPath == "" {
        return nil, fmt.Errorf("socket path is required")
    }

    clientOpts := workloadapi.WithClientOptions(
        workloadapi.WithAddr(cfg.SocketPath),
    )

    // This line times out after 30 seconds
    source, err := workloadapi.NewX509Source(ctx, clientOpts)
    if err != nil {
        return nil, fmt.Errorf("create X509 source (Workload API may be unavailable): %w", err)
    }

    return &Client{source: source}, nil
}
```

#### Error Output

```
=== RUN   TestClientConnection
    integration_test.go:42:
        Error Trace:    integration_test.go:42
        Error:          Received unexpected error:
                        create X509 source (Workload API may be unavailable): context deadline exceeded
        Test:           TestClientConnection
        Messages:       Failed to create SPIRE client
--- FAIL: TestClientConnection (30.00s)
```

#### What I've Verified

1. **Socket exists**: Verified via Minikube node SSH
   ```bash
   minikube ssh -- "test -S /tmp/spire-agent/public/api.sock && echo exists"
   # Output: exists
   ```

2. **SPIRE agent is running**:
   ```bash
   kubectl get pods -n spire-system
   # spire-agent-xyz   1/1   Running
   ```

3. **Socket permissions**: Socket is readable (verified via ls -l)

4. **hostPath volume**: Properly mounted in test pod
   ```yaml
   volumes:
     - name: spire-agent-socket
       hostPath:
         path: /tmp/spire-agent/public
         type: Directory
   volumeMounts:
     - name: spire-agent-socket
       mountPath: /spire-socket
       readOnly: true
   ```

5. **Environment variable**: `SPIRE_AGENT_SOCKET=unix:///spire-socket/api.sock` is set correctly

#### Distroless Agent Consideration

The SPIRE agent runs in a distroless container (no shell), which makes debugging difficult. The socket is created by the agent and exposed via hostPath volume.

#### What Works

- Unit tests (with mocked SPIRE client) pass instantly
- Manual testing with `spire-agent api fetch x509` works when agent has shell
- The agent itself is healthy (logs show no errors)

#### Questions

1. **Is there a way to test socket connectivity** before calling `workloadapi.NewX509Source()`? The SDK doesn't seem to provide a "ping" or health check method.

2. **Could this be a timing issue?** The agent might not be ready when tests start. How long should I wait for the Workload API to be available?

3. **Is there a better pattern for integration testing** with SPIRE in Kubernetes? Should I use init containers, readiness probes, or retry logic?

4. **Debug logging**: How can I enable debug logging in the `go-spiffe` SDK to see what's happening during the connection attempt?

#### Attempted Solutions

- ✅ Increased timeout from 10s to 30s (still fails)
- ✅ Verified socket path format (`unix://` prefix)
- ✅ Checked socket exists before running test
- ❌ Cannot use `kubectl exec` into agent pod (distroless)
- ❌ Cannot run `spire-agent api fetch` (no binary in distroless)

---

## Additional Context for Internal Use

### Kubernetes Configuration

The test pod is deployed with:

```yaml
apiVersion: v1
kind: Pod
metadata:
  name: integration-test
  namespace: spire-system
spec:
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

### CI Script Context

From `scripts/run-integration-tests-ci.sh`:

```bash
# Socket verification (works for non-distroless)
if kubectl exec -n "$NS" "$AGENT_POD" -- test -S /tmp/spire-agent/public/api.sock; then
    success "Socket verified via agent pod"
elif minikube ssh -- "test -S /tmp/spire-agent/public/api.sock"; then
    success "Socket verified via Minikube node"
fi

# Test pod deployment
kubectl apply -f - <<EOF
apiVersion: v1
kind: Pod
metadata:
  name: integration-test
  namespace: spire-system
spec:
  # ... config from above ...
EOF

# Wait for pod to be ready
kubectl wait --for=condition=Ready pod/integration-test -n spire-system --timeout=60s

# Run tests
kubectl exec integration-test -n spire-system -- ./integration.test -test.v
```

### Possible Root Causes

1. **gRPC connection timeout**: The SDK might be trying to establish a gRPC connection but the agent isn't listening on the expected address

2. **Socket permissions**: Even though the socket exists, it might not be accessible from within the container

3. **Agent registration**: The workload might not have a registration entry in SPIRE server

4. **Network policy**: Kubernetes network policies might be blocking the connection

5. **SELinux/AppArmor**: Security contexts might prevent socket access

6. **Agent not ready**: The agent might still be initializing even though the pod shows "Running"

### Questions for Stack Overflow Community

The formatted question above focuses on:
- Clear problem statement
- Minimal reproducible example
- What has been tried
- Specific technical questions
- Relevant error messages
- Environment details

This should help get quality responses from the SPIRE/SPIFFE community.

## Next Steps

1. **Add debug logging** to see actual gRPC connection attempts
2. **Verify workload registration** exists in SPIRE server
3. **Test socket connectivity** from within the test pod using a simple gRPC client
4. **Check agent logs** for connection attempts during test execution
5. **Try non-distroless agent** temporarily to enable debugging

## References

- [SPIRE Workload API Documentation](https://spiffe.io/docs/latest/spire-about/spire-concepts/#workload-api)
- [go-spiffe SDK Documentation](https://pkg.go.dev/github.com/spiffe/go-spiffe/v2)
- [SPIRE Integration Testing Examples](https://github.com/spiffe/spire/tree/main/test/integration)
