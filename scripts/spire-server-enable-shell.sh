#!/usr/bin/env bash
# Switch SPIRE server to non-distroless image for CLI access
# This enables using spire-server CLI commands for registration
#
# Usage: ./scripts/spire-server-enable-shell.sh [enable|disable]

set -Eeuo pipefail

NS="${NS:-spire-system}"
ACTION="${1:-enable}"

# Colors
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

success() { echo -e "${GREEN}✅ $1${NC}"; }
error() { echo -e "${RED}❌ $1${NC}"; }
info() { echo -e "${YELLOW}ℹ️  $1${NC}"; }
header() {
    echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
    echo -e "${BLUE}  $1${NC}"
    echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
}

header "SPIRE Server Image Manager"
echo ""

case "$ACTION" in
    enable|shell|non-distroless)
        info "Switching SPIRE server to non-distroless image..."
        echo ""
        echo "This enables:"
        echo "  • spire-server CLI access via kubectl exec"
        echo "  • Registration entry management"
        echo "  • Debugging and troubleshooting"
        echo ""
        info "Updating image to ghcr.io/spiffe/spire-server:1.9.0..."

        # Try both StatefulSet and Deployment
        if kubectl get statefulset spire-server -n "$NS" >/dev/null 2>&1; then
            kubectl set image statefulset/spire-server -n "$NS" \
                spire-server=ghcr.io/spiffe/spire-server:1.9.0

            success "Image updated on StatefulSet"
            echo ""
            info "Waiting for rollout to complete..."
            kubectl rollout status statefulset/spire-server -n "$NS" --timeout=120s

        elif kubectl get deployment spire-server -n "$NS" >/dev/null 2>&1; then
            kubectl set image deployment/spire-server -n "$NS" \
                spire-server=ghcr.io/spiffe/spire-server:1.9.0

            success "Image updated on Deployment"
            echo ""
            info "Waiting for rollout to complete..."
            kubectl rollout status deployment/spire-server -n "$NS" --timeout=120s
        else
            error "SPIRE server StatefulSet/Deployment not found in namespace $NS"
            exit 1
        fi

        echo ""
        success "SPIRE server is now using non-distroless image"
        echo ""
        info "You can now:"
        echo "  • Run registration setup: ./scripts/setup-spire-registrations.sh"
        echo "  • Exec into pod: kubectl exec -it spire-server-0 -n $NS -- /bin/sh"
        echo "  • Use CLI: kubectl exec spire-server-0 -n $NS -- spire-server entry list"
        echo ""
        ;;

    disable|distroless|secure)
        info "Switching SPIRE server back to distroless image..."
        echo ""
        echo "This provides:"
        echo "  • Minimal attack surface"
        echo "  • Production-ready security"
        echo "  • Smaller image size"
        echo ""
        info "Updating image to ghcr.io/spiffe/spire-server:1.9.0-distroless..."

        # Try both StatefulSet and Deployment
        if kubectl get statefulset spire-server -n "$NS" >/dev/null 2>&1; then
            kubectl set image statefulset/spire-server -n "$NS" \
                spire-server=ghcr.io/spiffe/spire-server:1.9.0-distroless

            success "Image updated on StatefulSet"
            echo ""
            info "Waiting for rollout to complete..."
            kubectl rollout status statefulset/spire-server -n "$NS" --timeout=120s

        elif kubectl get deployment spire-server -n "$NS" >/dev/null 2>&1; then
            kubectl set image deployment/spire-server -n "$NS" \
                spire-server=ghcr.io/spiffe/spire-server:1.9.0-distroless

            success "Image updated on Deployment"
            echo ""
            info "Waiting for rollout to complete..."
            kubectl rollout status deployment/spire-server -n "$NS" --timeout=120s
        else
            error "SPIRE server StatefulSet/Deployment not found in namespace $NS"
            exit 1
        fi

        echo ""
        success "SPIRE server is now using distroless image"
        echo ""
        info "Note: CLI access is no longer available"
        echo "  • Use SPIRE Server API for registration (if enabled)"
        echo "  • Or use SPIRE Controller Manager with CRDs"
        echo ""
        ;;

    status|check)
        info "Checking current SPIRE server image..."
        echo ""

        if kubectl get statefulset spire-server -n "$NS" >/dev/null 2>&1; then
            IMAGE=$(kubectl get statefulset spire-server -n "$NS" \
                -o jsonpath='{.spec.template.spec.containers[?(@.name=="spire-server")].image}')
            RESOURCE_TYPE="StatefulSet"
        elif kubectl get deployment spire-server -n "$NS" >/dev/null 2>&1; then
            IMAGE=$(kubectl get deployment spire-server -n "$NS" \
                -o jsonpath='{.spec.template.spec.containers[?(@.name=="spire-server")].image}')
            RESOURCE_TYPE="Deployment"
        else
            error "SPIRE server not found in namespace $NS"
            exit 1
        fi

        echo "Resource: $RESOURCE_TYPE/spire-server"
        echo "Image: $IMAGE"
        echo ""

        if echo "$IMAGE" | grep -q "distroless"; then
            info "Type: Distroless (minimal, no shell access)"
            echo "  ✓ Production-ready"
            echo "  ✗ No CLI access for registration"
            echo ""
            info "To enable CLI access: $0 enable"
        else
            info "Type: Non-distroless (includes shell and utilities)"
            echo "  ✓ CLI access available"
            echo "  ⚠ Not recommended for production"
            echo ""
            info "To switch to distroless: $0 disable"
        fi
        echo ""
        ;;

    *)
        error "Unknown action: $ACTION"
        echo ""
        echo "Usage: $0 [enable|disable|status]"
        echo ""
        echo "Actions:"
        echo "  enable, shell, non-distroless"
        echo "    Switch to non-distroless image (enables CLI access)"
        echo ""
        echo "  disable, distroless, secure"
        echo "    Switch to distroless image (production-ready)"
        echo ""
        echo "  status, check"
        echo "    Check current image type"
        echo ""
        exit 1
        ;;
esac
