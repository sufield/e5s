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
		resp.Body.Close()
		fmt.Printf("Status: %d\n", resp.StatusCode)
		fmt.Printf("Body: %s\n", string(body))
	}
}
