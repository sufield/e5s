# Testing Pre-Release: Internal Development Guide

**Target Audience**: Internal developers testing e5s library changes before publishing to GitHub.

**Purpose**: Test local e5s code changes in a realistic environment before releasing to end users.

**Time Required**: ~20 minutes (after infrastructure setup)

---

## When to Use This Guide

Use this guide when you:
- Are developing new features for the e5s library
- Need to test changes before creating a release
- Want to validate bug fixes in a real environment
- Are testing the tutorial steps before publishing

**For end users**: See [TUTORIAL.md](TUTORIAL.md) instead - this guide is for internal testing only.

---

## Prerequisites

Before starting, you must have:

1. **SPIRE Infrastructure Running**: Follow [SPIRE_SETUP.md](SPIRE_SETUP.md) to set up SPIRE in Minikube (~15 minutes)
   - Minikube cluster running
   - SPIRE Server and Agent installed
   - Server and client workloads registered

2. **Go** - Programming language (1.21 or higher)
   ```bash
   go version
   # Should output: go version go1.21.0 or higher
   ```

3. **Local e5s Code**: You should be in the e5s project directory
   ```bash
   # Verify you're in the right place
   ls -la e5s.go pkg/ examples/
   # Should show the e5s library source code
   ```

---

## Step 1: Create Test Application Directory

Create a test application that uses your local e5s code:

```bash
# Navigate to the e5s project root
cd /path/to/e5s  # Where your e5s code is located

# Create a test directory
mkdir -p test-demo
cd test-demo

# Initialize Go module
go mod init test-demo
```

---

## Step 2: Configure Local Dependency

Use the Go `replace` directive to point to your local e5s code instead of the released version:

```bash
# Add replace directive to point to local e5s code
# The '..' means parent directory (where e5s source code is)
go mod edit -replace github.com/sufield/e5s=..

# Add chi router dependency
go get github.com/go-chi/chi/v5

# Add e5s to require section (will use local code due to replace directive)
go get github.com/sufield/e5s
```

**Verify your `go.mod`:**

```bash
cat go.mod
```

**Your `go.mod` should look like:**
```
module test-demo

go 1.25.3

require (
    github.com/go-chi/chi/v5 v5.2.3
    github.com/sufield/e5s v0.0.0
)

replace github.com/sufield/e5s => ..
```

**What this does**:
- The `replace` directive tells Go to use the parent directory instead of downloading from GitHub
- Any `import "github.com/sufield/e5s"` in your code will use your local e5s code
- You can modify e5s code and immediately see changes in your test application
- Perfect for iterating on library changes

**Note**: You don't need to run `go mod tidy` until after you create the source files in Steps 3 and 4.

---

## Step 3: Create Server Application

Create `server/main.go`:

```go
package main

import (
	"fmt"
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

	// Run mTLS server (uses local e5s code)
	e5s.Run(r)
}
```

---

## Step 4: Create Client Application

Create `client/main.go`:

```go
package main

import (
	"fmt"
	"io"
	"log"

	"github.com/sufield/e5s"
)

func main() {
	// Perform mTLS GET request (uses local e5s code)
	resp, err := e5s.Get("https://localhost:8443/hello")
	if err != nil {
		log.Fatal(err)
	}
	defer resp.Body.Close()

	// Read and print response
	body, _ := io.ReadAll(resp.Body)
	fmt.Println(string(body))
}
```

---

## Step 5: Create Configuration File

Create `e5s.yaml` in the test-demo directory:

```yaml
spire:
  # Path to SPIRE Agent's Workload API socket
  workload_socket: unix:///tmp/spire-agent/public/api.sock

  # How long to wait for identity from SPIRE before failing startup
  initial_fetch_timeout: 30s

server:
  listen_addr: ":8443"
  # Accept any client in this trust domain
  allowed_client_trust_domain: "example.org"

client:
  # Connect to any server in this trust domain
  expected_server_trust_domain: "example.org"
```

---

## Step 6: Build Test Binaries

Build your test applications:

```bash
# From your test-demo directory
# These builds will use your LOCAL e5s code (due to replace directive)

# Build server
go build -o bin/server ./server

# Build client
go build -o bin/client ./client

# Verify the binaries were created
ls -lh bin/
```

**Important**: Every time you modify e5s library code, you need to rebuild these binaries to see the changes.

---

## Step 7: Port Forward SPIRE Agent Socket

In a separate terminal, port forward to access SPIRE from your local machine:

```bash
# Keep this running in a separate terminal
kubectl port-forward -n spire \
  $(kubectl get pod -n spire -l app.kubernetes.io/name=agent -o jsonpath='{.items[0].metadata.name}') \
  8081:8081
```

---

## Step 8: Create Symlink to Agent Socket

Create a symlink so your local application can connect to SPIRE:

```bash
# Create symlink from local machine to forwarded socket
# (Adjust path based on your Minikube profile)
ln -sf ~/.minikube/profiles/minikube/apiserver.sock /tmp/spire-agent.sock

# Verify symlink exists
ls -la /tmp/spire-agent.sock
```

**Update e5s.yaml** to use the symlink:

```yaml
spire:
  workload_socket: unix:///tmp/spire-agent.sock  # Updated path
  initial_fetch_timeout: 30s

server:
  listen_addr: ":8443"
  allowed_client_trust_domain: "example.org"

client:
  expected_server_trust_domain: "example.org"
```

---

## Step 9: Run and Test

### Terminal 1: Run Server

```bash
cd test-demo
./bin/server
```

**Expected output**:
```
2024/10/30 10:00:00 Starting mTLS server (config: e5s.yaml)...
2024/10/30 10:00:00 Server running - press Ctrl+C to stop
```

### Terminal 2: Run Client

```bash
cd test-demo
./bin/client
```

**Expected output**:
```
Hello, spiffe://example.org/client!
```

**Success!** Your local e5s code is working correctly.

---

## Step 10: Test Local Code Changes

Now you can iterate on e5s library changes:

### Example: Modify e5s.go

```bash
# Go back to e5s project root
cd ..  # Back to e5s directory

# Edit e5s.go (make some changes)
vim e5s.go  # or your favorite editor

# Return to test-demo directory
cd test-demo

# Rebuild with your changes
go build -o bin/server ./server
go build -o bin/client ./client

# Test again
./bin/server  # In terminal 1
./bin/client  # In terminal 2
```

**Workflow**:
1. Make changes to e5s library code
2. Rebuild test applications
3. Test immediately
4. Iterate quickly

This is much faster than publishing to GitHub and downloading each time!

---

## Step 11: Verify Security Enforcement

Test that unregistered workloads cannot connect:

Create `unregistered-client/main.go`:

```go
package main

import (
	"fmt"
	"io"
	"log"

	"github.com/sufield/e5s"
)

func main() {
	log.Println("Attempting to connect as unregistered workload...")

	resp, err := e5s.Get("https://localhost:8443/hello")
	if err != nil {
		log.Fatalf("Connection failed (expected): %v", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	fmt.Println(string(body))
}
```

**Run unregistered client:**

```bash
go run unregistered-client/main.go
```

**Expected failure:**
```
Attempting to connect as unregistered workload...
Connection failed (expected): failed to create X509Source: no identity issued
```

This confirms your local e5s code properly enforces zero-trust security.

---

## Step 12: Run Tutorial Tests

Before publishing, verify the end-user tutorial works:

```bash
# Copy tutorial examples
cp -r ../examples/highlevel/server ./tutorial-server
cp -r ../examples/highlevel/client ./tutorial-client

# Build tutorial examples
go build -o bin/tutorial-server ./tutorial-server
go build -o bin/tutorial-client ./tutorial-client

# Test tutorial examples work
./bin/tutorial-server  # Terminal 1
./bin/tutorial-client  # Terminal 2
```

This ensures the tutorial will work for end users after you publish.

---

## Common Testing Scenarios

### Testing Config Changes

If you modify `internal/config/`:

```bash
# Rebuild after config changes
go build -o bin/server ./server
go build -o bin/client ./client

# Test with different e5s.yaml configurations
# Test with environment variables
E5S_CONFIG=/path/to/alt/config.yaml ./bin/server
```

### Testing SPIRE Integration Changes

If you modify `pkg/spire/`:

```bash
# Rebuild
go build -o bin/server ./server
go build -o bin/client ./client

# Watch SPIRE logs while testing
kubectl logs -n spire -l app.kubernetes.io/name=server -c spire-server --follow

# Test certificate rotation
# (SVIDs rotate every ~30 minutes, watch for automatic rotation)
```

### Testing TLS Config Changes

If you modify `pkg/spiffehttp/`:

```bash
# Rebuild
go build -o bin/server ./server
go build -o bin/client ./client

# Use openssl to inspect TLS handshake
openssl s_client -connect localhost:8443 -showcerts

# Verify TLS 1.3 is enforced
# Verify client certificate is required
```

---

## Clean Up

When you're done testing:

```bash
# Stop local applications (Ctrl+C in each terminal)

# Clean up test directory
cd test-demo
rm -rf bin/

# Or remove entire test directory
cd ..
rm -rf test-demo
```

**To clean up SPIRE infrastructure**, follow the cleanup instructions in [SPIRE_SETUP.md](SPIRE_SETUP.md).

---

## Publishing Checklist

Before publishing a new version of e5s:

- [ ] All tests pass: `make test`
- [ ] Security checks pass: `make sec-all`
- [ ] Examples build: `make examples`
- [ ] Tutorial tested with local code (this guide)
- [ ] Documentation updated (README.md, TUTORIAL.md, ADVANCED.md)
- [ ] CHANGELOG updated
- [ ] Version bumped in code
- [ ] Git tag created
- [ ] Published to GitHub

After publishing, verify:

- [ ] Tutorial works with published version: `go get github.com/sufield/e5s@latest`
- [ ] Examples work for end users

---

## Troubleshooting

**Issue: "replace directive not working"**

```bash
# Verify replace directive is in go.mod
cat go.mod | grep replace

# Should show:
# replace github.com/sufield/e5s => ..

# Re-run go mod tidy
go mod tidy

# Verify e5s.go exists in parent directory
ls -la ../e5s.go
```

**Issue: "changes not reflected in build"**

```bash
# Always rebuild after changing e5s code
go build -o bin/server ./server
go build -o bin/client ./client

# Or use go run (rebuilds automatically)
go run ./server/main.go
```

**Issue: "import cycle detected"**

This means you're importing test code into the library. Keep test code separate from library code.

**For other issues**: See [TROUBLESHOOTING.md](TROUBLESHOOTING.md)

---

## Resources

- **End User Tutorial**: See [TUTORIAL.md](TUTORIAL.md) for the published library tutorial
- **SPIRE Setup**: See [SPIRE_SETUP.md](SPIRE_SETUP.md) for infrastructure setup
- **Troubleshooting**: See [TROUBLESHOOTING.md](TROUBLESHOOTING.md) for common issues
- **Advanced Patterns**: See [ADVANCED.md](ADVANCED.md) for advanced usage
- **Library Docs**: See [main README](../../README.md)

---

## Summary

You've successfully:
- Set up local development environment for e5s library
- Used `replace` directive to test local code changes
- Built and tested mTLS applications with local e5s code
- Verified security enforcement works correctly
- Learned how to iterate quickly on library changes

**Key Takeaway**: The `replace` directive lets you test library changes locally before publishing, ensuring end users get a working, tested library.

**Next Step**: Once testing is complete, follow the publishing checklist above to release a new version.
