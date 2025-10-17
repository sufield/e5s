#!/usr/bin/env bash
# cluster-up.sh - Start Minikube cluster and deploy SPIRE
# Usage: ./cluster-up.sh [start|verify|help]

set -euo pipefail

# Color codes for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Script directory
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
INFRA_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"
PROJECT_ROOT="$(cd "${INFRA_DIR}/../../.." && pwd)"

# Configuration
CLUSTER_NAME="${CLUSTER_NAME:-minikube}"
CPUS="${MINIKUBE_CPUS:-2}"
MEMORY="${MINIKUBE_MEMORY:-4g}"
DISK="${MINIKUBE_DISK:-20g}"
DRIVER="${MINIKUBE_DRIVER:-docker}"
K8S_VERSION="${K8S_VERSION:-v1.31.0}"

# Logging functions
log_info() {
    echo -e "${BLUE}→${NC} $*"
}

log_success() {
    echo -e "${GREEN}✓${NC} $*"
}

log_warn() {
    echo -e "${YELLOW}⚠${NC} $*"
}

log_error() {
    echo -e "${RED}✗${NC} $*" >&2
}

# Check prerequisites
check_prerequisites() {
    log_info "Checking prerequisites..."

    local missing_tools=()

    if ! command -v minikube &>/dev/null; then
        missing_tools+=("minikube")
    fi

    if ! command -v kubectl &>/dev/null; then
        missing_tools+=("kubectl")
    fi

    if ! command -v helmfile &>/dev/null; then
        log_warn "helmfile not found, will use direct helm commands"
    fi

    if ! command -v helm &>/dev/null; then
        missing_tools+=("helm")
    fi

    if [ ${#missing_tools[@]} -gt 0 ]; then
        log_error "Missing required tools: ${missing_tools[*]}"
        log_info "Install instructions:"
        log_info "  minikube: https://minikube.sigs.k8s.io/docs/start/"
        log_info "  kubectl: https://kubernetes.io/docs/tasks/tools/"
        log_info "  helm: https://helm.sh/docs/intro/install/"
        log_info "  helmfile: https://github.com/helmfile/helmfile#installation"
        exit 1
    fi

    log_success "Prerequisites check passed"
}

# Start Minikube cluster
start_minikube() {
    log_info "Starting Minikube cluster '${CLUSTER_NAME}'..."

    # Check if cluster already exists
    if minikube status -p "${CLUSTER_NAME}" &>/dev/null; then
        local status
        status=$(minikube status -p "${CLUSTER_NAME}" -o json | grep -o '"Host":"[^"]*"' | cut -d'"' -f4)

        if [ "${status}" == "Running" ]; then
            log_success "Minikube cluster '${CLUSTER_NAME}' is already running"
            return 0
        else
            log_warn "Minikube cluster '${CLUSTER_NAME}' exists but not running—deleting and recreating..."
            minikube delete -p "${CLUSTER_NAME}"
        fi
    fi

    # Start new cluster
    log_info "Creating new Minikube cluster with:"
    log_info "  CPUs: ${CPUS}"
    log_info "  Memory: ${MEMORY}"
    log_info "  Disk: ${DISK}"
    log_info "  Driver: ${DRIVER}"
    log_info "  Kubernetes: ${K8S_VERSION}"

    minikube start \
        -p "${CLUSTER_NAME}" \
        --cpus="${CPUS}" \
        --memory="${MEMORY}" \
        --disk-size="${DISK}" \
        --driver="${DRIVER}" \
        --kubernetes-version="${K8S_VERSION}" \
        --extra-config=apiserver.service-account-signing-key-file=/var/lib/minikube/certs/sa.key \
        --extra-config=apiserver.service-account-key-file=/var/lib/minikube/certs/sa.pub \
        --extra-config=apiserver.service-account-issuer=api \
        --extra-config=apiserver.authorization-mode=Node,RBAC \
        --addons=metrics-server \
        --addons=dashboard

    log_success "Minikube cluster '${CLUSTER_NAME}' started successfully"

    # Set kubectl context
    kubectl config use-context "${CLUSTER_NAME}"
    log_success "kubectl context set to '${CLUSTER_NAME}'"
}

# Deploy SPIRE using helmfile
deploy_spire_helmfile() {
    log_info "Deploying SPIRE using helmfile..."

    cd "${INFRA_DIR}"

    # Add SPIRE Helm repository
    log_info "Adding SPIRE Helm repository..."
    helm repo add spiffe https://spiffe.github.io/helm-charts-hardened/ || true
    helm repo update

    # Check if secrets file exists, create from template if not
    if [ ! -f "${INFRA_DIR}/values-minikube-secrets.yaml" ]; then
        log_warn "Secrets file not found, creating from template..."
        cp "${INFRA_DIR}/values-minikube-secrets.yaml.template" \
           "${INFRA_DIR}/values-minikube-secrets.yaml" 2>/dev/null || true

        # Generate bootstrap token
        BOOTSTRAP_TOKEN=$(openssl rand -base64 32)
        if [ -f "${INFRA_DIR}/values-minikube-secrets.yaml" ]; then
            sed -i.bak \
                "s/REPLACE_WITH_BOOTSTRAP_TOKEN_OR_USE_SOPS/${BOOTSTRAP_TOKEN}/g" \
                "${INFRA_DIR}/values-minikube-secrets.yaml"
            sed -i.bak \
                "s/REPLACE_WITH_JOIN_TOKEN_OR_USE_SOPS/${BOOTSTRAP_TOKEN}/g" \
                "${INFRA_DIR}/values-minikube-secrets.yaml"
            rm -f "${INFRA_DIR}/values-minikube-secrets.yaml.bak"
            log_success "Generated bootstrap token"
        fi
    fi

    # Deploy with helmfile
    if command -v helmfile &>/dev/null; then
        log_info "Using helmfile for deployment..."
        helmfile -e dev apply --skip-diff-on-install
    else
        log_warn "helmfile not found, using direct helm commands..."
        deploy_spire_helm
        return
    fi

    log_success "SPIRE deployed successfully"
}

# Deploy SPIRE using direct helm commands (fallback)
deploy_spire_helm() {
    log_info "Deploying SPIRE using helm..."

    cd "${INFRA_DIR}"

    # Create namespace
    kubectl create namespace spire-system --dry-run=client -o yaml | kubectl apply -f -

    # Deploy SPIRE server
    log_info "Deploying SPIRE server..."
    helm upgrade --install spire-server spiffe/spire-server \
        --namespace spire-system \
        --values values-minikube.yaml \
        --values values-minikube-secrets.yaml \
        --wait \
        --timeout 5m

    # Deploy SPIRE agent
    log_info "Deploying SPIRE agent..."
    helm upgrade --install spire-agent spiffe/spire-agent \
        --namespace spire-system \
        --values values-minikube.yaml \
        --values values-minikube-secrets.yaml \
        --wait \
        --timeout 5m

    log_success "SPIRE deployed successfully"
}

# Wait for deployments to be ready
wait_for_ready() {
    log_info "Waiting for SPIRE deployments to be ready..."

    "${SCRIPT_DIR}/wait-ready.sh"

    log_success "All SPIRE components are ready"
}

# Verify deployment
verify_deployment() {
    log_info "Verifying SPIRE deployment..."

    # Check pods
    log_info "Checking pods in spire-system namespace..."
    kubectl get pods -n spire-system

    # Check services
    log_info "Checking services..."
    kubectl get svc -n spire-system

    # Check SPIRE server health
    log_info "Checking SPIRE server health..."
    if kubectl exec -n spire-system deployment/spire-server -- \
        /opt/spire/bin/spire-server healthcheck 2>/dev/null; then
        log_success "SPIRE server is healthy"
    else
        log_warn "SPIRE server health check failed (may still be starting)"
    fi

    # Get NodePort for access
    local node_port
    node_port=$(kubectl get svc -n spire-system spire-server -o jsonpath='{.spec.ports[0].nodePort}' 2>/dev/null || echo "N/A")

    if [ "${node_port}" != "N/A" ]; then
        local minikube_ip
        minikube_ip=$(minikube ip -p "${CLUSTER_NAME}")
        log_success "SPIRE server accessible at: ${minikube_ip}:${node_port}"
    fi

    # Display socket path
    log_info "SPIRE agent socket: /tmp/spire-agent/public/api.sock"

    log_success "Verification complete"
}

# Display cluster info
display_info() {
    log_info "Cluster Information:"
    echo ""
    echo "  Cluster: ${CLUSTER_NAME}"
    echo "  Context: $(kubectl config current-context)"
    echo "  API Server: $(kubectl cluster-info | grep 'Kubernetes control plane' | awk '{print $NF}')"
    echo ""
    log_info "Useful commands:"
    echo "  kubectl get pods -n spire-system"
    echo "  kubectl logs -n spire-system spire-server-0 -c spire-server"
    echo "  kubectl logs -n spire-system daemonset/spire-agent"
    echo "  minikube dashboard -p ${CLUSTER_NAME}"
    echo "  minikube service list -p ${CLUSTER_NAME}"
    echo ""
}

# Show usage
show_usage() {
    cat <<EOF
Usage: $(basename "$0") [COMMAND]

Commands:
    start       Start Minikube and deploy SPIRE (default)
    verify      Verify existing deployment
    help        Show this help message

Environment Variables:
    CLUSTER_NAME        Minikube profile name (default: minikube)
    MINIKUBE_CPUS       CPU count (default: 2)
    MINIKUBE_MEMORY     Memory allocation (default: 4g)
    MINIKUBE_DISK       Disk size (default: 20g)
    MINIKUBE_DRIVER     VM driver (default: docker)
    K8S_VERSION         Kubernetes version (default: v1.28.0)

Examples:
    # Start with defaults
    ./cluster-up.sh

    # Start with custom resources
    MINIKUBE_CPUS=4 MINIKUBE_MEMORY=8g ./cluster-up.sh

    # Verify deployment
    ./cluster-up.sh verify

EOF
}

# Main execution
main() {
    local command="${1:-start}"

    case "${command}" in
        start)
            check_prerequisites
            start_minikube
            deploy_spire_helmfile
            wait_for_ready
            verify_deployment
            display_info
            ;;
        verify)
            check_prerequisites
            verify_deployment
            ;;
        help|--help|-h)
            show_usage
            ;;
        *)
            log_error "Unknown command: ${command}"
            show_usage
            exit 1
            ;;
    esac
}

# Run main function
main "$@"
