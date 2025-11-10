#!/usr/bin/env bash
# Collect environment version information for release documentation
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

env_name="${1:-dev}"              # e.g. dev, staging, prod
out_dir="${2:-artifacts}"         # output directory
mkdir -p "${out_dir}"

timestamp="$(date -u +"%Y-%m-%dT%H:%M:%SZ")"
outfile="${out_dir}/env-versions-${env_name}.txt"

{
  echo "# Environment Versions (${env_name})"
  echo "# Generated at: ${timestamp}"
  echo ""

  echo "## e5s Project & Runtime Environment"
  # Delegate to e5s CLI for common tool versions (Go, Docker, kubectl, Helm, Minikube)
  # Check local bin/ directory first, then PATH
  if [ -x "${PROJECT_ROOT}/bin/e5s" ]; then
    "${PROJECT_ROOT}/bin/e5s" version --format plain
  elif command -v e5s &> /dev/null; then
    e5s version --format plain
  else
    echo "e5s: not installed (build with 'make build-cli')"
    echo ""
    # Fallback to manual checks if CLI not available
    echo "e5s_version=${E5S_VERSION:-dev}"
    echo "go=$(go version 2>/dev/null || echo 'not installed')"
  fi

  # Add e5s-specific info not in CLI version command
  if [ -f "go.mod" ]; then
    go_spiffe_version=$(grep "github.com/spiffe/go-spiffe" go.mod | awk '{print $2}' | head -1)
    echo "go_spiffe_sdk=${go_spiffe_version:-not found in go.mod}"
  else
    echo "go_spiffe_sdk=go.mod not found"
  fi
  echo "git_commit=$(git rev-parse --short HEAD 2>/dev/null || echo 'not a git repo')"
  echo "git_branch=$(git rev-parse --abbrev-ref HEAD 2>/dev/null || echo 'not a git repo')"
  echo ""

  echo "## SPIFFE / SPIRE"

  # Check Helm chart version from helmfile
  if [ -f "examples/minikube-lowlevel/infra/helmfile.yaml" ]; then
    spire_chart_version=$(grep -A 5 "chart: spiffe/spire" examples/minikube-lowlevel/infra/helmfile.yaml | grep "version:" | awk '{print $2}' | head -1)
    echo "spire_helm_chart=${spire_chart_version:-not found in helmfile}"

    # Get app version from Helm chart if available
    if command -v helm &> /dev/null && [ -n "$spire_chart_version" ]; then
      app_version=$(helm search repo spiffe/spire --version "$spire_chart_version" -o yaml 2>/dev/null | grep "app_version:" | awk '{print $3}' | head -1)
      if [ -n "$app_version" ]; then
        echo "spire_server_chart=v${app_version}"
        echo "spire_agent_chart=v${app_version}"
      fi
    fi
  fi

  # Try to get runtime versions if cluster is running
  if command -v kubectl &> /dev/null; then
    runtime_server_version=$(kubectl exec -n spire-system spire-server-0 -c spire-server -- spire-server --version 2>/dev/null | head -1 || echo '')
    if [ -n "$runtime_server_version" ]; then
      echo "spire_server_runtime=${runtime_server_version}"
    fi
  fi
  echo ""

  echo "## Additional Container Tools"
  if command -v kind &> /dev/null; then
    echo "kind=$(kind version 2>/dev/null || echo 'error getting version')"
  else
    echo "kind=not installed"
  fi
  echo ""

  echo "## Security Tools"
  if command -v govulncheck &> /dev/null; then
    # govulncheck doesn't have a --version flag, so just check if it exists
    echo "govulncheck=installed"
  else
    echo "govulncheck=not installed"
  fi

  if command -v gosec &> /dev/null; then
    echo "gosec=$(gosec --version 2>/dev/null || echo 'installed')"
  else
    echo "gosec=not installed"
  fi

  if command -v gitleaks &> /dev/null; then
    echo "gitleaks=$(gitleaks version 2>/dev/null || echo 'installed')"
  else
    echo "gitleaks=not installed"
  fi

  if command -v golangci-lint &> /dev/null; then
    echo "golangci-lint=$(golangci-lint --version 2>/dev/null | head -1 || echo 'installed')"
  else
    echo "golangci-lint=not installed"
  fi
  echo ""

  echo "## System Information"
  echo "os=$(uname -s)"
  echo "arch=$(uname -m)"
  echo "kernel=$(uname -r)"
  echo ""

} > "${outfile}"

echo "Wrote environment versions to ${outfile}"
cat "${outfile}"
