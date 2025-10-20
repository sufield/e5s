# Falco Troubleshooting Guide

## Current Issue: Falco Rules Syntax Error (FIXED)

### Symptoms
- Service status: `activating (auto-restart)`
- Exit code: 1/FAILURE
- Error: `LOAD_ERR_COMPILE_OUTPUT: invalid formatting token proc.cap_permitted)`

### Root Cause
Invalid field name in custom rules. The rule used `proc.cap_permitted` but Falco requires `thread.cap_permitted` for capability fields.

**Error in `/etc/falco/falco_rules.local.yaml` line 152:**
```yaml
# Wrong:
caps=%proc.cap_permitted

# Correct:
caps=%thread.cap_permitted
```

## **Quick Fix (Recommended)**

Run this one command:
```bash
sudo bash security/fix-rules-syntax.sh
```

This will:
1. Backup current rules
2. Install corrected rules
3. Validate syntax
4. Restart Falco
5. Verify it's running

Expected output:
```
✓ Rules are valid!
✓✓✓ SUCCESS! Falco is running!
```

## Manual Fix (Alternative)

If you prefer to fix manually:

### Step 1: Edit the rules file
```bash
sudo nano /etc/falco/falco_rules.local.yaml
```

### Step 2: Find the "Privileged Container with CAP_SYS_ADMIN" rule (around line 147)
Look for:
```yaml
- rule: Privileged Container with CAP_SYS_ADMIN
  desc: Detect privileged containers that could break out
  condition: >
    container_started and
    container.privileged = true or
    proc.cap_permitted contains CAP_SYS_ADMIN  # ← CHANGE THIS
  output: >
    Privileged container started
    (container=%container.name image=%container.image caps=%proc.cap_permitted)  # ← AND THIS
```

### Step 3: Replace proc.cap_permitted with thread.cap_permitted
The rule should look like this:
```yaml
- rule: Privileged Container with CAP_SYS_ADMIN
  desc: Detect privileged containers that could break out
  condition: >
    container_started and
    container.privileged = true or
    thread.cap_permitted contains CAP_SYS_ADMIN
  output: >
    Privileged container started
    (container=%container.name image=%container.image caps=%thread.cap_permitted)
```

### Step 4: Save and restart
```bash
# Validate
sudo falco --validate /etc/falco/falco_rules.local.yaml

# Restart
sudo systemctl restart falco-modern-bpf.service

# Check status
sudo systemctl status falco-modern-bpf.service
```

## Verification

After applying the fix, verify Falco is working:

```bash
# 1. Check service status
systemctl status falco-modern-bpf.service
# Expected: active (running)

# 2. Check rules are loaded
grep -c "^- rule:" /etc/falco/falco_rules.local.yaml
# Expected: 18

# 3. View live logs
sudo journalctl -u falco-modern-bpf.service -f
# Should show: "Enabled event sources: syscall"

# 4. Run test suite
bash security/test-falco.sh
```

## Common Issues

### Issue 1: Service keeps restarting

**Symptom**: `activating (auto-restart)`

**Solution**: Check for configuration errors
```bash
sudo falco --validate /etc/falco/falco.yaml
sudo journalctl -u falco-modern-bpf.service -n 50
```

### Issue 2: Rules not loading

**Symptom**: No custom rules in logs

**Solution**: Verify rules file
```bash
ls -lh /etc/falco/falco_rules.local.yaml
grep -c "^- rule:" /etc/falco/falco_rules.local.yaml
sudo falco --validate /etc/falco/falco_rules.local.yaml
```

### Issue 3: JSON output not working

**Symptom**: Plain text logs instead of JSON

**Solution**: Enable JSON output
```bash
sudo sed -i 's/json_output: false/json_output: true/' /etc/falco/falco.yaml
sudo systemctl restart falco-modern-bpf.service
```

### Issue 4: File output not working

**Symptom**: No `/var/log/falco.log` file

**Solution**: Check file_output configuration
```bash
grep -A 5 "file_output:" /etc/falco/falco.yaml
# Should show: enabled: true, filename: /var/log/falco.log

# If needed, fix it:
sudo bash security/fix-config-duplicates.sh
```

## Advanced Troubleshooting

### View detailed startup logs
```bash
sudo journalctl -u falco-modern-bpf.service -b --no-pager | less
```

### Check driver status
```bash
# Modern eBPF (should be active)
systemctl status falco-modern-bpf.service

# Check kernel version (should be 6.8+)
uname -r
```

### Validate all configuration files
```bash
sudo falco --validate /etc/falco/falco.yaml
sudo falco --validate /etc/falco/falco_rules.yaml
sudo falco --validate /etc/falco/falco_rules.local.yaml
```

### Test Falco manually
```bash
# Run in foreground to see all output
sudo falco -o engine.kind=modern_ebpf
# Press Ctrl+C to stop

# If this works, the issue is with the service config
```

### Reset to defaults
```bash
# Restore original config
sudo cp /etc/falco/falco.yaml.bak /etc/falco/falco.yaml

# Reinstall custom rules
sudo cp security/falco_rules.yaml /etc/falco/falco_rules.local.yaml

# Restart
sudo systemctl restart falco-modern-bpf.service
```

## Getting Help

### Collect diagnostic information
```bash
# Create diagnostic bundle
mkdir -p /tmp/falco-diag
sudo journalctl -u falco-modern-bpf.service -n 200 > /tmp/falco-diag/service.log
sudo falco --version > /tmp/falco-diag/version.txt
grep -A 10 "file_output:" /etc/falco/falco.yaml > /tmp/falco-diag/config-snippet.txt
grep -c "^- rule:" /etc/falco/falco_rules.local.yaml > /tmp/falco-diag/rule-count.txt
systemctl status falco-modern-bpf.service > /tmp/falco-diag/status.txt 2>&1

# View bundle
tar -czf /tmp/falco-diag.tar.gz -C /tmp falco-diag
ls -lh /tmp/falco-diag.tar.gz
```

### Resources
- **Official Docs**: https://falco.org/docs/troubleshooting/
- **Project Guide**: `security/FALCO_GUIDE.md`
- **GitHub Issues**: https://github.com/falcosecurity/falco/issues
- **Slack**: https://slack.falco.org

## Success Checklist

After troubleshooting, verify:
- [ ] `systemctl status falco-modern-bpf.service` shows "active (running)"
- [ ] `grep -c "^- rule:" /etc/falco/falco_rules.local.yaml` shows "18"
- [ ] `sudo journalctl -u falco-modern-bpf.service -n 20` shows no errors
- [ ] `/var/log/falco.log` exists and is being written to
- [ ] `bash security/test-falco.sh` passes all checks

## Quick Reference

**Start/Stop/Restart**:
```bash
sudo systemctl start falco-modern-bpf.service
sudo systemctl stop falco-modern-bpf.service
sudo systemctl restart falco-modern-bpf.service
```

**View Logs**:
```bash
# Live
sudo journalctl -u falco-modern-bpf.service -f

# Last 50 lines
sudo journalctl -u falco-modern-bpf.service -n 50

# Since boot
sudo journalctl -u falco-modern-bpf.service -b

# JSON output
tail -f /var/log/falco.log | jq .
```

**Configuration Files**:
- Main config: `/etc/falco/falco.yaml`
- Default rules: `/etc/falco/falco_rules.yaml`
- Custom rules: `/etc/falco/falco_rules.local.yaml`
- Backup: `/etc/falco/falco.yaml.bak`

---

**Last Updated**: 2025-10-19
**Status**: Rules syntax error fixed (proc.cap_permitted → thread.cap_permitted)
**Fix**: Run `sudo bash security/fix-rules-syntax.sh`
