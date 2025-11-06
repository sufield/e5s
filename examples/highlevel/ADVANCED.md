# Advanced Usage Examples

This covers advanced examples and configurations for production use of the e5s library.

## Table of Contents

- [Levels of Control](#levels-of-control)
- [Manual Signal Handling](#manual-signal-handling)
- [Explicit Config Paths](#explicit-config-paths)
- [Environment Variable Configuration](#environment-variable-configuration)
- [Context Timeouts](#context-timeouts)
- [Custom Request Headers](#custom-request-headers)
- [Error Handling Patterns](#error-handling-patterns)
- [Production Server Configuration](#production-server-configuration)
- [Health Check Setup](#health-check-setup)
- [Logging and Monitoring](#logging-and-monitoring)

---

## Levels of Control

The e5s library provides two levels of API based on how much control you need:

### Level 1: Simple Server (`e5s.Serve`)

**Use when:** You want the simplest API with automatic signal handling.

```go
func main() {
    // Set config path
    os.Setenv("E5S_CONFIG", "e5s.yaml")

    r := chi.NewRouter()
    r.Get("/hello", handleHello)

    // Serve handles everything
    if err := e5s.Serve(r); err != nil {
        log.Fatal(err)
    }
}
```

**What it handles:**
- Config file discovery (E5S_CONFIG)
- SPIRE connection and mTLS
- Signal handling (SIGINT/SIGTERM)
- Graceful shutdown

**You control:**
- Error handling (via returned error)

### Level 2: Explicit Config Path (`e5s.Start`)

**Use when:** You need to specify an exact config file path.

```go
func main() {
    ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
    defer stop()

    r := chi.NewRouter()
    r.Get("/hello", handleHello)

    shutdown, err := e5s.Start("/etc/app/production.yaml", r)
    if err != nil {
        log.Fatal(err)
    }
    defer shutdown()

    <-ctx.Done()
}
```

**What it handles:**
- SPIRE connection and mTLS

**You control:**
- Config file path
- Signal setup and handling
- Shutdown timing and logging
- Error handling

### Client Levels

Similarly, the client API has three levels:

#### Level 1: Zero Configuration (`e5s.Get`)

**Use when:** Making a single request with minimal code.

```go
func main() {
    resp, err := e5s.Get("https://api.example.com:8443/data")
    if err != nil {
        log.Fatal(err)
    }
    defer resp.Body.Close()  // Also triggers cleanup

    body, _ := io.ReadAll(resp.Body)
    fmt.Println(string(body))
}
```

**What it handles:**
- Config file discovery (E5S_CONFIG or e5s.yaml)
- Client creation
- Making the request
- Cleanup (tied to body close)

#### Level 2: Explicit Config Path (`e5s.Client`)

**Use when:** You need a specific config file path.

```go
func main() {
    client, shutdown, err := e5s.Client("/etc/app/production.yaml")
    if err != nil {
        log.Fatal(err)
    }
    defer shutdown()

    resp, _ := client.Get("https://api.example.com:8443/data")
    defer resp.Body.Close()
}
```

**What it handles:**
- Client creation

**You control:**
- Config file path
- Request lifecycle
- Shutdown timing
- Error handling

---

## Manual Signal Handling

When you need more control over shutdown behavior, use `e5s.Start()` instead of `e5s.Serve()`.

### Custom Shutdown Timeout

```go
func main() {
    ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
    defer stop()

    r := chi.NewRouter()
    r.Get("/hello", handleHello)

    shutdown, err := e5s.Start("e5s.yaml", r)
    if err != nil {
        log.Fatal(err)
    }

    log.Println("Server running - press Ctrl+C to stop")

    // Wait for signal
    <-ctx.Done()
    stop()

    // Custom shutdown logic with timeout
    shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
    defer cancel()

    // Do cleanup work here before calling shutdown
    log.Println("Flushing metrics...")
    flushMetrics(shutdownCtx)

    log.Println("Closing database connections...")
    db.Close()

    // Finally shutdown the server
    if err := shutdown(); err != nil {
        log.Printf("Server shutdown error: %v", err)
    }

    log.Println("Shutdown complete")
}
```

### Multiple Signal Handlers

```go
func main() {
    sigChan := make(chan os.Signal, 1)
    signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM, syscall.SIGUSR1)

    r := chi.NewRouter()
    r.Get("/hello", handleHello)

    shutdown, err := e5s.Start("e5s.yaml", r)
    if err != nil {
        log.Fatal(err)
    }
    defer shutdown()

    log.Println("Server running")

    for {
        sig := <-sigChan
        switch sig {
        case syscall.SIGUSR1:
            log.Println("Reloading configuration...")
            reloadConfig()
        case syscall.SIGINT, syscall.SIGTERM:
            log.Println("Shutting down...")
            return
        }
    }
}
```

---

## Explicit Config Paths

For most use cases, use `e5s.Serve()` (server) or `e5s.Get()` (client) with E5S_CONFIG environment variable. When you need to specify an explicit config file path, use `e5s.Start()` and `e5s.Client()` instead.

### Server with Explicit Config Path

```go
package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os/signal"
	"syscall"

	"github.com/go-chi/chi/v5"
	"github.com/sufield/e5s"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	r := chi.NewRouter()

	r.Get("/hello", func(w http.ResponseWriter, req *http.Request) {
		id, ok := e5s.PeerID(req)
		if !ok {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		fmt.Fprintf(w, "Hello, %s!\n", id)
	})

	// Explicit config file path
	shutdown, err := e5s.Start("/etc/app/production.yaml", r)
	if err != nil {
		log.Fatal(err)
	}
	defer shutdown()

	log.Println("Server running - press Ctrl+C to stop")

	<-ctx.Done()
	stop()
	log.Println("Shutting down gracefully...")
}
```

### Client with Explicit Config Path

```go
package main

import (
	"fmt"
	"io"
	"log"

	"github.com/sufield/e5s"
)

func main() {
	// Explicit config file path
	client, shutdown, err := e5s.Client("/etc/app/production.yaml")
	if err != nil {
		log.Fatal(err)
	}
	defer shutdown()

	resp, err := client.Get("https://api.example.com:8443/data")
	if err != nil {
		log.Fatal(err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	fmt.Println(string(body))
}
```

**Use cases for explicit paths:**
- Loading different configs for different services in the same codebase
- Non-standard config locations (e.g., `/etc/app/config.yaml`)
- Testing with specific config files
- Multi-tenant applications with per-tenant configs

---

## Environment Variable Configuration

Use environment variables to configure different behavior in different environments (dev, staging, prod).

### Server with Environment Variables

```go
package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/go-chi/chi/v5"
	"github.com/sufield/e5s"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

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

	// Get config file path from environment or use default
	configFile := os.Getenv("E5S_CONFIG")
	if configFile == "" {
		configFile = "e5s.yaml"
	}

	log.Printf("Starting mTLS server (config: %s)...", configFile)
	shutdown, err := e5s.Start(configFile, r)
	if err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
	defer shutdown()

	log.Println("Server running - press Ctrl+C to stop")

	<-ctx.Done()
	stop()
	log.Println("Shutting down gracefully...")
}
```

**Usage:**
```bash
# Development
E5S_CONFIG=./dev.yaml ./server

# Production
E5S_CONFIG=/etc/e5s/prod.yaml ./server
```

### Client with Environment Variables

```go
package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/sufield/e5s"
)

func main() {
	// Get config file path from environment or use default
	configFile := os.Getenv("E5S_CONFIG")
	if configFile == "" {
		configFile = "e5s.yaml"
	}

	// Get server address from environment or use default
	serverAddr := os.Getenv("SERVER_ADDR")
	if serverAddr == "" {
		serverAddr = "https://localhost:8443"
	}

	log.Printf("Creating mTLS client (config: %s)...", configFile)
	client, shutdown, err := e5s.Client(configFile)
	if err != nil {
		log.Fatalf("Failed to create client: %v", err)
	}
	defer shutdown()

	log.Println("Client created successfully")

	// Create context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Make request to server
	url := fmt.Sprintf("%s/hello", serverAddr)
	log.Printf("Making request to %s...", url)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		log.Fatalf("Failed to create request: %v", err)
	}

	resp, err := client.Do(req)
	if err != nil {
		log.Fatalf("Request failed: %v", err)
	}
	defer resp.Body.Close()

	// Read and print response
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Fatalf("Failed to read response: %v", err)
	}

	log.Printf("Response (status %d): %s", resp.StatusCode, string(body))
}
```

**Usage:**
```bash
# Connect to local server
./client

# Connect to remote server
SERVER_ADDR=https://api.example.com:8443 ./client

# Use different config
E5S_CONFIG=./prod.yaml SERVER_ADDR=https://prod.example.com:8443 ./client
```

---

## Context Timeouts

Control request timeout behavior using Go contexts.

### Per-Request Timeout

```go
func makeRequest(client *http.Client, url string) error {
	// Create context with 30-second timeout for this specific request
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status: %d", resp.StatusCode)
	}

	return nil
}
```

### Different Timeouts for Different Operations

```go
func main() {
	client, shutdown, err := e5s.Client("e5s.yaml")
	if err != nil {
		log.Fatal(err)
	}
	defer shutdown()

	// Quick health check (5 second timeout)
	healthCtx, cancel1 := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel1()

	healthReq, _ := http.NewRequestWithContext(healthCtx, "GET", "https://api.example.com/healthz", nil)
	healthResp, err := client.Do(healthReq)
	if err != nil {
		log.Printf("Health check failed: %v", err)
	} else {
		healthResp.Body.Close()
		log.Println("Health check passed")
	}

	// Long-running operation (2 minute timeout)
	longCtx, cancel2 := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel2()

	longReq, _ := http.NewRequestWithContext(longCtx, "POST", "https://api.example.com/batch", nil)
	longResp, err := client.Do(longReq)
	if err != nil {
		log.Fatalf("Batch operation failed: %v", err)
	}
	defer longResp.Body.Close()
}
```

---

## Custom Request Headers

Add custom headers to your mTLS requests.

### Adding Headers

```go
func makeAuthenticatedRequest(client *http.Client, url, apiKey string) (*http.Response, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}

	// Add custom headers
	req.Header.Set("X-API-Key", apiKey)
	req.Header.Set("X-Request-ID", generateRequestID())
	req.Header.Set("User-Agent", "e5s-client/1.0")

	return client.Do(req)
}

func generateRequestID() string {
	// Generate unique request ID for tracing
	return fmt.Sprintf("%d", time.Now().UnixNano())
}
```

### POST Requests with JSON Body

```go
func postJSON(client *http.Client, url string, data interface{}) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Marshal data to JSON
	jsonData, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("failed to marshal JSON: %w", err)
	}

	// Create request with JSON body
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("server error (%d): %s", resp.StatusCode, body)
	}

	return nil
}
```

---

## Error Handling Patterns

Production-grade error handling for mTLS clients.

### Retry with Exponential Backoff

```go
func makeRequestWithRetry(client *http.Client, url string, maxRetries int) error {
	var lastErr error

	for attempt := 0; attempt < maxRetries; attempt++ {
		// Exponential backoff: 1s, 2s, 4s, 8s, ...
		if attempt > 0 {
			backoff := time.Duration(1<<uint(attempt-1)) * time.Second
			log.Printf("Retry attempt %d after %v", attempt+1, backoff)
			time.Sleep(backoff)
		}

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
		if err != nil {
			cancel()
			return err
		}

		resp, err := client.Do(req)
		cancel()

		if err == nil {
			defer resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return nil // Success!
			}
			lastErr = fmt.Errorf("status code: %d", resp.StatusCode)
		} else {
			lastErr = err
		}
	}

	return fmt.Errorf("failed after %d attempts: %w", maxRetries, lastErr)
}
```

### Circuit Breaker Pattern

```go
type CircuitBreaker struct {
	failures    int
	lastFailure time.Time
	threshold   int
	timeout     time.Duration
	mu          sync.Mutex
}

func (cb *CircuitBreaker) Call(fn func() error) error {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	// Check if circuit is open
	if cb.failures >= cb.threshold {
		if time.Since(cb.lastFailure) < cb.timeout {
			return fmt.Errorf("circuit breaker open")
		}
		// Reset after timeout
		cb.failures = 0
	}

	// Attempt call
	err := fn()
	if err != nil {
		cb.failures++
		cb.lastFailure = time.Now()
		return err
	}

	// Success - reset failures
	cb.failures = 0
	return nil
}

// Usage
func main() {
	client, shutdown, err := e5s.Client("e5s.yaml")
	if err != nil {
		log.Fatal(err)
	}
	defer shutdown()

	cb := &CircuitBreaker{
		threshold: 5,
		timeout:   30 * time.Second,
	}

	err = cb.Call(func() error {
		resp, err := client.Get("https://api.example.com/data")
		if err != nil {
			return err
		}
		defer resp.Body.Close()
		return nil
	})

	if err != nil {
		log.Printf("Request failed: %v", err)
	}
}
```

---

## Production Server Configuration

### Multiple Config Files per Environment

Create separate config files for each environment:

**dev.yaml:**
```yaml
spire:
  workload_socket: unix:///tmp/spire-agent/public/api.sock
  initial_fetch_timeout: 60s  # Higher timeout in dev (agent may start slowly)

server:
  listen_addr: ":8443"
  allowed_client_trust_domain: "dev.example.org"

client:
  expected_server_trust_domain: "dev.example.org"
```

**prod.yaml:**
```yaml
spire:
  workload_socket: unix:///run/spire/sockets/agent.sock
  initial_fetch_timeout: 10s  # Lower timeout in prod (fail fast)

server:
  listen_addr: ":8443"
  # Use specific SPIFFE IDs in production for tighter security
  allowed_client_spiffe_id: "spiffe://prod.example.org/frontend"

client:
  # Verify exact server identity in production
  expected_server_spiffe_id: "spiffe://prod.example.org/api-server"
```

---

## Health Check Setup

### Separate Health Check Port

For platforms requiring unauthenticated health checks, run a separate HTTP server:

```go
func main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// Unauthenticated health check server on port 8080
	healthMux := http.NewServeMux()
	healthMux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("ok"))
	})
	healthMux.HandleFunc("/ready", func(w http.ResponseWriter, r *http.Request) {
		// Add readiness checks here (e.g., database connection)
		w.Write([]byte("ready"))
	})

	healthServer := &http.Server{
		Addr:    ":8080",
		Handler: healthMux,
	}

	go func() {
		log.Println("Health check server listening on :8080")
		if err := healthServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("Health server error: %v", err)
		}
	}()

	// mTLS server on port 8443
	r := chi.NewRouter()
	r.Get("/api/data", func(w http.ResponseWriter, req *http.Request) {
		id, ok := e5s.PeerID(req)
		if !ok {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		fmt.Fprintf(w, "Data for %s\n", id)
	})

	shutdown, err := e5s.Start("e5s.yaml", r)
	if err != nil {
		log.Fatal(err)
	}
	defer shutdown()

	log.Println("mTLS server running on :8443")

	<-ctx.Done()
	stop()

	// Graceful shutdown both servers
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	healthServer.Shutdown(shutdownCtx)
	log.Println("Shutdown complete")
}
```

---

## Logging and Monitoring

### Structured Logging with Request Context

```go
import (
	"log/slog"
	"time"
)

func main() {
	// Setup structured logger
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	r := chi.NewRouter()

	// Logging middleware
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			start := time.Now()

			// Extract SPIFFE ID
			id, _ := e5s.PeerID(req)

			// Call next handler
			next.ServeHTTP(w, req)

			// Log request
			logger.Info("request",
				"method", req.Method,
				"path", req.URL.Path,
				"spiffe_id", id,
				"duration_ms", time.Since(start).Milliseconds(),
			)
		})
	})

	r.Get("/api/data", func(w http.ResponseWriter, req *http.Request) {
		id, ok := e5s.PeerID(req)
		if !ok {
			logger.Warn("unauthorized access attempt",
				"remote_addr", req.RemoteAddr,
			)
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		logger.Info("serving request",
			"spiffe_id", id,
			"endpoint", "/api/data",
		)

		fmt.Fprintf(w, "Hello, %s!\n", id)
	})

	shutdown, err := e5s.Start("e5s.yaml", r)
	if err != nil {
		logger.Error("failed to start server", "error", err)
		os.Exit(1)
	}
	defer shutdown()

	logger.Info("server started", "addr", ":8443")

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	<-ctx.Done()
	logger.Info("shutting down")
}
```

### Metrics Collection

```go
import (
	"sync/atomic"
	"time"
)

type Metrics struct {
	requestCount   atomic.Uint64
	errorCount     atomic.Uint64
	lastRequestAt  atomic.Int64
}

func (m *Metrics) RecordRequest() {
	m.requestCount.Add(1)
	m.lastRequestAt.Store(time.Now().Unix())
}

func (m *Metrics) RecordError() {
	m.errorCount.Add(1)
}

func (m *Metrics) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		m.RecordRequest()

		// Wrap response writer to capture status code
		wrapped := &statusWriter{ResponseWriter: w, status: 200}
		next.ServeHTTP(wrapped, req)

		if wrapped.status >= 400 {
			m.RecordError()
		}
	})
}

type statusWriter struct {
	http.ResponseWriter
	status int
}

func (w *statusWriter) WriteHeader(status int) {
	w.status = status
	w.ResponseWriter.WriteHeader(status)
}

// Expose metrics endpoint
func (m *Metrics) Handler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{
			"requests": %d,
			"errors": %d,
			"last_request_unix": %d
		}`,
			m.requestCount.Load(),
			m.errorCount.Load(),
			m.lastRequestAt.Load(),
		)
	}
}
```

---

## See Also

- [TUTORIAL.md](TUTORIAL.md) - Getting started guide for beginners
- [README.md](README.md) - High-level API overview
- [../../docs/QUICKSTART_LIBRARY.md](../../docs/QUICKSTART_LIBRARY.md) - Low-level API usage
