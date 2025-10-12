package spire

import (
	"context"
	"fmt"
	"strings"

	"github.com/pocket/hexagon/spire/internal/domain"
	"github.com/pocket/hexagon/spire/internal/ports"
	"github.com/spiffe/go-spiffe/v2/spiffeid"
)

// IdentityCredentialParser implements the IdentityCredentialParser port using go-spiffe SDK.
// Uses SDK constructors (FromString, FromSegments, FromPath) for proper validation and normalization.
type IdentityCredentialParser struct{}

// NewIdentityCredentialParser creates a new SDK-based identity credential parser
func NewIdentityCredentialParser() ports.IdentityCredentialParser {
	return &IdentityCredentialParser{}
}

// ParseFromString parses and validates a SPIFFE ID from a URI string.
//
// Validation:
// - Trims whitespace from input
// - Validates SPIFFE URI scheme
// - Validates trust domain format
// - Normalizes path components
//
// Format: spiffe://<trust-domain>/<path>
// Example: spiffe://example.org/workload/server
//
// Note: ctx is unused (kept for interface parity).
func (p *IdentityCredentialParser) ParseFromString(_ context.Context, id string) (*domain.IdentityCredential, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return nil, fmt.Errorf("%w: identity credential cannot be empty", domain.ErrInvalidIdentityCredential)
	}

	// Use go-spiffe SDK for validation and parsing
	spiffeID, err := spiffeid.FromString(id)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", domain.ErrInvalidIdentityCredential, err)
	}

	// Convert to domain IdentityCredential via translation helper
	return TranslateSPIFFEIDToIdentityCredential(spiffeID)
}

// ParseFromPath builds a SPIFFE ID from a trust domain and a path.
//
// Path handling:
// - Trims whitespace from inputs
// - Root IDs (empty or "/" path) use FromSegments for proper construction
// - Non-root paths use FromPath with normalization:
//   - Ensures single leading slash
//   - Removes double slashes (//)
//   - SDK handles further normalization
//
// Examples:
//   - ParseFromPath(td, "")          → spiffe://example.org
//   - ParseFromPath(td, "/")         → spiffe://example.org
//   - ParseFromPath(td, "workload")  → spiffe://example.org/workload
//   - ParseFromPath(td, "//svc//a")  → spiffe://example.org/svc/a
//
// Note: ctx is unused (kept for interface parity).
func (p *IdentityCredentialParser) ParseFromPath(_ context.Context, trustDomain *domain.TrustDomain, path string) (*domain.IdentityCredential, error) {
	if trustDomain == nil {
		return nil, fmt.Errorf("%w: trust domain cannot be nil", domain.ErrInvalidIdentityCredential)
	}

	// Parse trust domain using SDK
	td, err := spiffeid.TrustDomainFromString(strings.TrimSpace(trustDomain.String()))
	if err != nil {
		return nil, fmt.Errorf("%w: %w", domain.ErrInvalidIdentityCredential, err)
	}

	// Normalize path input
	path = strings.TrimSpace(path)

	// Build SPIFFE ID using appropriate SDK constructor
	var spiffeID spiffeid.ID
	switch {
	case path == "" || path == "/":
		// Root ID: use FromSegments (zero segments) - avoids manual string building
		spiffeID, err = spiffeid.FromSegments(td /* no segments = root */)
	default:
		// Non-root path: normalize and use FromPath
		// Ensure single leading slash
		if !strings.HasPrefix(path, "/") {
			path = "/" + path
		}
		// Clean up double slashes (defensive - SDK also normalizes)
		for strings.Contains(path, "//") {
			path = strings.ReplaceAll(path, "//", "/")
		}
		spiffeID, err = spiffeid.FromPath(td, path)
	}

	if err != nil {
		return nil, fmt.Errorf("%w: %w", domain.ErrInvalidIdentityCredential, err)
	}

	// Convert to domain IdentityCredential via translation helper
	return TranslateSPIFFEIDToIdentityCredential(spiffeID)
}

var _ ports.IdentityCredentialParser = (*IdentityCredentialParser)(nil)
