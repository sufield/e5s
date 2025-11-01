#!/bin/bash
set -e

# Quick prerelease testing script for e5s library developers
# This script automates the setup and deployment of test applications

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
TEST_DIR="$PROJECT_ROOT/test-demo"

echo "=== e5s Pre-Release Testing Setup ==="
echo ""

# Step 1: Create test directory structure
echo "1. Creating test directory structure..."
mkdir -p "$TEST_DIR"/{server,client,bin}
cd "$TEST_DIR"

# Step 2: Initialize Go module if needed
if [ ! -f go.mod ]; then
    echo "2. Initializing Go module..."
    go mod init test-demo
    go mod edit -replace github.com/sufield/e5s=..
    go get github.com/go-chi/chi/v5
    go get github.com/sufield/e5s
else
    echo "2. Go module already initialized"
fi

# Step 3: Create server code
echo "3. Creating server code..."
cat > server/main.go <<'EOF'
package main

import (
	"fmt"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/sufield/e5s"
)

func main() {
	r := chi.NewRouter()

	r.Get("/healthz", func(w http.ResponseWriter, req *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})

	r.Get("/hello", func(w http.ResponseWriter, req *http.Request) {
		id, ok := e5s.PeerID(req)
		if !ok {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		fmt.Fprintf(w, "Hello, %s!\n", id)
	})

	e5s.Run(r)
}
EOF

# Step 4: Create client code
echo "4. Creating client code..."
cat > client/main.go <<'EOF'
package main

import (
	"fmt"
	"io"
	"log"
	"os"

	"github.com/sufield/e5s"
)

func main() {
	serverURL := os.Getenv("SERVER_URL")
	if serverURL == "" {
		serverURL = "https://localhost:8443/hello"
	}

	resp, err := e5s.Get(serverURL)
	if err != nil {
		log.Fatal(err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	fmt.Println(string(body))
}
EOF

# Step 5: Create e5s.yaml
echo "5. Creating e5s.yaml..."
cat > e5s.yaml <<'EOF'
spire:
  workload_socket: unix:///tmp/spire-agent.sock

  # (Optional) How long to wait for identity from SPIRE before failing startup
  # If not set, defaults to 30s
  # initial_fetch_timeout: 30s

server:
  listen_addr: ":8443"
  allowed_client_trust_domain: "example.org"

client:
  expected_server_trust_domain: "example.org"
EOF

# Step 6: Build binaries
echo "6. Building static binaries..."
CGO_ENABLED=0 go build -o bin/server ./server
CGO_ENABLED=0 go build -o bin/client ./client

# Step 7: Create Kubernetes manifests
echo "7. Creating Kubernetes manifests..."

cat > k8s-e5s-config.yaml <<'EOF'
apiVersion: v1
kind: ConfigMap
metadata:
  name: e5s-config
  namespace: default
data:
  e5s.yaml: |
    spire:
      workload_socket: unix:///spire/agent-socket/spire-agent.sock
      # initial_fetch_timeout: 30s

    server:
      listen_addr: ":8443"
      allowed_client_trust_domain: "example.org"

    client:
      expected_server_trust_domain: "example.org"
EOF

cat > k8s-server.yaml <<'EOF'
apiVersion: apps/v1
kind: Deployment
metadata:
  name: e5s-server
  namespace: default
spec:
  replicas: 1
  selector:
    matchLabels:
      app: e5s-server
  template:
    metadata:
      labels:
        app: e5s-server
    spec:
      serviceAccountName: default
      containers:
      - name: server
        image: e5s-server:dev
        imagePullPolicy: Never
        volumeMounts:
        - name: spire-agent-socket
          mountPath: /spire/agent-socket
          readOnly: true
        - name: config
          mountPath: /app/e5s.yaml
          subPath: e5s.yaml
      volumes:
      - name: spire-agent-socket
        csi:
          driver: "csi.spiffe.io"
          readOnly: true
      - name: config
        configMap:
          name: e5s-config
---
apiVersion: v1
kind: Service
metadata:
  name: e5s-server
  namespace: default
spec:
  selector:
    app: e5s-server
  ports:
  - protocol: TCP
    port: 8443
    targetPort: 8443
EOF

cat > k8s-client-job.yaml <<'EOF'
apiVersion: batch/v1
kind: Job
metadata:
  name: e5s-client
  namespace: default
spec:
  template:
    metadata:
      labels:
        app: e5s-client
    spec:
      serviceAccountName: default
      restartPolicy: Never
      containers:
      - name: client
        image: e5s-client:dev
        imagePullPolicy: Never
        env:
        - name: SERVER_URL
          value: "https://e5s-server:8443/hello"
        command: ["/app/client"]
        volumeMounts:
        - name: spire-agent-socket
          mountPath: /spire/agent-socket
          readOnly: true
        - name: config
          mountPath: /app/e5s.yaml
          subPath: e5s.yaml
      volumes:
      - name: spire-agent-socket
        csi:
          driver: "csi.spiffe.io"
          readOnly: true
      - name: config
        configMap:
          name: e5s-config
EOF

# Step 8: Build Docker images in Minikube
echo "8. Building Docker images in Minikube..."
eval $(minikube docker-env)

docker build -t e5s-server:dev -f - . <<'DOCKERFILE'
FROM alpine:latest
WORKDIR /app
COPY bin/server .
ENTRYPOINT ["/app/server"]
DOCKERFILE

docker build -t e5s-client:dev -f - . <<'DOCKERFILE'
FROM alpine:latest
WORKDIR /app
COPY bin/client .
ENTRYPOINT ["/app/client"]
DOCKERFILE

# Step 9: Deploy to Kubernetes
echo "9. Deploying to Kubernetes..."
kubectl apply -f k8s-e5s-config.yaml
kubectl apply -f k8s-server.yaml

echo "   Waiting for server to be ready..."
kubectl wait --for=condition=ready pod -l app=e5s-server -n default --timeout=60s

# Step 10: Test with client
echo "10. Testing with client..."
kubectl delete job e5s-client 2>/dev/null || true
kubectl apply -f k8s-client-job.yaml

echo "    Waiting for client to complete..."
sleep 10

echo ""
echo "=== Test Results ==="
kubectl logs -l app=e5s-client --tail=20

echo ""
echo "=== Setup Complete ==="
echo ""
echo "Your test environment is ready!"
echo ""
echo "To iterate on code changes:"
echo "  ./scripts/rebuild-and-test.sh"
echo ""
echo "To view server logs:"
echo "  kubectl logs -l app=e5s-server -f"
echo ""
echo "To clean up:"
echo "  ./scripts/cleanup-prerelease.sh"
echo ""
