# Falco Runtime Security Monitoring for SPIRE mTLS

## Overview

This guide explains how to use [Falco](https://falco.org) for runtime security monitoring of SPIRE-based mTLS applications. Falco detects anomalous behavior and security threats by monitoring system calls, container events, and application behavior in real-time.

**Why Falco for SPIRE mTLS?**
- **Runtime Protection**: Detects attacks that static analysis (gosec, Trivy) cannot catch
- **Container Security**: Monitors mTLS servers running in Kubernetes
- **SPIFFE/SPIRE Awareness**: Custom rules detect unauthorized Workload API access
- **Zero Trust Validation**: Verifies mTLS applications behave as expected
- **Compliance**: Provides audit trails for security certifications

## Prerequisites

- Minikube (v1.30+)
- kubectl (v1.27+)
- Helm (v3.12+) or Helmfile
- Kernel 6.8+ (for modern eBPF support)

## Quick Start

Deploy Falco alongside SPIRE in Minikube:

```bash
# Deploy SPIRE + Falco together
ENABLE_FALCO=true helmfile -e dev apply

# Or if using make
ENABLE_FALCO=true make minikube-up
```

This automatically:
- Deploys Falco 4.19.4 as a DaemonSet
- Loads 5 custom SPIRE mTLS security rules
- Uses modern eBPF driver (no kernel module)
- Configures JSON output and structured logging

## Deployment

### Integrated Deployment (Helmfile)

Deploy Falco automatically with SPIRE infrastructure:

```bash
cd examples/minikube-lowlevel/infra

# Deploy with Falco enabled
ENABLE_FALCO=true helmfile -e dev apply
```

This uses the pre-configured `values-falco.yaml` which includes:
- Modern eBPF driver (no kernel modules)
- 5 custom SPIRE mTLS security rules
- JSON output for structured logging
- Resource limits for Minikube (100m CPU, 256Mi memory)
- Automatic integration with SPIRE components

**View alerts:**
```bash
kubectl logs -n falco -l app.kubernetes.io/name=falco -f
```

**Disable Falco:**
```bash
# Default behavior (Falco not deployed)
helmfile -e dev apply
```

See `examples/minikube-lowlevel/infra/README.md` for complete documentation.

### Standalone Deployment (Manual Helm)

For deploying Falco separately or in production clusters:

```bash
# Add Falco Helm repository
helm repo add falcosecurity https://falcosecurity.github.io/charts
helm repo update

# Install Falco with our custom SPIRE rules
helm install falco falcosecurity/falco \
  --namespace falco --create-namespace \
  --values examples/minikube-lowlevel/infra/values-falco.yaml

# View alerts
kubectl logs -n falco -l app.kubernetes.io/name=falco -f
```

## Custom Rules for SPIRE mTLS

Our Falco rules (in `values-falco.yaml`) detect:

### 1. Unauthorized SPIRE Socket Access

```yaml
- Unauthorized Access to SPIRE Socket
  Trigger: Any process (except allowed) opens /tmp/spire-agent/public/api.sock
  Priority: CRITICAL
  Example: cat /tmp/spire-agent/public/api.sock
```

### 2. mTLS Server Port Monitoring

```yaml
- mTLS Server Wrong Port Binding
  Trigger: mtls-server binds to port other than 8443 or 8080
  Priority: WARNING
  Example: Modify SERVER_ADDRESS to :9000
```

### 3. Certificate File Tampering

```yaml
- Suspicious Certificate File Modification
  Trigger: Unauthorized write/delete of .crt, .pem, .key files
  Priority: CRITICAL
  Example: rm /etc/ssl/certs/ca-certificates.crt
```

### 4. Shell Spawning in Containers

```yaml
- Shell Spawned in mTLS Container
  Trigger: bash/sh executed inside security-sensitive containers
  Priority: WARNING
  Example: kubectl exec -it mtls-server-xxx -- bash
```

### 5. SPIRE Directory Permission Changes

```yaml
- SPIRE Directory Permission Modification
  Trigger: Permission changes on /tmp/spire-agent or /var/run/spire
  Priority: CRITICAL
```

## Testing the Rules

### Test 1: Unauthorized SPIRE Socket Access

```bash
# SSH into Minikube
minikube ssh

# Trigger alert (as non-SPIRE process)
cat /tmp/spire-agent/public/api.sock

# Expected Falco alert:
# Priority: CRITICAL
# Rule: Unauthorized Access to SPIRE Socket
```

### Test 2: Shell in mTLS Container

```bash
# Get pod name
POD=$(kubectl get pod -l app=mtls-server -o jsonpath='{.items[0].metadata.name}')

# Spawn shell
kubectl exec -it "$POD" -- bash

# Expected Falco alert:
# Priority: WARNING
# Rule: Shell Spawned in mTLS Container
```

### Test 3: Wrong Port Binding

Modify your mTLS server deployment to use port 9000 instead of 8443:

```bash
# Expected Falco alert:
# Priority: WARNING
# Rule: mTLS Server Wrong Port Binding
```

## Viewing Alerts

### Real-time Monitoring

```bash
# Live alerts from Falco pods
kubectl logs -n falco -l app.kubernetes.io/name=falco -f

# Filter by priority
kubectl logs -n falco -l app.kubernetes.io/name=falco | grep CRITICAL

# Export to file
kubectl logs -n falco -l app.kubernetes.io/name=falco > falco-alerts.log
```

### Structured JSON Output

Falco outputs JSON for easy parsing:

```bash
# Get latest alerts as JSON
kubectl logs -n falco -l app.kubernetes.io/name=falco --tail=20 | jq .

# Filter critical alerts
kubectl logs -n falco -l app.kubernetes.io/name=falco | jq 'select(.priority == "Critical")'
```

## Alert Integrations

### Slack Notifications

Install Falcosidekick for alert routing:

```bash
helm install falcosidekick falcosecurity/falcosidekick \
  --namespace falco \
  --set config.slack.webhookurl=https://hooks.slack.com/services/YOUR/WEBHOOK/URL \
  --set config.slack.minimumpriority=warning
```

Update Falco to forward alerts:

```bash
helm upgrade falco falcosecurity/falco \
  --namespace falco \
  --reuse-values \
  --set falco.httpOutput.enabled=true \
  --set falco.httpOutput.url="http://falcosidekick:2801"
```

### Elasticsearch/Kibana

Use Falcosidekick to forward to Elasticsearch:

```bash
helm install falcosidekick falcosecurity/falcosidekick \
  --namespace falco \
  --set config.elasticsearch.hostport="http://elasticsearch:9200" \
  --set config.elasticsearch.index="falco" \
  --set config.elasticsearch.minimumpriority="warning"
```

## Tuning and Optimization

### Reduce False Positives

Edit `values-falco.yaml` to allow specific processes:

```yaml
customRules:
  rules-spire.yaml: |-
    - list: allowed_spire_processes
      items: [spire-agent, spire-server, mtls-server, my-custom-client]
```

### Performance Tuning

Falco has minimal overhead (~5-10% CPU). To reduce further:

1. **Adjust resource limits** in `values-falco.yaml`:
   ```yaml
   resources:
     requests:
       cpu: 50m
       memory: 128Mi
     limits:
       cpu: 200m
       memory: 256Mi
   ```

2. **Increase priority threshold** (only show WARNING and above):
   ```yaml
   falco:
     priority: warning
   ```

3. **Disable verbose output**:
   ```yaml
   falco:
     log_level: info  # Change from debug
   ```

### Rule Priority Levels

- **CRITICAL**: Unauthorized access, tampering, container escape
- **WARNING**: Unexpected behavior, potential security issue
- **NOTICE**: Normal but significant events
- **INFO**: Informational messages, monitoring

## Troubleshooting

### Falco Pods Not Starting

```bash
# Check pod status
kubectl get pods -n falco

# View pod events
kubectl describe pod -n falco -l app.kubernetes.io/name=falco

# Check logs
kubectl logs -n falco -l app.kubernetes.io/name=falco
```

Common issues:
- **Driver not supported**: Ensure kernel 6.8+ for modern_ebpf
- **Insufficient permissions**: Falco needs privileged access
- **Resource limits**: Increase CPU/memory if pods are OOMKilled

### No Alerts Appearing

```bash
# Verify Falco is running
kubectl get pods -n falco

# Check if rules are loaded
kubectl logs -n falco -l app.kubernetes.io/name=falco | grep "Loading rules"

# Test with simple trigger
minikube ssh
cat /tmp/spire-agent/public/api.sock  # Should trigger alert
```

### High CPU Usage

```bash
# Check current resource usage
kubectl top pods -n falco

# Adjust buffer size in values-falco.yaml
falco:
  syscall_buf_size_preset: 4  # Reduce from default 8

# Redeploy
ENABLE_FALCO=true helmfile -e dev apply
```

## Integration with CI/CD

### GitHub Actions Example

```yaml
falco-runtime-check:
  runs-on: ubuntu-24.04
  steps:
    - uses: actions/checkout@v4

    - name: Start Minikube with Falco
      run: |
        ENABLE_FALCO=true make minikube-up

    - name: Deploy mTLS Server
      run: kubectl apply -f examples/mtls-server-image.yaml

    - name: Monitor for security alerts
      run: |
        kubectl logs -n falco -l app.kubernetes.io/name=falco > falco.log &
        sleep 60

    - name: Check for critical alerts
      run: |
        if kubectl logs -n falco -l app.kubernetes.io/name=falco | grep -q "CRITICAL"; then
          echo "Critical Falco alerts detected!"
          kubectl logs -n falco -l app.kubernetes.io/name=falco
          exit 1
        fi
```

## Rule Development

### Anatomy of a Falco Rule

```yaml
- rule: Rule Name
  desc: Human-readable description
  condition: >
    evt.type = open and           # Event type
    fd.name contains "/secret" and  # File descriptor name
    proc.name = mtls-server and   # Process name
    not user.name = root          # Negation
  output: >
    Alert message with variables
    (proc=%proc.name file=%fd.name user=%user.name)
  priority: CRITICAL
  tags: [spire, security, custom]
```

### Useful Falco Fields

**Process fields:**
- `proc.name` - Process name
- `proc.cmdline` - Full command line
- `proc.pid` - Process ID

**File descriptor fields:**
- `fd.name` - File/socket name
- `fd.type` - Type (file, ipv4, ipv6, unix)
- `fd.sip` / `fd.sport` - Source IP/port

**Container fields:**
- `container.name` - Container name
- `container.image` - Image name
- `container.privileged` - Is privileged

**Event fields:**
- `evt.type` - Syscall type (open, write, connect, etc.)
- `evt.dir` - Direction (< for enter, > for exit)

**Field Reference**: https://falco.org/docs/reference/rules/supported-fields/

### Testing Custom Rules

1. **Edit values-falco.yaml** to add your custom rule:
   ```yaml
   customRules:
     rules-spire.yaml: |-
       - rule: My Custom Rule
         desc: Description
         condition: evt.type = open
         output: Custom alert
         priority: WARNING
   ```

2. **Redeploy Falco**:
   ```bash
   ENABLE_FALCO=true helmfile -e dev apply
   ```

3. **Trigger the rule and verify**:
   ```bash
   # Trigger the rule
   # ...

   # Check alerts
   kubectl logs -n falco -l app.kubernetes.io/name=falco --tail=20
   ```

## Best Practices

1. **Start with default rules**, then add custom rules incrementally
2. **Monitor for 1 week** before enabling strict enforcement
3. **Tune false positives** by analyzing alert patterns
4. **Integrate with SIEM** (Splunk, ELK) for centralized monitoring
5. **Set up alerting** for CRITICAL and WARNING priorities
6. **Document exceptions** when allowing specific behaviors
7. **Review rules quarterly** as application behavior changes
8. **Test in staging** before deploying to production
9. **Monitor Falco itself** (CPU, memory, alert volume)
10. **Keep Falco updated** for latest vulnerability detections

## Comparison with Other Tools

| Tool | Type | Scope | Falco Advantage |
|------|------|-------|-----------------|
| **gosec** | Static analysis | Go code security | Falco catches runtime attacks |
| **Trivy** | Vulnerability scanner | Dependencies, OS packages | Falco detects exploitation attempts |
| **SELinux/AppArmor** | Mandatory Access Control | File/network access | Falco provides detailed alerts |
| **Seccomp** | Syscall filtering | Syscall whitelist | Falco monitors all syscalls |
| **Audit.d** | Kernel auditing | System events | Falco has container awareness |

**Recommended**: Use Falco **in addition to** static analysis tools, not as a replacement.

## Resources

- **Official Docs**: https://falco.org/docs
- **Rule Reference**: https://falco.org/docs/rules
- **Field Reference**: https://falco.org/docs/reference/rules/supported-fields
- **Community Rules**: https://github.com/falcosecurity/rules
- **Falcosidekick**: https://github.com/falcosecurity/falcosidekick
- **Helm Charts**: https://github.com/falcosecurity/charts

## Support

For issues with Falco integration:
1. Check Falco pod logs: `kubectl logs -n falco -l app.kubernetes.io/name=falco`
2. Review values-falco.yaml configuration
3. Test with simple rules first
4. Open issue in project repository with Falco version and alert logs

## License

Falco is open-source (Apache 2.0). Custom rules in this repository are licensed under the same license as the main project.
