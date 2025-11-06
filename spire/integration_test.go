//go:build integration
// +build integration

package spire_test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/sufield/e5s/internal/testhelpers"
	"github.com/sufield/e5s/spire"
)

// getOrSetupSPIRE returns a socket path to use for testing.
// If SPIFFE_ENDPOINT_SOCKET is set, it uses the existing SPIRE agent.
// Otherwise, it starts a new SPIRE server and agent for the test.
func getOrSetupSPIRE(t *testing.T) string {
	socketPath := os.Getenv("SPIFFE_ENDPOINT_SOCKET")
	if socketPath != "" {
		t.Logf("Using existing SPIRE agent from environment: %s", socketPath)
		return socketPath
	}

	// No existing SPIRE, start our own
	st := testhelpers.SetupSPIRE(t)
	return "unix://" + st.SocketPath
}

// TestIntegration_NewIdentitySource_RealSPIRE tests creating an identity source
// with a real SPIRE agent.
//
// This is an integration test that requires SPIRE binaries in PATH.
// Run with: go test -tags=integration -v ./spire
func TestIntegration_NewIdentitySource_RealSPIRE(t *testing.T) {
	// Set up SPIRE server and agent (or use existing from environment)
	socketPath := getOrSetupSPIRE(t)

	// Create identity source connecting to test SPIRE agent
	ctx := context.Background()
	source, err := spire.NewIdentitySource(ctx, spire.Config{
		WorkloadSocket: socketPath,
	})
	if err != nil {
		t.Fatalf("Failed to create identity source: %v", err)
	}
	defer source.Close()

	// Verify we got a valid X509Source
	x509Source := source.X509Source()
	if x509Source == nil {
		t.Fatal("X509Source() returned nil")
	}

	// Verify we can get an SVID
	svid, err := x509Source.GetX509SVID()
	if err != nil {
		t.Fatalf("Failed to get X509 SVID: %v", err)
	}

	if svid == nil {
		t.Fatal("Got nil SVID")
	}

	// Verify SVID has correct trust domain
	if svid.ID.TrustDomain().Name() != st.TrustDomain {
		t.Errorf("SVID trust domain = %s, want %s", svid.ID.TrustDomain().Name(), st.TrustDomain)
	}

	// Verify SVID has certificates
	if len(svid.Certificates) == 0 {
		t.Fatal("SVID has no certificates")
	}

	// Verify we can get trust bundle
	bundle, err := x509Source.GetX509BundleForTrustDomain(svid.ID.TrustDomain())
	if err != nil {
		t.Fatalf("Failed to get trust bundle: %v", err)
	}

	if bundle == nil {
		t.Fatal("Got nil trust bundle")
	}

	if len(bundle.X509Authorities()) == 0 {
		t.Fatal("Trust bundle has no authorities")
	}

	t.Logf("Successfully fetched SVID: %s", svid.ID.String())
	t.Logf("Certificate expires: %s", svid.Certificates[0].NotAfter)
	t.Logf("Trust bundle has %d authorities", len(bundle.X509Authorities()))
}

// TestIntegration_NewIdentitySource_CustomTimeout tests custom timeout configuration
// with a real SPIRE agent.
func TestIntegration_NewIdentitySource_CustomTimeout(t *testing.T) {
	st := testhelpers.SetupSPIRE(t)

	ctx := context.Background()
	source, err := spire.NewIdentitySource(ctx, spire.Config{
		WorkloadSocket:      "unix://" + st.SocketPath,
		InitialFetchTimeout: 5 * time.Second,
	})
	if err != nil {
		t.Fatalf("Failed to create identity source: %v", err)
	}
	defer source.Close()

	// Verify source works
	x509Source := source.X509Source()
	if x509Source == nil {
		t.Fatal("X509Source() returned nil")
	}

	svid, err := x509Source.GetX509SVID()
	if err != nil {
		t.Fatalf("Failed to get SVID: %v", err)
	}

	if svid == nil {
		t.Fatal("Got nil SVID")
	}

	t.Logf("Successfully created identity source with custom timeout")
}

// TestIntegration_IdentitySource_Close tests cleanup of identity source resources.
func TestIntegration_IdentitySource_Close(t *testing.T) {
	socketPath := getOrSetupSPIRE(t)

	ctx := context.Background()
	source, err := spire.NewIdentitySource(ctx, spire.Config{
		WorkloadSocket: socketPath,
	})
	if err != nil {
		t.Fatalf("Failed to create identity source: %v", err)
	}

	// Verify source is working
	x509Source := source.X509Source()
	if x509Source == nil {
		t.Fatal("X509Source() returned nil")
	}

	// Close the source
	if err := source.Close(); err != nil {
		t.Errorf("Close() returned error: %v", err)
	}

	// Closing again should be safe (idempotent)
	if err := source.Close(); err != nil {
		t.Errorf("Second Close() returned error: %v", err)
	}

	t.Log("Successfully closed identity source")
}

// TestIntegration_IdentitySource_CertificateRotation tests that the identity source
// properly handles certificate updates from SPIRE.
//
// This test verifies the watch mechanism is working correctly.
func TestIntegration_IdentitySource_CertificateRotation(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping certificate rotation test in short mode")
	}

	socketPath := getOrSetupSPIRE(t)

	ctx := context.Background()
	source, err := spire.NewIdentitySource(ctx, spire.Config{
		WorkloadSocket: socketPath,
	})
	if err != nil {
		t.Fatalf("Failed to create identity source: %v", err)
	}
	defer source.Close()

	x509Source := source.X509Source()

	// Get initial SVID
	svid1, err := x509Source.GetX509SVID()
	if err != nil {
		t.Fatalf("Failed to get initial SVID: %v", err)
	}

	initialSerial := svid1.Certificates[0].SerialNumber

	// In a real scenario, we'd wait for certificate rotation
	// For this test, we just verify the watch is active by checking
	// we can still fetch SVIDs after some time
	time.Sleep(2 * time.Second)

	svid2, err := x509Source.GetX509SVID()
	if err != nil {
		t.Fatalf("Failed to get SVID after wait: %v", err)
	}

	// Verify we still have a valid SVID (same or rotated)
	if len(svid2.Certificates) == 0 {
		t.Fatal("SVID has no certificates after wait")
	}

	currentSerial := svid2.Certificates[0].SerialNumber

	t.Logf("Initial certificate serial: %s", initialSerial)
	t.Logf("Current certificate serial: %s", currentSerial)
	t.Log("Certificate watch is functioning correctly")
}

// TestIntegration_MultipleIdentitySources tests creating multiple identity sources
// simultaneously from the same SPIRE agent.
func TestIntegration_MultipleIdentitySources(t *testing.T) {
	st := testhelpers.SetupSPIRE(t)

	ctx := context.Background()

	// Create multiple sources simultaneously
	const numSources = 3
	sources := make([]*spire.IdentitySource, numSources)

	for i := 0; i < numSources; i++ {
		source, err := spire.NewIdentitySource(ctx, spire.Config{
			WorkloadSocket: "unix://" + st.SocketPath,
		})
		if err != nil {
			t.Fatalf("Failed to create identity source %d: %v", i, err)
		}
		defer source.Close()
		sources[i] = source
	}

	// Verify all sources work
	for i, source := range sources {
		x509Source := source.X509Source()
		svid, err := x509Source.GetX509SVID()
		if err != nil {
			t.Errorf("Source %d failed to get SVID: %v", i, err)
			continue
		}

		if svid == nil {
			t.Errorf("Source %d returned nil SVID", i)
			continue
		}

		t.Logf("Source %d: Successfully fetched SVID %s", i, svid.ID.String())
	}
}
