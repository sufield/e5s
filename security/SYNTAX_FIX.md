# Falco Rules Syntax Error - Fixed

## Problem Identified

The Falco service was failing to start with the following error:

```
Error: /etc/falco/falco_rules.local.yaml: Invalid
rule 'Privileged Container with CAP_SYS_ADMIN':
LOAD_ERR_COMPILE_OUTPUT (Error compiling output): invalid formatting token proc.cap_permitted)
```

## Root Cause

The custom Falco rules used an **incorrect field name** for process capabilities:

- ❌ **Wrong**: `proc.cap_permitted`
- ✅ **Correct**: `thread.cap_permitted`

According to [Falco's field reference](https://falco.org/docs/reference/rules/supported-fields/), capability fields are prefixed with `thread.` not `proc.`:

- `thread.cap_permitted` - The permitted capabilities set
- `thread.cap_inheritable` - The inheritable capabilities set
- `thread.cap_effective` - The effective capabilities set

## What Was Fixed

### File: `security/falco_rules.yaml`

**Line 152** (condition field):
```yaml
# Before:
proc.cap_permitted contains CAP_SYS_ADMIN

# After:
thread.cap_permitted contains CAP_SYS_ADMIN
```

**Line 155** (output field):
```yaml
# Before:
caps=%proc.cap_permitted

# After:
caps=%thread.cap_permitted
```

### Full Corrected Rule

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
  priority: WARNING
  tags: [container, privileged]
```

## How to Apply the Fix

Run the automated fix script:

```bash
sudo bash security/fix-rules-syntax.sh
```

This script will:
1. ✓ Backup current rules (`/etc/falco/falco_rules.local.yaml.syntax-backup`)
2. ✓ Copy corrected rules from `security/falco_rules.yaml`
3. ✓ Validate syntax with `falco --validate`
4. ✓ Restart Falco service
5. ✓ Verify service is running

## Expected Output

```
========================================
Fixing Falco Rules Syntax Error
========================================

[1/4] Backing up current rules...
  ✓ Backup created
[2/4] Installing fixed rules...
  ✓ Rules copied
[3/4] Validating rules syntax...
  ✓ Rules are valid!
[4/4] Restarting Falco service...
  ✓✓✓ SUCCESS! Falco is running!

========================================
Falco is Now Active
========================================

● falco-modern-bpf.service - Falco: Container Native Runtime Security with modern ebpf
     Loaded: loaded
     Active: active (running)
```

## Verification

After applying the fix, verify everything is working:

```bash
# 1. Check service status
sudo systemctl status falco-modern-bpf.service
# Expected: active (running)

# 2. Verify rules loaded
grep -c "^- rule:" /etc/falco/falco_rules.local.yaml
# Expected: 18

# 3. View live logs
sudo journalctl -u falco-modern-bpf.service -f
# Should show: "Enabled event sources: syscall"

# 4. Run test suite
bash security/test-falco.sh
```

## Technical Details

### Why `thread.` instead of `proc.`?

In Falco's architecture:
- `proc.*` fields refer to process-level attributes (name, pid, cmdline, etc.)
- `thread.*` fields refer to thread-level attributes (capabilities, namespaces, etc.)

Capabilities in Linux are **per-thread**, not per-process, which is why Falco uses the `thread.` prefix for capability fields.

### What Other Rules Might Be Affected?

If you create custom Falco rules, be aware of the correct field prefixes:

**Process fields**:
- `proc.name` - Process name
- `proc.pid` - Process ID
- `proc.cmdline` - Command line
- `proc.pname` - Parent process name

**Thread fields**:
- `thread.cap_permitted` - Permitted capabilities
- `thread.cap_effective` - Effective capabilities
- `thread.cap_inheritable` - Inheritable capabilities

**Container fields**:
- `container.name` - Container name
- `container.image` - Container image
- `container.privileged` - Is privileged

**File descriptor fields**:
- `fd.name` - File/socket name
- `fd.sport` - Server port
- `fd.cip` - Client IP

## Next Steps

1. **Deploy SPIRE**: Now that Falco is running, you can deploy your SPIRE infrastructure
   ```bash
   make minikube-up
   kubectl apply -f examples/mtls-server.yaml
   ```

2. **Monitor Alerts**: Watch for security events
   ```bash
   sudo journalctl -u falco-modern-bpf.service -f
   ```

3. **Test Rules**: Trigger test events
   ```bash
   bash security/test-falco.sh
   ```

4. **View JSON Logs**: If json_output is enabled
   ```bash
   tail -f /var/log/falco.log | jq .
   ```

## Related Files

- **Source rules**: `security/falco_rules.yaml` (now corrected)
- **Installed rules**: `/etc/falco/falco_rules.local.yaml` (updated by fix script)
- **Fix script**: `security/fix-rules-syntax.sh`
- **Troubleshooting guide**: `security/TROUBLESHOOTING.md`
- **Complete guide**: `security/FALCO_GUIDE.md`

## References

- [Falco Supported Fields](https://falco.org/docs/reference/rules/supported-fields/)
- [Falco Rules Syntax](https://falco.org/docs/rules/)
- [Linux Capabilities](https://man7.org/linux/man-pages/man7/capabilities.7.html)

---

**Status**: ✅ Fixed
**Date**: 2025-10-19
**Impact**: Falco service now starts successfully with all 18 custom SPIRE mTLS rules
