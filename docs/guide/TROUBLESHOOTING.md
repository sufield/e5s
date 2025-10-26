# Troubleshooting Guide - SPIRE mTLS

This guide helps diagnose and fix common issues with the SPIRE mTLS server and client.

## Prerequisites

- Access to `kubectl` for the cluster
- Ability to view pod logs
- Understanding of SPIRE architecture (server, agent, workload API)

---

## Common Issues

### Issue: "connection refused"

**Symptoms**: Client cannot connect to server

**Cause**: Server not running or wrong port

**Diagnosis**:
```bash
# Check if pod is running
kubectl get pod -l app=mtls-server

# Check pod logs
kubectl logs -l app=mtls-server --tail=50

# Check what's listening inside the pod
kubectl exec -it <pod-name> -- netstat -tlnp | grep 8443
```

**Fix**:
- Verify the server pod is in Running state
- Check server logs for startup errors
- Verify port 8443 is being used in both server and client
- Ensure Kubernetes service is properly configured

---

### Issue: "create X509Source: connection error"

**Symptoms**: Server or client fails to start with X509Source error

**Cause**: SPIRE agent not running or socket not mounted

**Diagnosis**:
```bash
# Check SPIRE agent status
kubectl get pods -n spire-system -l app=spire-agent

# Verify socket is mounted in pod
kubectl exec -it <pod-name> -- ls -l /spire-socket/api.sock

# Check volume mount in deployment
kubectl describe pod <pod-name> | grep -A 5 "Mounts:"
```

**Fix**:
1. Ensure SPIRE agent is running:
   ```bash
   kubectl get daemonset -n spire-system spire-agent
   ```

2. Verify the volume mount in your deployment YAML:
   ```yaml
   volumeMounts:
   - name: spire-agent-socket
     mountPath: /spire-socket
     readOnly: true
   volumes:
   - name: spire-agent-socket
     hostPath:
       path: /run/spire/sockets
       type: Directory
   ```

3. Restart the SPIRE agent if needed:
   ```bash
   kubectl rollout restart daemonset/spire-agent -n spire-system
   ```

---

### Issue: "unexpected peer ID"

**Symptoms**: Server rejects client connections with "unexpected peer ID" error

**Cause**: Client has different SPIFFE ID than server expects

**Diagnosis**:
```bash
# List all registration entries
kubectl exec -n spire-system spire-server-0 -c spire-server -- \
  /opt/spire/bin/spire-server entry show

# Check server logs for expected vs actual peer ID
kubectl logs -l app=mtls-server --tail=100 | grep "unexpected peer ID"
```

**Fix**:
1. Verify the client's SPIFFE ID matches server configuration:
   - Server expects: Check `ALLOWED_CLIENT_ID` environment variable or code
   - Client actual: Check registration entry for the client workload

2. Update registration entry if needed (see examples/README.md)

3. Ensure both server and client use the same trust domain (example.org)

---

### Issue: "certificate signed by unknown authority"

**Symptoms**: TLS handshake fails with certificate verification error

**Cause**: Client and server have different trust domains or trust bundles

**Diagnosis**:
```bash
# Check trust bundle
kubectl exec -n spire-system spire-server-0 -c spire-server -- \
  /opt/spire/bin/spire-server bundle show

# Verify trust domain in SPIFFE IDs
kubectl exec -n spire-system spire-server-0 -c spire-server -- \
  /opt/spire/bin/spire-server entry show | grep "spiffe://"
```

**Fix**:
1. Verify both pods connect to same SPIRE infrastructure
2. Check trust domain matches (should be example.org by default)
3. Ensure SPIRE server and agents are properly federated if using multiple trust domains
4. Restart workloads to pick up new trust bundles:
   ```bash
   kubectl rollout restart deployment/mtls-server
   kubectl rollout restart deployment/test-client
   ```

---

### Issue: Server panics on startup

**Symptoms**: Server pod crashes immediately after starting

**Cause**: Usually configuration validation failed

**Diagnosis**:
```bash
# Check pod logs for detailed error
kubectl logs -l app=mtls-server --tail=100

# Check environment variables in pod
kubectl exec -it <pod-name> -- env | grep -E "(SPIRE|ALLOWED|SERVER)"

# Check pod events
kubectl describe pod <pod-name> | grep -A 10 "Events:"
```

**Fix**:
1. Verify SPIFFE ID format in configuration (must be: `spiffe://trust-domain/path`)
2. Check for missing required environment variables
3. Validate socket path is correct: `unix:///spire-socket/api.sock`
4. Review deployment YAML for configuration errors

---

### Issue: "context deadline exceeded" or timeout errors

**Symptoms**: Requests timeout even though server is running

**Cause**: Network issues, slow SPIRE operations, or timeout too short

**Diagnosis**:
```bash
# Check network connectivity between pods
CLIENT_POD=$(kubectl get pod -l app=test-client -o jsonpath='{.items[0].metadata.name}')
kubectl exec -it "$CLIENT_POD" -- ping mtls-server

# Check SPIRE agent performance
kubectl logs -n spire-system -l app=spire-agent --tail=100

# Check for resource constraints
kubectl top pods -n spire-system
kubectl top pods -l app=mtls-server
```

**Fix**:
1. Increase timeout in client configuration:
   ```go
   cfg.HTTP.Timeout = 30 * time.Second
   ```

2. Check SPIRE agent has sufficient resources:
   ```bash
   kubectl describe pod -n spire-system -l app=spire-agent
   ```

3. Verify network policies aren't blocking traffic

---

### Issue: "no such file or directory" for socket

**Symptoms**: Cannot find SPIRE agent socket at expected path

**Cause**: Socket path mismatch or volume mount issue

**Diagnosis**:
```bash
# Check what sockets exist in the pod
kubectl exec -it <pod-name> -- find / -name "*.sock" 2>/dev/null

# Verify volume mount
kubectl get pod <pod-name> -o yaml | grep -A 10 "volumeMounts:"
```

**Fix**:
1. Verify socket path in code matches mount path
2. Common paths:
   - `/spire-socket/api.sock` (if using volume mount)
   - `/run/spire/sockets/agent.sock` (if using hostPath)
3. Update deployment YAML to match expected path

---

## Diagnostic Commands

### Check SPIRE Infrastructure Status

```bash
# Check all SPIRE components
kubectl get pods -n spire-system

# Check SPIRE server health
kubectl exec -n spire-system spire-server-0 -c spire-server -- \
  /opt/spire/bin/spire-server healthcheck

# Check SPIRE agent health on specific node
kubectl exec -n spire-system <agent-pod-name> -- \
  /opt/spire/bin/spire-agent healthcheck
```

### Inspect Workload Registration

```bash
# List all entries
kubectl exec -n spire-system spire-server-0 -c spire-server -- \
  /opt/spire/bin/spire-server entry show

# Show specific entry
kubectl exec -n spire-system spire-server-0 -c spire-server -- \
  /opt/spire/bin/spire-server entry show -spiffeID spiffe://example.org/server
```

### View Application Logs

```bash
# Server logs
kubectl logs -l app=mtls-server --tail=100 -f

# Client logs
kubectl logs -l app=test-client --tail=100 -f

# SPIRE agent logs
kubectl logs -n spire-system -l app=spire-agent --tail=100 -f

# SPIRE server logs
kubectl logs -n spire-system spire-server-0 -c spire-server --tail=100 -f
```

---

## Debug Mode

To enable verbose logging for troubleshooting:

### Server Debug Mode

Set environment variable in deployment:
```yaml
env:
- name: DEBUG
  value: "true"
```

Or run with debug flag:
```bash
kubectl exec -it "$POD" -- /tmp/mtls-server -debug
```

### Client Debug Mode

Similar approach for client applications.

---

## Performance Issues

### Slow Request Processing

**Symptoms**: Requests take longer than expected

**Diagnosis**:
```bash
# Check server resource usage
kubectl top pod -l app=mtls-server

# Check for CPU/memory throttling
kubectl describe pod -l app=mtls-server | grep -A 5 "Limits:"

# Profile the application (if profiling enabled)
kubectl port-forward svc/mtls-server 8443:8443
curl http://localhost:6060/debug/pprof/
```

**Fix**:
1. Increase resource limits if needed
2. Review application code for bottlenecks
3. Check SPIRE certificate rotation frequency
4. Consider connection pooling for clients

### High Certificate Rotation Overhead

**Symptoms**: Frequent certificate updates causing performance issues

**Diagnosis**:
Check SPIRE server configuration for TTL settings:
```bash
kubectl get configmap -n spire-system spire-server -o yaml
```

**Fix**:
Adjust `default_x509_svid_ttl` in SPIRE server configuration (requires cluster admin access).

---

## Getting More Help

If issues persist:

1. Collect diagnostic information:
   ```bash
   # Save all relevant logs
   kubectl logs -l app=mtls-server --tail=500 > server-logs.txt
   kubectl logs -l app=test-client --tail=500 > client-logs.txt
   kubectl logs -n spire-system -l app=spire-agent --tail=500 > agent-logs.txt
   kubectl logs -n spire-system spire-server-0 -c spire-server --tail=500 > spire-server-logs.txt

   # Save configuration
   kubectl get pod -l app=mtls-server -o yaml > server-pod.yaml
   kubectl get pod -l app=test-client -o yaml > client-pod.yaml
   ```

2. Review related documentation:
   - [Manual Testing Guide](MANUAL_TESTING_GUIDE.md) - Quick start and basic usage
   - [Testing Guide](TESTING_GUIDE.md) - Detailed test scenarios
   - [SPIRE Documentation](https://spiffe.io/docs/latest/spire/) - Official SPIRE docs

---

## Common Gotchas

1. **Socket path format**: Must include `unix://` prefix (e.g., `unix:///spire-socket/api.sock`)
2. **SPIFFE ID format**: Must be `spiffe://trust-domain/path` (no trailing slash)
3. **Trust domain mismatch**: Client and server must use same trust domain
4. **Volume mount permissions**: Socket must be readable by the application user
5. **Registration timing**: Workload must be registered before it can get certificates
6. **Certificate propagation**: May take a few seconds for new certificates to be available
