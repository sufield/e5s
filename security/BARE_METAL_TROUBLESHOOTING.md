# Falco Bare-Metal Troubleshooting Guide

> **⚠️ RECOMMENDED**: Use [Helm-based deployment](../examples/minikube-lowlevel/infra/README.md#optional-falco-runtime-security) for Kubernetes environments. This guide is for bare-metal/development workstation installations only.

## Quick Fix for Common Issues

If Falco service is failing to start, run the automated setup:

```bash
sudo bash security/setup-falco.sh
```

This handles:
- Installing Falco with modern eBPF driver
- Applying corrected SPIRE mTLS rules
- Configuring JSON output and logging
- Starting and verifying the service

## Common Issues and Solutions

### Issue 1: Service Keeps Restarting

**Symptom**: `systemctl status falco` shows `activating (auto-restart)`

**Cause**: Configuration or rules syntax error

**Solution**:
```bash
# Check for configuration errors
sudo journalctl -u falco -n 50 --no-pager

# Validate configuration
sudo falco --validate /etc/falco/falco.yaml
sudo falco --validate /etc/falco/falco_rules.local.yaml
```

### Issue 2: Rules Not Loading

**Symptom**: No custom SPIRE rules in logs

**Solution**:
```bash
# Verify custom rules file exists
ls -lh /etc/falco/falco_rules.local.yaml

# Check rule count (should be 18 or 5 depending on version)
grep -c "^- rule:" /etc/falco/falco_rules.local.yaml

# Reinstall rules
sudo cp security/falco_rules.yaml /etc/falco/falco_rules.local.yaml
sudo systemctl restart falco
```

### Issue 3: Invalid Field Name Errors

**Symptom**: Error like `invalid formatting token proc.cap_permitted` or `unknown field fd.dip`

**Cause**: Incorrect Falco field names in custom rules

**Common Mistakes**:

#### Network Field Names
- ❌ **Wrong**: `fd.dip`, `fd.dport` (destination IP/port) - these don't exist
- ✅ **Correct**: Use context-specific fields:
  - For `connect` (outbound): `fd.sip`/`fd.sport` = destination, `fd.cip`/`fd.cport` = source
  - For `accept` (inbound): `fd.cip`/`fd.cport` = client, `fd.sip`/`fd.sport` = server

**Example Fix**:
```yaml
# Wrong:
output: (dest=%fd.dip:%fd.dport src=%fd.sip:%fd.sport)

# Correct (for outbound connect):
output: (dest=%fd.sip:%fd.sport src=%fd.cip:%fd.cport)
```

#### Capability Field Format
- ❌ **Wrong**: `proc.cap_permitted` or `CAP_SYS_ADMIN` (uppercase)
- ✅ **Correct**: `thread.cap_permitted contains "cap_sys_admin"` (lowercase string)

**Example Fix**:
```yaml
# Wrong:
condition: proc.cap_permitted contains CAP_SYS_ADMIN
output: (caps=%proc.cap_permitted)

# Correct:
condition: thread.cap_permitted contains "cap_sys_admin"
output: (caps=%thread.cap_permitted)
```

### Issue 4: JSON Output Not Working

**Symptom**: Plain text logs instead of JSON

**Solution**:
```bash
# Enable JSON output
sudo sed -i 's/json_output: false/json_output: true/' /etc/falco/falco.yaml
sudo systemctl restart falco
```

### Issue 5: File Output Not Working

**Symptom**: No `/var/log/falco.log` file created

**Solution**:
```bash
# Check file_output configuration
grep -A 5 "file_output:" /etc/falco/falco.yaml

# Should show:
# file_output:
#   enabled: true
#   filename: /var/log/falco.log

# If missing, add it manually or re-run setup script
sudo bash security/setup-falco.sh
```

### Issue 6: Driver Not Loading

**Symptom**: Error like `Unable to load the driver` or `Driver not found`

**Solution**:
```bash
# Check kernel version (should be 6.8+ for modern_ebpf)
uname -r

# Install kernel headers if missing
sudo apt install -y linux-headers-$(uname -r)

# Try loading driver manually
sudo falco-driver-loader

# If modern_ebpf fails, try regular ebpf or kmod
sudo falco -o engine.kind=ebpf  # Try eBPF
sudo falco -o engine.kind=kmod  # Try kernel module
```

## Advanced Troubleshooting

### Validate All Configuration

```bash
# Main config
sudo falco --validate /etc/falco/falco.yaml

# Default rules
sudo falco --validate /etc/falco/falco_rules.yaml

# Custom SPIRE rules
sudo falco --validate /etc/falco/falco_rules.local.yaml
```

**Note**: `--validate` only checks YAML syntax, not field names. Always test with actual run.

### Test Falco Manually

```bash
# Run in foreground to see all output
sudo falco -o engine.kind=modern_ebpf

# Press Ctrl+C to stop

# If this works, the issue is with the service configuration
```

### View Detailed Startup Logs

```bash
# Last boot
sudo journalctl -u falco -b --no-pager

# Last 200 lines
sudo journalctl -u falco -n 200

# Follow live
sudo journalctl -u falco -f
```

### Check Driver Status

```bash
# Check service name (varies by driver)
systemctl status falco-modern-bpf.service  # Modern eBPF
systemctl status falco-bpf.service         # Regular eBPF
systemctl status falco-kmod.service        # Kernel module

# Check which is active
systemctl list-units | grep falco
```

## Field Reference

Common Falco fields for SPIRE mTLS rules:

**Process Fields**:
- `proc.name` - Process name
- `proc.cmdline` - Full command line
- `proc.pid` - Process ID

**File Descriptor Fields**:
- `fd.name` - File/socket path
- `fd.type` - Type (file, ipv4, unix, etc.)
- `fd.sip` / `fd.sport` - Server IP/port (context-dependent)
- `fd.cip` / `fd.cport` - Client IP/port (context-dependent)

**Container Fields**:
- `container.name` - Container name
- `container.image` - Image name
- `container.privileged` - Boolean

**User/Thread Fields**:
- `user.name` - Username
- `thread.cap_permitted` - Capabilities (string)

**Event Fields**:
- `evt.type` - Syscall (open, connect, accept, etc.)
- `evt.dir` - Direction (`<` = enter, `>` = exit)

**Reference**: https://falco.org/docs/reference/rules/supported-fields/

## Testing the Installation

### Verify Service Status

```bash
# Check service is running
systemctl status falco

# Should show: active (running)
```

### Verify Rules Loaded

```bash
# Count custom rules
grep -c "^- rule:" /etc/falco/falco_rules.local.yaml

# View rule names
grep "^- rule:" /etc/falco/falco_rules.local.yaml
```

### Test Rule Triggering

```bash
# Test SPIRE socket access rule (if SPIRE is running)
cat /tmp/spire-agent/public/api.sock 2>/dev/null

# View alert in logs
sudo journalctl -u falco -n 20 | grep "SPIRE"
```

### View JSON Output

```bash
# Tail JSON log file
tail -f /var/log/falco.log | jq .

# Filter by priority
tail -f /var/log/falco.log | jq 'select(.priority == "Critical")'
```

## Reset to Defaults

If all else fails, reset and reinstall:

```bash
# Stop service
sudo systemctl stop falco

# Restore original config
sudo cp /etc/falco/falco.yaml.bak /etc/falco/falco.yaml

# Reinstall custom rules
sudo cp security/falco_rules.yaml /etc/falco/falco_rules.local.yaml

# Re-run setup
sudo bash security/setup-falco.sh
```

## Collecting Diagnostic Information

Create a diagnostic bundle for troubleshooting:

```bash
mkdir -p /tmp/falco-diag

# Collect logs
sudo journalctl -u falco -n 200 > /tmp/falco-diag/service.log

# Collect version info
falco --version > /tmp/falco-diag/version.txt
uname -a > /tmp/falco-diag/kernel.txt

# Collect config snippets
grep -A 10 "file_output:" /etc/falco/falco.yaml > /tmp/falco-diag/config-file-output.txt
grep -A 5 "json_output:" /etc/falco/falco.yaml > /tmp/falco-diag/config-json-output.txt

# Collect rule count
grep -c "^- rule:" /etc/falco/falco_rules.local.yaml > /tmp/falco-diag/rule-count.txt

# Collect service status
systemctl status falco > /tmp/falco-diag/status.txt 2>&1

# Create archive
tar -czf /tmp/falco-diag.tar.gz -C /tmp falco-diag
ls -lh /tmp/falco-diag.tar.gz
```

## Configuration File Locations

- **Main config**: `/etc/falco/falco.yaml`
- **Default rules**: `/etc/falco/falco_rules.yaml`
- **Custom SPIRE rules**: `/etc/falco/falco_rules.local.yaml`
- **Backup**: `/etc/falco/falco.yaml.bak`
- **Log output**: `/var/log/falco.log`

## Quick Reference Commands

**Service Management**:
```bash
sudo systemctl start falco
sudo systemctl stop falco
sudo systemctl restart falco
sudo systemctl status falco
```

**View Logs**:
```bash
# Live
sudo journalctl -u falco -f

# Last 50 lines
sudo journalctl -u falco -n 50

# Since boot
sudo journalctl -u falco -b

# JSON output
tail -f /var/log/falco.log | jq .
```

**Validation**:
```bash
# Validate config
sudo falco --validate /etc/falco/falco.yaml

# Validate rules
sudo falco --validate /etc/falco/falco_rules.local.yaml

# Test run (5 seconds)
sudo timeout 5 falco -o engine.kind=modern_ebpf
```

## Key Learnings

1. **Field Names Matter**: Always check [Falco Supported Fields](https://falco.org/docs/reference/rules/supported-fields/) documentation
2. **Types Matter**: String fields need quotes (`"cap_sys_admin"`), numeric fields don't
3. **Context Matters**: Network fields (`fd.sip`, `fd.cip`, etc.) mean different things in `connect` vs `accept` syscalls
4. **Validation ≠ Compilation**: `--validate` only checks YAML syntax. Always test with actual `falco` run to catch field errors
5. **Test Incrementally**: Add one rule at a time if troubleshooting complex issues

## Getting Help

**Resources**:
- **Main Guide**: [security/FALCO_GUIDE.md](FALCO_GUIDE.md) - Comprehensive Falco guide
- **Kubernetes Deployment**: [examples/minikube-lowlevel/infra/README.md](../examples/minikube-lowlevel/infra/README.md#optional-falco-runtime-security)
- **Falco Docs**: https://falco.org/docs/troubleshooting/
- **GitHub Issues**: https://github.com/falcosecurity/falco/issues
- **Slack**: https://slack.falco.org

---

**Last Updated**: 2025-11-03
**Status**: Consolidated from SOLUTION.md and TROUBLESHOOTING.md
**Recommended**: Use Helm deployment for Kubernetes (see FALCO_GUIDE.md)
