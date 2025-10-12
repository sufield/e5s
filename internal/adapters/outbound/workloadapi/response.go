// response.go contains response types and validation methods for the workloadapi package.
package workloadapi

import (
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/pocket/hexagon/spire/internal/ports"
	"github.com/spiffe/go-spiffe/v2/spiffeid"
)

const (
	// SpiffePrefix is the required prefix for all SPIFFE IDs.
	SpiffePrefix = "spiffe://"

	// maxPEMLengthBytes limits the maximum PEM certificate size to prevent oversized payloads.
	maxPEMLengthBytes = 1 << 20 // 1 MiB

	// expirySkewTolerance allows small clock skew between expires_at and certificate NotAfter.
	expirySkewTolerance = 5 * time.Minute
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
// Security validations:
//   - SPIFFE ID: Uses spiffeid.FromString for proper scheme, trust domain, and path validation
//   - X.509 SVID: Parses PEM, verifies it's a valid certificate, extracts URI SAN SPIFFE ID
//   - Expiration: Checks expires_at matches certificate NotAfter (within clock skew tolerance)
//   - Size limits: Enforces maximum PEM size to prevent DoS
//   - Whitespace: Trims and validates after normalization
//
// Returns:
//   - error: Non-nil if validation fails with detailed error message
func (r *X509SVIDResponse) Validate() error {
	if r == nil {
		return errors.New("response cannot be nil")
	}

	// Trim whitespace from all fields
	r.SPIFFEID = strings.TrimSpace(r.SPIFFEID)
	r.X509SVID = strings.TrimSpace(r.X509SVID)

	// Basic field presence checks
	if r.SPIFFEID == "" {
		return errors.New("SPIFFE ID cannot be empty")
	}

	// Strong SPIFFE ID validation using go-spiffe SDK
	id, err := spiffeid.FromString(r.SPIFFEID)
	if err != nil {
		return fmt.Errorf("invalid SPIFFE ID: %w", err)
	}

	if r.X509SVID == "" {
		return errors.New("X.509 SVID certificate cannot be empty")
	}
	if len(r.X509SVID) > maxPEMLengthBytes {
		return fmt.Errorf("X.509 SVID too large (> %d bytes)", maxPEMLengthBytes)
	}

	if r.ExpiresAt <= 0 {
		return fmt.Errorf("invalid expiration timestamp: got %d", r.ExpiresAt)
	}

	// Parse PEM and confirm it's a valid certificate
	cert, err := parseLeafCertPEM(r.X509SVID)
	if err != nil {
		return fmt.Errorf("invalid X.509 SVID: %w", err)
	}

	// Check the certificate's SPIFFE ID matches the response field
	certID, err := spiffeid.FromString(spiffeIDFromCert(cert))
	if err != nil {
		return fmt.Errorf("certificate does not contain a valid SPIFFE ID: %w", err)
	}
	if certID != id {
		return fmt.Errorf("SPIFFE ID mismatch: header %q, cert %q", id.String(), certID.String())
	}

	// Check expiry consistency (tolerate small clock skew)
	exp := time.Unix(r.ExpiresAt, 0).UTC()
	notAfter := cert.NotAfter.UTC()
	if exp.Before(notAfter.Add(-expirySkewTolerance)) || exp.After(notAfter.Add(expirySkewTolerance)) {
		return fmt.Errorf("expires_at (%s) does not match cert NotAfter (%s)", exp, notAfter)
	}

	// Optional: Check if certificate is already expired at validation time
	// Commented out because some use cases may need to process expired certs
	// if time.Now().After(notAfter) {
	//     return errors.New("certificate already expired")
	// }

	return nil
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

// parseLeafCertPEM parses a PEM-encoded certificate and validates it.
//
// Returns:
//   - *x509.Certificate: Parsed certificate
//   - error: Non-nil if PEM decode fails, not a CERTIFICATE block, or x509 parsing fails
func parseLeafCertPEM(pemStr string) (*x509.Certificate, error) {
	block, _ := pem.Decode([]byte(pemStr))
	if block == nil || block.Type != "CERTIFICATE" || len(block.Bytes) == 0 {
		return nil, errors.New("PEM decode failed or not a CERTIFICATE")
	}
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("x509 parse failed: %w", err)
	}
	return cert, nil
}

// spiffeIDFromCert extracts the SPIFFE ID (first URI SAN) from the certificate.
//
// Returns:
//   - string: SPIFFE ID from URI SAN, or empty string if not present
func spiffeIDFromCert(cert *x509.Certificate) string {
	for _, u := range cert.URIs {
		if u != nil && strings.EqualFold(u.Scheme, "spiffe") {
			return u.String()
		}
	}
	return ""
}

// Compile-time interface compliance verification
var (
	_ ports.X509SVIDResponse = (*X509SVIDResponse)(nil)
)
