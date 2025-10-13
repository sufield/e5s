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
//
// Design Philosophy:
//   - Uses typed SDK builders (FromString, FromSegments) for proper validation
//   - Avoids string round-trips by using TranslateTrustDomainToSPIFFEID helper
//   - Minimal normalization: TrimSpace only, let SDK enforce SPIFFE spec
//   - Consistent error wrapping with %w for errors.Is/As compatibility
//
// Concurrency: Safe for concurrent use (stateless, pure functions).
type IdentityCredentialParser struct{}

// NewIdentityCredentialParser creates a new SDK-based identity credential parser
func NewIdentityCredentialParser() ports.IdentityCredentialParser {
	return &IdentityCredentialParser{}
}

// ParseFromString parses and validates a SPIFFE ID from a URI string.
//
// Validation:
//   - Trims whitespace from input
//   - Validates SPIFFE URI scheme (must be "spiffe://")
//   - Validates trust domain format (DNS name)
//   - Normalizes path components per SPIFFE spec
//
// Format: spiffe://<trust-domain>/<path>
// Example: spiffe://example.org/workload/server
//
// Error handling:
//   - Returns domain.ErrInvalidIdentityCredential (wrapped with %w) for all failures
//   - SDK errors are chained for detailed context
//
// Note: ctx is unused (kept for interface parity with port definition).
func (p *IdentityCredentialParser) ParseFromString(_ context.Context, id string) (*domain.IdentityCredential, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return nil, fmt.Errorf("%w: identity credential cannot be empty", domain.ErrInvalidIdentityCredential)
	}

	// Use go-spiffe SDK for validation and parsing
	// SDK enforces SPIFFE spec: scheme, DNS trust domain, path normalization
	spiffeID, err := spiffeid.FromString(id)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", domain.ErrInvalidIdentityCredential, err)
	}

	// Convert to domain IdentityCredential via translation helper
	return TranslateSPIFFEIDToIdentityCredential(spiffeID)
}

// ParseFromPath builds a SPIFFE ID from a trust domain and a path.
//
// Path Semantics:
//   - Uses FromSegments for type-safe construction (avoids string manipulation)
//   - Root IDs: empty path or "/" → zero segments → "spiffe://example.org"
//   - Non-root: splits path into segments, filters empties (handles "//" gracefully)
//   - SDK validates each segment per SPIFFE spec
//
// Segment Extraction:
//   - Trims leading/trailing slashes
//   - Splits on "/" and filters empty strings
//   - Empty result → root ID (zero segments)
//
// Examples:
//   - ParseFromPath(td, "")          → spiffe://example.org (root)
//   - ParseFromPath(td, "/")         → spiffe://example.org (root)
//   - ParseFromPath(td, "workload")  → spiffe://example.org/workload
//   - ParseFromPath(td, "/svc/api")  → spiffe://example.org/svc/api
//   - ParseFromPath(td, "//svc//a")  → spiffe://example.org/svc/a (empties filtered)
//
// Error handling:
//   - Returns domain.ErrInvalidIdentityCredential for nil trust domain or invalid segments
//   - Uses TranslateTrustDomainToSPIFFEID for type-safe TD conversion (no string round-trip)
//
// Note: ctx is unused (kept for interface parity with port definition).
func (p *IdentityCredentialParser) ParseFromPath(_ context.Context, trustDomain *domain.TrustDomain, path string) (*domain.IdentityCredential, error) {
	// Use typed translator to avoid string round-trip
	sdkTD, err := TranslateTrustDomainToSPIFFEID(trustDomain)
	if err != nil {
		// TranslateTrustDomainToSPIFFEID already wraps with domain.ErrInvalidTrustDomain
		// Wrap again with IdentityCredential context for this method's error contract
		return nil, fmt.Errorf("%w: %w", domain.ErrInvalidIdentityCredential, err)
	}

	// Extract segments from path (handles root, empty, double slashes)
	segments := pathToSegments(path)

	// Build SPIFFE ID using FromSegments (type-safe, no string manipulation)
	// Zero segments → root ID (e.g., "spiffe://example.org")
	spiffeID, err := spiffeid.FromSegments(sdkTD, segments...)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", domain.ErrInvalidIdentityCredential, err)
	}

	// Convert to domain IdentityCredential via translation helper
	return TranslateSPIFFEIDToIdentityCredential(spiffeID)
}

// pathToSegments extracts clean path segments from a path string.
//
// Processing:
//  1. TrimSpace: remove leading/trailing whitespace
//  2. Trim slashes: remove leading/trailing "/" (e.g., "/a/b/" → "a/b")
//  3. Split on "/" and filter empty strings (handles "//" gracefully)
//  4. Return nil for root paths (empty or "/" input)
//
// Examples:
//   - pathToSegments("")           → nil (root)
//   - pathToSegments("/")          → nil (root)
//   - pathToSegments("a")          → ["a"]
//   - pathToSegments("/a/b")       → ["a", "b"]
//   - pathToSegments("a//b")       → ["a", "b"] (empty filtered)
//   - pathToSegments("  /a/b/  ")  → ["a", "b"] (trimmed)
//
// This function is package-private and stateless for testability.
func pathToSegments(p string) []string {
	p = strings.TrimSpace(p)
	p = strings.Trim(p, "/")
	if p == "" {
		return nil // root ID
	}
	parts := strings.Split(p, "/")
	out := make([]string, 0, len(parts))
	for _, s := range parts {
		if s != "" {
			out = append(out, s)
		}
	}
	return out
}

// Compile-time interface verification
var _ ports.IdentityCredentialParser = (*IdentityCredentialParser)(nil)
