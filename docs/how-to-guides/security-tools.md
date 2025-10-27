---
type: how-to
audience: intermediate
---

# Security Tools Overview

This document explains the security tools used in the SPIRE mTLS project and when each tool is used in the development and deployment pipeline.

## Categories

### Static Analysis (Pre-Deployment)
Tools that scan code and dependencies **before** deployment to find security vulnerabilities.

### Runtime Monitoring (Post-Deployment)
Tools that monitor **running systems** in production/staging to detect attacks and anomalous behavior.

---

## Falco - Runtime Security Monitoring

**Category:** Runtime Monitoring

**What it does:**
- Monitors **running systems** in production/staging environments
- Detects **anomalous behavior** at runtime using eBPF
- Intercepts system calls at the kernel level
- Generates alerts for suspicious activity

**What it monitors:**
- Process execution (e.g., unexpected shell spawning in containers)
- File access (e.g., reading `/etc/shadow`, modifying certificates)
- Network connections (e.g., outbound connections from servers)
- Container behavior (e.g., privilege escalation, escape attempts)
- System calls (e.g., `connect`, `open`, `execve`, `ptrace`)

**When it runs:**
After deployment, continuously monitoring live systems 24/7

**Example alerts:**
```
"Shell spawned in mTLS server container" ← Someone executed bash in your container
"Unauthorized access to SPIRE socket" ← Wrong process accessing SPIRE API
"Outbound connection from mTLS server" ← Server making unexpected network connections
"Certificate file modified" ← Unauthorized changes to .crt/.pem files
```

**Installation:**
```bash
sudo bash security/setup-falco.sh
```

**Viewing alerts:**
```bash
# Live monitoring
sudo journalctl -u falco-modern-bpf.service -f

# JSON logs
tail -f /var/log/falco.log | jq .
```

---

## gosec - Go Security Checker

**Category:** Static Analysis

**What it does:**
- Scans **Go source code** for security vulnerabilities
- Identifies common security anti-patterns
- Checks for insecure coding practices

**What it finds:**
- SQL injection vulnerabilities
- Path traversal issues
- Weak cryptography usage
- Unsafe file permissions
- Command injection risks
- Use of unsafe functions

**When it runs:**
During development, before code is committed or deployed

**Example findings:**
```
G104: Unchecked error (could hide security issues)
G304: File path provided as taint input (path traversal risk)
G401: Use of weak cryptographic primitive (MD5, DES)
```

**Installation:**
```bash
go install github.com/securego/gosec/v2/cmd/gosec@latest
```

**Usage:**
```bash
gosec ./...
```

---

## govulncheck - Go Vulnerability Checker

**Category:** Static Analysis

**What it does:**
- Scans **Go dependencies** for known security vulnerabilities
- Checks against the Go vulnerability database
- Identifies vulnerable packages and versions

**What it finds:**
- Known CVEs in dependencies
- Vulnerable library versions
- Security advisories for Go packages

**When it runs:**
Before deployment, ideally weekly or with every dependency update

**Example findings:**
```
Vulnerability in golang.org/x/crypto
- Fixed in: v0.1.0
- Your version: v0.0.0-20191011191535-87dc89f01550
- CVE: CVE-2020-9283
```

**Installation:**
```bash
go install golang.org/x/vuln/cmd/govulncheck@latest
```

**Usage:**
```bash
govulncheck ./...
```

---

## golangci-lint - Comprehensive Linter

**Category:** Static Analysis (includes security checks)

**What it does:**
- Runs 22+ linters including security-focused ones
- Checks code quality, style, and security
- Integrates multiple tools in one

**What it finds:**
- Security issues (via gosec integration)
- Code quality problems
- Performance issues
- Style violations
- Potential bugs

**When it runs:**
During development, in CI/CD pipeline

**Installation:**
```bash
# Via script (recommended)
curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $(go env GOPATH)/bin

# Or via go install
go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
```

**Usage:**
```bash
golangci-lint run --timeout=5m
```

---

## Trivy - Container Image Scanner

**Category:** Static Analysis

**What it does:**
- Scans **container images** for vulnerabilities
- Checks OS packages and application dependencies
- Identifies misconfigurations

**What it finds:**
- CVEs in base images (e.g., Ubuntu, Alpine)
- Vulnerable packages in containers
- Secrets in container layers
- Dockerfile misconfigurations

**When it runs:**
After building container images, before pushing to registry

**Installation:**
See: https://aquasecurity.github.io/trivy/latest/getting-started/installation/

**Usage:**
```bash
docker build -t mtls-server:scan -f examples/zeroconfig-example/Dockerfile .
trivy image mtls-server:scan
```

---

## The Security Pipeline

```
┌─────────────────────────────────────────────────────────────────────┐
│                        Development Time                             │
├─────────────────────────────────────────────────────────────────────┤
│                                                                     │
│  Write Code                                                         │
│      ↓                                                              │
│  gosec              ← Scan Go code for security issues             │
│      ↓                                                              │
│  govulncheck        ← Check dependencies for CVEs                  │
│      ↓                                                              │
│  golangci-lint      ← Comprehensive linting + security             │
│                                                                     │
└─────────────────────────────────────────────────────────────────────┘
                              ↓
┌─────────────────────────────────────────────────────────────────────┐
│                          Build Time                                 │
├─────────────────────────────────────────────────────────────────────┤
│                                                                     │
│  Build Container                                                    │
│      ↓                                                              │
│  Trivy              ← Scan container image for vulnerabilities     │
│      ↓                                                              │
│  Push to Registry                                                   │
│                                                                     │
└─────────────────────────────────────────────────────────────────────┘
                              ↓
┌─────────────────────────────────────────────────────────────────────┐
│                          Runtime                                    │
├─────────────────────────────────────────────────────────────────────┤
│                                                                     │
│  Deploy to Kubernetes                                               │
│      ↓                                                              │
│  Falco              ← Monitor live system behavior                 │
│      ↓                                                              │
│  Alert on:                                                          │
│    • Unexpected processes                                           │
│    • Unauthorized file access                                       │
│    • Suspicious network connections                                 │
│    • Container escape attempts                                      │
│    • Privilege escalation                                           │
│                                                                     │
└─────────────────────────────────────────────────────────────────────┘
```

---

## Reference

| Tool | Type | Scans | Finds | When | Command |
|------|------|-------|-------|------|---------|
| **gosec** | Static | Go code | Security bugs in source | Development | `gosec ./...` |
| **govulncheck** | Static | Dependencies | Vulnerable libraries | Development | `govulncheck ./...` |
| **golangci-lint** | Static | Go code | Quality + security | Development | `golangci-lint run` |
| **Trivy** | Static | Container images | CVEs in images | Build | `trivy image <name>` |
| **Falco** | Runtime | System calls | Attacks, anomalies | Production | `journalctl -u falco -f` |

---

## Usage

### Pre-Deployment Checks

Run all static analysis tools:

```bash
make security
```

This runs:
- `gosec ./...` - Go code security scan
- `govulncheck ./...` - Dependency vulnerability scan
- `golangci-lint run` - Comprehensive linting

### Container Scanning (Optional)

```bash
docker build -t mtls-server:scan -f examples/zeroconfig-example/Dockerfile .
trivy image mtls-server:scan
```

### Runtime Monitoring

Install Falco:

```bash
sudo bash security/setup-falco.sh
```

Monitor alerts:

```bash
sudo journalctl -u falco-modern-bpf.service -f
```

---

## Distinctions

### Static Analysis vs Runtime Monitoring

**Static Analysis (gosec, govulncheck, golangci-lint, Trivy):**
- Analyzes **code and images** before running
- Finds **potential** vulnerabilities
- Runs **once** per build/commit
- No performance impact on production
- Can have false positives

**Runtime Monitoring (Falco):**
- Monitors **running systems** in real-time
- Detects **actual** attacks and breaches
- Runs **continuously** in production
- Small performance overhead (~1-3% CPU)
- Alerts on real suspicious behavior

### Analogy

Think of it like home security:

- **Static Analysis** = Building inspector checking the house before you move in
  - Checks for structural issues
  - Reviews electrical wiring
  - Finds potential problems

- **Runtime Monitoring** = Security camera and alarm system
  - Watches for intruders
  - Alerts when doors/windows open unexpectedly
  - Detects suspicious activity

You need both.

---

## Best Practices

### Development Workflow

1. **Write code** with security in mind
2. **Run gosec** before committing
3. **Check govulncheck** weekly
4. **Use golangci-lint** in IDE/pre-commit hooks
5. **Scan images** before pushing to registry

### Deployment Workflow

1. **Build container** with minimal base image (distroless)
2. **Scan with Trivy** for vulnerabilities
3. **Deploy** to Kubernetes with security context
4. **Monitor with Falco** for runtime threats

### Continuous Monitoring

- **Falco alerts**: Review daily, investigate CRITICAL immediately
- **Dependency updates**: Check `govulncheck` weekly
- **Security scan**: Run `make security` before every release
- **Quarterly audit**: Full security review including all tools

---

## Getting Help

### Documentation

- **Falco**: `security/FALCO_GUIDE.md`
- **gosec**: https://github.com/securego/gosec
- **govulncheck**: https://pkg.go.dev/golang.org/x/vuln/cmd/govulncheck
- **golangci-lint**: https://golangci-lint.run/
- **Trivy**: https://aquasecurity.github.io/trivy/

### Support

- Falco Slack: https://slack.falco.org
- SPIFFE Slack: https://slack.spiffe.io
- #golang-security on Gophers Slack

---

## Summary

- **Static analysis tools** (gosec, govulncheck, golangci-lint, Trivy) find security issues **in code and images** before deployment
- **Falco** monitors **running systems** in production to detect attacks and anomalous behavior at runtime
- **Both are essential** for comprehensive security
- This project uses **all of them** as part of defense-in-depth strategy

---

**Related Docs**:
- `security/README.md` - Security overview
- `security/FALCO_GUIDE.md` - Detailed Falco documentation
- `security/QUICK_START.md` - Quick installation guide
