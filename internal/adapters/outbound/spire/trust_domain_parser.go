package spire

import (
	"context"
	"fmt"
	"strings"

	"github.com/pocket/hexagon/spire/internal/domain"
	"github.com/pocket/hexagon/spire/internal/ports"
	"github.com/spiffe/go-spiffe/v2/spiffeid"
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
//  3. Path/query check (common mistake: "example.org/foo" or "example.org?x=y")
//  4. SDK validation (DNS format, punycode, normalization)
//
// The result is always canonical: lowercased, no scheme, no path.
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
//   - Rejects scheme (e.g., "spiffe://example.org") with clear error
//   - Rejects path/query (e.g., "example.org/foo" or "example.org?x=y") with clear error
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
//   - "xn--bcher-kva.example"    → "xn--bcher-kva.example" (punycode preserved)
//   - "spiffe://example.org"     → error "must not include scheme"
//   - "example.org/foo"          → error "must not include path or query"
//   - "example..org"             → error (SDK validates double dots)
//   - ""                         → error "cannot be empty"
//
// Error Contract:
//   - Returns domain.ErrInvalidTrustDomain (wrapped with %w) for all failures
//   - Provides specific context for common user mistakes
//   - Chains SDK errors for detailed validation failures
//
// Note: ctx is unused (kept for interface parity with port definition).
//
// Concurrency: Safe for concurrent use (stateless, pure function).
func (p *TrustDomainParser) FromString(_ context.Context, name string) (*domain.TrustDomain, error) {
	// Step 1: Trim whitespace and check for empty input
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, fmt.Errorf("%w: trust domain name cannot be empty", domain.ErrInvalidTrustDomain)
	}

	// Step 2: Early validation for common mistakes (better developer experience)

	// Check for scheme (common mistake: using full SPIFFE ID instead of trust domain)
	// Example mistake: "spiffe://example.org" instead of "example.org"
	if strings.Contains(name, "://") {
		return nil, fmt.Errorf("%w: must not include scheme (got %q)", domain.ErrInvalidTrustDomain, name)
	}

	// Check for path or query components (common mistake: partial SPIFFE ID)
	// Example mistakes: "example.org/workload" or "example.org?param=value"
	if strings.ContainsAny(name, "/?#") {
		return nil, fmt.Errorf("%w: must not include path or query (got %q)", domain.ErrInvalidTrustDomain, name)
	}

	// Step 3: Delegate to SDK for authoritative validation and canonicalization
	// SDK handles:
	// - DNS name format validation (RFC 1035)
	// - Punycode handling (internationalized domain names)
	// - Lowercasing (trust domains are case-insensitive per DNS spec)
	// - Special character rejection (e.g., underscores, double dots)
	td, err := spiffeid.TrustDomainFromString(name)
	if err != nil {
		// Wrap SDK error with our sentinel for consistent error checking
		return nil, fmt.Errorf("%w: %w", domain.ErrInvalidTrustDomain, err)
	}

	// td.String() returns the canonical form (lowercased, normalized)
	// This is the single source of truth for trust domain representation
	return domain.NewTrustDomainFromName(td.String()), nil
}

// Compile-time interface verification
var _ ports.TrustDomainParser = (*TrustDomainParser)(nil)
