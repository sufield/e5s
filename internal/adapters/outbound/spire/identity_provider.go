package spire

import (
	"context"
	"fmt"

	"github.com/pocket/hexagon/spire/internal/domain"
	"github.com/spiffe/go-spiffe/v2/spiffeid"
)

// FetchX509SVID fetches an X.509 SVID from SPIRE and converts it to an IdentityDocument.
//
// SVID Selection:
// - If multiple SVIDs are available, prefers the one matching the configured trust domain
// - Falls back to the first SVID if no match or no trust domain configured
//
// Security notes:
// - Validates that certificate chain and private key are present
// - The private key is embedded in the domain document for mTLS operations
// - Makes an RPC call each time (consider caching via X509Source for production)
//
// This method is safe for concurrent use.
func (c *SPIREClient) FetchX509SVID(ctx context.Context) (*domain.IdentityDocument, error) {
	// Apply client timeout only if no deadline exists and timeout is valid
	if _, ok := ctx.Deadline(); !ok && c.timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, c.timeout)
		defer cancel()
	}

	// Fetch X.509 context from SPIRE Workload API
	x509Ctx, err := c.client.FetchX509Context(ctx)
	if err != nil {
		return nil, fmt.Errorf("fetch X.509 context: %w", err)
	}

	// Validate we received at least one SVID
	if len(x509Ctx.SVIDs) == 0 {
		return nil, domain.ErrNoAttestationData
	}

	// Select SVID deterministically:
	// Prefer the SVID matching our configured trust domain if available
	svid := x509Ctx.SVIDs[0] // Default to first
	if c.trustDomain != "" {
		tdWanted, tdErr := spiffeid.TrustDomainFromString(c.trustDomain)
		if tdErr == nil {
			// Scan for matching trust domain
			for _, s := range x509Ctx.SVIDs {
				if s.ID.TrustDomain() == tdWanted {
					svid = s
					break
				}
			}
		}
	}

	// Validate certificate chain is present and valid
	if len(svid.Certificates) == 0 || svid.Certificates[0] == nil {
		return nil, fmt.Errorf("%w: empty or invalid certificate chain", domain.ErrIdentityDocumentInvalid)
	}

	// Validate private key is present (required for mTLS)
	if svid.PrivateKey == nil {
		return nil, fmt.Errorf("%w: private key missing", domain.ErrIdentityDocumentInvalid)
	}

	// Extract identity credential from SPIFFE ID
	spiffeID := svid.ID
	trustDomain := domain.NewTrustDomainFromName(spiffeID.TrustDomain().String())
	path := spiffeID.Path()

	// Handle empty path (root ID) - domain model uses "/" for root
	if path == "" {
		path = "/"
	}

	identityCredential := domain.NewIdentityCredentialFromComponents(trustDomain, path)

	// Create identity document from SVID components
	// Note: Private key is embedded for mTLS operations. If your design allows,
	// consider keeping keys internal and exposing only sign/decrypt operations.
	doc := domain.NewIdentityDocumentFromComponents(
		identityCredential,
		svid.Certificates[0], // Leaf certificate
		svid.PrivateKey,      // Private key for mTLS
		svid.Certificates,    // Full chain including leaf
		svid.Certificates[0].NotAfter,
	)

	return doc, nil
}
