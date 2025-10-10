package workloadapi_test

// Workload API Response Tests
//
// These tests verify X509SVIDResponse accessor methods and nil safety.
// Tests cover GetSPIFFEID, GetX509SVID, GetExpiresAt, ToIdentity, and nil handling.
//
// Run these tests with:
//
//	go test ./internal/adapters/outbound/workloadapi/... -v -run TestX509SVIDResponse
//	go test ./internal/adapters/outbound/workloadapi/... -cover

import (
	"testing"

	wlapi "github.com/pocket/hexagon/spire/internal/adapters/outbound/workloadapi"
	"github.com/stretchr/testify/assert"
)

// TestX509SVIDResponse_Methods tests response accessor methods
func TestX509SVIDResponse_Methods(t *testing.T) {
	t.Parallel()

	resp := &wlapi.X509SVIDResponse{
		SPIFFEID:  "spiffe://example.org/test",
		X509SVID:  "PEM data",
		ExpiresAt: 1234567890,
	}

	assert.Equal(t, "spiffe://example.org/test", resp.GetSPIFFEID())
	assert.Equal(t, "PEM data", resp.GetX509SVID())
	assert.Equal(t, int64(1234567890), resp.GetExpiresAt())
	assert.Equal(t, "spiffe://example.org/test", resp.ToIdentity())
}

// TestX509SVIDResponse_NilSafety tests nil response safety
func TestX509SVIDResponse_NilSafety(t *testing.T) {
	t.Parallel()

	var resp *wlapi.X509SVIDResponse

	assert.Empty(t, resp.GetSPIFFEID())
	assert.Empty(t, resp.GetX509SVID())
	assert.Equal(t, int64(0), resp.GetExpiresAt())
	assert.Empty(t, resp.ToIdentity())
}
