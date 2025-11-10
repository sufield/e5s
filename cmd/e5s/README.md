# e5s CLI Tool

Command-line utility for working with the e5s mTLS library.

## Command Organization

The e5s CLI provides two types of tools:

### Control Plane Tools

Commands that inspect configuration, construct SPIFFE IDs, and query cluster state. These tools **don't send mTLS traffic** - they help you configure and understand your environment.

* `e5s spiffe-id` - Construct SPIFFE IDs from components
* `e5s discover` - Discover SPIFFE IDs from Kubernetes resources
* `e5s validate` - Validate e5s configuration files
* `e5s version` - Show version and environment information

### Data Plane Tools

Commands that **actually send or receive mTLS traffic** using the e5s library. These are debugging and testing tools.

* `e5s client request` - Make mTLS requests (like curl for e5s)

## Installation

```bash
go install github.com/sufield/e5s/cmd/e5s@latest
```

Or build from source:

```bash
cd /path/to/e5s
make build-cli
```

## Commands

### `version` - Show Version Information

Display version information for e5s, TLS configuration, and runtime environment.

**Show current runtime versions (JSON by default):**

```bash
e5s version
```

Output:
```json
{
  "e5s_version": "0.1.0",
  "e5s_commit": "abc123",
  "e5s_built": "2025-01-15",
  "tls_config": {
    "min_tls_version": "TLS 1.3",
    "client_auth": "Required (mTLS)"
  },
  "runtime": [
    {"component": "Go", "version": "go1.25.3", "status": "installed"}
  ]
}
```

**Show development requirements:**
```bash
e5s version --mode dev
```

**Show production requirements:**
```bash
e5s version --mode prod
```

**Using jq to format and extract data:**

```bash
# Pretty-print JSON for human readability
e5s version | jq .

# Extract specific version
e5s version | jq -r '.e5s_version'
# Output: 0.1.0

# Get Go version
e5s version | jq -r '.runtime[] | select(.component=="Go") | .version'
# Output: go1.25.3

# List all installed tools
e5s version | jq -r '.runtime[] | select(.status=="installed") | .component'
# Output:
# Go
# Docker
# Helm
# Minikube

# Get TLS configuration as key=value
e5s version | jq -r '.tls_config | to_entries[] | "\(.key)=\(.value)"'
# Output:
# min_tls_version=TLS 1.3
# max_tls_version=TLS 1.3
# client_auth=Required (mTLS)

# Find missing required tools
e5s version | jq -r '.runtime[] | select(.status=="not found" and .required==true) | .component'
# Output: (lists any missing required tools)

# Get dev requirements for specific component
e5s version --mode dev | jq '.requirements[] | select(.component=="Go")'
# Output:
# {
#   "component": "Go",
#   "version": "go1.25.3",
#   "required": true
# }

# Create environment variables from version info
eval $(e5s version | jq -r '"E5S_VERSION=\(.e5s_version)\nE5S_COMMIT=\(.e5s_commit)"')
echo $E5S_VERSION  # Output: 0.1.0

# Generate markdown table from requirements
e5s version --mode dev | jq -r '.requirements[] | "| \(.component) | \(.version) | \(if .required then "Required" else "Optional" end) |"'
# Output:
# | e5s | 0.1.0 | Required |
# | Go | go1.25.3 | Required |
# ...

# Check if specific tool is installed
if e5s version | jq -e '.runtime[] | select(.component=="Docker" and .status=="installed")' > /dev/null; then
  echo "Docker is installed"
fi

# Export runtime versions to shell variables
eval $(e5s version | jq -r '.runtime[] | "export \(.component | ascii_upcase | gsub("[^A-Z0-9]"; "_"))_VERSION=\"\(.version)\""')
echo $GO_VERSION     # Output: go1.25.3
echo $DOCKER_VERSION # Output: 28.5.2
```

**Plain text format for simple parsing:**

```bash
e5s version --format plain
# Output:
# e5s_version=0.1.0
# e5s_commit=abc123
# e5s_built=2025-01-15
# go=go1.25.3
# docker=28.5.2

# Extract specific value with grep
e5s version --format plain | grep '^go=' | cut -d= -f2
# Output: go1.25.3
```

**Use in CI/CD to check environment:**

```bash
# Verify all required tools are installed (exit on first missing tool)
MISSING=$(e5s version | jq -r '.runtime[] | select(.status=="not found" and .required==true) | .component' | head -1)
if [ -n "$MISSING" ]; then
    echo "Error: Required tool $MISSING is not installed"
    exit 1
fi

# Check minimum Go version
CURRENT_GO=$(e5s version | jq -r '.runtime[] | select(.component=="Go") | .version')
REQUIRED_GO=$(e5s version --mode dev | jq -r '.requirements[] | select(.component=="Go") | .version')
if [ "$CURRENT_GO" != "$REQUIRED_GO" ]; then
    echo "Warning: Go version mismatch. Current: $CURRENT_GO, Required: $REQUIRED_GO"
fi
```

### `spiffe-id` - Construct SPIFFE IDs

Construct SPIFFE IDs from components to prevent manual errors.

**Kubernetes service account (auto-detect trust domain):**

```bash
e5s spiffe-id k8s default api-client
```

Output: spiffe://example.org/ns/default/sa/api-client

The trust domain is auto-detected from your SPIRE Helm installation or ConfigMap. To use a specific trust domain:

```bash
e5s spiffe-id k8s default api-client --trust-domain=example.org
```

**From deployment YAML file:**

```bash
e5s spiffe-id from-deployment ./k8s/client-deployment.yaml
```

Output: spiffe://example.org/ns/production/sa/api-client

This extracts namespace, service account, and auto-detects trust domain from the deployment file.

**Custom path:**

```bash
e5s spiffe-id custom example.org service api-server
```

Output: spiffe://example.org/service/api-server

**Use in shell scripts:**

```bash
ALLOWED_CLIENT_ID=$(e5s spiffe-id k8s production api-client)
echo "allowed_client_spiffe_id: \"$ALLOWED_CLIENT_ID\"" >> e5s.yaml
```

**Use with envsubst:**

```bash
export CLIENT_SPIFFE_ID=$(e5s spiffe-id k8s default web-frontend)
envsubst < e5s.yaml.template > e5s.yaml
```

### `discover` - Discover SPIFFE IDs from Kubernetes

Discover actual SPIFFE IDs from running Kubernetes resources.

**Discover trust domain:**

```bash
e5s discover trust-domain
```

Output: example.org

This auto-detects the trust domain from your SPIRE Helm installation or ConfigMap.

**From pod name:**

```bash
e5s discover pod e5s-client
```

Output: spiffe://example.org/ns/default/sa/default

**From label selector:**

```bash
e5s discover label app=api-client --namespace production
```

Output: spiffe://example.org/ns/production/sa/api-client-sa

**From deployment:**

```bash
e5s discover deployment web-frontend
```

Output: spiffe://example.org/ns/default/sa/web-sa

All discovery commands auto-detect the trust domain. To override:

```bash
e5s discover deployment web-frontend --trust-domain my-domain.com
```

Output: spiffe://my-domain.com/ns/default/sa/web-sa

**Use in deployment scripts (UNIX philosophy - composable):**

Recommended: Use label selector (no manual pod selection)

```bash
CLIENT_ID=$(e5s discover label app=api-client)
```

Or pipe kubectl output to e5s

```bash
CLIENT_ID=$(kubectl get pods -l app=api-client -o name | head -1 | cut -d/ -f2 | xargs e5s discover pod)
```

Generate config with discovered ID

```bash
cat > server-config.yaml <<EOF
server:
  listen_addr: ":8443"
  allowed_client_spiffe_id: "$CLIENT_ID"
EOF
```

Validate and deploy

```bash
e5s validate server-config.yaml
```

```bash
kubectl create configmap server-config --from-file=server-config.yaml
```

```bash
kubectl apply -f deployment.yaml
```

### `validate` - Validate Configuration Files

Validate e5s YAML configuration files before deployment.

**Auto-detect mode:**

```bash
e5s validate e5s.yaml
```

**Explicit server validation:**

```bash
e5s validate e5s.yaml --mode server
```

**Explicit client validation:**

```bash
e5s validate e5s.yaml --mode client
```

**Use in CI/CD:**

Validate before deploying

```bash
if e5s validate config/production.yaml; then
    echo "✓ Configuration is valid"
    kubectl apply -f deployment.yaml
else
    echo "✗ Invalid configuration"
    exit 1
fi
```

### `client request` - Make mTLS Requests (Data-Plane Debugging)

Send mTLS requests using e5s client - like `curl` but with SPIFFE authentication.

**Simple GET request:**

```bash
e5s client request \
  --config ./e5s.yaml \
  --url https://localhost:8443/time
```

**Debug mode (shows TLS handshake, config details):**

```bash
e5s client request \
  --config ./e5s-debug.yaml \
  --url https://server.example.com:1234/time \
  --debug \
  --verbose
```

**POST request:**

```bash
e5s client request \
  --config ./e5s.yaml \
  --url https://api.example.com/endpoint \
  --method POST
```

**Use cases:**

1. Debug mTLS handshake issues
2. Verify server certificate validation
3. Test SPIFFE ID authorization
4. Reproduce production issues locally with custom configs

**sshd-like debugging workflow:**

Server on custom port (debug mode)

```bash 
e5s-example-server -config ./e5s-debug-server.yaml -debug
```

Client connecting to debug server

```bash
e5s client request \
  --config ./e5s-debug-client.yaml \
  --url https://debug-server:1234/time \
  --debug
```

## Real-World Examples

### Example 1: Zero-Trust Server Configuration

1. Deploy your client first

```bash 
kubectl apply -f client-deployment.yaml
```

2. Discover what SPIFFE ID the client actually has

```bash
CLIENT_ID=$(e5s discover deployment api-client)
```

3. Generate server config that ONLY allows that specific client

```bash
cat > e5s-server.yaml <<EOF
spire:
  workload_socket: unix:///spire/agent-socket/spire-agent.sock

server:
  listen_addr: ":8443"
  allowed_client_spiffe_id: "$CLIENT_ID"  # Zero-trust!
EOF
```

4. Validate the configuration

```bash
e5s validate e5s-server.yaml
```

5. Deploy

```bash
kubectl create configmap e5s-server-config --from-file=e5s-server.yaml
```

```bash
kubectl apply -f server-deployment.yaml
```

### Example 2: Multi-Environment Deployment

```bash
#!/bin/bash
# deploy.sh - Deploy e5s server to any environment

ENVIRONMENT=$1  # dev, staging, prod
NAMESPACE="app-$ENVIRONMENT"

# Construct expected client SPIFFE ID for this environment
CLIENT_ID=$(e5s spiffe-id k8s example.org "$NAMESPACE" api-client)

echo "Environment: $ENVIRONMENT"
echo "Namespace: $NAMESPACE"
echo "Allowed client: $CLIENT_ID"

# Generate config
cat > e5s-$ENVIRONMENT.yaml <<EOF
spire:
  workload_socket: unix:///spire/agent-socket/spire-agent.sock

server:
  listen_addr: ":8443"
  allowed_client_spiffe_id: "$CLIENT_ID"
EOF

# Validate
if ! e5s validate e5s-$ENVIRONMENT.yaml; then
    echo "✗ Configuration validation failed"
    exit 1
fi

# Deploy
kubectl -n "$NAMESPACE" create configmap e5s-config --from-file=e5s-$ENVIRONMENT.yaml --dry-run=client -o yaml | kubectl apply -f -
kubectl -n "$NAMESPACE" rollout restart deployment/api-server
```

Usage:

```bash
./deploy.sh dev
```

```bash
./deploy.sh staging
```

```bash
./deploy.sh prod
```

### Example 3: Validate All Configs in CI

```bash
#!/bin/bash
# validate-configs.sh - Run in CI pipeline

FAILED=0

for config in config/*.yaml; do
    echo "Validating $config..."
    if e5s validate "$config"; then
        echo "✓ $config is valid"
    else
        echo "✗ $config is invalid"
        FAILED=1
    fi
    echo ""
done

exit $FAILED
```

Add to `.gitlab-ci.yml`:
```yaml
validate-configs:
  stage: test
  script:
    - go install github.com/sufield/e5s/cmd/e5s@latest
    - ./validate-configs.sh
```

## Benefits

1. **Prevent Manual Errors** - No more typos in SPIFFE IDs
2. **Discover Reality** - Find actual SPIFFE IDs from running workloads
3. **Validate Before Deploy** - Catch config errors in CI, not production
4. **Scriptable** - Integrate into deployment pipelines
5. **Portable** - Single binary, no dependencies

## Help

Get help for any command:

```bash
e5s help
e5s spiffe-id --help
e5s discover --help
e5s validate --help
```
