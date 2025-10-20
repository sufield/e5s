// Package httpclient provides an mTLS HTTP client adapter using the go-spiffe SDK.
//
// This adapter implements the ports.MTLSClient interface, providing:
//   - Automatic X.509 SVID rotation via SPIRE Workload API
//   - Identity-based mTLS authentication
//   - Server identity verification using SPIFFE IDs
//   - TLS 1.3+ enforcement
//
// # Usage
//
// Create a client with configuration:
//
//	cfg := &ports.MTLSConfig{
//	    WorkloadAPI: ports.WorkloadAPIConfig{
//	        SocketPath: "unix:///tmp/spire-agent/public/api.sock",
//	    },
//	    SPIFFE: ports.SPIFFEConfig{
//	        AllowedPeerID: "spiffe://example.org/server", // Or use AllowedTrustDomain
//	    },
//	    HTTP: ports.HTTPConfig{
//	        ReadTimeout:  10 * time.Second,
//	        WriteTimeout: 10 * time.Second,
//	    },
//	}
//
//	client, err := httpclient.New(ctx, cfg)
//	if err != nil {
//	    log.Fatal(err)
//	}
//	defer client.Close()
//
// Make requests:
//
//	req, _ := http.NewRequest("GET", "https://localhost:8443/api/hello", http.NoBody)
//	resp, err := client.Do(ctx, req)
//	if err != nil {
//	    log.Fatal(err)
//	}
//	defer resp.Body.Close()
//
// # Security
//
// The client verifies server identity using SPIFFE IDs, not DNS hostnames.
// This means you can use "localhost" or IP addresses in URLs - the SPIFFE ID
// is what matters for authentication.
//
// The client enforces TLS 1.3+ and performs mutual TLS (both client and
// server present certificates).
//
// # Configuration
//
// WorkloadAPI.SocketPath: Path to SPIRE agent socket (required)
//
// SPIFFE authorization (exactly one must be set):
//   - AllowedPeerID: Verify server has exact SPIFFE ID (e.g., "spiffe://example.org/server")
//   - AllowedTrustDomain: Accept any server in trust domain (e.g., "example.org")
//
// HTTP timeouts (optional):
//   - ReadTimeout: Maximum time for reading response
//   - WriteTimeout: Maximum time for writing request
//   - IdleTimeout: Maximum time for idle connections
//
// # Thread Safety
//
// The client is safe for concurrent use. Multiple goroutines can call Do()
// simultaneously.
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
//	client, err := httpclient.New(ctx, cfg)
//	if err != nil {
//	    return err
//	}
//	defer client.Close()
package httpclient
