#!/bin/bash
# Install required tools for e5s development on Ubuntu 24.04
# This script installs: Go 1.25.3, Docker, kubectl, Minikube, Helm

set -e

echo "Installing required tools for Ubuntu 24.04..."
echo ""

# Check OS
if [ ! -f /etc/os-release ]; then
    echo "Error: /etc/os-release not found. This script is for Ubuntu 24.04 only."
    exit 1
fi

if ! grep -q "Ubuntu" /etc/os-release || ! grep -q "24.04" /etc/os-release; then
    echo "Error: This script is designed for Ubuntu 24.04 only."
    echo "Current OS:"
    cat /etc/os-release | grep PRETTY_NAME
    exit 1
fi

echo "Detected Ubuntu 24.04 ✓"
echo ""

# Install Go 1.25.3
echo "Installing Go 1.25.3..."
if command -v go >/dev/null 2>&1; then
    CURRENT_GO=$(go version | awk '{print $3}' | sed 's/go//')
    echo "  Current Go version: $CURRENT_GO"
    if [ "$CURRENT_GO" != "1.25.3" ]; then
        echo "  Removing old Go version..."
        sudo rm -rf /usr/local/go
    else
        echo "  Go 1.25.3 already installed ✓"
    fi
fi

if ! command -v go >/dev/null 2>&1 || ! go version | grep -q "go1.25.3"; then
    echo "  Downloading Go 1.25.3..."
    cd /tmp && curl -LO https://go.dev/dl/go1.25.3.linux-amd64.tar.gz
    echo "  Installing to /usr/local/go..."
    sudo tar -C /usr/local -xzf go1.25.3.linux-amd64.tar.gz
    rm go1.25.3.linux-amd64.tar.gz
    if ! grep -q "/usr/local/go/bin" ~/.bashrc; then
        echo 'export PATH=$PATH:/usr/local/go/bin' >> ~/.bashrc
        echo "  Added Go to PATH in ~/.bashrc"
    fi
    export PATH=$PATH:/usr/local/go/bin
    /usr/local/go/bin/go version
    echo "  ✓ Go 1.25.3 installed"
fi
echo ""

# Install Docker
echo "Installing Docker..."
if command -v docker >/dev/null 2>&1; then
    echo "  Docker already installed: $(docker --version) ✓"
else
    echo "  Adding Docker repository..."
    sudo apt-get update -qq
    sudo apt-get install -y -qq ca-certificates curl
    sudo install -m 0755 -d /etc/apt/keyrings
    sudo curl -fsSL https://download.docker.com/linux/ubuntu/gpg -o /etc/apt/keyrings/docker.asc
    sudo chmod a+r /etc/apt/keyrings/docker.asc
    echo "deb [arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/docker.asc] https://download.docker.com/linux/ubuntu $(. /etc/os-release && echo $VERSION_CODENAME) stable" | \
        sudo tee /etc/apt/sources.list.d/docker.list > /dev/null
    echo "  Installing Docker..."
    sudo apt-get update -qq
    sudo apt-get install -y -qq docker-ce docker-ce-cli containerd.io docker-buildx-plugin docker-compose-plugin
    echo "  Adding user to docker group..."
    sudo usermod -aG docker $USER
    echo "  ✓ Docker installed (logout/login required for docker group)"
fi
echo ""

# Install kubectl
echo "Installing kubectl..."
if command -v kubectl >/dev/null 2>&1; then
    echo "  kubectl already installed: $(kubectl version --client --short 2>/dev/null || kubectl version --client) ✓"
else
    echo "  Downloading latest kubectl..."
    cd /tmp && curl -LO "https://dl.k8s.io/release/$(curl -L -s https://dl.k8s.io/release/stable.txt)/bin/linux/amd64/kubectl"
    sudo install -o root -g root -m 0755 kubectl /usr/local/bin/kubectl
    rm kubectl
    echo "  ✓ kubectl installed: $(kubectl version --client --short 2>/dev/null || kubectl version --client)"
fi
echo ""

# Install Minikube
echo "Installing Minikube..."
if command -v minikube >/dev/null 2>&1; then
    echo "  Minikube already installed: $(minikube version --short) ✓"
else
    echo "  Downloading latest Minikube..."
    cd /tmp && curl -LO https://storage.googleapis.com/minikube/releases/latest/minikube-linux-amd64
    sudo install minikube-linux-amd64 /usr/local/bin/minikube
    rm minikube-linux-amd64
    echo "  ✓ Minikube installed: $(minikube version --short)"
fi
echo ""

# Install Helm
echo "Installing Helm..."
if command -v helm >/dev/null 2>&1; then
    echo "  Helm already installed: $(helm version --short) ✓"
else
    echo "  Downloading and installing Helm..."
    # Pin to specific commit hash for security (OpenSSF Scorecard requirement)
    # Commit: 0ee89d2d4ee91d7edd21a9445f39f4eb0fed2973 from 2025-11-10
    cd /tmp && curl -fsSL https://raw.githubusercontent.com/helm/helm/0ee89d2d4ee91d7edd21a9445f39f4eb0fed2973/scripts/get-helm-3 | bash
    echo "  ✓ Helm installed: $(helm version --short)"
fi
echo ""

echo "======================================"
echo "✓ All tools installed successfully!"
echo "======================================"
echo ""
echo "IMPORTANT: If Docker was just installed, you need to:"
echo "  1. Logout and login again (for docker group to take effect)"
echo "  2. OR run: newgrp docker"
echo ""
echo "IMPORTANT: If Go was just installed, run:"
echo "  source ~/.bashrc"
echo "  OR start a new terminal session"
echo ""
echo "Verify installation with: make verify-tools"
