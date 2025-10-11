// response.go contains response types and validation methods for the workloadapi package.
package workloadapi

import (
	"errors"
	"fmt"
	"strings"

	"github.com/pocket/hexagon/spire/internal/ports"
)

const (
	// SpiffePrefix is the required prefix for all SPIFFE IDs
	SpiffePrefix = "spiffe://"
)

// X509SVIDResponse is the response format for X.509 SVID requests.
//
// This struct represents the identity document returned by the Workload API server,
// containing the workload's SPIFFE ID, X.509 certificate (SVID), and expiration time.
//
// JSON Format:
//
//	{
//	  "spiffe_id": "spiffe://example.org/workload",
//	  "x509_svid": "-----BEGIN CERTIFICATE-----\n...",
//	  "expires_at": 1704067200
//	}
//
// Thread Safety: X509SVIDResponse is safe for concurrent reads after creation.
type X509SVIDResponse struct {
	// SPIFFEID is the workload's SPIFFE ID (e.g., "spiffe://example.org/workload")
	SPIFFEID string `json:"spiffe_id"`

	// X509SVID is the PEM-encoded X.509 certificate (leaf certificate)
	X509SVID string `json:"x509_svid"`

	// ExpiresAt is the certificate expiration time as Unix timestamp (seconds since epoch)
	ExpiresAt int64 `json:"expires_at"`
}

// Validate checks that the response contains all required fields with valid values.
//
// Returns:
//   - error: Non-nil if validation fails (empty SPIFFE ID, missing SVID, invalid expiration)
func (r *X509SVIDResponse) Validate() error {
	if r.SPIFFEID == "" {
		return errors.New("SPIFFE ID cannot be empty")
	}
	if !strings.HasPrefix(r.SPIFFEID, SpiffePrefix) {
		return fmt.Errorf("invalid SPIFFE ID format: must start with %q: got %q", SpiffePrefix, r.SPIFFEID)
	}
	if r.X509SVID == "" {
		return errors.New("X.509 SVID certificate cannot be empty")
	}
	if r.ExpiresAt <= 0 {
		return fmt.Errorf("invalid expiration timestamp: must be positive: got %d", r.ExpiresAt)
	}
	return nil
}

// ToIdentity converts the response to a SPIFFE ID string.
//
// This is a convenience method for internal conversion to ports.Identity.
// Returns empty string if response is nil.
func (r *X509SVIDResponse) ToIdentity() string {
	if r == nil {
		return ""
	}
	return r.SPIFFEID
}

// GetSPIFFEID returns the workload's SPIFFE ID.
//
// Returns empty string if response is nil (nil-safe for defensive programming).
func (r *X509SVIDResponse) GetSPIFFEID() string {
	if r == nil {
		return ""
	}
	return r.SPIFFEID
}

// GetX509SVID returns the PEM-encoded X.509 SVID certificate.
//
// The certificate is the leaf certificate in PEM format, which includes the
// SPIFFE ID in the URI SAN (Subject Alternative Name) extension.
//
// Returns empty string if response is nil (nil-safe).
func (r *X509SVIDResponse) GetX509SVID() string {
	if r == nil {
		return ""
	}
	return r.X509SVID
}

// GetExpiresAt returns the certificate expiration time as Unix timestamp.
//
// The timestamp represents seconds since Unix epoch (January 1, 1970 UTC).
// Callers should compare against time.Now().Unix() to check validity.
//
// Returns 0 if response is nil (nil-safe).
func (r *X509SVIDResponse) GetExpiresAt() int64 {
	if r == nil {
		return 0
	}
	return r.ExpiresAt
}

// Compile-time interface compliance verification
var (
	_ ports.X509SVIDResponse = (*X509SVIDResponse)(nil)
)
