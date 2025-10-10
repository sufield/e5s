# Troubleshooting Guide

This guide covers common issues when running mTLS server and client examples with SPIRE.

## Table of Contents

- [Connection Issues](#connection-issues)
- [SPIRE Agent Issues](#spire-agent-issues)
- [Workload Registration Issues](#workload-registration-issues)
- [TLS Handshake Issues](#tls-handshake-issues)
- [Kubernetes-Specific Issues](#kubernetes-specific-issues)
- [Debugging Tips](#debugging-tips)

---

## Connection Issues

### "Failed to create X509Source: context deadline exceeded"

**Problem**: Cannot connect to SPIRE agent socket.

**Root Causes**:
- SPIRE agent not running
- Incorrect socket path
- File permissions preventing access
- Socket file doesn't exist

**Solution**:

1. **Verify SPIRE agent is running**:
```bash
# Check process
ps aux | grep spire-agent

# In Kubernetes
kubectl get pods -n spire-system -l app=spire-agent
```

2. **Check socket exists and has correct permissions**:
```bash
# Check socket file
ls -la /tmp/spire-agent/public/api.sock

# Expected output:
# srwxrwxrwx 1 root root 0 Oct 10 12:00 /tmp/spire-agent/public/api.sock
```

3. **Verify socket path in environment**:
```bash
# Check your configuration
echo $SPIRE_AGENT_SOCKET

# Should match: unix:///tmp/spire-agent/public/api.sock
```

4. **Check file permissions**:
```bash
# Ensure current user can access socket directory
ls -la /tmp/spire-agent/public/

# If permission denied, check socket is world-accessible
sudo chmod 777 /tmp/spire-agent/public/api.sock
```

**In Kubernetes**:
```bash
# Verify socket mounted in pod
kubectl exec -it <pod-name> -- ls -la /spire-agent-socket/

# Check hostPath exists on node
minikube ssh "ls -la /run/spire/sockets/"
```

---

### Connection Refused

**Problem**: Server not reachable.

**Symptoms**:
- `dial tcp: connect: connection refused`
- `no route to host`

**Solution**:

1. **Verify server is running**:
```bash
# Check process
ps aux | grep mtls-server

# In Kubernetes
kubectl get pods -l app=mtls-server
```

2. **Check server is listening on expected port**:
```bash
# Local
netstat -tlnp | grep 8443
# or
ss -tlnp | grep 8443

# Expected output:
# tcp  0  0  0.0.0.0:8443  0.0.0.0:*  LISTEN  12345/mtls-server
```

3. **Verify network connectivity**:
```bash
# Test TCP connectivity
nc -zv localhost 8443

# In Kubernetes - check service
kubectl get svc mtls-server
kubectl describe svc mtls-server
```

4. **Check firewall rules**:
```bash
# Linux
sudo iptables -L -n | grep 8443

# If blocked, allow port
sudo iptables -A INPUT -p tcp --dport 8443 -j ACCEPT
```

**In Kubernetes**:
```bash
# Test service DNS resolution
kubectl run -it --rm debug --image=alpine --restart=Never -- sh
# Inside pod:
apk add curl
curl -k https://mtls-server:8443/health
```

---

## SPIRE Agent Issues

### "rpc error: code = Unknown desc = workload is not registered"

**Problem**: Workload not registered in SPIRE server.

**Solution**: See [Workload Registration Issues](#workload-registration-issues) below.

---

### SPIRE Agent Socket Permission Denied

**Problem**: Socket exists but process cannot access it.

**Solution**:

1. **Check socket permissions**:
```bash
ls -la /tmp/spire-agent/public/api.sock
```

2. **Ensure socket directory is accessible**:
```bash
# Check entire path
ls -la /tmp/
ls -la /tmp/spire-agent/
ls -la /tmp/spire-agent/public/
```

3. **Fix permissions if needed**:
```bash
# Make socket accessible (temporary fix)
sudo chmod 777 /tmp/spire-agent/public/api.sock

# Better: Add user to spire group (permanent fix)
sudo usermod -a -G spire $(whoami)
# Then logout and login
```

---

## Workload Registration Issues

### "No identity issued" or "no such registration entry"

**Problem**: Workload not registered in SPIRE server.

**Symptoms**:
- `no identity issued`
- `no such registration entry`
- `rpc error: code = Unknown desc = workload is not registered`

**Solution**:

1. **Register the workload**:
```bash
# For local development (using unix:uid selector)
spire-server entry create \
  -spiffeID spiffe://example.org/mtls-server \
  -parentID spiffe://example.org/agent \
  -selector unix:uid:$(id -u)

spire-server entry create \
  -spiffeID spiffe://example.org/mtls-client \
  -parentID spiffe://example.org/agent \
  -selector unix:uid:$(id -u)
```

2. **For Kubernetes** (using k8s selectors):
```bash
kubectl exec -n spire-system spire-server-0 -- \
  spire-server entry create \
    -spiffeID spiffe://example.org/mtls-server \
    -parentID spiffe://example.org/spire-agent \
    -selector k8s:ns:default \
    -selector k8s:pod-label:app:mtls-server

kubectl exec -n spire-system spire-server-0 -- \
  spire-server entry create \
    -spiffeID spiffe://example.org/mtls-client \
    -parentID spiffe://example.org/spire-agent \
    -selector k8s:ns:default \
    -selector k8s:pod-label:app:mtls-client
```

3. **Verify registration**:
```bash
# Local
spire-server entry show

# Kubernetes
kubectl exec -n spire-system spire-server-0 -- \
  spire-server entry show
```

4. **Check selectors match your workload**:
```bash
# For local development, check UID matches
id -u

# For Kubernetes, check pod labels
kubectl get pod -l app=mtls-server --show-labels
```

---

### Registration Entry Exists But Identity Not Issued

**Problem**: Entry is registered but workload still can't get identity.

**Possible Causes**:
- Selectors don't match workload attributes
- SPIRE agent not attested
- Trust domain mismatch

**Solution**:

1. **Verify selectors match workload**:
```bash
# Check registration entry
spire-server entry show -spiffeID spiffe://example.org/mtls-server

# Compare with actual workload attributes
id  # Check UID matches unix:uid selector
```

2. **Check SPIRE agent is attested**:
```bash
spire-server agent list

# Expected output should show your agent
# If empty, agent needs to attest to server
```

3. **Verify trust domain matches**:
```bash
# Check SPIRE server trust domain
spire-server bundle show

# Should show: trust_domain: example.org
# Must match SPIFFE IDs in registration entries
```

4. **Check SPIRE agent logs**:
```bash
# Look for attestation errors
journalctl -u spire-agent -f

# In Kubernetes
kubectl logs -n spire-system -l app=spire-agent -f
```

---

## TLS Handshake Issues

### "TLS handshake failed" or "remote error: tls: bad certificate"

**Problem**: mTLS authentication failed during handshake.

**Possible Causes**:
1. Client and server not in same trust domain
2. Server's `ALLOWED_CLIENT_ID` doesn't match client's SPIFFE ID
3. Client's `EXPECTED_SERVER_ID` doesn't match server's SPIFFE ID
4. SVID expired or not yet issued
5. Certificate validation failed

**Solution**:

1. **Verify both can fetch SVIDs**:
```bash
# Test client can fetch SVID
SPIRE_AGENT_SOCKET=unix:///tmp/spire-agent/public/api.sock \
  spire-agent api fetch x509

# Should show client's SPIFFE ID and certificate
```

2. **Check trust domains match**:
```bash
# Both client and server must have same trust domain
spire-server bundle show

# Look for:
# trust_domain: example.org
```

3. **Verify authorizer configuration**:

**Server side**:
```bash
# If ALLOWED_CLIENT_ID is set, it must match client's SPIFFE ID
echo $ALLOWED_CLIENT_ID
# Should be empty OR match: spiffe://example.org/mtls-client
```

**Client side**:
```bash
# If EXPECTED_SERVER_ID is set, it must match server's SPIFFE ID
echo $EXPECTED_SERVER_ID
# Should be empty OR match: spiffe://example.org/mtls-server
```

4. **Check for SVID expiration**:
```bash
# Fetch and inspect SVID
spire-agent api fetch x509 -write /tmp/

# Check expiration
openssl x509 -in /tmp/svid.0.pem -noout -dates

# SVIDs typically expire in 1 hour (default TTL)
```

5. **Enable debug logging**:
```bash
# Run server with verbose logging
LOG_LEVEL=debug ./bin/mtls-server

# Run client with verbose logging
LOG_LEVEL=debug ./bin/mtls-client
```

---

### "certificate signed by unknown authority"

**Problem**: Certificate chain validation failed.

**Solution**:

1. **Verify trust bundle is accessible**:
```bash
# Check SPIRE agent can fetch bundle
spire-agent api fetch x509

# Should show both SVID and bundle
```

2. **Check server and client use same SPIRE agent**:
```bash
# Both should point to same socket
echo $SPIRE_AGENT_SOCKET

# In Kubernetes, verify both mount same socket
kubectl describe pod <server-pod>
kubectl describe pod <client-pod>
```

3. **Verify trust domain consistency**:
```bash
# Server and client must be in same trust domain
spire-server bundle show
```

---

## Kubernetes-Specific Issues

### Pod Can't Access SPIRE Agent Socket

**Problem**: Pod cannot reach SPIRE agent socket via hostPath volume.

**Solution**:

1. **Verify socket exists on node**:
```bash
# SSH to node
minikube ssh "ls -la /run/spire/sockets/"

# Expected output:
# srwxrwxrwx 1 root root 0 Oct 10 12:00 api.sock
```

2. **Check volume mount in pod**:
```bash
kubectl describe pod <pod-name>

# Look for:
# Volumes:
#   spire-agent-socket:
#     Type:          HostPath (bare host directory volume)
#     Path:          /run/spire/sockets
```

3. **Verify mount point inside pod**:
```bash
kubectl exec -it <pod-name> -- ls -la /spire-agent-socket/

# Should show api.sock
```

4. **Check SPIRE agent DaemonSet**:
```bash
# SPIRE agent must be running on same node as workload
kubectl get pods -n spire-system -l app=spire-agent -o wide
kubectl get pods -l app=mtls-server -o wide

# Node names should match
```

---

### "ImagePullBackOff" or "ErrImageNeverPull"

**Problem**: Kubernetes can't pull Docker image.

**Solution**:

1. **For Minikube, use Minikube's Docker daemon**:
```bash
# Point to Minikube's Docker
eval $(minikube docker-env)

# Rebuild images
docker build -t mtls-server:latest -f examples/mtls-adapters/server/Dockerfile .
docker build -t mtls-client:latest -f examples/mtls-adapters/client/Dockerfile .

# Verify images exist
docker images | grep mtls
```

2. **Check imagePullPolicy**:
```yaml
# In deployment YAML
spec:
  containers:
  - name: server
    image: mtls-server:latest
    imagePullPolicy: IfNotPresent  # or Never for local images
```

3. **For remote registry**:
```bash
# Tag and push images
docker tag mtls-server:latest your-registry.io/mtls-server:v1.0
docker push your-registry.io/mtls-server:v1.0

# Update deployment manifest to use registry image
```

---

### Pod CrashLoopBackOff

**Problem**: Pod keeps restarting.

**Solution**:

1. **Check pod logs**:
```bash
kubectl logs <pod-name>
kubectl logs <pod-name> --previous  # Check previous instance
```

2. **Check pod events**:
```bash
kubectl describe pod <pod-name>

# Look in Events section for error details
```

3. **Common causes**:
- SPIRE socket not accessible → Check volume mounts
- Registration entry missing → Register workload
- Configuration error → Check environment variables

---

## Debugging Tips

### Enable Verbose Logging

**Server**:
```bash
# Set log level environment variable
LOG_LEVEL=debug ./bin/mtls-server

# Or in Kubernetes
kubectl set env deployment/mtls-server LOG_LEVEL=debug
```

**Client**:
```bash
LOG_LEVEL=debug ./bin/mtls-client
```

---

### Test SPIRE Connectivity Independently

```bash
# Fetch X.509 SVID directly from agent
SPIRE_AGENT_SOCKET=unix:///tmp/spire-agent/public/api.sock \
  spire-agent api fetch x509

# Expected output:
# Received 1 svid after 123ms
# SPIFFE ID:  spiffe://example.org/mtls-server
# SVID Valid After:  2025-10-10 10:00:00 +0000 UTC
# SVID Valid Until:  2025-10-10 11:00:00 +0000 UTC
```

---

### Inspect TLS Certificates

```bash
# Fetch SVID to file
spire-agent api fetch x509 -write /tmp/

# Inspect certificate
openssl x509 -in /tmp/svid.0.pem -text -noout

# Check:
# - Subject: CN should be SPIFFE ID
# - Issuer: Should be trust domain CA
# - Validity: Should not be expired
# - SAN: Should include SPIFFE ID URI
```

---

### Check Network Connectivity

```bash
# Test port is open
nc -zv localhost 8443

# Test TLS handshake (without client cert - will fail but shows server is listening)
openssl s_client -connect localhost:8443

# In Kubernetes
kubectl run -it --rm debug --image=nicolaka/netshoot --restart=Never -- bash
# Inside pod:
curl -k https://mtls-server:8443/health
```

---

### Capture Network Traffic

```bash
# Capture TLS handshake
sudo tcpdump -i any -w /tmp/mtls.pcap port 8443

# Run client in another terminal
./bin/mtls-client

# Stop tcpdump (Ctrl+C)
# Analyze with Wireshark
wireshark /tmp/mtls.pcap
```

---

### Check SPIRE Server Logs

```bash
# Local
journalctl -u spire-server -f

# Kubernetes
kubectl logs -n spire-system -l app=spire-server -f

# Look for:
# - Registration entry creation
# - Agent attestation
# - SVID issuance
# - Error messages
```

---

### Validate Configuration

**Check environment variables**:
```bash
# Server
env | grep -E '(SPIRE|SERVER|ALLOWED)'

# Client
env | grep -E '(SPIRE|SERVER|EXPECTED)'
```

**Check loaded configuration**:
```bash
# Most applications print config on startup
./bin/mtls-server

# Look for:
# Starting mTLS server with configuration:
#   Socket: unix:///tmp/spire-agent/public/api.sock
#   Address: :8443
```

---

## Common Error Messages

### Summary of Error Messages and Solutions

| Error Message | Likely Cause | Solution |
|--------------|--------------|----------|
| `context deadline exceeded` | Can't connect to SPIRE agent | Check socket path and permissions |
| `no such registration entry` | Workload not registered | Create registration entry |
| `connection refused` | Server not running | Start server, check port |
| `tls: bad certificate` | Identity verification failed | Check SPIFFE IDs match authorizer config |
| `certificate signed by unknown authority` | Trust bundle mismatch | Verify same trust domain |
| `permission denied` | Socket permissions | Fix file permissions |
| `ImagePullBackOff` | Can't pull image | Use Minikube Docker env or fix registry |

---

## Getting Help

If you're still experiencing issues:

1. **Check SPIRE logs** for detailed error messages
2. **Enable debug logging** on both client and server
3. **Verify SPIRE setup** independently of your application
4. **Review configuration** for typos or mismatches
5. **Consult main README** for setup steps

## Additional Resources

- [Main README](README.md) - Setup and usage guide
- [Kubernetes Deployment Guide](KUBERNETES.md) - Kubernetes-specific setup
- [SPIRE Troubleshooting](https://spiffe.io/docs/latest/spire/troubleshooting/) - Official SPIRE docs
- [go-spiffe Examples](https://github.com/spiffe/go-spiffe/tree/main/v2/examples) - SDK examples
