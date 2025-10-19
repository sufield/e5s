// test-client is an infrastructure testing tool for verifying mTLS SPIFFE deployments.
//
// Purpose:
//   - Validates that SPIRE infrastructure is correctly configured
//   - Tests mTLS connectivity between workloads
//   - Verifies server endpoints are accessible with SPIFFE authentication
//   - Confirms trust domain configuration is working
//
// When to use:
//   - After deploying SPIRE server and agent to Kubernetes/Minikube
//   - After registering workload identities with SPIRE
//   - When verifying mTLS server deployments
//   - For troubleshooting SPIFFE authentication issues
//   - As a reference for building production SPIFFE clients
//
// How it works:
//  1. Connects to SPIRE Workload API via socket (auto-detected or from env)
//  2. Obtains its own X.509 SVID (SPIFFE Verifiable Identity Document)
//  3. Creates an mTLS HTTP client that trusts the same trust domain
//  4. Tests multiple server endpoints and reports results
//
// Usage (in Kubernetes):
//
//	kubectl cp examples/test-client.go $POD:/workspace/test-client.go
//	kubectl exec $POD -- go run test-client.go
//
// Usage (local with SPIRE):
//
//	export SPIFFE_ENDPOINT_SOCKET=unix:///tmp/spire-agent/public/api.sock
//	go run examples/test-client.go
//
// Environment variables:
//
//	SPIFFE_ENDPOINT_SOCKET - Path to SPIRE agent socket (optional, auto-detected)
//
// Note: This is a testing/verification tool. For production client code, see the
// internal/adapters/outbound/httpclient package which provides a production-ready
// SPIFFE HTTP client with advanced features.
package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

	"github.com/spiffe/go-spiffe/v2/spiffeid"
	"github.com/spiffe/go-spiffe/v2/spiffetls/tlsconfig"
	"github.com/spiffe/go-spiffe/v2/workloadapi"
)

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Connect to SPIRE agent and get X.509 SVID
	src, err := workloadapi.NewX509Source(ctx)
	if err != nil {
		log.Fatal(err)
	}
	defer src.Close()

	// Allow any server in example.org trust domain
	td := spiffeid.RequireTrustDomainFromString("example.org")
	tlsCfg := tlsconfig.MTLSClientConfig(src, src, tlsconfig.AuthorizeMemberOf(td))

	// Create HTTP client
	client := &http.Client{
		Transport: &http.Transport{TLSClientConfig: tlsCfg},
		Timeout:   10 * time.Second,
	}

	// Test different endpoints
	endpoints := []string{
		"https://mtls-server:8443/",
		"https://mtls-server:8443/api/hello",
		"https://mtls-server:8443/api/identity",
		"https://mtls-server:8443/health",
	}

	for _, url := range endpoints {
		fmt.Printf("\n=== Testing: %s ===\n", url)
		resp, err := client.Get(url)
		if err != nil {
			log.Printf("ERROR: %v\n", err)
			continue
		}
		body, _ := io.ReadAll(resp.Body)
		if err := resp.Body.Close(); err != nil {
			log.Printf("Warning: error closing response body: %v\n", err)
		}
		fmt.Printf("Status: %d\n", resp.StatusCode)
		fmt.Printf("Body: %s\n", string(body))
	}
}
