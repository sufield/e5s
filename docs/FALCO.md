## Library Security Implementation

The mTLS security enforcement is implemented in **[`pkg/spiffehttp`](../pkg/spiffehttp/)** with SPIFFE identity verification, TLS 1.3 enforcement, and peer authentication. See the [library quickstart](../docs/QUICKSTART_LIBRARY.md) for usage examples.

## Contents

### Falco Runtime Security Monitoring

- **[setup-falco.sh](setup-falco.sh)** - Main installation script - Run this to install everything
- **[FALCO_GUIDE.md](FALCO_GUIDE.md)** - Comprehensive guide for Falco integration
- **[falco_rules.yaml](falco_rules.yaml)** - 18 custom Falco rules for SPIRE mTLS monitoring
- **[test-falco.sh](test-falco.sh)** - Test Falco rules by triggering sample alerts
- **[SOLUTION.md](SOLUTION.md)** - Technical details of implementation

### Security Layers

This project implements defense-in-depth with multiple security layers:

```
┌─────────────────────────────────────────────────────┐
│  Layer 1: Static Analysis (Build Time)             │
│  - gosec: Go code security scanning                 │
│  - golangci-lint: 22+ security linters              │
│  - Trivy: Container vulnerability scanning          │
│  - govulncheck: Go dependency vulnerabilities       │
├─────────────────────────────────────────────────────┤
│  Layer 2: Kubernetes Security (Deploy Time)         │
│  - Pod Security Context (runAsNonRoot, etc.)        │
│  - Network Policies (mTLS-only traffic)             │
│  - RBAC (minimal permissions)                       │
│  - Seccomp Profile (RuntimeDefault)                 │
├─────────────────────────────────────────────────────┤
│  Layer 3: SPIFFE/SPIRE (Identity Layer)             │
│  - X.509 SVIDs (automatic rotation)                 │
│  - Trust Domain isolation                           │
│  - Workload API socket protection                   │
│  - mTLS peer authentication                         │
├─────────────────────────────────────────────────────┤
│  Layer 4: Application Security (Runtime)            │
│  - TLS 1.3 only                                     │
│  - Strong cipher suites                             │
│  - Certificate validation                           │
│  - Identity-based authorization                     │
├─────────────────────────────────────────────────────┤
│  Layer 5: Runtime Monitoring (Falco)                │
│  - Syscall monitoring (eBPF)                        │
│  - Container behavior analysis                      │
│  - Anomaly detection                                │
│  - Real-time threat alerts                          │
└─────────────────────────────────────────────────────┘
```

## Quick Start

### 1. Install All Security Tools

```bash
# Install static analysis tools
go install github.com/securego/gosec/v2/cmd/gosec@latest
go install golang.org/x/vuln/cmd/govulncheck@latest

# Install Falco (requires sudo)
sudo bash security/setup-falco.sh

# Install Trivy (optional, for container scanning)
# See: https://aquasecurity.github.io/trivy/latest/getting-started/installation/
```

### 2. Run Security Scans

```bash
# Run all security checks
make security

# Or run individually:
gosec ./...                     # Go code security
govulncheck ./...               # Dependency vulnerabilities
golangci-lint run              # 22+ linters
sudo journalctl -u falco -f    # Runtime monitoring (if Falco installed)
```

### 3. Deploy with Security Hardening

```bash
# Deploy SPIRE infrastructure
make minikube-up

# Deploy mTLS server with security context
kubectl apply -f examples/mtls-server-image.yaml

# Verify pod security
kubectl get pod -l app=mtls-server -o yaml | grep -A 10 securityContext
```

## Falco Integration

### What is Falco?

Falco is a cloud-native runtime security tool that detects threats by monitoring:
- **Syscalls**: open, write, connect, exec, etc.
- **Container events**: Shell spawning, file modifications, network connections
- **Kubernetes activities**: ConfigMap changes, service account access

### Why Use Falco for SPIRE mTLS?

1. **Detect SPIRE Socket Tampering**
   - Unauthorized access to `/tmp/spire-agent/public/api.sock`
   - Permission modifications on SPIRE directories

2. **Monitor mTLS Servers**
   - Unexpected shell spawning in containers
   - Wrong port bindings (should be 8443 only)
   - Outbound connections (servers should be inbound-only)

3. **Certificate Security**
   - Detect unauthorized .crt/.pem/.key file modifications
   - Monitor TLS handshake patterns

4. **Container Escape Detection**
   - /proc manipulation attempts
   - Privileged container warnings
   - CAP_SYS_ADMIN usage

### Installation

**One-command setup (recommended):**
```bash
sudo bash security/setup-falco.sh
```

This automated script will:
- Install Falco (if not already installed)
- Configure JSON output and file logging
- Install 18 custom SPIRE mTLS security rules
- Test driver compatibility and start service

### Viewing Alerts

**Real-time monitoring:**
```bash
sudo journalctl -u falco -f
```

**Filter by priority:**
```bash
sudo journalctl -u falco | grep -i "critical\|warning"
```

**JSON output:**
```bash
tail -f /var/log/falco.log | jq 'select(.priority == "Critical")'
```

## Security Scanning Results

### Current Status (2025-11-03)

| Tool | Status | Issues | Notes |
|------|--------|--------|-------|
| **gosec** | ✅ PASS | 0 | All G104/G304 issues fixed |
| **govulncheck** | ✅ PASS | 0 | No known vulnerabilities |
| **golangci-lint** | ⚠️ PASS | 89 style | No security issues, only code quality |
| **Trivy** | ℹ️ N/A | - | Run on container images |
| **Falco** | ℹ️ Runtime | - | Monitors deployed apps |

**Last security audit**: 2025-11-03
**Next audit due**: 2026-02-03 (quarterly)

### Running Security Scans

**gosec (Go code security):**
```bash
gosec -fmt=text ./...
```

**govulncheck (dependency vulnerabilities):**
```bash
govulncheck ./...
```

**golangci-lint (comprehensive linting):**
```bash
golangci-lint run --timeout=5m
```

**Trivy (container scanning):**
```bash
docker build -t mtls-server:scan -f examples/zeroconfig-example/Dockerfile .
trivy image mtls-server:scan
```

## Security Best Practices

### Development

1. **Never commit secrets**
   - No API keys, passwords, or private keys in code
   - Use `.gitignore` for sensitive files
   - Review `git diff` before committing

2. **Keep dependencies updated**
   - Run `go get -u ./...` quarterly
   - Check `govulncheck ./...` weekly
   - Subscribe to security advisories

3. **Use security linters**
   - Run `gosec` before every commit
   - Configure pre-commit hooks (see `.pre-commit-config.yaml`)
   - Address all CRITICAL and WARNING findings

### Deployment

1. **Use minimal container images**
   - ✅ Distroless (current)
   - ❌ Ubuntu/Debian (unnecessary attack surface)

2. **Apply pod security context**
   ```yaml
   securityContext:
     runAsNonRoot: true
     runAsUser: 65532
     fsGroup: 65534
     allowPrivilegeEscalation: false
     capabilities:
       drop: [ALL]
     seccompProfile:
       type: RuntimeDefault
   ```

3. **Enable Falco monitoring**
   - Install Falco on all production clusters
   - Configure alerts to Slack/PagerDuty
   - Review alerts daily

4. **Rotate SPIRE credentials**
   - Default SVID TTL: 1 hour (automatic)
   - Restart SPIRE agent monthly
   - Monitor SPIRE server health

### Incident Response

**If Falco alerts fire:**

1. **Check alert priority**
   - CRITICAL: Investigate immediately
   - WARNING: Review within 1 hour
   - NOTICE/INFO: Review daily

2. **Investigate the alert**
   ```bash
   # Get full alert details
   sudo journalctl -u falco -n 100 | grep "rule_name"

   # Check process details
   kubectl exec -it <pod> -- ps aux

   # Review container logs
   kubectl logs <pod> --tail=100
   ```

3. **Remediate**
   - If legitimate: Add exception to Falco rules
   - If attack: Kill pod, review audit logs, update security policies
   - Document in incident log

4. **Improve detection**
   - Update Falco rules if needed
   - Add monitoring for similar patterns
   - Conduct post-mortem

## Compliance and Auditing

### Audit Trails

**Falco logs:**
```bash
# All events
tail -f /var/log/falco.log

# Export for compliance
sudo journalctl -u falco --since "2025-10-01" --until "2025-10-31" > falco-october-2025.log
```

**Kubernetes audit logs:**
```bash
# Enable in kube-apiserver
--audit-log-path=/var/log/kubernetes/audit.log
--audit-policy-file=/etc/kubernetes/audit-policy.yaml
```

### Security Certifications

This project follows security standards for:
- **SOC 2 Type II**: Runtime monitoring with Falco
- **ISO 27001**: Multi-layer defense-in-depth
- **PCI DSS**: mTLS for all communications
- **NIST 800-53**: Identity-based access control (SPIFFE)

### Quarterly Security Review Checklist

- [ ] Run all security scans (gosec, govulncheck, golangci-lint)
- [ ] Update dependencies (`go get -u ./...`)
- [ ] Review Falco rules for false positives
- [ ] Check for new CVEs in container base images
- [ ] Audit SPIRE server configuration
- [ ] Review access logs for anomalies
- [ ] Update security documentation
- [ ] Conduct penetration testing (if prod)

## Contributing Security Improvements

### Reporting Security Vulnerabilities

**DO NOT open public GitHub issues for security vulnerabilities.**

Instead, report via GitHub Security Advisories: https://github.com/sufield/e5s/security/advisories/new

### Adding Falco Rules

1. **Test the rule locally:**
   ```bash
   # Edit rules
   sudo nano /etc/falco/falco_rules.local.yaml

   # Validate
   sudo falco --validate /etc/falco/falco_rules.local.yaml

   # Restart
   sudo systemctl restart falco

   # Trigger and verify
   <test command>
   sudo journalctl -u falco -n 20
   ```

2. **Document the rule:**
   - Add to `security/falco_rules.yaml`
   - Include trigger example in comments
   - Explain expected vs. anomalous behavior

3. **Submit PR:**
   - Include rule file changes
   - Add test case in PR description
   - Tag with `security` label

## Resources

### Internal Documentation
- [FALCO_GUIDE.md](FALCO_GUIDE.md) - Comprehensive Falco guide
- [../README.md](../README.md) - Main project documentation
- [../examples/README.md](../examples/README.md) - Deployment examples

### External Resources
- **Falco**: https://falco.org/docs
- **SPIFFE/SPIRE**: https://spiffe.io/docs
- **gosec**: https://github.com/securego/gosec
- **OWASP**: https://owasp.org/www-project-go-secure-coding-practices-guide/

### Security Communities
- Falco Slack: https://slack.falco.org
- SPIFFE Slack: https://slack.spiffe.io
- #golang-security on Gophers Slack

---

**Last Updated**: 2025-11-03
**Contact**: https://github.com/sufield/e5s/security
