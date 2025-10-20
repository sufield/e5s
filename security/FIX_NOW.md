# Fix Falco - Run This Now

## The Problem

Falco is failing to start due to **two issues**:

1. ✅ **Fixed**: Rules syntax error (`proc.cap_permitted` → `thread.cap_permitted`)
2. ⚠️ **New Issue**: Missing `enabled: true` in `file_output` configuration

## The Solution

Run this **single command** to fix everything:

```bash
sudo bash security/complete-falco-fix.sh
```

## What It Does

This comprehensive fix script will:

1. ✅ Install corrected rules (thread.cap_permitted)
2. ✅ Add `enabled: true` to file_output
3. ✅ Enable JSON output
4. ✅ Validate all configurations
5. ✅ Restart Falco service
6. ✅ Verify it's running

## Expected Output

```
========================================
Complete Falco Fix
========================================

[Step 1/6] Fixing rules syntax...
  ✓ Corrected rules installed
  ✓ Rules syntax valid

[Step 2/6] Configuring file output...
  ✓ Added enabled: true

[Step 3/6] Enabling JSON output...
  ✓ JSON output enabled

[Step 4/6] Validating configuration...
  ✓ Main configuration valid

[Step 5/6] Restarting Falco service...

[Step 6/6] Verifying service status...

╔════════════════════════════════════╗
║  ✓✓✓ SUCCESS! Falco is Running!  ║
╚════════════════════════════════════╝

● falco-modern-bpf.service - Falco: Container Native Runtime Security
     Active: active (running)

Summary of fixes applied:
  ✓ Rules syntax corrected (thread.cap_permitted)
  ✓ File output enabled
  ✓ JSON output enabled
  ✓ All validations passed
  ✓ Service running successfully
```

## What If It Still Fails?

If the service still doesn't start, run the diagnostic:

```bash
sudo bash security/diagnose-crash.sh
```

This will show the exact error message when Falco tries to start.

## Why Did This Happen?

### Issue 1: Rules Syntax Error
The custom rule used `proc.cap_permitted`, but Falco requires `thread.cap_permitted` because Linux capabilities are per-thread, not per-process.

### Issue 2: Missing enabled Field
The `file_output` section in `/etc/falco/falco.yaml` was missing the required `enabled: true` field:

```yaml
# Before (broken):
file_output:
  keep_alive: false
  filename: /var/log/falco.log

# After (fixed):
file_output:
  enabled: true
  keep_alive: false
  filename: /var/log/falco.log
```

Without `enabled: true`, Falco loads the configuration successfully but crashes when trying to initialize outputs.

## Next Steps After Fix

Once Falco is running:

1. **View live alerts**:
   ```bash
   sudo journalctl -u falco-modern-bpf.service -f
   ```

2. **Test the rules**:
   ```bash
   bash security/test-falco.sh
   ```

3. **Deploy SPIRE and monitor**:
   ```bash
   make minikube-up
   kubectl apply -f examples/mtls-server.yaml
   ```

4. **View JSON logs**:
   ```bash
   tail -f /var/log/falco.log | jq .
   ```

## Files Reference

- **Run this**: `security/complete-falco-fix.sh` ⭐
- Corrected rules: `security/falco_rules.yaml`
- Diagnostic tool: `security/diagnose-crash.sh`
- Troubleshooting: `security/TROUBLESHOOTING.md`
- Complete guide: `security/FALCO_GUIDE.md`

---

**TL;DR**: Run `sudo bash security/complete-falco-fix.sh` right now!
