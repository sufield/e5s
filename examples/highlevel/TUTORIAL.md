# Getting Started Tutorial: Building mTLS Applications with e5s

**Target Audience**: Developers who want to build secure mTLS services using SPIFFE/SPIRE.

**What You'll Learn**:
- Build mTLS server and client applications using e5s
- Configure identity-based authentication
- Verify mutual TLS authentication is working
- Test zero-trust security enforcement

**Time Required**: ~15 minutes (after infrastructure setup)

---

## Prerequisites

1. **SPIRE Infrastructure Running**: Run `make start-stack` to set up SPIRE in Minikube (~15 minutes)
   - Minikube cluster running
   - SPIRE Server and Agent installed
   - Server and client workloads registered

2. **Verify Required Tools**

   ```bash
   make verify-tools
   ```

---

# Building Your mTLS Application

With the SPIRE infrastructure running, you can build applications that use it for mTLS. In this section, you'll write a simple server and client using the e5s library, which abstracts all the complexity of SPIRE integration.

**Goal**: Build and run mTLS applications that automatically obtain certificates from SPIRE.

---

## Step 1: Install Dependencies

Create a new directory for your application and install dependencies:

```bash
mkdir -p ~/demo
```

```bash
cd ~/demo
```

Initialize Go module

```bash
go mod init demo
```

# Install e5s library

```bash
go get github.com/sufield/e5s@latest
```
# Install chi router (for HTTP routing)

```bash
go get github.com/go-chi/chi/v5
```

Your `go.mod` should look like:

```
module demo

go 1.25

require (
    github.com/go-chi/chi/v5 v5.2.3
    github.com/sufield/e5s v0.1.0
)
```

### Create Server Application

Create `server/main.go`:

```go
package main

import (
	"fmt"
	"log"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/sufield/e5s"
)

func main() {
	// Create HTTP router
	r := chi.NewRouter()

	// Health check endpoint
	r.Get("/healthz", func(w http.ResponseWriter, req *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})

	// Authenticated endpoint that requires mTLS
	r.Get("/hello", func(w http.ResponseWriter, req *http.Request) {
		// Extract peer identity from mTLS connection
		id, ok := e5s.PeerID(req)
		if !ok {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		fmt.Fprintf(w, "Hello, %s!\n", id)
	})

	// Start server with automatic signal handling and shutdown
	if err := e5s.Serve("e5s-server.yaml", r); err != nil {
		log.Fatal(err)
	}
}
```

**What this code does:**
- Creates a chi router with two endpoints: `/healthz` and `/hello`
- The `/hello` endpoint extracts the client's SPIFFE ID using `e5s.PeerID()`
- Calls `e5s.Serve("e5s-server.yaml", r)` which:
  - Loads server config from e5s-server.yaml
  - Connects to SPIRE and sets up mTLS
  - Handles interrupt signals (Ctrl+C) automatically
  - Performs graceful shutdown on exit

Just define your routes and call `e5s.Serve()` - signal handling and shutdown are automatic!

**For advanced control** (manual shutdown, custom signals, etc.), use `e5s.Start()`. See [ADVANCED.md](ADVANCED.md).

### Create Client Application

Create `client/main.go`:

```go
package main

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"os"

	"github.com/sufield/e5s"
)

func main() {
	// Get server address from environment variable
	serverAddr := os.Getenv("SERVER_ADDR")
	if serverAddr == "" {
		serverAddr = "https://localhost:8443"
	}

	// Create mTLS client with automatic cleanup
	err := e5s.WithClient("e5s-client.yaml", func(client *http.Client) error {
		// Perform mTLS GET request
		resp, err := client.Get(serverAddr + "/hello")
		if err != nil {
			return err
		}
		defer resp.Body.Close()

		// Read and print response
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return err
		}
		fmt.Println(string(body))
		return nil
	})
	if err != nil {
		log.Fatal(err)
	}
}
```

**What this code does:**
- Calls `e5s.WithClient("e5s-client.yaml", func...)` which:
  - Loads client config from e5s-client.yaml
  - Creates an mTLS-enabled HTTP client
  - Passes it to your function
  - Automatically cleans up resources when done
- Reads server address from `SERVER_ADDR` environment variable (defaults to `https://localhost:8443`)
- Uses the standard Go `http.Client` interface to make requests
- Cleanup is automatic - no defer needed!

For multiple requests, reuse the client inside the callback function.

**For advanced use** (long-lived clients, manual lifecycle management, etc.), use `e5s.Client()`. See [ADVANCED.md](ADVANCED.md).

### Create Configuration Files

Create two separate config files, one for the server and one for the client:

**e5s-server.yaml:**
```yaml
spire:
  workload_socket: unix:///tmp/spire-agent/public/api.sock

server:
  listen_addr: ":8443"
  allowed_client_trust_domain: "example.org"
```

**e5s-client.yaml:**
```yaml
spire:
  workload_socket: unix:///tmp/spire-agent/public/api.sock

client:
  expected_server_trust_domain: "example.org"
```

Each binary gets its own configuration file that describes only what that process does.

### Build Binaries

From demo directory, build server:

```bash
go build -o bin/server ./server
```

Build client

```bash
go build -o bin/client ./client
```

---

## Step 2: Create Kubernetes Manifests

### Server Deployment

Create `k8s-server.yaml`:

```yaml
apiVersion: v1
kind: Pod
metadata:
  name: e5s-server
  namespace: default
  labels:
    app: e5s-server
spec:
  containers:
  - name: server
    image: golang:1.25
    command: ["/app/bin/server"]
    volumeMounts:
    - name: app
      mountPath: /app
    - name: spire-agent-socket
      mountPath: /tmp/spire-agent/public
      readOnly: true
    env:
    - name: E5S_CONFIG
      value: "/app/e5s-server.yaml"
    ports:
    - containerPort: 8443
      name: https
  volumes:
  - name: app
    hostPath:
      path: /path/to/your/app  # Update this path
      type: Directory
  - name: spire-agent-socket
    hostPath:
      path: /var/run/spire/sockets
      type: Directory
```

### Client Job

Create `k8s-client.yaml`:

```yaml
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
      restartPolicy: Never
      containers:
      - name: client
        image: golang:1.25
        command: ["/app/bin/client"]
        volumeMounts:
        - name: app
          mountPath: /app
        - name: spire-agent-socket
          mountPath: /tmp/spire-agent/public
          readOnly: true
        env:
        - name: E5S_CONFIG
          value: "/app/e5s-client.yaml"
        - name: SERVER_ADDR
          value: "https://e5s-server.default.svc.cluster.local:8443"
      volumes:
      - name: app
        hostPath:
          path: /path/to/your/app  # Update this path
          type: Directory
      - name: spire-agent-socket
        hostPath:
          path: /var/run/spire/sockets
          type: Directory
```

---

## Step 3: Run Locally (Easier for Development)

Let's run locally and connect to SPIRE in Minikube:

### Port Forward SPIRE Agent Socket

In a separate terminal, keep this running:

```bash
kubectl port-forward -n spire \
  $(kubectl get pod -n spire -l app.kubernetes.io/name=agent -o jsonpath='{.items[0].metadata.name}') \
  8081:8081
```

### Update Config Files for Local Development

Temporarily modify both config files to use the port-forwarded socket:

**e5s-server.yaml:**
```yaml
spire:
  workload_socket: unix:///tmp/spire-agent.sock

server:
  listen_addr: ":8443"
  allowed_client_trust_domain: "example.org"
```

**e5s-client.yaml:**
```yaml
spire:
  workload_socket: unix:///tmp/spire-agent.sock

client:
  expected_server_trust_domain: "example.org"
```

### Create Symlink to Agent Socket

Create symlink from local machine to forwarded socket:

```bash
ln -sf ~/.minikube/profiles/minikube/apiserver.sock /tmp/spire-agent.sock
```

---

## Step 4: Run and Test

### Terminal 1: Run Server

```bash
./bin/server
```

**Expected output**:

```
2024/10/30 10:00:00 Starting mTLS server (config: e5s-server.yaml)...
2024/10/30 10:00:00 Server running - press Ctrl+C to stop
```

### Terminal 2: Run Client

```bash
SERVER_ADDR=https://localhost:8443 ./bin/client
```

**Expected output**:

```
2024/10/30 10:00:01 Creating mTLS client (config: e5s-client.yaml)...
2024/10/30 10:00:01 Client created successfully
2024/10/30 10:00:01 Making request to https://localhost:8443/hello...
2024/10/30 10:00:01 Response (status 200): Hello, spiffe://example.org/client!
```

**Success!** You've established mutual TLS authentication:
- Client authenticated to server
- Server authenticated to client
- Server extracted client's SPIFFE ID

---

## Step 5: Verify mTLS is Working

Now let's verify mTLS setup by testing both success and failure cases.

---

### ✅ **SUCCESS CASE: Registered Client** (`client/main.go`)

**Test: Registered Client Connects Successfully**

The client you ran in Step 4 successfully connected because it was registered with SPIRE.

**Why it worked:**

- Client was registered in Part A, Step 3
- SPIRE issued it a certificate with SPIFFE ID: `spiffe://example.org/client`
- Server accepted it because it's in the allowed trust domain (`example.org`)

**Expected success output** (from Step 4):

```
2024/10/30 10:00:01 Creating mTLS client (config: e5s-client.yaml)...
2024/10/30 10:00:01 Client created successfully
2024/10/30 10:00:01 Making request to https://localhost:8443/hello...
2024/10/30 10:00:01 Response (status 200): Hello, spiffe://example.org/client!
```

This proves that:
- ✅ Registered workloads can obtain certificates from SPIRE
- ✅ mTLS handshake succeeds when both parties have valid certificates
- ✅ Server can extract and verify client identity

---

### ❌ **FAILURE CASE: Unregistered Client** (`unregistered-client/main.go`)

**Test: Unregistered Client Connection Blocked**

This proves that SPIRE enforces identity-based access control. Let's try to connect with a client that is NOT registered with SPIRE.

**Create an unregistered client:**

Create `unregistered-client/main.go`:

```go
package main

import (
	"fmt"
	"io"
	"log"
	"os"

	"github.com/sufield/e5s"
)

func main() {
	log.Println("Attempting to connect as unregistered workload...")

	client, cleanup, err := e5s.Client("e5s-client.yaml")
	if err != nil {
		log.Fatalf("Connection failed (expected): %v", err)
	}
	defer func() {
		if err := cleanup(); err != nil {
			log.Printf("Cleanup error: %v", err)
		}
	}()

	// Get server address from environment variable
	serverAddr := os.Getenv("SERVER_ADDR")
	if serverAddr == "" {
		serverAddr = "https://localhost:8443"
	}

	resp, err := client.Get(serverAddr + "/hello")
	if err != nil {
		log.Fatalf("Request failed (expected): %v", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	fmt.Println(string(body))
}
```

**Try to run it:**

```bash
cd ~/demo
go run unregistered-client/main.go
```

**Expected failure:**
```
Attempting to connect as unregistered workload...
Connection failed (expected): failed to create X509Source: no identity issued
```

**Why it fails:**
1. The unregistered client tries to contact the SPIRE Agent
2. SPIRE Agent checks if this workload has a registration entry
3. No entry exists for this workload
4. SPIRE refuses to issue a certificate
5. Without a certificate, the client cannot establish mTLS connection

This proves that:
- ✅ Only registered workloads can obtain certificates
- ✅ Unregistered workloads cannot communicate with mTLS services
- ✅ SPIRE enforces zero-trust security model
- ✅ Identity must be explicitly granted, not assumed

---

### ❌ **FAILURE CASE: curl** (No SPIFFE Identity)

**Test: Traditional HTTP Client Blocked**

Standard tools like curl also cannot connect because they don't have SPIFFE identities.

This also fails because curl doesn't have a SPIFFE identity:
```bash
curl -k https://localhost:8443/hello
```

**Expected failure:**
```
curl: (35) error:14094410:SSL routines:ssl3_read_bytes:sslv3 alert handshake failure
```

**Why it fails**: The server requires client certificate authentication. curl cannot obtain a certificate from SPIRE without being a registered workload.

This demonstrates that traditional HTTP clients cannot bypass SPIFFE-based mTLS security.

---

### Check Certificate Details

You can inspect the certificates using openssl.

View server certificate:
```bash
openssl s_client -connect localhost:8443 -showcerts
```

Look for:
- Subject Alternative Name (SAN): `URI:spiffe://example.org/server`
- Issuer: SPIRE Server

---

## Step 6: Understand What Just Happened

Let's break down the mTLS flow:

1. **Server Startup**:
   - Connects to SPIRE Agent via Unix socket
   - Requests its SVID (SPIFFE Verifiable Identity Document)
   - Receives X.509 certificate with SPIFFE ID: `spiffe://example.org/server`
   - Starts HTTPS listener with mTLS enabled

2. **Client Startup**:
   - Connects to SPIRE Agent via Unix socket
   - Requests its SVID
   - Receives X.509 certificate with SPIFFE ID: `spiffe://example.org/client`
   - Configures HTTP client with mTLS

3. **TLS Handshake**:
   - Client presents its certificate to server
   - Server verifies client certificate was issued by SPIRE
   - Server checks client is in allowed trust domain (`example.org`)
   - Server presents its certificate to client
   - Client verifies server certificate was issued by SPIRE
   - Client checks server is in expected trust domain (`example.org`)

4. **Request Processing**:
   - TLS handshake succeeds (mutual authentication)
   - e5s extracts client SPIFFE ID from certificate
   - Server handler receives authenticated identity
   - Server responds to authenticated client

5. **Certificate Rotation** (automatic):
   - SPIRE automatically rotates certificates before expiry
   - e5s library automatically picks up new certificates
   - No downtime, no manual intervention

---

## Next Steps

Now that you have mTLS working:

1. **Try Specific SPIFFE ID Authorization**:
   Update `e5s-server.yaml` to require specific client IDs:
   ```yaml
   server:
     listen_addr: ":8443"
     allowed_client_spiffe_id: "spiffe://example.org/client"
   ```

2. **Observe Certificate Rotation**:
   Certificates rotate automatically. Watch logs to see rotation happen.

   Certificates typically rotate every hour. Watch for new certificates being fetched.

3. **Deploy to Kubernetes**:
   - Build container images for server and client
   - Deploy using the k8s manifests from Step 2
   - Watch them communicate over mTLS in the cluster

4. **Add More Endpoints**:
   - Add authenticated API endpoints
   - Extract identity in middleware
   - Implement authorization based on SPIFFE ID

5. **Production Hardening**:
   - Use specific SPIFFE IDs instead of trust domains
   - Add request logging
   - Set up monitoring and alerting
   - Review [runtime security monitoring](../../docs/how-to/monitor-with-falco.md)

---

## Clean Up

When you're done testing:

Stop local applications (Ctrl+C in each terminal).

Clean up your application binaries:
```bash
cd ~/mtls-demo
rm -rf bin/
```

**To clean up SPIRE infrastructure**, run `make stop-stack`.

---

## Resources

- **SPIRE Setup**: Run `make start-stack` to set up infrastructure, `make stop-stack` to clean up
- **Troubleshooting Guide**: See [../../docs/reference/troubleshooting.md](../../docs/reference/troubleshooting.md) for common issues
- **Advanced Examples**: See [ADVANCED.md](ADVANCED.md) for advanced patterns and control
- **e5s Library Docs**: See [main README](../../README.md) for library documentation
- **SPIRE Documentation**: https://spiffe.io/docs/latest/spire/
- **SPIFFE Standard**: https://github.com/spiffe/spiffe

---

## Summary

You've successfully:
- Built mTLS server and client applications using e5s
- Configured identity-based authentication with SPIRE
- Verified mutual TLS authentication is working
- Tested zero-trust security enforcement (unregistered clients blocked)
- Understood automatic certificate rotation

The e5s library handles all the complexity:
- SPIRE Workload API connection
- Certificate fetching and rotation
- TLS 1.3 configuration
- mTLS handshake setup
- SPIFFE ID verification

You just write `e5s.Start()`, `e5s.Client()`, and `e5s.PeerID()` - the library does the rest!
