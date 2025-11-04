#!/bin/bash
# Falco Security Monitoring Setup for SPIRE mTLS
# ===============================================
#
# ⚠️  DEPRECATION NOTICE ⚠️
# -------------------------
# This script is for BARE-METAL / DEVELOPMENT WORKSTATION installations only.
#
# For Kubernetes/Minikube deployments, use the recommended Helm-based approach:
#   ENABLE_FALCO=true helmfile -e dev apply
#
# Documentation:
#   - Helm deployment: examples/minikube-lowlevel/infra/README.md
#   - Complete guide: security/FALCO_GUIDE.md
#   - Troubleshooting: security/BARE_METAL_TROUBLESHOOTING.md
#
# -------------------------
#
# This script performs a complete installation and configuration of Falco
# with custom SPIRE mTLS security rules.
#
# What it does:
# 1. Installs Falco if not already installed
# 2. Configures JSON output and file logging
# 3. Installs 18 custom SPIRE mTLS security rules
# 4. Tests and starts Falco service
#
# Usage:
#   sudo bash security/setup-falco.sh
#
# Requirements:
#   - Ubuntu 24.04 or compatible
#   - Kernel 6.8+ (for modern eBPF support)
#   - Root privileges

set -euo pipefail

GREEN='\033[0;32m'
RED='\033[0;31m'
BLUE='\033[0;34m'
YELLOW='\033[1;33m'
NC='\033[0m'

# Check root
if [[ $EUID -ne 0 ]]; then
   echo -e "${RED}Error: This script must be run as root${NC}"
   exit 1
fi

echo -e "${GREEN}╔════════════════════════════════════════════════╗${NC}"
echo -e "${GREEN}║  Falco Security Monitoring Setup for SPIRE   ║${NC}"
echo -e "${GREEN}╚════════════════════════════════════════════════╝${NC}"
echo ""

SCRIPT_DIR="/home/zepho/work/pocket/hexagon/spire/security"
RULES_SOURCE="$SCRIPT_DIR/falco_rules.yaml"
RULES_DEST="/etc/falco/falco_rules.local.yaml"
FALCO_CONFIG="/etc/falco/falco.yaml"

# ============================================================================
# Step 1: Install Falco
# ============================================================================
echo -e "${BLUE}[1/6] Checking Falco installation...${NC}"

if ! command -v falco &> /dev/null; then
    echo "  Falco not found. Installing..."

    # Add Falco repository
    curl -fsSL https://falco.org/repo/falcosecurity-packages.asc | \
        gpg --dearmor -o /usr/share/keyrings/falco-archive-keyring.gpg

    echo "deb [signed-by=/usr/share/keyrings/falco-archive-keyring.gpg] https://download.falco.org/packages/deb stable main" | \
        tee /etc/apt/sources.list.d/falcosecurity.list

    apt-get update -qq
    apt-get install -y falco

    echo "  ✓ Falco installed"
else
    FALCO_VERSION=$(falco --version 2>&1 | head -1 | awk '{print $3}')
    echo "  ✓ Falco already installed (version $FALCO_VERSION)"
fi

# ============================================================================
# Step 2: Configure Falco
# ============================================================================
echo ""
echo -e "${BLUE}[2/6] Configuring Falco...${NC}"

# Backup original config
if [[ ! -f "$FALCO_CONFIG.original" ]]; then
    cp "$FALCO_CONFIG" "$FALCO_CONFIG.original"
    echo "  ✓ Original config backed up"
fi

# Enable JSON output
if grep -q "^json_output: false" "$FALCO_CONFIG"; then
    sed -i 's/^json_output: false/json_output: true/' "$FALCO_CONFIG"
    echo "  ✓ JSON output enabled"
elif ! grep -q "^json_output:" "$FALCO_CONFIG"; then
    echo "json_output: true" >> "$FALCO_CONFIG"
    echo "  ✓ JSON output enabled"
else
    echo "  ✓ JSON output already enabled"
fi

# Configure file output
if grep -A 3 "^file_output:" "$FALCO_CONFIG" | grep -q "enabled:"; then
    # Check if enabled
    if grep -A 3 "^file_output:" "$FALCO_CONFIG" | grep -q "enabled: false"; then
        sed -i '/^file_output:/,/^[^ ]/ s/enabled: false/enabled: true/' "$FALCO_CONFIG"
        echo "  ✓ File output enabled"
    else
        echo "  ✓ File output already enabled"
    fi
else
    # Add enabled: true
    sed -i '/^file_output:/a\  enabled: true' "$FALCO_CONFIG"
    echo "  ✓ File output enabled"
fi

# Verify file_output has filename
if ! grep -A 5 "^file_output:" "$FALCO_CONFIG" | grep -q "filename:"; then
    sed -i '/^file_output:/a\  filename: /var/log/falco.log' "$FALCO_CONFIG"
    echo "  ✓ File output path configured"
fi

echo "  ✓ Falco configuration complete"

# ============================================================================
# Step 3: Install Custom Rules
# ============================================================================
echo ""
echo -e "${BLUE}[3/6] Installing custom SPIRE mTLS rules...${NC}"

if [[ ! -f "$RULES_SOURCE" ]]; then
    echo -e "${RED}  ✗ Rules file not found: $RULES_SOURCE${NC}"
    exit 1
fi

# Backup existing rules
if [[ -f "$RULES_DEST" ]]; then
    BACKUP="$RULES_DEST.backup-$(date +%Y%m%d-%H%M%S)"
    cp "$RULES_DEST" "$BACKUP"
    echo "  ✓ Existing rules backed up to: $BACKUP"
fi

# Install fresh rules
rm -f "$RULES_DEST"
cat "$RULES_SOURCE" > "$RULES_DEST"
chmod 644 "$RULES_DEST"

RULE_COUNT=$(grep -c "^- rule:" "$RULES_DEST")
echo "  ✓ Installed $RULE_COUNT custom rules"

# ============================================================================
# Step 4: Validate Configuration
# ============================================================================
echo ""
echo -e "${BLUE}[4/6] Validating configuration...${NC}"

# Validate rules syntax
if falco --validate "$RULES_DEST" 2>&1 | grep -qi "error"; then
    echo -e "${RED}  ✗ Rules validation failed${NC}"
    falco --validate "$RULES_DEST" 2>&1
    exit 1
fi
echo "  ✓ Rules syntax valid"

# Validate main config
if falco --validate "$FALCO_CONFIG" 2>&1 | grep -qi "error"; then
    echo -e "${RED}  ✗ Config validation failed${NC}"
    falco --validate "$FALCO_CONFIG" 2>&1
    exit 1
fi
echo "  ✓ Main config valid"

# ============================================================================
# Step 5: Test Falco Startup
# ============================================================================
echo ""
echo -e "${BLUE}[5/6] Testing Falco startup and driver compatibility...${NC}"

TEST_LOG="/tmp/falco-setup-test.log"

# Test modern_ebpf
echo "  Testing modern_ebpf driver..."
timeout 3 falco -o engine.kind=modern_ebpf 2>&1 > "$TEST_LOG" &
FALCO_PID=$!
sleep 3

if ps -p $FALCO_PID > /dev/null 2>&1; then
    kill $FALCO_PID 2>/dev/null || true
    wait $FALCO_PID 2>/dev/null || true
    echo "  ✓ modern_ebpf driver works"
    DRIVER="modern_ebpf"
    SERVICE="falco-modern-bpf.service"
else
    wait $FALCO_PID 2>/dev/null || EXIT_CODE=$?
    if [[ $EXIT_CODE -eq 124 ]]; then
        echo "  ✓ modern_ebpf driver works"
        DRIVER="modern_ebpf"
        SERVICE="falco-modern-bpf.service"
    else
        # Check for compilation errors
        if grep -q "LOAD_ERR" "$TEST_LOG"; then
            echo -e "${RED}  ✗ Rules compilation failed${NC}"
            grep -A 10 "Error:" "$TEST_LOG" | head -20
            exit 1
        fi

        # Try ebpf
        echo "  Testing ebpf driver..."
        timeout 3 falco -o engine.kind=ebpf 2>&1 > "$TEST_LOG" &
        FALCO_PID=$!
        sleep 3

        if ps -p $FALCO_PID > /dev/null 2>&1; then
            kill $FALCO_PID 2>/dev/null || true
            wait $FALCO_PID 2>/dev/null || true
            echo "  ✓ ebpf driver works"
            DRIVER="ebpf"
            SERVICE="falco-bpf.service"
        else
            wait $FALCO_PID 2>/dev/null || EXIT_CODE=$?
            if [[ $EXIT_CODE -eq 124 ]]; then
                echo "  ✓ ebpf driver works"
                DRIVER="ebpf"
                SERVICE="falco-bpf.service"
            else
                echo -e "${RED}  ✗ All drivers failed${NC}"
                tail -30 "$TEST_LOG"
                exit 1
            fi
        fi
    fi
fi

# ============================================================================
# Step 6: Start Falco Service
# ============================================================================
echo ""
echo -e "${BLUE}[6/6] Starting Falco service...${NC}"

# Stop all Falco services
systemctl stop falco-modern-bpf.service 2>/dev/null || true
systemctl stop falco-bpf.service 2>/dev/null || true
systemctl stop falco-kmod.service 2>/dev/null || true
systemctl stop falco.service 2>/dev/null || true

# Enable and start the working service
systemctl enable "$SERVICE" 2>/dev/null || true
systemctl start "$SERVICE"
sleep 3

if systemctl is-active --quiet "$SERVICE"; then
    echo "  ✓ Service started successfully"
    echo ""
    echo -e "${GREEN}╔════════════════════════════════════════════════╗${NC}"
    echo -e "${GREEN}║         SUCCESS! Falco is Running!            ║${NC}"
    echo -e "${GREEN}╚════════════════════════════════════════════════╝${NC}"
    echo ""

    systemctl status "$SERVICE" --no-pager | head -15

    echo ""
    echo -e "${GREEN}Installation Summary:${NC}"
    echo "  ✓ Falco version: $(falco --version 2>&1 | head -1 | awk '{print $3}')"
    echo "  ✓ Custom rules: $RULE_COUNT SPIRE mTLS security rules"
    echo "  ✓ Driver: $DRIVER"
    echo "  ✓ Service: $SERVICE"
    echo "  ✓ Config: $FALCO_CONFIG"
    echo "  ✓ Rules: $RULES_DEST"
    echo "  ✓ Logs: /var/log/falco.log"
    echo "  ✓ JSON output: enabled"

    echo ""
    echo -e "${GREEN}Monitoring Coverage:${NC}"
    echo "  • SPIRE Workload API security"
    echo "  • mTLS server behavior"
    echo "  • Container security (escape attempts, privilege escalation)"
    echo "  • Certificate and TLS operations"
    echo "  • Go binary behavior"
    echo "  • Network anomalies"
    echo "  • Kubernetes resource access"

    echo ""
    echo -e "${GREEN}Next Steps:${NC}"
    echo ""
    echo "  1. View live alerts:"
    echo "     sudo journalctl -u $SERVICE -f"
    echo ""
    echo "  2. View JSON logs:"
    echo "     tail -f /var/log/falco.log | jq ."
    echo ""
    echo "  3. Test a rule (unauthorized socket access):"
    echo "     cat /tmp/spire-agent/public/api.sock 2>/dev/null || echo 'Socket not found'"
    echo ""
    echo "  4. Deploy SPIRE infrastructure:"
    echo "     make minikube-up"
    echo "     kubectl apply -f examples/mtls-server.yaml"
    echo ""
    echo "  5. Review documentation:"
    echo "     cat security/FALCO_GUIDE.md"
    echo ""

    exit 0
else
    echo -e "${RED}  ✗ Service failed to start${NC}"
    echo ""
    systemctl status "$SERVICE" --no-pager || true
    echo ""
    echo "Recent logs:"
    journalctl -u "$SERVICE" --since "1 minute ago" --no-pager | tail -30
    exit 1
fi
