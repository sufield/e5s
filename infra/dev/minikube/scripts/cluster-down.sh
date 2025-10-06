#!/usr/bin/env bash
# cluster-down.sh - Stop Minikube cluster and cleanup SPIRE
# Usage: ./cluster-down.sh [stop|delete|help]

set -euo pipefail

# Color codes
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

# Script directory
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
INFRA_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"

# Configuration
CLUSTER_NAME="${CLUSTER_NAME:-minikube}"

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

# Check if cluster exists
check_cluster() {
    if ! minikube status -p "${CLUSTER_NAME}" &>/dev/null; then
        log_error "Cluster '${CLUSTER_NAME}' does not exist"
        return 1
    fi
    return 0
}

# Destroy SPIRE deployment
destroy_spire() {
    log_info "Destroying SPIRE deployment..."

    cd "${INFRA_DIR}"

    if command -v helmfile &>/dev/null; then
        log_info "Using helmfile to destroy..."
        if helmfile -e dev list &>/dev/null; then
            helmfile -e dev destroy || log_warn "helmfile destroy had warnings"
        else
            log_warn "No helmfile releases found"
        fi
    else
        log_info "Using helm to uninstall..."
        helm uninstall spire-agent -n spire-system 2>/dev/null || log_warn "spire-agent not found"
        helm uninstall spire-server -n spire-system 2>/dev/null || log_warn "spire-server not found"
    fi

    # Delete namespace
    log_info "Deleting spire-system namespace..."
    kubectl delete namespace spire-system --wait=true --timeout=60s 2>/dev/null || \
        log_warn "spire-system namespace not found or already deleted"

    log_success "SPIRE deployment destroyed"
}

# Stop Minikube cluster
stop_cluster() {
    log_info "Stopping Minikube cluster '${CLUSTER_NAME}'..."

    if ! check_cluster; then
        log_warn "Cluster does not exist, nothing to stop"
        return 0
    fi

    # Destroy SPIRE first
    if kubectl get namespace spire-system &>/dev/null; then
        destroy_spire
    else
        log_info "SPIRE not deployed, skipping destroy"
    fi

    # Stop cluster
    minikube stop -p "${CLUSTER_NAME}"
    log_success "Cluster '${CLUSTER_NAME}' stopped"
}

# Delete Minikube cluster
delete_cluster() {
    log_info "Deleting Minikube cluster '${CLUSTER_NAME}'..."

    if ! check_cluster; then
        log_warn "Cluster does not exist, nothing to delete"
        return 0
    fi

    # Confirm deletion
    if [ -t 0 ]; then  # Interactive terminal
        read -p "Are you sure you want to DELETE cluster '${CLUSTER_NAME}'? (yes/no): " -r
        echo
        if [[ ! $REPLY =~ ^[Yy][Ee][Ss]$ ]]; then
            log_info "Deletion cancelled"
            return 0
        fi
    fi

    # Destroy SPIRE first
    if kubectl get namespace spire-system &>/dev/null 2>&1; then
        destroy_spire
    fi

    # Delete cluster
    minikube delete -p "${CLUSTER_NAME}"
    log_success "Cluster '${CLUSTER_NAME}' deleted"

    # Cleanup kubectl context
    kubectl config delete-context "${CLUSTER_NAME}" 2>/dev/null || true
    kubectl config delete-cluster "${CLUSTER_NAME}" 2>/dev/null || true
    log_success "kubectl context cleaned up"
}

# Cleanup temporary files
cleanup_temp_files() {
    log_info "Cleaning up temporary files..."

    # Remove generated secrets (keep template)
    if [ -f "${INFRA_DIR}/values-minikube-secrets.yaml" ]; then
        if grep -q "REPLACE_WITH" "${INFRA_DIR}/values-minikube-secrets.yaml"; then
            log_info "Keeping template secrets file"
        else
            log_warn "Removing generated secrets file (backup saved as .bak)"
            cp "${INFRA_DIR}/values-minikube-secrets.yaml" \
               "${INFRA_DIR}/values-minikube-secrets.yaml.bak"
            # Reset to template
            sed -i.tmp \
                's/value: "[^"]*"/value: "REPLACE_WITH_BOOTSTRAP_TOKEN_OR_USE_SOPS"/g' \
                "${INFRA_DIR}/values-minikube-secrets.yaml"
            rm -f "${INFRA_DIR}/values-minikube-secrets.yaml.tmp"
        fi
    fi

    # Remove helmfile cache
    rm -rf "${INFRA_DIR}/.helmfile" 2>/dev/null || true

    log_success "Temporary files cleaned up"
}

# Show status
show_status() {
    log_info "Cluster Status:"
    echo ""

    if ! check_cluster; then
        echo "  Cluster: Not found"
        return
    fi

    minikube status -p "${CLUSTER_NAME}" || true
    echo ""

    if kubectl get namespace spire-system &>/dev/null 2>&1; then
        log_info "SPIRE Deployment Status:"
        kubectl get pods -n spire-system 2>/dev/null || log_warn "No pods found"
    else
        echo "  SPIRE: Not deployed"
    fi
}

# Show usage
show_usage() {
    cat <<EOF
Usage: $(basename "$0") [COMMAND]

Commands:
    stop        Stop cluster but keep data (default)
    delete      Delete cluster and all data
    destroy     Destroy SPIRE deployment only
    clean       Cleanup temporary files
    status      Show cluster status
    help        Show this help message

Environment Variables:
    CLUSTER_NAME    Minikube profile name (default: minikube)

Examples:
    # Stop cluster (can be restarted)
    ./cluster-down.sh stop

    # Delete cluster completely
    ./cluster-down.sh delete

    # Destroy only SPIRE deployment
    ./cluster-down.sh destroy

    # Show status
    ./cluster-down.sh status

EOF
}

# Main execution
main() {
    local command="${1:-stop}"

    case "${command}" in
        stop)
            stop_cluster
            show_status
            ;;
        delete)
            delete_cluster
            cleanup_temp_files
            ;;
        destroy)
            destroy_spire
            ;;
        clean)
            cleanup_temp_files
            ;;
        status)
            show_status
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

# Run main
main "$@"
