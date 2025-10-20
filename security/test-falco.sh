#!/bin/bash
# Falco Testing Script
# =====================
#
# This script tests the custom Falco rules for SPIRE mTLS monitoring
# by triggering various security events and checking for alerts.
#
# Usage:
#   bash security/test-falco.sh
#
# Note: Run this AFTER Falco is installed and running

set -euo pipefail

# Colors
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m'

echo -e "${GREEN}========================================${NC}"
echo -e "${GREEN}Falco Rules Testing for SPIRE mTLS${NC}"
echo -e "${GREEN}========================================${NC}"
echo ""

# Check if Falco is running
echo -e "${GREEN}[1/6] Checking Falco status...${NC}"
if systemctl is-active --quiet falco-modern-bpf.service; then
    echo "✓ Falco is running (modern eBPF driver)"
    FALCO_SERVICE="falco-modern-bpf.service"
elif systemctl is-active --quiet falco-bpf.service; then
    echo "✓ Falco is running (eBPF driver)"
    FALCO_SERVICE="falco-bpf.service"
elif systemctl is-active --quiet falco.service; then
    echo "✓ Falco is running (kernel module)"
    FALCO_SERVICE="falco.service"
else
    echo -e "${RED}✗ Falco is not running${NC}"
    echo "Start Falco with: sudo systemctl start falco-modern-bpf.service"
    exit 1
fi

# Check custom rules
echo ""
echo -e "${GREEN}[2/6] Checking custom rules...${NC}"
RULE_COUNT=$(grep -c "^- rule:" /etc/falco/falco_rules.local.yaml 2>/dev/null || echo "0")
if [[ $RULE_COUNT -gt 0 ]]; then
    echo "✓ Custom rules loaded: $RULE_COUNT rules"
else
    echo -e "${YELLOW}⚠ No custom rules found${NC}"
    echo "Install with: sudo cp security/falco_rules.yaml /etc/falco/falco_rules.local.yaml"
    echo "Then restart: sudo systemctl restart ${FALCO_SERVICE}"
fi

# Test 1: SPIRE socket access (if socket exists)
echo ""
echo -e "${GREEN}[3/6] Test 1: Unauthorized SPIRE Socket Access${NC}"
if [[ -S /tmp/spire-agent/public/api.sock ]]; then
    echo "Triggering alert by accessing SPIRE socket..."
    timeout 1 cat /tmp/spire-agent/public/api.sock 2>/dev/null || true
    echo "✓ Test command executed"
    echo "Expected alert: 'Unauthorized Access to SPIRE Socket' (CRITICAL)"
else
    echo -e "${YELLOW}⚠ SPIRE socket not found at /tmp/spire-agent/public/api.sock${NC}"
    echo "Deploy SPIRE with: make minikube-up"
fi

# Test 2: Certificate file read
echo ""
echo -e "${GREEN}[4/6] Test 2: Certificate File Access${NC}"
if [[ -f /etc/ssl/certs/ca-certificates.crt ]]; then
    echo "Triggering alert by reading certificate file..."
    head -1 /etc/ssl/certs/ca-certificates.crt > /dev/null
    echo "✓ Test command executed"
    echo "Expected alert: May trigger certificate monitoring rules (INFO)"
else
    echo -e "${YELLOW}⚠ Test certificate file not found${NC}"
fi

# Test 3: Check if running in container
echo ""
echo -e "${GREEN}[5/6] Test 3: Container Detection${NC}"
if [[ -f /.dockerenv ]] || grep -q docker /proc/1/cgroup 2>/dev/null; then
    echo "✓ Running in container - container rules will be active"
    echo "Deploy mTLS server with: kubectl apply -f examples/mtls-server.yaml"
else
    echo "ℹ Not running in container - some rules won't trigger"
    echo "Container-specific rules need Kubernetes deployment"
fi

# Test 4: View recent alerts
echo ""
echo -e "${GREEN}[6/6] Recent Falco Alerts (last 2 minutes)${NC}"
echo "Command: sudo journalctl -u ${FALCO_SERVICE} --since '2 minutes ago' | grep -i 'priority\|rule'"
echo ""
echo -e "${YELLOW}Run this command to see alerts:${NC}"
echo "  sudo journalctl -u ${FALCO_SERVICE} --since '2 minutes ago' -o cat | grep -E 'Priority:|Rule:|Output:'"
echo ""

# Summary
echo ""
echo -e "${GREEN}========================================${NC}"
echo -e "${GREEN}Testing Summary${NC}"
echo -e "${GREEN}========================================${NC}"
echo ""
echo "Falco Status:"
echo "  Service: ${FALCO_SERVICE}"
echo "  Custom Rules: ${RULE_COUNT}"
echo ""
echo "Available Test Commands:"
echo ""
echo "1. View live alerts:"
echo "   sudo journalctl -u ${FALCO_SERVICE} -f"
echo ""
echo "2. View JSON logs:"
echo "   tail -f /var/log/falco.log | jq ."
echo ""
echo "3. Trigger SPIRE socket alert:"
echo "   cat /tmp/spire-agent/public/api.sock"
echo ""
echo "4. Deploy and monitor mTLS server:"
echo "   kubectl apply -f examples/mtls-server.yaml"
echo "   sudo journalctl -u ${FALCO_SERVICE} -f"
echo ""
echo "5. Test shell in container:"
echo "   kubectl exec -it <mtls-server-pod> -- bash"
echo ""
echo "6. Search for specific alerts:"
echo "   sudo journalctl -u ${FALCO_SERVICE} | grep 'CRITICAL'"
echo ""
echo "For complete guide, see: security/FALCO_GUIDE.md"
echo ""

exit 0
