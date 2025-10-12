package workloadapi_test

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"net/url"
	"time"
)

// generateTestSPIFFECert generates a valid X.509 certificate with SPIFFE ID in URI SAN.
// Returns PEM-encoded certificate string and expiration timestamp.
func generateTestSPIFFECert(spiffeID string, expiresAt time.Time) (string, int64, error) {
	// Generate private key
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return "", 0, err
	}

	// Parse SPIFFE ID as URL
	spiffeURL, err := url.Parse(spiffeID)
	if err != nil {
		return "", 0, err
	}

	// Create certificate template
	serialNumber, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return "", 0, err
	}

	template := &x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			CommonName: "test-workload",
		},
		NotBefore:             time.Now().Add(-1 * time.Hour),
		NotAfter:              expiresAt,
		KeyUsage:              x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
		BasicConstraintsValid: true,
		URIs:                  []*url.URL{spiffeURL},
	}

	// Self-sign the certificate (for testing only)
	certDER, err := x509.CreateCertificate(rand.Reader, template, template, &priv.PublicKey, priv)
	if err != nil {
		return "", 0, err
	}

	// Encode to PEM
	certPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "CERTIFICATE",
		Bytes: certDER,
	})

	return string(certPEM), expiresAt.Unix(), nil
}
