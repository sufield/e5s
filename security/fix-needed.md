# Falco Custom Rules - Compilation Errors

## Environment

- **Falco Version**: 0.41.3 (x86_64)
- **OS**: Ubuntu 24.04.3 LTS
- **Kernel**: 6.8.0-85-generic
- **Installation Method**: `apt install falco`
- **Driver Tested**: modern_ebpf, ebpf, kmod (all fail with same error)

## Problem Statement

I'm trying to create custom Falco rules for monitoring SPIRE mTLS applications. The rules file validates successfully with `falco --validate`, but when Falco actually tries to load the rules, it fails with `LOAD_ERR_COMPILE_OUTPUT` errors.

## What I'm Trying to Achieve

Create 18 custom Falco rules to monitor:
- SPIRE Workload API socket access
- mTLS server security (shell spawning, file access, network activity)
- Container security (privilege escalation, escape attempts)
- Go binary behavior
- Kubernetes resource access

## Current Errors

### Error 1: Invalid Formatting Token (Capabilities Field)

```
Error: /etc/falco/falco_rules.local.yaml: Invalid
1 Errors:
In rules content: (/etc/falco/falco_rules.local.yaml:0:0)
    rule 'Privileged Container with CAP_SYS_ADMIN': (/etc/falco/falco_rules.local.yaml:146:2)
    condition expression: ("container_started...":147:3)
------
...and container.privileged = true or thread.cap_permitted contains CAP_SYS_ADMIN
                                                                                ^
------
LOAD_ERR_VALIDATE: Undefined macro 'container_started' used in filter.
```

**Attempted Fix**: Changed `container_started` to `spawned_process and container`

**Result**: Still fails

### Error 2: Invalid Formatting Token (Network Fields)

```
Error: /etc/falco/falco_rules.local.yaml: Invalid
1 Errors:
In rules content: (/etc/falco/falco_rules.local.yaml:0:0)
    rule 'Outbound Connection from mTLS Server': (/etc/falco/falco_rules.local.yaml:207:2)
    rule output: (/etc/falco/falco_rules.local.yaml:216:10)
------
  output: >
          ^
------
LOAD_ERR_COMPILE_OUTPUT: invalid formatting token fd.dip:%fd.dport source=%fd.sip:%fd.sport
```

**Attempted Fix**: Removed colons, changed to `dest_ip=%fd.dip dest_port=%fd.dport`

**Result**: Still fails with extra content appearing in error message

## Minimal Reproducible Example

### Rule That Fails

```yaml
- rule: Outbound Connection from mTLS Server
  desc: Detect unexpected outbound connections (mTLS servers should be inbound-only)
  condition: >
    evt.type = connect and
    evt.dir = > and
    fd.type = ipv4 and
    proc.name in (mtls-server, zeroconfig-example) and
    not fd.sip in (127.0.0.1, 172.17.0.0/16, 10.0.0.0/8) and
    fd.dport != 53
  output: >
    Unexpected outbound connection from mTLS server
    (proc=%proc.name dest_ip=%fd.dip dest_port=%fd.dport src_ip=%fd.sip src_port=%fd.sport)
  priority: WARNING
  tags: [network, mtls, outbound]
```

### How to Reproduce

1. Save rule to `/etc/falco/falco_rules.local.yaml`
2. Run: `sudo falco --validate /etc/falco/falco_rules.local.yaml`
   - **Result**: "schema validation: ok"
3. Run: `sudo falco -o engine.kind=modern_ebpf`
   - **Result**: Error with "invalid formatting token"

### Actual Error Output

```
Sun Oct 19 20:49:21 2025: Loading rules from:
Sun Oct 19 20:49:21 2025:    /etc/falco/falco_rules.yaml | schema validation: ok
Sun Oct 19 20:49:21 2025:    /etc/falco/falco_rules.local.yaml | schema validation: ok
Error: /etc/falco/falco_rules.local.yaml: Invalid
1 Errors:
In rules content: (/etc/falco/falco_rules.local.yaml:0:0)
    rule 'Outbound Connection from mTLS Server': (/etc/falco/falco_rules.local.yaml:207:2)
    rule output: (/etc/falco/falco_rules.local.yaml:216:10)
------
  output: >
          ^
------
LOAD_ERR_COMPILE_OUTPUT (Error compiling output): invalid formatting token fd.dip dest_port=%fd.dport src_ip=%fd.sip src_port=%fd.sport) container_id=%container.id container_name=%container.name container_image_repository=%container.image.repository container_image_tag=%container.image.tag k8s_pod_name=%k8s.pod.name k8s_ns_name=%k8s.ns.name
```

**Important**: Notice the extra content after the closing parenthesis: `container_id=%container.id...` - this is **not** in my source file!

## What I've Tried

### Attempt 1: Fixed Field Names
- Changed `proc.cap_permitted` → `thread.cap_permitted`
- Changed `container_started` → `spawned_process and container`
- **Result**: Different error (network fields)

### Attempt 2: Removed Colons from Output
- Changed `%fd.dip:%fd.dport` → `dest_ip=%fd.dip dest_port=%fd.dport`
- **Result**: Still fails with extra content in error

### Attempt 3: Clean File Reinstall
- Deleted `/etc/falco/falco_rules.local.yaml`
- Copied fresh file from source
- Verified file content is correct
- **Result**: Same error with mysterious extra content

### Attempt 4: Validated Syntax
```bash
$ sudo falco --validate /etc/falco/falco_rules.local.yaml
# Returns successfully (no errors)
```
- Schema validation passes
- But runtime compilation fails

## Questions

1. **Why does schema validation pass but compilation fail?**
   - `falco --validate` says "ok"
   - But `falco` run fails with compilation error

2. **Where is the extra content coming from?**
   - Error shows: `container_id=%container.id container_name=%container.name...`
   - This text is NOT in my rules file
   - Checked with `cat`, `grep`, and manual inspection

3. **What is the correct syntax for network output fields?**
   - Tried: `%fd.dip:%fd.dport` (fails: invalid colon)
   - Tried: `dest_ip=%fd.dip dest_port=%fd.dport` (fails: invalid formatting token)
   - What's the correct way?

4. **Are there Falco output format restrictions I'm missing?**
   - Is there documentation on what tokens/characters are forbidden in output?
   - Are there field-specific formatting requirements?

5. **Could this be a bug in Falco 0.41.3?**
   - The "extra content" appearing in errors is very strange
   - Could this be a parser bug that's appending metadata?

## Expected Behavior

The rule should:
1. Validate successfully (✅ currently working)
2. Compile successfully (❌ currently failing)
3. Load into Falco (❌ blocked by compilation error)
4. Trigger alerts on network connections (⏸️ can't test yet)

## Additional Context

### Working Rules Examples

These simpler rules work fine:

```yaml
- rule: Unauthorized Access to SPIRE Socket
  desc: Detect access to SPIRE Workload API socket by unauthorized processes
  condition: >
    evt.type in (open, openat) and
    fd.name contains "/tmp/spire-agent/public/api.sock" and
    not proc.name in (mtls-server, test-client, zeroconfig-example, spire-agent)
  output: >
    Unauthorized process accessing SPIRE socket
    (proc=%proc.name pid=%proc.pid user=%user.name file=%fd.name)
  priority: CRITICAL
  tags: [spire, security, unauthorized_access]
```

**Difference**: No network fields, no capabilities fields

### Files Available

- Default rules: `/etc/falco/falco_rules.yaml` (working)
- Custom rules: `/etc/falco/falco_rules.local.yaml` (failing)
- Main config: `/etc/falco/falco.yaml`

### Verification Commands Used

```bash
# Validate syntax
sudo falco --validate /etc/falco/falco_rules.local.yaml

# Test run
sudo falco -o engine.kind=modern_ebpf

# Check file content
cat /etc/falco/falco_rules.local.yaml | sed -n '207,225p'

# Count rules
grep -c "^- rule:" /etc/falco/falco_rules.local.yaml
# Returns: 18
```

## What I Need

1. **Correct syntax** for output fields with network information (IP, port)
2. **Explanation** of why extra content appears in error messages
3. **Documentation** on Falco output format restrictions
4. **Workaround** to get these rules working on Falco 0.41.3

## References

- [Falco Documentation](https://falco.org/docs/)
- [Falco Rules Syntax](https://falco.org/docs/rules/)
- [Falco Supported Fields](https://falco.org/docs/reference/rules/supported-fields/)
- Default rules in `/etc/falco/falco_rules.yaml` for comparison

---

**Note**: This is for a defensive security project monitoring SPIRE mTLS infrastructure. The rules are for detecting unauthorized access, not for malicious purposes.
