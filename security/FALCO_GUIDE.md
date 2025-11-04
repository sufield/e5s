# Falco Runtime Security Monitoring for SPIRE mTLS

## Overview

This guide explains how to use [Falco](https://falco.org) for runtime security monitoring of SPIRE-based mTLS applications. Falco detects anomalous behavior and security threats by monitoring system calls, container events, and application behavior in real-time.

**Why Falco for SPIRE mTLS?**
- **Runtime Protection**: Detects attacks that static analysis (gosec, Trivy) cannot catch
- **Container Security**: Monitors mTLS servers running in Kubernetes/Docker
- **SPIFFE/SPIRE Awareness**: Custom rules detect unauthorized Workload API access
- **Zero Trust Validation**: Verifies mTLS applications behave as expected
- **Compliance**: Provides audit trails for security certifications

## Deployment Options

**For Kubernetes (Recommended):**
- Use Helm chart deployment with automatic integration
- See [Kubernetes Deployment](#kubernetes-deployment-recommended) section below
- Pre-configured with SPIRE-specific rules in `examples/minikube-lowlevel/infra/values-falco.yaml`

**For Bare Metal / Development:**
- Use manual installation script
- See [Manual Installation](#manual-installation-bare-metal) section below
- Requires systemd and sudo access

## Prerequisites

- **OS**: Ubuntu 24.04 (Noble Numbat) with kernel 6.8+
- **Permissions**: Root/sudo access
- **Tools**: Docker, kubectl (for container monitoring)
- **Resources**: Kernel headers (`linux-headers-$(uname -r)`)

## Quick Start

Choose your deployment method:

### Option A: Kubernetes with Helm (Recommended)

Deploy Falco alongside SPIRE in Minikube:

```bash
# Deploy SPIRE + Falco together
ENABLE_FALCO=true helmfile -e dev apply

# Or if using make
ENABLE_FALCO=true make minikube-up
```

This automatically:
- Deploys Falco 4.19.4 as a DaemonSet
- Loads custom SPIRE mTLS security rules
- Uses modern eBPF driver (no kernel module)
- Configures JSON output and logging

See [Kubernetes Deployment](#kubernetes-deployment-recommended) for details.

### Option B: Manual Installation (Bare Metal)

For non-Kubernetes environments:

```bash
sudo bash security/setup-falco.sh
```

This will:
- Install Falco 0.38+ with eBPF driver
- Apply custom SPIRE mTLS rules from `falco_rules.yaml`
- Configure JSON output and logging
- Start the Falco systemd service

See [Manual Installation](#manual-installation-bare-metal) for details.

### 2. Deploy SPIRE Infrastructure

```bash
# Start Minikube with SPIRE
make minikube-up

# Verify SPIRE agent socket exists
minikube ssh "ls -la /tmp/spire-agent/public/api.sock"
```

### 3. Deploy mTLS Server

```bash
# Deploy server to Kubernetes
kubectl apply -f examples/mtls-server-image.yaml

# Wait for pod to start
kubectl wait --for=condition=Ready pod -l app=mtls-server --timeout=60s
```

### 4. Monitor Falco Alerts

**Real-time monitoring:**
```bash
sudo journalctl -u falco -f
```

**Check logs:**
```bash
tail -f /var/log/falco.log | jq .
```

**Filter critical alerts:**
```bash
sudo journalctl -u falco | grep -i "critical\|warning"
```

## Custom Rules for SPIRE mTLS

Our Falco rules (in `security/falco_rules.yaml`) detect:

### 1. SPIRE Workload API Security

```yaml
- Unauthorized Access to SPIRE Socket
  Trigger: Any process (except allowed) opens /tmp/spire-agent/public/api.sock
  Priority: CRITICAL
  Example: cat /tmp/spire-agent/public/api.sock
```

### 2. mTLS Server Anomalies

```yaml
- Unexpected Shell Spawned in mTLS Container
  Trigger: bash/sh executed inside mtls-server container
  Priority: WARNING
  Example: kubectl exec -it mtls-server-xxx -- bash
```

```yaml
- Unexpected Network Port Binding
  Trigger: mtls-server binds to port other than 8443
  Priority: WARNING
  Example: Modify SERVER_ADDRESS to :9000
```

### 3. Certificate Tampering

```yaml
- Certificate File Modification
  Trigger: Unauthorized write/delete of .crt, .pem, .key files
  Priority: CRITICAL
  Example: rm /etc/ssl/certs/ca-certificates.crt
```

### 4. Container Escape Attempts

```yaml
- Container Escape Attempt via Proc
  Trigger: Container process accesses /proc/*/root/* or /proc/sys/kernel/core_pattern
  Priority: CRITICAL
```

### 5. Network Security

```yaml
- Outbound Connection from mTLS Server
  Trigger: mtls-server initiates outbound connections (should be inbound-only)
  Priority: WARNING
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
# Output: Unauthorized process accessing SPIRE socket (proc=cat ...)
```

### Test 2: Shell in mTLS Container

```bash
# Get pod name
POD=$(kubectl get pod -l app=mtls-server -o jsonpath='{.items[0].metadata.name}')

# Spawn shell
kubectl exec -it "$POD" -- bash

# Expected Falco alert:
# Priority: WARNING
# Rule: Unexpected Shell Spawned in mTLS Container
# Output: Shell spawned in mTLS server container (container=mtls-server proc=bash)
```

### Test 3: Sensitive File Access

```bash
# Inside mtls-server container
kubectl exec -it "$POD" -- cat /etc/shadow

# Expected Falco alert:
# Priority: CRITICAL
# Rule: Sensitive File Read by mTLS Process
# Output: mTLS process reading sensitive file (proc=cat file=/etc/shadow)
```

### Test 4: Wrong Port Binding

Modify `examples/mtls-server.yaml`:
```yaml
env:
  - name: SERVER_ADDRESS
    value: ":9000"  # Change from :8443
```

Deploy and check alerts:
```bash
kubectl apply -f examples/mtls-server.yaml

# Expected Falco alert:
# Priority: WARNING
# Rule: Unexpected Network Port Binding
# Output: mTLS server binding to unexpected port (port=9000 expected=8443)
```

## Integration with CI/CD

### GitHub Actions Example

Add to `.github/workflows/security.yml`:

```yaml
falco-runtime-check:
  runs-on: ubuntu-24.04
  steps:
    - uses: actions/checkout@v4

    - name: Install Falco
      run: sudo bash security/install-falco.sh

    - name: Start Minikube
      run: make minikube-up

    - name: Deploy mTLS Server
      run: kubectl apply -f examples/mtls-server-image.yaml

    - name: Monitor for 60 seconds
      run: |
        sudo journalctl -u falco -f > falco.log &
        sleep 60
        pkill journalctl

    - name: Check for critical alerts
      run: |
        if grep -q "CRITICAL" falco.log; then
          echo "Critical Falco alerts detected!"
          cat falco.log
          exit 1
        fi
```

### Kubernetes Deployment (Recommended)

#### Option 1: Integrated Deployment (Helmfile)

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

#### Option 2: Standalone Deployment (Manual Helm)

For deploying Falco separately or in production clusters:

```bash
# Add Falco Helm repository
helm repo add falcosecurity https://falcosecurity.github.io/charts
helm repo update

# Install Falco with custom SPIRE rules
helm install falco falcosecurity/falco \
  --namespace falco --create-namespace \
  --set driver.kind=modern_ebpf \
  --set tty=true \
  --set falco.jsonOutput=true \
  --set-file customRules.rules-spire.yaml=security/falco_rules.yaml

# View alerts
kubectl logs -n falco -l app.kubernetes.io/name=falco -f
```

**Using our values file:**
```bash
helm install falco falcosecurity/falco \
  --namespace falco --create-namespace \
  --values examples/minikube-lowlevel/infra/values-falco.yaml
```

### Manual Installation (Bare Metal)

For non-Kubernetes environments (development workstations, bare-metal servers):

**Automated setup:**
```bash
sudo bash security/setup-falco.sh
```

**Manual steps:**

1. **Install Falco:**
   ```bash
   curl -s https://falco.org/repo/falcosecurity-packages.asc | \
     sudo gpg --dearmor -o /usr/share/keyrings/falco-archive-keyring.gpg
   echo "deb [signed-by=/usr/share/keyrings/falco-archive-keyring.gpg] https://download.falco.org/packages/deb stable main" | \
     sudo tee /etc/apt/sources.list.d/falcosecurity.list
   sudo apt update
   sudo apt install -y falco
   ```

2. **Install custom rules:**
   ```bash
   sudo cp security/falco_rules.yaml /etc/falco/falco_rules.local.yaml
   ```

3. **Configure Falco:**
   ```bash
   sudo nano /etc/falco/falco.yaml
   # Set: json_output: true
   # Set: file_output.enabled: true
   ```

4. **Start service:**
   ```bash
   sudo systemctl enable falco
   sudo systemctl start falco
   ```

## Alert Integrations

### 1. Slack Notifications

Install Falcosidekick:
```bash
helm install falcosidekick falcosecurity/falcosidekick \
  --namespace falco \
  --set config.slack.webhookurl=https://hooks.slack.com/services/YOUR/WEBHOOK/URL \
  --set config.slack.minimumpriority=warning
```

Update `/etc/falco/falco.yaml`:
```yaml
http_output:
  enabled: true
  url: "http://falcosidekick:2801"
```

### 2. JSON File Output

Already configured in `install-falco.sh`:
```yaml
file_output:
  enabled: true
  filename: /var/log/falco.log
```

Parse with `jq`:
```bash
tail -f /var/log/falco.log | jq 'select(.priority == "Critical")'
```

### 3. Syslog

Configure in `/etc/falco/falco.yaml`:
```yaml
syslog_output:
  enabled: true
```

### 4. Elasticsearch/Kibana

Use Falcosidekick to forward to Elasticsearch:
```yaml
config:
  elasticsearch:
    hostport: "http://elasticsearch:9200"
    index: "falco"
    type: "_doc"
    minimumpriority: "notice"
```

## Tuning and Optimization

### Reduce False Positives

Edit `/etc/falco/falco_rules.local.yaml`:

```yaml
# Allow specific processes to access SPIRE socket
- list: allowed_spire_processes
  items: [mtls-server, test-client, zeroconfig-example, my-custom-client]
```

### Performance Tuning

Falco has ~5-10% CPU overhead. To reduce:

1. **Decrease syscall buffer size** (in `/etc/falco/falco.yaml`):
   ```yaml
   syscall_buf_size_preset: 4  # Default is 8
   ```

2. **Disable unused rules**:
   ```yaml
   rules_file:
     - /etc/falco/falco_rules.yaml
     # - /etc/falco/k8s_audit_rules.yaml  # Disable if not using K8s audit
   ```

3. **Use eBPF instead of kernel module** (already default on Ubuntu 24.04):
   ```bash
   # Verify eBPF is being used
   falco-driver-loader --help
   ```

### Rule Priority Levels

- **EMERGENCY**: System unusable (not used in our rules)
- **ALERT**: Immediate action required (not used in our rules)
- **CRITICAL**: Unauthorized access, tampering, container escape
- **WARNING**: Unexpected behavior, potential security issue
- **NOTICE**: Normal but significant events
- **INFO**: Informational messages, monitoring

## Troubleshooting

### Falco Not Starting

```bash
# Check service status
sudo systemctl status falco

# View detailed logs
sudo journalctl -u falco -n 50 --no-pager

# Common issues:
# 1. Driver not loaded
sudo falco-driver-loader

# 2. Check kernel headers
dpkg -l | grep linux-headers-$(uname -r)
```

### No Alerts Appearing

```bash
# Verify Falco is monitoring
ps aux | grep falco

# Check if rules are loaded
sudo cat /etc/falco/falco_rules.local.yaml

# Test with simple rule trigger
echo "test" > /tmp/test.crt  # Should trigger certificate file rule
```

### High CPU Usage

```bash
# Check buffer size
grep syscall_buf_size_preset /etc/falco/falco.yaml

# Reduce to 4 or 2 if needed
sudo sed -i 's/syscall_buf_size_preset: 8/syscall_buf_size_preset: 4/' /etc/falco/falco.yaml
sudo systemctl restart falco
```

### Container Events Not Detected

```bash
# Ensure Falco can access Docker socket
ls -la /var/run/docker.sock

# Add falco user to docker group
sudo usermod -aG docker falco
sudo systemctl restart falco
```

## Rule Development Guide

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
  tags: [category, subcategory]
```

### Useful Falco Fields

**Process fields:**
- `proc.name` - Process name
- `proc.cmdline` - Full command line
- `proc.pid` - Process ID
- `proc.ppid` - Parent process ID
- `proc.pname` - Parent process name

**File descriptor fields:**
- `fd.name` - File/socket name
- `fd.type` - Type (file, ipv4, ipv6, unix)
- `fd.sip` / `fd.dip` - Source/destination IP
- `fd.sport` / `fd.dport` - Source/destination port

**Container fields:**
- `container.name` - Container name
- `container.image.repository` - Image name
- `container.privileged` - Is privileged container

**User fields:**
- `user.name` - Username
- `user.uid` - User ID

**Event fields:**
- `evt.type` - Syscall type (open, write, connect, etc.)
- `evt.dir` - Direction (< for enter, > for exit)
- `evt.arg.xxx` - Syscall arguments

### Testing Custom Rules

1. **Add rule to local file:**
   ```bash
   sudo nano /etc/falco/falco_rules.local.yaml
   ```

2. **Validate syntax:**
   ```bash
   sudo falco --validate /etc/falco/falco_rules.local.yaml
   ```

3. **Restart Falco:**
   ```bash
   sudo systemctl restart falco
   ```

4. **Trigger the rule:**
   ```bash
   # Example: Trigger file write rule
   echo "test" > /tmp/test-trigger
   ```

5. **Check alerts:**
   ```bash
   sudo journalctl -u falco -n 20
   ```

## Comparison with Other Tools

| Tool | Type | Scope | Falco Advantage |
|------|------|-------|-----------------|
| **gosec** | Static analysis | Go code security | Falco catches runtime attacks |
| **Trivy** | Vulnerability scanner | Dependencies, OS packages | Falco detects exploitation attempts |
| **SELinux/AppArmor** | Mandatory Access Control | File/network access | Falco provides detailed alerts |
| **Seccomp** | Syscall filtering | Syscall whitelist | Falco monitors all syscalls |
| **Audit.d** | Kernel auditing | System events | Falco has container awareness |

**Recommended**: Use Falco **in addition to** static analysis tools, not as a replacement.

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

## Resources

- **Official Docs**: https://falco.org/docs
- **Rule Reference**: https://falco.org/docs/rules
- **Field Reference**: https://falco.org/docs/reference/rules/supported-fields
- **Community Rules**: https://github.com/falcosecurity/rules
- **Falcosidekick**: https://github.com/falcosecurity/falcosidekick
- **Helm Charts**: https://github.com/falcosecurity/charts

## Support

For issues with Falco integration:
1. Check this guide's troubleshooting section
2. Review Falco logs: `sudo journalctl -u falco -n 100`
3. Test with simple rules first
4. Open issue in project repository with Falco version and alert logs

## License

Falco is open-source (Apache 2.0). Custom rules in this repository are licensed under the same license as the main project.
