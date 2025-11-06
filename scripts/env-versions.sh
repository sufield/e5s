#!/usr/bin/env bash
# Collect environment version information for release documentation
set -euo pipefail

env_name="${1:-dev}"              # e.g. dev, staging, prod
out_dir="${2:-artifacts}"         # output directory
mkdir -p "${out_dir}"

timestamp="$(date -u +"%Y-%m-%dT%H:%M:%SZ")"
outfile="${out_dir}/env-versions-${env_name}.txt"

{
  echo "# Environment Versions (${env_name})"
  echo "# Generated at: ${timestamp}"
  echo ""

  echo "## e5s Project"
  echo "e5s version: ${E5S_VERSION:-dev}"
  echo "Go version: $(go version 2>/dev/null || echo 'go not installed')"
  if [ -f "go.mod" ]; then
    go_spiffe_version=$(grep "github.com/spiffe/go-spiffe" go.mod | awk '{print $2}' | head -1)
    echo "go-spiffe SDK: ${go_spiffe_version:-not found in go.mod}"
  else
    echo "go-spiffe SDK: go.mod not found"
  fi
  echo "git commit: $(git rev-parse --short HEAD 2>/dev/null || echo 'not a git repo')"
  echo "git branch: $(git rev-parse --abbrev-ref HEAD 2>/dev/null || echo 'not a git repo')"
  echo ""

  echo "## Kubernetes / Cluster Tools"
  if command -v minikube &> /dev/null; then
    echo "minikube: $(minikube version --short 2>/dev/null || echo 'error getting version')"
  else
    echo "minikube: not installed"
  fi

  if command -v kubectl &> /dev/null; then
    echo "kubectl (client): $(kubectl version --client --short 2>/dev/null | head -1 || echo 'error getting version')"
  else
    echo "kubectl: not installed"
  fi

  if command -v helm &> /dev/null; then
    echo "helm: $(helm version --short 2>/dev/null || echo 'error getting version')"
  else
    echo "helm: not installed"
  fi
  echo ""

  echo "## SPIFFE / SPIRE"

  # Check Helm chart version from helmfile
  if [ -f "examples/minikube-lowlevel/infra/helmfile.yaml" ]; then
    spire_chart_version=$(grep -A 5 "chart: spiffe/spire" examples/minikube-lowlevel/infra/helmfile.yaml | grep "version:" | awk '{print $2}' | head -1)
    echo "SPIRE Helm Chart: ${spire_chart_version:-not found in helmfile}"

    # Get app version from Helm chart if available
    if command -v helm &> /dev/null && [ -n "$spire_chart_version" ]; then
      app_version=$(helm search repo spiffe/spire --version "$spire_chart_version" -o yaml 2>/dev/null | grep "app_version:" | awk '{print $3}' | head -1)
      if [ -n "$app_version" ]; then
        echo "SPIRE Server (from Helm chart): v${app_version}"
        echo "SPIRE Agent (from Helm chart): v${app_version}"
      fi
    fi
  fi

  # Try to get runtime versions if cluster is running
  if command -v kubectl &> /dev/null; then
    runtime_server_version=$(kubectl exec -n spire-system spire-server-0 -c spire-server -- spire-server --version 2>/dev/null | head -1 || echo '')
    if [ -n "$runtime_server_version" ]; then
      echo "SPIRE Server (runtime): ${runtime_server_version}"
    fi
  fi
  echo ""

  echo "## Container Tools"
  if command -v docker &> /dev/null; then
    echo "docker: $(docker --version 2>/dev/null || echo 'error getting version')"
  else
    echo "docker: not installed"
  fi

  if command -v kind &> /dev/null; then
    echo "kind: $(kind version 2>/dev/null || echo 'error getting version')"
  else
    echo "kind: not installed"
  fi
  echo ""

  echo "## Security Tools"
  if command -v govulncheck &> /dev/null; then
    # govulncheck doesn't have a --version flag, so just check if it exists
    echo "govulncheck: installed"
  else
    echo "govulncheck: not installed"
  fi

  if command -v gosec &> /dev/null; then
    echo "gosec: $(gosec --version 2>/dev/null || echo 'installed')"
  else
    echo "gosec: not installed"
  fi

  if command -v gitleaks &> /dev/null; then
    echo "gitleaks: $(gitleaks version 2>/dev/null || echo 'installed')"
  else
    echo "gitleaks: not installed"
  fi

  if command -v golangci-lint &> /dev/null; then
    echo "golangci-lint: $(golangci-lint --version 2>/dev/null | head -1 || echo 'installed')"
  else
    echo "golangci-lint: not installed"
  fi
  echo ""

  echo "## System Information"
  echo "OS: $(uname -s)"
  echo "Arch: $(uname -m)"
  echo "Kernel: $(uname -r)"
  echo ""

} > "${outfile}"

echo "Wrote environment versions to ${outfile}"
cat "${outfile}"
