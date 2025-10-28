package spire

import (
	"context"
	"fmt"
	"strings"

	"github.com/spiffe/go-spiffe/v2/spiffeid"

	"github.com/sufield/e5s/internal/domain"
	"github.com/sufield/e5s/internal/ports"
)

// IdentityCredentialParser implements the IdentityCredentialParser port using go-spiffe SDK.
//
// Design Philosophy:
//   - Uses typed SDK builders (FromString, FromSegments) for proper validation
//   - Minimal normalization: TrimSpace only, let SDK enforce SPIFFE spec
//   - Consistent error wrapping with %w and context prefixes for production logs
//
// Concurrency: Safe for concurrent use (stateless, pure functions).
type IdentityCredentialParser struct{}

// NewIdentityCredentialParser creates a new SDK-based identity credential parser
func NewIdentityCredentialParser() ports.IdentityCredentialParser {
	return &IdentityCredentialParser{}
}

// ParseFromString validates and parses a SPIFFE ID (e.g., "spiffe://example.org/svc/a").
//
// Validation:
//   - Trims whitespace from input
//   - Validates SPIFFE URI scheme (must be "spiffe://")
//   - Validates trust domain format (DNS name)
//   - Normalizes path components per SPIFFE spec
//
// Error handling:
//   - Returns domain.ErrInvalidIdentityCredential (wrapped with %w) for all failures
//   - Errors include context prefix for production logs
//
// Note: ctx is unused (kept for interface parity with port definition).
func (p *IdentityCredentialParser) ParseFromString(_ context.Context, id string) (*domain.IdentityCredential, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return nil, fmt.Errorf("%w: empty input", domain.ErrInvalidIdentityCredential)
	}

	// Use go-spiffe SDK for validation and parsing
	// SDK enforces SPIFFE spec: scheme, DNS trust domain, path normalization
	spiffeID, err := spiffeid.FromString(id)
	if err != nil {
		return nil, fmt.Errorf("%w: parse SPIFFE ID: %w", domain.ErrInvalidIdentityCredential, err)
	}

	// Convert to domain IdentityCredential via translation helper
	return TranslateSPIFFEIDToIdentityCredential(spiffeID)
}

// ParseFromPath constructs a SPIFFE ID from a trust domain and path.
//
// Path Semantics:
//   - Root IDs: empty path or "/" → "spiffe://<td>"
//   - Non-root: splits path into segments, filters empties (handles "//" gracefully)
//
// Examples:
//   - ParseFromPath(td, "")          → spiffe://example.org (root)
//   - ParseFromPath(td, "/")         → spiffe://example.org (root)
//   - ParseFromPath(td, "workload")  → spiffe://example.org/workload
//   - ParseFromPath(td, "/svc/api")  → spiffe://example.org/svc/api
//   - ParseFromPath(td, "//svc//a")  → spiffe://example.org/svc/a (empties filtered)
//
// Error handling:
//   - Returns domain.ErrInvalidIdentityCredential for nil/empty trust domain or invalid segments
//   - Errors include context prefixes for production logs
//
// Note: ctx is unused (kept for interface parity with port definition).
func (p *IdentityCredentialParser) ParseFromPath(_ context.Context, trustDomain *domain.TrustDomain, path string) (*domain.IdentityCredential, error) {
	// Sharp nil guard: fail fast with clear error before SDK translation
	if trustDomain == nil || trustDomain.IsZero() {
		return nil, fmt.Errorf("%w: trust domain is nil/empty", domain.ErrInvalidIdentityCredential)
	}

	// Translate trust domain to SDK type (avoids string round-trip)
	td, err := TranslateTrustDomainToSPIFFEID(trustDomain)
	if err != nil {
		return nil, fmt.Errorf("%w: translate trust domain: %w", domain.ErrInvalidIdentityCredential, err)
	}

	// Extract segments from path (handles root, empty, double slashes)
	segments := segmentsFromPath(path)

	// Build SPIFFE ID using FromSegments (type-safe, no string manipulation)
	// Zero segments → root ID (e.g., "spiffe://example.org")
	spiffeID, err := spiffeid.FromSegments(td, segments...)
	if err != nil {
		return nil, fmt.Errorf("%w: build SPIFFE ID from segments: %w", domain.ErrInvalidIdentityCredential, err)
	}

	// Convert to domain IdentityCredential via translation helper
	return TranslateSPIFFEIDToIdentityCredential(spiffeID)
}

// segmentsFromPath splits path on "/" and filters empties; returns nil for root.
//
// Processing:
//  1. TrimSpace: remove leading/trailing whitespace
//  2. Trim slashes: remove leading/trailing "/" (e.g., "/a/b/" → "a/b")
//  3. FieldsFunc: split and filter empty segments in one pass (handles "//" gracefully)
//  4. Return nil for root paths (empty or "/" input)
//
// Examples:
//   - segmentsFromPath("")           → nil (root)
//   - segmentsFromPath("/")          → nil (root)
//   - segmentsFromPath("a")          → ["a"]
//   - segmentsFromPath("/a/b")       → ["a", "b"]
//   - segmentsFromPath("a//b")       → ["a", "b"] (empty filtered)
//   - segmentsFromPath("  /a/b/  ")  → ["a", "b"] (trimmed)
//
// Note: Returning nil for root is intentional—spiffeid.FromSegments(td) yields the root ID.
func segmentsFromPath(p string) []string {
	p = strings.TrimSpace(p)
	p = strings.Trim(p, "/")
	if p == "" {
		return nil // root ID
	}
	// FieldsFunc avoids empty segments from double slashes in one pass
	return strings.FieldsFunc(p, func(r rune) bool { return r == '/' })
}

// Compile-time interface verification
var _ ports.IdentityCredentialParser = (*IdentityCredentialParser)(nil)
