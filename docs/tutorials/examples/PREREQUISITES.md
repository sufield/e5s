# Prerequisites - Read This First!

**⚠️  STOP! Do not skip this document.**

If you try to run the examples without understanding these prerequisites, your workloads will fail with cryptic errors like "no identity issued" or "permission denied".

---

## What is SPIRE?

SPIRE (the SPIFFE Runtime Environment) is an identity system that issues cryptographic identities (called SVIDs) to workloads. Think of it as a certificate authority specifically designed for microservices.

**Key point**: SPIRE does NOT automatically issue identities to workloads. You must explicitly register them first.

---

## Why Can't I Just Run the Examples?

**Your workload cannot get an identity until you register it with SPIRE.**

This is a security feature, not a bug. SPIRE will NOT automatically issue identities to unknown workloads. You must explicitly tell SPIRE:
- "This specific pod/container is allowed to get identity X"
- "Use these Kubernetes labels/selectors to identify it"

### Common Errors When Registration is Skipped

If you skip workload registration, you'll see errors like:
- `no identity issued`
- `no such SPIFFE ID`
- `permission denied`
- `failed to fetch X.509 SVID`
- `workload is not registered`

**These errors mean you skipped the registration step.**

---

## The Required Flow

```
1. Deploy SPIRE infrastructure (server + agent)
   ↓
2. **REGISTER your workloads** ← YOU MUST DO THIS
   ↓
3. Deploy your application pods
   ↓
4. SPIRE agent attests the pods and issues identities
   ↓
5. Your application can now use mTLS
```

**If you skip step 2 (workload registration)**, your application will fail to get an identity and cannot establish mTLS connections.

---

## Who Registers Workloads?

**You do**, using the `spire-server entry create` command. This is a **manual step** that you perform **BEFORE** deploying your workloads.

In production environments:
- Platform teams typically pre-register workloads
- CI/CD pipelines can automate registration
- GitOps tools (like Flux, ArgoCD) can manage registration entries
- But **someone has to explicitly create these entries**

---

## What Happens During Registration?

When you register a workload, you create a "registration entry" that tells SPIRE:

```bash
spire-server entry create \
  -spiffeID spiffe://example.org/server \    # The identity to issue
  -parentID <agent-spiffe-id> \              # Which agent can issue it
  -selector k8s:ns:default \                 # Match pods in 'default' namespace
  -selector k8s:sa:default \                 # Match ServiceAccount 'default'
  -selector k8s:container-name:mtls-server   # Match container named 'mtls-server'
```

This registration entry says: **"When you see a container named `mtls-server` in the `default` namespace using the `default` ServiceAccount, give it the identity `spiffe://example.org/server`."**

### How Matching Works

**All selectors must match** for the workload to receive the identity:
- If your pod is in a different namespace → No identity issued
- If your container has a different name → No identity issued
- If any selector doesn't match → No identity issued

This is by design - it prevents unauthorized workloads from obtaining identities they shouldn't have.

---

## Registration vs Deployment Order

**Correct order**:
```
1. Register workload with SPIRE (create registration entry)
2. Deploy the pod
3. SPIRE agent sees the pod, matches selectors, issues identity
```

**Incorrect order** (will fail):
```
1. Deploy the pod
2. Pod tries to connect to SPIRE
3. SPIRE has no registration entry → Denies identity
4. Pod fails with "no identity issued"
```

**Important**: Registration entries can be created before or after the SPIRE infrastructure is deployed, but they **MUST exist before your application pods start**.

---

## How to Verify Registration

After creating registration entries, verify them:

```bash
# List all registration entries
kubectl exec -n spire-system statefulset/spire-server -c spire-server -- \
  /opt/spire/bin/spire-server entry show

# You should see output like:
# Entry ID      : <some-uuid>
# SPIFFE ID     : spiffe://example.org/server
# Parent ID     : spiffe://example.org/spire/agent/...
# Selectors     : k8s:container-name:mtls-server
#                 k8s:ns:default
#                 k8s:sa:default
```

---

## Required Conceptual Knowledge

Before starting the examples, you should understand:

- ✅ **What SPIRE is**: An identity system that issues cryptographic identities (SVIDs) to workloads
- ✅ **Registration is required**: Workloads MUST be registered before they can get identities
- ✅ **Who registers**: YOU (the operator/developer) perform registration using `spire-server entry create`
- ✅ **When to register**: BEFORE deploying your application pods
- ✅ **Selectors must match**: ALL selectors must match your pod's metadata (namespace, ServiceAccount, container name)
- ✅ **No auto-discovery**: SPIRE does not automatically discover and register workloads

If you're unclear on any of these concepts, **do not proceed to the examples yet**. Re-read this document first.

---

## Required Tools

Before running the examples, ensure you have these tools installed:

| Tool | Version | Installation |
|------|---------|--------------|
| Go | 1.25.1+ | https://go.dev/dl/ |
| Minikube | 1.32.0+ | https://minikube.sigs.k8s.io/docs/start/ |
| kubectl | 1.28.0+ | https://kubernetes.io/docs/tasks/tools/ |
| Helm | 3.13.0+ | https://helm.sh/docs/intro/install/ |

### Verify Installation

```bash
# Check Go version
go version

# Check Minikube is installed
minikube version

# Check kubectl is configured for Minikube
kubectl config use-context minikube

# Check Helm is installed
helm version

# Or run our automated check
make check-prereqs-k8s
```

---

## Production vs Development Considerations

### Development (These Examples)
- Uses Minikube for easy local testing
- Manual registration via `kubectl exec` commands
- Single trust domain (`example.org`)
- Simplified security (e.g., uses `default` ServiceAccount)

### Production Deployments
- Uses real Kubernetes clusters (EKS, GKE, AKS, etc.)
- Automated registration via CI/CD pipelines or GitOps
- Multiple trust domains and federation between them
- Dedicated ServiceAccounts per workload
- Network policies and RBAC for defense in depth
- Monitoring and observability (metrics, logs, traces)
- Certificate rotation handled automatically by SPIRE
- High availability SPIRE server deployment

---

## What's Next?

Now that you understand the prerequisites, you can proceed to:

1. **[examples/README.md](README.md)** - Complete deployment guide with step-by-step instructions
2. **[examples/zeroconfig-example/](zeroconfig-example/)** - Zero-configuration mTLS server example
3. **[examples/test-client.go](test-client.go)** - Infrastructure testing tool

**Remember**: Always complete workload registration (step 3 in the README) before deploying your application pods!

---

## Quick Reference: Registration Command Template

For quick reference, here's the registration command template you'll use:

```bash
# Get the agent SPIFFE ID first (required for -parentID)
AGENT_ID=$(kubectl exec -n spire-system statefulset/spire-server -c spire-server -- \
    /opt/spire/bin/spire-server agent list | grep "SPIFFE ID" | awk -F': ' '{print $2}')

# Register a workload
kubectl exec -n spire-system statefulset/spire-server -c spire-server -- \
  /opt/spire/bin/spire-server entry create \
    -spiffeID spiffe://example.org/YOUR-WORKLOAD-NAME \
    -parentID "$AGENT_ID" \
    -selector k8s:ns:YOUR-NAMESPACE \
    -selector k8s:sa:YOUR-SERVICE-ACCOUNT \
    -selector k8s:container-name:YOUR-CONTAINER-NAME
```

Replace:
- `YOUR-WORKLOAD-NAME` - Identity path (e.g., `server`, `client`, `api`)
- `YOUR-NAMESPACE` - Kubernetes namespace (e.g., `default`, `production`)
- `YOUR-SERVICE-ACCOUNT` - Kubernetes ServiceAccount (e.g., `default`, `mtls-server-sa`)
- `YOUR-CONTAINER-NAME` - Container name from your pod spec (must match exactly!)

---

## Still Confused?

If you're still unclear about any of these concepts:

1. Read the [SPIFFE/SPIRE Overview](https://spiffe.io/docs/latest/spiffe-about/overview/)
2. Read the [SPIRE Concepts Guide](https://spiffe.io/docs/latest/spire-about/spire-concepts/)
3. Watch [SPIFFE/SPIRE Introduction Videos](https://spiffe.io/community/)
4. Ask in the [SPIFFE Slack](https://slack.spiffe.io/)

**Do not proceed to the examples until you understand**:
- What SPIRE is
- Why registration is required
- When registration happens (before deployment)
- How to verify registration worked
