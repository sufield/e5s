#!/usr/bin/env bash
# wait-ready.sh - Wait for SPIRE deployments to be ready
# Usage: ./wait-ready.sh [timeout]

set -euo pipefail

# Color codes
RED='\033[0;31m'
GREEN='\033[0;32m'
BLUE='\033[0;34m'
NC='\033[0m'

# Configuration
NAMESPACE="${SPIRE_NAMESPACE:-spire-system}"
TIMEOUT="${1:-300}"  # Default 5 minutes
CHECK_INTERVAL=5

# Logging
log_info() {
    echo -e "${BLUE}→${NC} $*"
}

log_success() {
    echo -e "${GREEN}✓${NC} $*"
}

log_error() {
    echo -e "${RED}✗${NC} $*" >&2
}

# Wait for namespace
wait_for_namespace() {
    log_info "Waiting for namespace '${NAMESPACE}'..."

    local elapsed=0
    while ! kubectl get namespace "${NAMESPACE}" &>/dev/null; do
        if [ $elapsed -ge $TIMEOUT ]; then
            log_error "Timeout waiting for namespace"
            return 1
        fi

        sleep 2
        elapsed=$((elapsed + 2))
    done

    log_success "Namespace '${NAMESPACE}' exists"
}

# Wait for deployment
wait_for_deployment() {
    local name="$1"
    local timeout="${2:-$TIMEOUT}"

    log_info "Waiting for deployment '${name}' to be ready..."

    if ! kubectl wait --for=condition=available \
        --timeout="${timeout}s" \
        -n "${NAMESPACE}" \
        "deployment/${name}" 2>/dev/null; then

        log_error "Timeout waiting for deployment '${name}'"

        # Show pod status for debugging
        log_info "Pod status:"
        kubectl get pods -n "${NAMESPACE}" -l "app.kubernetes.io/name=${name}" || true

        log_info "Pod logs:"
        kubectl logs -n "${NAMESPACE}" -l "app.kubernetes.io/name=${name}" --tail=50 || true

        return 1
    fi

    log_success "Deployment '${name}' is ready"
}

# Wait for daemonset
wait_for_daemonset() {
    local name="$1"
    local timeout="${2:-$TIMEOUT}"

    log_info "Waiting for daemonset '${name}' to be ready..."

    local elapsed=0
    while true; do
        local desired
        local ready

        desired=$(kubectl get daemonset -n "${NAMESPACE}" "${name}" \
            -o jsonpath='{.status.desiredNumberScheduled}' 2>/dev/null || echo "0")
        ready=$(kubectl get daemonset -n "${NAMESPACE}" "${name}" \
            -o jsonpath='{.status.numberReady}' 2>/dev/null || echo "0")

        if [ "${desired}" -gt 0 ] && [ "${ready}" -eq "${desired}" ]; then
            log_success "Daemonset '${name}' is ready (${ready}/${desired})"
            return 0
        fi

        if [ $elapsed -ge $timeout ]; then
            log_error "Timeout waiting for daemonset '${name}' (${ready}/${desired})"

            # Show pod status
            log_info "Pod status:"
            kubectl get pods -n "${NAMESPACE}" -l "app.kubernetes.io/name=${name}" || true

            log_info "Pod logs:"
            kubectl logs -n "${NAMESPACE}" -l "app.kubernetes.io/name=${name}" --tail=50 || true

            return 1
        fi

        sleep $CHECK_INTERVAL
        elapsed=$((elapsed + CHECK_INTERVAL))
    done
}

# Wait for pods to be running
wait_for_pods() {
    log_info "Waiting for all pods in '${NAMESPACE}' to be running..."

    if ! kubectl wait --for=condition=ready \
        --timeout="${TIMEOUT}s" \
        -n "${NAMESPACE}" \
        --all pods 2>/dev/null; then

        log_error "Timeout waiting for pods"

        log_info "Current pod status:"
        kubectl get pods -n "${NAMESPACE}" || true

        return 1
    fi

    log_success "All pods are running"
}

# Check SPIRE server health
check_spire_server_health() {
    log_info "Checking SPIRE server health..."

    local elapsed=0
    while true; do
        if kubectl exec -n "${NAMESPACE}" deployment/spire-server -- \
            /opt/spire/bin/spire-server healthcheck 2>/dev/null; then
            log_success "SPIRE server is healthy"
            return 0
        fi

        if [ $elapsed -ge $TIMEOUT ]; then
            log_error "Timeout waiting for SPIRE server health check"
            return 1
        fi

        sleep $CHECK_INTERVAL
        elapsed=$((elapsed + CHECK_INTERVAL))
    done
}

# Check SPIRE agent health
check_spire_agent_health() {
    log_info "Checking SPIRE agent health..."

    # Get first agent pod
    local pod
    pod=$(kubectl get pods -n "${NAMESPACE}" -l "app.kubernetes.io/name=spire-agent" \
        -o jsonpath='{.items[0].metadata.name}' 2>/dev/null || echo "")

    if [ -z "${pod}" ]; then
        log_error "No SPIRE agent pods found"
        return 1
    fi

    local elapsed=0
    while true; do
        if kubectl exec -n "${NAMESPACE}" "${pod}" -- \
            /opt/spire/bin/spire-agent healthcheck \
            -socketPath /tmp/spire-agent/public/api.sock 2>/dev/null; then
            log_success "SPIRE agent is healthy"
            return 0
        fi

        if [ $elapsed -ge $TIMEOUT ]; then
            log_error "Timeout waiting for SPIRE agent health check"
            return 1
        fi

        sleep $CHECK_INTERVAL
        elapsed=$((elapsed + CHECK_INTERVAL))
    done
}

# Main execution
main() {
    log_info "Waiting for SPIRE components to be ready (timeout: ${TIMEOUT}s)..."
    echo ""

    # Wait for namespace
    if ! wait_for_namespace; then
        exit 1
    fi

    # Wait for SPIRE server deployment
    if kubectl get deployment -n "${NAMESPACE}" spire-server &>/dev/null; then
        if ! wait_for_deployment "spire-server" "$TIMEOUT"; then
            exit 1
        fi
    else
        log_info "SPIRE server deployment not found, skipping..."
    fi

    # Wait for SPIRE agent daemonset
    if kubectl get daemonset -n "${NAMESPACE}" spire-agent &>/dev/null; then
        if ! wait_for_daemonset "spire-agent" "$TIMEOUT"; then
            exit 1
        fi
    else
        log_info "SPIRE agent daemonset not found, skipping..."
    fi

    # Wait for all pods
    if ! wait_for_pods; then
        exit 1
    fi

    # Health checks
    if kubectl get deployment -n "${NAMESPACE}" spire-server &>/dev/null; then
        if ! check_spire_server_health; then
            log_error "SPIRE server health check failed"
            exit 1
        fi
    fi

    if kubectl get daemonset -n "${NAMESPACE}" spire-agent &>/dev/null; then
        if ! check_spire_agent_health; then
            log_error "SPIRE agent health check failed"
            exit 1
        fi
    fi

    echo ""
    log_success "All SPIRE components are ready and healthy!"
    echo ""

    # Display final status
    log_info "Final status:"
    kubectl get pods -n "${NAMESPACE}"
}

# Run main
main "$@"
