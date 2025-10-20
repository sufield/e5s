// Package zerotrustclient provides a zero-config mTLS HTTP client for zero-trust networking.
//
// This package offers a high-level abstraction over SPIFFE/SPIRE, providing automatic:
//   - SPIRE agent socket detection
//   - X.509 SVID fetching and rotation
//   - Server identity verification
//   - TLS 1.3+ enforcement
//
// # Quick Start
//
// Create a client with minimal configuration:
//
//	client, err := zerotrustclient.New(ctx, &zerotrustclient.Config{
//	    ServerID: "spiffe://example.org/server",
//	})
//	if err != nil {
//	    log.Fatal(err)
//	}
//	defer client.Close()
//
//	// Make a GET request
//	resp, err := client.Get(ctx, "https://localhost:8443/api/hello")
//	if err != nil {
//	    log.Fatal(err)
//	}
//	defer resp.Body.Close()
//
// # Configuration
//
// All configuration fields are optional with sensible defaults:
//
//	type Config struct {
//	    ServerID          string  // Expected server SPIFFE ID (e.g., "spiffe://example.org/server")
//	    ServerTrustDomain string  // Accept any server in trust domain (e.g., "example.org")
//	    SocketPath        string  // SPIRE agent socket (auto-detected if empty)
//	}
//
// # Auto-Detection
//
// The client automatically detects the SPIRE agent socket from:
//  1. SPIFFE_ENDPOINT_SOCKET environment variable
//  2. SPIRE_AGENT_SOCKET environment variable
//  3. Common paths: /tmp/spire-agent/public/api.sock, /var/run/spire/sockets/agent.sock
//
// # Server Verification
//
// Choose one of two verification modes:
//
// Exact ID match (recommended for production):
//
//	Config{ServerID: "spiffe://example.org/server"}
//
// Trust domain match (accepts any server in domain):
//
//	Config{ServerTrustDomain: "example.org"}
//
// # Advanced Usage
//
// For custom HTTP methods, use Do() with a manually constructed request:
//
//	req, err := http.NewRequest("PUT", url, body)
//	if err != nil {
//	    log.Fatal(err)
//	}
//	req.Header.Set("Content-Type", "application/json")
//
//	resp, err := client.Do(ctx, req)
//
// # Thread Safety
//
// The client is safe for concurrent use. Multiple goroutines can call
// Do(), Get(), and Post() simultaneously.
//
// # Resource Management
//
// Always call Close() to release resources:
//   - Stops certificate rotation
//   - Closes SPIRE Workload API connection
//   - Closes idle HTTP connections
//
// Use defer to ensure cleanup:
//
//	client, err := zerotrustclient.New(ctx, cfg)
//	if err != nil {
//	    return err
//	}
//	defer client.Close()
//
// # DNS vs SPIFFE ID
//
// The client verifies server identity using SPIFFE IDs, not DNS hostnames.
// This means you can use "localhost" or IP addresses in URLs - only the
// SPIFFE ID matters for authentication.
//
// Example:
//
//	// These are all equivalent for identity verification:
//	client.Get(ctx, "https://localhost:8443/api")
//	client.Get(ctx, "https://127.0.0.1:8443/api")
//	client.Get(ctx, "https://server.example.com:8443/api")
//
// The server's SPIFFE ID (e.g., "spiffe://example.org/server") is verified,
// not the DNS name.
package zerotrustclient
