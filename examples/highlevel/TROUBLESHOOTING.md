# Troubleshooting Guide

Common issues and solutions when working with e5s and SPIRE.

---

## Issue: "failed to create X509Source: workload endpoint socket address is not configured"

**Cause**: The SPIRE Agent socket path in `e5s.yaml` doesn't match the actual socket location.

**Solution**: Ensure the socket path is correct.

```bash
# Check where SPIRE Agent socket actually is
kubectl exec -n spire \
  $(kubectl get pod -n spire -l app.kubernetes.io/name=agent -o jsonpath='{.items[0].metadata.name}') \
  -- ls -la /tmp/spire-agent/public/

# Update e5s.yaml with correct path
```

**Example config:**
```yaml
spire:
  workload_socket: unix:///tmp/spire-agent/public/api.sock
```

---

## Issue: "initial SPIRE fetch timed out after 30s"

**Causes**:
1. SPIRE Agent is not running
2. Socket path is incorrect
3. Workload is not registered in SPIRE
4. SPIRE Agent is slow to start (especially in development)

**Solution 1: Verify SPIRE Agent is running**

```bash
# Check SPIRE Agent pod status
kubectl get pods -n spire -l app.kubernetes.io/name=agent

# Should show Running status
# NAME                READY   STATUS    RESTARTS   AGE
# spire-agent-xxxxx   1/1     Running   0          5m
```

**Solution 2: Verify workload registration**

```bash
# List all registration entries
SERVER_POD=$(kubectl get pod -n spire -l app.kubernetes.io/name=server -o jsonpath='{.items[0].metadata.name}')
kubectl exec -n spire $SERVER_POD -c spire-server -- \
  /opt/spire/bin/spire-server entry show

# You should see entries for your server and client workloads
```

**Solution 3: Increase timeout in e5s.yaml**

For development environments where SPIRE Agent may start slowly:

```yaml
spire:
  workload_socket: unix:///tmp/spire-agent/public/api.sock
  initial_fetch_timeout: 60s  # Increase from default 30s
```

For production, keep the timeout lower to fail fast if there are issues.

---

## Issue: "tls: failed to verify certificate: x509: certificate signed by unknown authority"

**Cause**: Client doesn't trust the server's certificate authority (SPIRE), or they're using different SPIRE deployments.

**Solution**: Ensure both client and server:
1. Connect to the same SPIRE deployment
2. Use the same trust domain
3. Have matching socket paths

**Check your e5s.yaml:**
```yaml
spire:
  workload_socket: unix:///tmp/spire-agent/public/api.sock  # Must be same for both

server:
  allowed_client_trust_domain: "example.org"  # Trust domain

client:
  expected_server_trust_domain: "example.org"  # Must match server's trust domain
```

**Verify trust domain:**
```bash
# Check SPIRE Server trust domain
kubectl exec -n spire $SERVER_POD -c spire-server -- \
  /opt/spire/bin/spire-server show

# Look for: Trust domain: example.org
```

---

## Issue: "unauthorized" response from server

**Cause**: Client presented a valid certificate, but it doesn't match the server's authorization policy.

**Scenario 1: Trust domain mismatch**

```yaml
# Server config
server:
  allowed_client_trust_domain: "example.org"  # Server expects this

# Client has SPIFFE ID: spiffe://different.org/client  # Wrong trust domain!
```

**Solution**: Ensure the client's SPIFFE ID is in the allowed trust domain:

```bash
# Check client's actual SPIFFE ID
kubectl exec -n spire $SERVER_POD -c spire-server -- \
  /opt/spire/bin/spire-server entry show | grep -A 5 "client"

# Update server config to match
```

**Scenario 2: Specific SPIFFE ID mismatch**

```yaml
# Server config requires specific ID
server:
  allowed_client_spiffe_id: "spiffe://example.org/frontend"

# But client has: spiffe://example.org/client  # Wrong workload!
```

**Solution**: Either:
1. Update server config to accept the client's SPIFFE ID
2. Use trust domain-based authorization instead (more flexible):
   ```yaml
   server:
     allowed_client_trust_domain: "example.org"
   ```

---

## Issue: Connection hangs or times out

**Cause**: Network connectivity issues, firewall rules, or server not listening.

**Solution 1: Verify server is running**

```bash
# Check if server process is running
ps aux | grep server

# Check if server is listening on the port
netstat -tuln | grep 8443
# or
ss -tuln | grep 8443
```

**Solution 2: Check firewall rules**

```bash
# On Linux, check iptables
sudo iptables -L -n | grep 8443

# On macOS, check if firewall is blocking
sudo pfctl -s rules | grep 8443
```

**Solution 3: Verify DNS resolution (if using hostnames)**

```bash
# Test DNS resolution
nslookup server.example.org

# Test connectivity
ping server.example.org
```

---

## Issue: "no identity issued" when running locally

**Cause**: The workload is running outside Kubernetes but trying to get identity from SPIRE in Kubernetes.

**Solution**: Use port forwarding to expose SPIRE Agent socket (see Tutorial Step 3):

```bash
# Port forward SPIRE Agent
kubectl port-forward -n spire \
  $(kubectl get pod -n spire -l app.kubernetes.io/name=agent -o jsonpath='{.items[0].metadata.name}') \
  8081:8081
```

Alternatively, run SPIRE Agent locally:
- Download SPIRE from https://github.com/spiffe/spire/releases
- Configure local agent to connect to SPIRE Server
- Register your local workload

---

## Issue: Certificate rotation causing connection failures

**Cause**: This should never happen with e5s - certificate rotation is automatic and seamless.

**If you see this**:
1. Check SPIRE Agent logs for errors:
   ```bash
   kubectl logs -n spire -l app.kubernetes.io/name=agent
   ```

2. Check that SVIDs are being renewed:
   ```bash
   # SVIDs typically have 1-hour TTL and rotate every 30 minutes
   kubectl exec -n spire $SERVER_POD -c spire-server -- \
     /opt/spire/bin/spire-server entry show
   ```

3. Report this as a bug - automatic rotation is a core feature

---

## Issue: Performance issues or high memory usage

**Cause**: Multiple X509Source instances, or not cleaning up properly.

**Solution**: Reuse X509Source across requests:

**Wrong (creates new source per request):**
```go
// DON'T DO THIS
for i := 0; i < 1000; i++ {
    resp, err := e5s.Get(url)  // Creates new source each time!
    // ...
}
```

**Right (reuse client):**
```go
// DO THIS
client, shutdown, err := e5s.NewClient()
if err != nil {
    log.Fatal(err)
}
defer shutdown()

for i := 0; i < 1000; i++ {
    resp, err := client.Get(url)  // Reuses same source
    // ...
}
```

---

## Getting More Help

If you're still stuck:

1. **Enable debug logging** (if available in future versions)
2. **Check SPIRE logs**:
   ```bash
   # SPIRE Server logs
   kubectl logs -n spire -l app.kubernetes.io/name=server -c spire-server

   # SPIRE Agent logs
   kubectl logs -n spire -l app.kubernetes.io/name=agent
   ```

3. **Verify SPIRE health**:
   ```bash
   kubectl exec -n spire $SERVER_POD -c spire-server -- \
     /opt/spire/bin/spire-server healthcheck
   ```

4. **Open an issue**: [e5s GitHub Issues](https://github.com/sufield/e5s/issues)

---

## Common Pitfalls

### 1. Forgetting to register workloads
- **Symptom**: "no identity issued"
- **Fix**: Create registration entry in SPIRE Server

### 2. Socket path mismatch
- **Symptom**: "workload endpoint socket address is not configured"
- **Fix**: Verify socket path matches between config and actual location

### 3. Trust domain confusion
- **Symptom**: "certificate signed by unknown authority"
- **Fix**: Ensure all workloads use same SPIRE deployment and trust domain

### 4. Running outside Kubernetes without proper setup
- **Symptom**: Can't connect to SPIRE Agent
- **Fix**: Use port forwarding or run SPIRE Agent locally

### 5. Not cleaning up resources
- **Symptom**: Memory leaks
- **Fix**: Always call the `shutdown()` function returned by `NewClient()` or `StartServer()`

---

## Prevention Tips

1. **Start simple**: Use the high-level API (`e5s.Get()`, `e5s.Run()`) until you need more control
2. **Test locally first**: Use the tutorial setup before moving to complex environments
3. **Verify SPIRE first**: Ensure SPIRE is working before debugging e5s
4. **Use trust domains initially**: Switch to specific SPIFFE IDs only after everything works
5. **Keep configs consistent**: Use the same trust domain everywhere
6. **Check logs early**: SPIRE logs are your friend when things go wrong
