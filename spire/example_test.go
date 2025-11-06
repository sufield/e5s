package spire_test

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/sufield/e5s/spire"
)

// ExampleNewIdentitySource demonstrates creating an identity source
// that connects to the SPIRE Workload API for automatic certificate rotation.
//
// This example requires a running SPIRE agent.
func ExampleNewIdentitySource() {
	ctx := context.Background()

	// Connect to SPIRE with default configuration
	// (auto-detects socket location)
	source, err := spire.NewIdentitySource(ctx, spire.Config{})
	if err != nil {
		log.Fatalf("Unable to create identity source: %v", err)
	}
	defer source.Close()

	// Get the X509Source for use with TLS configurations
	x509Source := source.X509Source()

	// The x509Source can now be used with spiffehttp.NewServerTLSConfig
	// or spiffehttp.NewClientTLSConfig
	_ = x509Source
}

// ExampleNewIdentitySource_customSocket demonstrates specifying
// a custom SPIRE agent socket location.
func ExampleNewIdentitySource_customSocket() {
	ctx := context.Background()

	// Connect to SPIRE at a custom socket location
	source, err := spire.NewIdentitySource(ctx, spire.Config{
		WorkloadSocket: "unix:///custom/path/to/agent.sock",
	})
	if err != nil {
		log.Fatalf("Unable to create identity source: %v", err)
	}
	defer source.Close()

	x509Source := source.X509Source()
	_ = x509Source
}

// ExampleNewIdentitySource_tcpEndpoint demonstrates connecting to
// a remote SPIRE agent over TCP.
func ExampleNewIdentitySource_tcpEndpoint() {
	ctx := context.Background()

	// Connect to remote SPIRE agent via TCP
	source, err := spire.NewIdentitySource(ctx, spire.Config{
		WorkloadSocket: "tcp://spire-agent.example.org:8081",
	})
	if err != nil {
		log.Fatalf("Unable to create identity source: %v", err)
	}
	defer source.Close()

	x509Source := source.X509Source()
	_ = x509Source
}

// ExampleNewIdentitySource_initialFetchTimeout demonstrates configuring
// the timeout for the initial identity fetch from SPIRE.
func ExampleNewIdentitySource_initialFetchTimeout() {
	ctx := context.Background()

	// Set a custom timeout for initial certificate fetch
	// Useful in production where you want to fail fast
	source, err := spire.NewIdentitySource(ctx, spire.Config{
		InitialFetchTimeout: 10 * time.Second, // Fail after 10 seconds
	})
	if err != nil {
		log.Fatalf("Unable to create identity source: %v", err)
	}
	defer source.Close()

	x509Source := source.X509Source()
	_ = x509Source
}

// ExampleIdentitySource_Close demonstrates proper cleanup of the identity source.
func ExampleIdentitySource_Close() {
	ctx := context.Background()

	source, err := spire.NewIdentitySource(ctx, spire.Config{})
	if err != nil {
		log.Fatal(err)
	}

	// Always close the source when done
	// This stops the background certificate rotation
	defer source.Close()

	// Use source...
	_ = source.X509Source()

	// Close is called automatically via defer when function exits
}

// Example_socketAutoDetection demonstrates the automatic socket detection
// order used by the identity source.
func Example_socketAutoDetection() {
	// The identity source auto-detects the SPIRE agent socket in this order:
	// 1. Config.WorkloadSocket (if provided)
	// 2. SPIFFE_ENDPOINT_SOCKET environment variable
	// 3. /tmp/spire-agent/public/api.sock (common default)
	// 4. /var/run/spire/sockets/agent.sock (alternate location)

	ctx := context.Background()

	// No socket specified - uses auto-detection
	source, err := spire.NewIdentitySource(ctx, spire.Config{})
	if err != nil {
		log.Fatal(err)
	}
	defer source.Close()

	fmt.Println("Connected to SPIRE agent")
}

// Example_certificateRotation demonstrates that certificate rotation
// happens automatically in the background.
func Example_certificateRotation() {
	ctx := context.Background()

	source, err := spire.NewIdentitySource(ctx, spire.Config{})
	if err != nil {
		log.Fatal(err)
	}
	defer source.Close()

	// The identity source automatically:
	// - Rotates certificates before they expire
	// - Updates trust bundles when they change
	// - Maintains a persistent connection to SPIRE
	//
	// No manual intervention required!

	x509Source := source.X509Source()

	// This TLS config will always use the latest certificates
	// from SPIRE, even after rotation occurs
	_ = x509Source
}
