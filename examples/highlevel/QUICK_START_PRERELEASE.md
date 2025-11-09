# Quick Start: Pre-Release Testing

---

## Prerequisites (One-Time Setup)

1. **SPIRE must be running in Minikube**

   If not already done:
   ```bash
   cd examples/highlevel
   ```

   Follow [SPIRE_SETUP.md](SPIRE_SETUP.md) to set up SPIRE in Minikube (~15 minutes)

2. **Verify tools are installed**
   ```bash
   make verify-tools
   ```

---

## Steps

### Step 1: Initial Setup (First Time Only)

```bash
cd /path/to/e5s

./scripts/test-prerelease.sh
```

**Output**: Test environment deployed, client responds with:
```
Hello, spiffe://example.org/ns/default/sa/default!
```

---

### Step 2: Make Code Changes

```bash
# Edit e5s library code
vim e5s.go
vim pkg/spire/source.go
# ... make your changes ...
```

---

### Step 3: Test Your Changes

```bash
./scripts/rebuild-and-test.sh
```

**Output**: Rebuilds everything, redeploys, shows test results

---

## Repeat Step 2-3 As Needed

```bash
# Edit code
vim e5s.go

# Test it
./scripts/rebuild-and-test.sh

# Edit more
vim pkg/spiffehttp/server.go

# Test again
./scripts/rebuild-and-test.sh
```

---

## When You're Done

```bash
./scripts/cleanup-prerelease.sh
```

---

## That's It!

**Total time**: ~5 minutes for initial setup, ~30 seconds per iteration

**Scripts location**: `scripts/`
- `test-prerelease.sh` - Initial setup
- `rebuild-and-test.sh` - Test after code changes
- `cleanup-prerelease.sh` - Remove all test resources

---

## Troubleshooting

**Script fails?**
```bash
# Check SPIRE is running
kubectl get pods -n spire

# Check Minikube
minikube status
```

**Need to restart SPIRE?**
```bash
# See examples/highlevel/SPIRE_SETUP.md
```

**Want to see logs?**

Server logs:
```bash
kubectl logs -l app=e5s-server -f
```

Client logs:
```bash
kubectl logs -l app=e5s-client
```

SPIRE logs:
```bash
kubectl logs -n spire -l app.kubernetes.io/name=server -c spire-server
```

**Manual cleanup:**
```bash
kubectl delete deployment e5s-server
kubectl delete service e5s-server
kubectl delete job e5s-client
kubectl delete configmap e5s-config
```

---

## What Gets Created

The scripts automatically create:

```
test-demo/
├── server/main.go           # Test server
├── client/main.go           # Test client
├── e5s.yaml                 # Library config file
├── client-config.yaml       # Application config file
├── k8s-configs.yaml         # Kubernetes ConfigMaps (e5s + client config)
├── k8s-server.yaml          # Server deployment
├── k8s-client-job.yaml      # Client job
├── k8s-unregistered-client-job.yaml  # Unauthorized client (for testing)
├── bin/
│   ├── server              # Compiled binary
│   └── client              # Compiled binary
└── go.mod                  # With replace directive
```

Plus Docker images: `e5s-server:dev` and `e5s-client:dev`

---

## Next Steps

After testing is complete:
1. Run full test suite: `make test`
2. Run security checks: `make sec-all`
3. Follow publishing checklist in `TESTING_PRERELEASE.md`
