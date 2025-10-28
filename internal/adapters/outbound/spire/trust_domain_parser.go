package spire

import (
	"context"
	"fmt"
	"strings"

	"github.com/spiffe/go-spiffe/v2/spiffeid"

	"github.com/sufield/e5s/internal/domain"
	"github.com/sufield/e5s/internal/ports"
)

// TrustDomainParser implements the TrustDomainParser port using go-spiffe SDK.
//
// Design Philosophy:
//   - Early validation for common user mistakes (better DX)
//   - SDK remains authority for canonicalization (handles punycode, normalization)
//   - Stable sentinel error wrapping (%w) for errors.Is compatibility
//   - Stateless, pure functions (concurrency-safe)
//
// Validation Order:
//  1. Empty/whitespace check (fast fail)
//  2. Scheme presence check (common mistake: "spiffe://example.org")
//  3. Path/query check (common mistake: "example.org/foo")
//  4. SDK validation (DNS format, punycode, normalization)
//
// The result is always canonical: lowercased, no scheme, no path.
//
// Note: Unicode domains must be provided in ASCII/punycode format (SDK requirement).
// If Unicode support is needed, consider adding idna.Lookup.ToASCII() conversion.
//
// Concurrency: Safe for concurrent use (stateless, no shared state).
type TrustDomainParser struct{}

// NewTrustDomainParser creates a new SDK-based trust domain parser.
func NewTrustDomainParser() ports.TrustDomainParser {
	return &TrustDomainParser{}
}

// FromString parses and validates a trust domain name.
//
// Input Processing:
//   - Trims leading/trailing whitespace
//   - Rejects scheme (e.g., "spiffe://") - common mistake
//   - Rejects path/query (e.g., "/" or "?") - common mistake
//   - Delegates to SDK for DNS validation and canonicalization
//
// Canonicalization (via SDK):
//   - Lowercases trust domain (DNS names are case-insensitive)
//   - Handles punycode (e.g., "xn--bcher-kva.example" stays as-is)
//   - Validates DNS name format per SPIFFE spec
//
// Examples:
//   - "example.org"              → "example.org" (canonical)
//   - "EXAMPLE.ORG"              → "example.org" (lowercased)
//   - "  example.org  "          → "example.org" (trimmed)
//   - "spiffe://example.org"     → error "must not include scheme"
//   - "example.org/foo"          → error "must not include path or query"
//   - "example..org"             → error (SDK validates double dots)
//   - ""                         → error "cannot be empty"
//
// Error Contract:
//   - Returns domain.ErrInvalidTrustDomain (wrapped with %w) for all failures
//   - Provides specific context without echoing full user input (log safety)
//   - Chains SDK errors for detailed validation failures
//
// Note: ctx is unused (kept for interface parity with port definition).
//
// Concurrency: Safe for concurrent use (stateless, pure function).
func (p *TrustDomainParser) FromString(_ context.Context, name string) (*domain.TrustDomain, error) {
	// Trim whitespace and check for empty input
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, fmt.Errorf("%w: trust domain cannot be empty", domain.ErrInvalidTrustDomain)
	}

	// DX guards for common mistakes

	// Check for scheme (common mistake: using full SPIFFE ID)
	if strings.Contains(name, "://") {
		return nil, fmt.Errorf("%w: trust domain must not include a scheme", domain.ErrInvalidTrustDomain)
	}

	// Check for path or query components (common mistake: partial URL/SPIFFE ID)
	if strings.ContainsAny(name, "/?#") {
		return nil, fmt.Errorf("%w: trust domain must not include path or query", domain.ErrInvalidTrustDomain)
	}

	// Delegate to SDK for authoritative validation and canonicalization
	// SDK handles: DNS format, punycode, lowercasing, special char rejection
	td, err := spiffeid.TrustDomainFromString(name)
	if err != nil {
		// Wrap SDK error with sentinel for consistent error checking
		return nil, fmt.Errorf("%w: %w", domain.ErrInvalidTrustDomain, err)
	}

	// td.String() returns canonical form (lowercased, normalized)
	return domain.NewTrustDomainFromName(td.String()), nil
}

// Compile-time interface verification
var _ ports.TrustDomainParser = (*TrustDomainParser)(nil)
