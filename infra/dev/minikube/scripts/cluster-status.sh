#!/usr/bin/env bash
# cluster-status.sh - Show Minikube cluster and SPIRE status
# Usage: ./cluster-status.sh

set -euo pipefail

# Script directory
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# Delegate to cluster-down.sh status command
# (cluster-down.sh implements the status functionality)
exec "${SCRIPT_DIR}/cluster-down.sh" status
