# Troubleshooting Guide

Common issues and solutions when working with e5s and SPIRE.

---

## Issue: "exists and cannot be imported into the current release" during SPIRE installation

**Error message:**
```
Error: INSTALLATION FAILED: Unable to continue with install: ClusterRole "spire-agent" in namespace "" exists
and cannot be imported into the current release: invalid ownership metadata;
annotation validation error: key "meta.helm.sh/release-namespace" must equal "spire": current value is "spire-system"
```

**Cause**: Leftover resources from a previous SPIRE installation attempt (possibly in a different namespace).

**Solution**: Clean up all SPIRE resources including cluster-scoped resources and reinstall from scratch.

Delete any existing Helm releases:
```bash
helm uninstall spire -n spire 2>/dev/null || true
helm uninstall spire-server -n spire 2>/dev/null || true
helm uninstall spire-agent -n spire 2>/dev/null || true
helm uninstall spire-crds -n spire 2>/dev/null || true
```

Delete namespace-scoped resources:
```bash
kubectl delete namespace spire 2>/dev/null || true
```

Delete cluster-scoped resources (ClusterRole, ClusterRoleBinding, CSIDriver, etc.):
```bash
kubectl delete clusterrole spire-agent spire-server spire-controller-manager 2>/dev/null || true
kubectl delete clusterrolebinding spire-agent spire-server spire-controller-manager 2>/dev/null || true
kubectl delete csidriver csi.spiffe.io 2>/dev/null || true
kubectl delete validatingwebhookconfiguration spire-server 2>/dev/null || true
kubectl delete mutatingwebhookconfiguration spire-controller-manager 2>/dev/null || true
```

Delete CRDs (Custom Resource Definitions):
```bash
kubectl delete crd clusterspiffeids.spire.spiffe.io 2>/dev/null || true
kubectl delete crd clusterstaticentries.spire.spiffe.io 2>/dev/null || true
kubectl delete crd clusterfederatedtrustdomains.spire.spiffe.io 2>/dev/null || true
kubectl delete crd controllermanagerconfigs.spire.spiffe.io 2>/dev/null || true
```

Wait for cleanup to complete:
```bash
sleep 5
```

Recreate the namespace:
```bash
kubectl create namespace spire
```

Install SPIRE CRDs:
```bash
helm install spire-crds spire-crds \
  --repo https://spiffe.github.io/helm-charts-hardened/ \
  --namespace spire
```

Install SPIRE:
```bash
helm install spire spire \
  --repo https://spiffe.github.io/helm-charts-hardened/ \
  --namespace spire \
  --set global.spire.trustDomain=example.org \
  --set global.spire.clusterName=minikube-cluster
```

**If namespace deletion hangs**, check for stuck resources:

Check what's blocking namespace deletion:
```bash
kubectl api-resources --verbs=list --namespaced -o name | \
  xargs -n 1 kubectl get --show-kind --ignore-not-found -n spire
```

Force cleanup if needed:
```bash
kubectl delete namespace spire --grace-period=0 --force
```

---

## Issue: SPIRE Agent stuck in "Pending" state in Minikube

**Symptom:**
```
kubectl get pods -n spire
NAME                    READY   STATUS    RESTARTS   AGE
spire-agent-xxxxx       0/1     Pending   0          5m
spire-server-0          2/2     Running   0          5m
```

**Error from `kubectl describe pod`:**
```
Warning  FailedScheduling  0/1 nodes are available: 1 node(s) didn't have free ports for the requested pod ports
```

**Cause**: The SPIRE agent DaemonSet uses `hostPort`, and in single-node Minikube clusters, port conflicts can prevent scheduling.

**Solution 1: Restart Minikube (cleanest approach)**

Delete and recreate Minikube cluster:
```bash
minikube delete
minikube start --cpus=4 --memory=8192 --driver=docker
```

Then reinstall SPIRE from Step 2 in the tutorial.

**Solution 2: Disable hostPort (for development)**

This requires creating a custom values file.

Create values file:
```bash
cat > spire-values.yaml <<EOF
spire-agent:
  hostPorts:
    enabled: false
EOF
```

Uninstall and reinstall with custom values:
```bash
helm uninstall spire -n spire
helm install spire spire \
  --repo https://spiffe.github.io/helm-charts-hardened/ \
  --namespace spire \
  --set global.spire.trustDomain=example.org \
  --set global.spire.clusterName=minikube-cluster \
  -f spire-values.yaml
```

**Solution 3: Continue anyway (agent not critical for local development)**

For the tutorial, you can continue even if the agent is pending. You'll use port-forwarding to access the SPIRE socket in Step 3 of Part B.

The server is running, which is what issues the certificates. The agent is just a local gateway to the server.

---

## Issue: "no matches for kind ClusterSPIFFEID" during SPIRE installation

**Error message:**
```
Error: INSTALLATION FAILED: unable to build kubernetes objects from release manifest:
resource mapping not found for name: "spire-spire-server-default" namespace: "" from "":
no matches for kind "ClusterSPIFFEID" in version "spire.spiffe.io/v1alpha1"
ensure CRDs are installed first
```

**Cause**: The SPIRE Helm chart requires Custom Resource Definitions (CRDs) to be installed before the chart itself.

**Solution**: Install CRDs first using the `spire-crds` Helm chart, then install SPIRE.

Install SPIRE CRDs:
```bash
helm install spire-crds spire-crds \
  --repo https://spiffe.github.io/helm-charts-hardened/ \
  --namespace spire \
  --create-namespace
```

Then install SPIRE (both server and agent):
```bash
helm install spire spire \
  --repo https://spiffe.github.io/helm-charts-hardened/ \
  --namespace spire \
  --set global.spire.trustDomain=example.org \
  --set global.spire.clusterName=minikube-cluster
```

**If you already have a failed installation**, clean up first:

Remove failed installation:
```bash
helm uninstall spire -n spire
helm uninstall spire-crds -n spire
```

Install CRDs:
```bash
helm install spire-crds spire-crds \
  --repo https://spiffe.github.io/helm-charts-hardened/ \
  --namespace spire \
  --create-namespace
```

Reinstall SPIRE:
```bash
helm install spire spire \
  --repo https://spiffe.github.io/helm-charts-hardened/ \
  --namespace spire \
  --set global.spire.trustDomain=example.org \
  --set global.spire.clusterName=minikube-cluster
```

---

## Issue: "failed to create X509Source: workload endpoint socket address is not configured"

**Cause**: The SPIRE Agent socket path in `e5s.yaml` doesn't match the actual socket location.

**Solution**: Ensure the socket path is correct.

Check where SPIRE Agent socket actually is:
```bash
kubectl exec -n spire \
  $(kubectl get pod -n spire -l app.kubernetes.io/name=agent -o jsonpath='{.items[0].metadata.name}') \
  -- ls -la /tmp/spire-agent/public/
```

Update e5s.yaml with correct path.

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

Check SPIRE Agent pod status:
```bash
kubectl get pods -n spire -l app.kubernetes.io/name=agent
```

Should show Running status:
```
NAME                READY   STATUS    RESTARTS   AGE
spire-agent-xxxxx   1/1     Running   0          5m
```

**Solution 2: Verify workload registration**

List all registration entries:
```bash
SERVER_POD=$(kubectl get pod -n spire -l app.kubernetes.io/name=server -o jsonpath='{.items[0].metadata.name}')
kubectl exec -n spire $SERVER_POD -c spire-server -- \
  /opt/spire/bin/spire-server entry show
```

You should see entries for your server and client workloads.

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

Check SPIRE Server trust domain:
```bash
kubectl exec -n spire $SERVER_POD -c spire-server -- \
  /opt/spire/bin/spire-server show
```

Look for: Trust domain: example.org

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

Check client's actual SPIFFE ID:
```bash
kubectl exec -n spire $SERVER_POD -c spire-server -- \
  /opt/spire/bin/spire-server entry show | grep -A 5 "client"
```

Update server config to match.

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

Check if server process is running:
```bash
ps aux | grep server
```

Check if server is listening on the port:
```bash
netstat -tuln | grep 8443
```

or:
```bash
ss -tuln | grep 8443
```

**Solution 2: Check firewall rules**

On Linux, check iptables:
```bash
sudo iptables -L -n | grep 8443
```

On macOS, check if firewall is blocking:
```bash
sudo pfctl -s rules | grep 8443
```

**Solution 3: Verify DNS resolution (if using hostnames)**

Test DNS resolution:
```bash
nslookup server.example.org
```

Test connectivity:
```bash
ping server.example.org
```

---

## Issue: "no identity issued" when running locally

**Cause**: The workload is running outside Kubernetes but trying to get identity from SPIRE in Kubernetes.

**Solution**: Use port forwarding to expose SPIRE Agent socket (see Tutorial Step 3):

Port forward SPIRE Agent:
```bash
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

**Wrong (creates new client per request):**
```go
// DON'T DO THIS
for i := 0; i < 1000; i++ {
    client, cleanup, _ := e5s.Client("e5s.yaml")  // Creates new source each time!
    resp, _ := client.Get(url)
    cleanup()
    // ...
}
```

**Right (reuse client):**
```go
// DO THIS
client, cleanup, err := e5s.Client("e5s.yaml")
if err != nil {
    log.Fatal(err)
}
defer func() {
    if err := cleanup(); err != nil {
        log.Printf("Cleanup error: %v", err)
    }
}()

for i := 0; i < 1000; i++ {
    resp, err := client.Get(url)  // Reuses same source
    // ...
}
```

---

## Getting More Help

If you're still stuck:

1. **Enable debug logging**:

   Set the `E5S_DEBUG` environment variable to see detailed configuration:
   ```bash
   export E5S_DEBUG=1
   # or: export E5S_DEBUG=true
   # or: export E5S_DEBUG=debug

   # Then run your application
   kubectl set env deployment/your-app E5S_DEBUG=1
   kubectl rollout restart deployment/your-app
   ```

   Debug output includes:
   - Configuration file path
   - Listen addresses
   - SPIFFE ID validation settings
   - Trust domain configuration

2. **Check SPIRE logs**:

   SPIRE Server logs:
   ```bash
   kubectl logs -n spire -l app.kubernetes.io/name=server -c spire-server
   ```

   SPIRE Agent logs:
   ```bash
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
- **Fix**: Always call the `shutdown()` function returned by `Client()` or `Start()`

---

## Prevention Tips

1. **Start simple**: Use the high-level API (`e5s.Start()`, `e5s.Client()`) for most use cases
2. **Test locally first**: Use the tutorial setup before moving to complex environments
3. **Verify SPIRE first**: Ensure SPIRE is working before debugging e5s
4. **Use trust domains initially**: Switch to specific SPIFFE IDs only after everything works
5. **Keep configs consistent**: Use the same trust domain everywhere
6. **Check logs early**: SPIRE logs are your friend when things go wrong
