package domain

import (
	"encoding/json"
	"fmt"
	"strings"
	"unicode"

	"github.com/pocket/hexagon/spire/internal/assert"
)

// IdentityCredential is an immutable value object representing a unique,
// URI-formatted identifier for a workload or agent.
//
// Components:
//   - Trust domain: The namespace/authority for this identity
//   - Path: The hierarchical identifier within the trust domain
//   - URI: Canonical string representation (precomputed, immutable)
//
// Format: spiffe://<trust-domain>/<path>
// Example: spiffe://example.org/workload/server
//
// Design Philosophy:
//   - Immutable: All fields computed once at construction, never modified
//   - Normalized: Path normalized at construction (leading slash, collapsed //)
//   - Zero-value safe: Provides IsZero() to detect uninitialized instances
//   - Serializable: Implements json.Marshaler for ergonomics
//
// SPIFFE Context:
//
//	This type models what SPIFFE calls a "SPIFFE ID". The scheme "spiffe://"
//	is hardcoded as this service is SPIFFE-specific. Parsing and validation
//	are delegated to IdentityCredentialParser port (implemented in adapters
//	using go-spiffe SDK).
//
// Concurrency: Safe for concurrent use (immutable value object).
type IdentityCredential struct {
	trustDomain *TrustDomain
	path        string
	uri         string // Precomputed canonical representation
}

// NewIdentityCredentialFromComponents creates an IdentityCredential from
// validated and normalized components.
//
// ⚠️  WARNING: This constructor PANICS on invalid input. It is intended for use
// by IdentityCredentialParser adapters after SDK validation. For user-facing
// code, prefer using the parser adapter which validates first and returns errors.
//
// Path Requirements (STRICT - panics if violated):
//   - Must be pre-normalized (no consecutive slashes, no trailing slashes)
//   - Must be whitespace-free (no leading, trailing, or internal whitespace including Unicode)
//   - Must not contain traversal segments: "." or ".." (SPIFFE spec forbids path traversal)
//   - Empty or "/" is acceptable (becomes root identity)
//   - Missing leading slash is acceptable (convenience: adds automatically)
//
// This constructor is typically called by IdentityCredentialParser adapters
// after SDK validation (spiffeid.FromString, spiffeid.FromSegments). The SDK
// ensures all paths are normalized and valid per SPIFFE spec. This function
// performs final validation to catch programming errors in test/domain code.
//
// Validation:
//   - trustDomain must be non-nil (panics if nil to catch programming errors early)
//   - path is validated and minimally normalized (see normalizePath)
//   - path must not contain whitespace, //, trailing /, or traversal segments (panics if invalid)
//   - URI precomputed from validated components
//
// Path Normalization:
//   - Empty string → "/"
//   - "workload" → "/workload" (convenience: adds leading slash)
//   - Already normalized paths pass through validation unchanged
//
// Examples:
//
//	NewIdentityCredentialFromComponents(td, "/workload")     → spiffe://example.org/workload
//	NewIdentityCredentialFromComponents(td, "")              → spiffe://example.org/
//	NewIdentityCredentialFromComponents(td, "workload")      → spiffe://example.org/workload (adds /)
//
// Panics:
//   - If trustDomain is nil (programming error, not runtime input validation)
//   - If path contains whitespace, //, trailing /, or traversal segments (normalizePath panics)
//
// Concurrency: Safe for concurrent use (pure function, no shared state).
func NewIdentityCredentialFromComponents(trustDomain *TrustDomain, path string) *IdentityCredential {
	// Guard: nil trust domain is a programming error, not runtime input error
	// Adapters should validate inputs before calling this constructor
	if trustDomain == nil {
		panic(fmt.Errorf("%w: trust domain cannot be nil", ErrInvalidIdentityCredential))
	}

	// Normalize path to ensure canonical representation
	norm := normalizePath(path)

	// Precompute canonical URI (immutable, computed once)
	// SPIFFE scheme is hardcoded as this service is SPIFFE-specific
	uri := "spiffe://" + trustDomain.String() + norm

	ic := &IdentityCredential{
		trustDomain: trustDomain,
		path:        norm,
		uri:         uri,
	}

	// Invariants: Verify normalization and URI construction logic
	assert.Invariant(ic.path != "",
		"normalizePath must return non-empty path (should default to '/' for empty input)")
	assert.Invariant(ic.uri != "" && strings.HasPrefix(ic.uri, "spiffe://") && strings.HasSuffix(ic.uri, ic.path),
		"URI construction must produce valid SPIFFE URI with correct path suffix")

	return ic
}

// normalizePath validates and normalizes a path component for SPIFFE ID construction.
//
// ⚠️  WARNING: This function PANICS on invalid input. It is designed to catch
// programmer errors early. For user input, validation should happen in adapters
// using go-spiffe SDK before calling domain constructors.
//
// Design Philosophy:
//   - Strict validation: Rejects invalid SPIFFE paths (whitespace, traversal segments)
//   - Minimal normalization: Only adds leading slash and handles root case
//   - Trust SDK: Relies on go-spiffe SDK for comprehensive validation in production
//   - Fail fast: Panics on programmer errors to prevent silent data corruption
//
// Validation Rules (PANICS if violated):
//  1. No leading/trailing whitespace (SPIFFE spec violations)
//  2. No internal whitespace (spaces, tabs, newlines, Unicode spaces, etc.)
//  3. No traversal segments: "." or ".." (SPIFFE spec forbids path traversal)
//  4. No consecutive slashes (e.g., "//") (indicates non-normalized input)
//  5. No trailing slashes except root "/" (indicates non-normalized input)
//
// Normalization Rules (non-panicking):
//  1. Empty or "/" → "/" (root identity)
//  2. Missing leading slash → add it (convenience: "foo" → "/foo")
//
// SPIFFE Spec Compliance:
//   - Paths must not contain spaces or special characters (per RFC 3986 unreserved/pct-encoded)
//   - Paths must not have . or .. segments (prevents traversal ambiguity)
//   - Paths should be in canonical form (no // or trailing /)
//
// This strict validation ensures:
//   - Callers don't accidentally pass user input without SDK validation
//   - Test code uses valid SPIFFE paths
//   - Silent corruption (e.g., "path " becoming "path") is impossible
//
// Examples:
//
//	normalizePath("")             → "/"
//	normalizePath("/")            → "/"
//	normalizePath("workload")     → "/workload"  (convenience: adds leading /)
//	normalizePath("/workload")    → "/workload"
//	normalizePath("/db:rw/user")  → "/db:rw/user" (colons allowed per RFC 3986)
//
// Panics (programmer errors):
//
//	normalizePath("  /  ")        → PANIC (leading whitespace at position 0)
//	normalizePath("/path ")       → PANIC (trailing whitespace at position 5)
//	normalizePath(" /path")       → PANIC (leading whitespace at position 0)
//	normalizePath("/path with spaces") → PANIC (internal whitespace at position 5)
//	normalizePath("//foo")        → PANIC (consecutive slashes)
//	normalizePath("/foo/")        → PANIC (trailing slash)
//	normalizePath("/.")           → PANIC (invalid traversal segment ".")
//	normalizePath("/../foo")      → PANIC (invalid traversal segment "..")
//
// Note: go-spiffe SDK (spiffeid.FromString, spiffeid.FromSegments) performs
// comprehensive validation in production. This function catches programming
// errors when paths are constructed directly in tests or domain logic.
//
// Spec reference: SPIFFE ID Standard v1.0
// https://github.com/spiffe/spiffe/blob/main/standards/SPIFFE-ID.md#22-path
func normalizePath(p string) string {
	// Step 1: Handle root identity (empty or single slash)
	if p == "" || p == "/" {
		return "/"
	}

	// Step 2: Validate no whitespace (leading, trailing, or internal)
	// Use IndexFunc for efficiency (short-circuits on first match) and consistent style
	if idx := strings.IndexFunc(p, unicode.IsSpace); idx >= 0 {
		// Determine position type for clear error message
		var posType string
		if idx == 0 {
			posType = "leading"
		} else if idx == len(p)-1 {
			posType = "trailing"
		} else {
			posType = "internal"
		}
		panic(fmt.Errorf("%w: path has %s whitespace at position %d: %q", ErrInvalidIdentityCredential, posType, idx, p))
	}

	// Step 4: Add leading slash if missing (convenience for test code)
	if !strings.HasPrefix(p, "/") {
		p = "/" + p
	}

	// Step 5: Validate no consecutive slashes (should be pre-normalized)
	if strings.Contains(p, "//") {
		panic(fmt.Errorf("%w: path has consecutive slashes (should be normalized): %q", ErrInvalidIdentityCredential, p))
	}

	// Step 6: Validate no trailing slash (should be pre-normalized, except root)
	if len(p) > 1 && strings.HasSuffix(p, "/") {
		panic(fmt.Errorf("%w: path has trailing slash (should be normalized): %q", ErrInvalidIdentityCredential, p))
	}

	// Step 7: Validate no dot or dotdot segments (SPIFFE spec forbids traversal)
	segments := strings.Split(p, "/")
	for _, seg := range segments {
		if seg == "." || seg == ".." {
			panic(fmt.Errorf("%w: path contains invalid traversal segment (%q): %q", ErrInvalidIdentityCredential, seg, p))
		}
	}

	return p
}

// String returns the canonical SPIFFE ID URI.
//
// Format: spiffe://<trust-domain>/<path>
//
// This representation is precomputed at construction and never changes
// (immutability contract).
//
// Example:
//
//	id.String() // "spiffe://example.org/workload/server"
func (i *IdentityCredential) String() string {
	return i.uri
}

// TrustDomain returns the trust domain component.
//
// The trust domain represents the namespace/authority for this identity.
// Never returns nil for properly constructed instances.
//
// Example:
//
//	id.TrustDomain().String() // "example.org"
func (i *IdentityCredential) TrustDomain() *TrustDomain {
	return i.trustDomain
}

// Path returns the normalized path component.
//
// The path is always normalized:
//   - Leading slash present (except root which is "/")
//   - No repeated slashes
//   - Never empty (defaults to "/" for root identity)
//
// Example:
//
//	id.Path() // "/workload/server"
func (i *IdentityCredential) Path() string {
	return i.path
}

// Equals checks if two IdentityCredentials are equal by comparing canonical URIs.
//
// Equality is based on canonical string representation, which ensures:
//   - Case-insensitive trust domains (DNS names)
//   - Normalized paths (no // differences)
//   - Proper SPIFFE ID semantics
//
// Returns false if either instance is nil (nil-safe).
//
// Properties:
//   - Reflexive: i.Equals(i) == true
//   - Symmetric: i.Equals(j) == j.Equals(i)
//   - Transitive: i.Equals(j) && j.Equals(k) → i.Equals(k)
//   - Nil-safe: i.Equals(nil) == false (never panics)
//
// Example:
//
//	id1 := NewIdentityCredentialFromComponents(td, "/workload")
//	id2 := NewIdentityCredentialFromComponents(td, "/workload")
//	id1.Equals(id2) // true
func (i *IdentityCredential) Equals(other *IdentityCredential) bool {
	// Nil-safe check (both receiver and parameter)
	if i == nil || other == nil {
		return false
	}
	// Compare canonical URIs (handles trust domain case-insensitivity)
	return i.uri == other.uri
}

// IsInTrustDomain checks if this identity belongs to the given trust domain.
//
// Returns true if and only if the identity's trust domain equals the provided
// trust domain. This is a convenience method equivalent to:
//
//	i.TrustDomain().Equals(td)
//
// Example:
//
//	id := NewIdentityCredentialFromComponents(td1, "/workload")
//	id.IsInTrustDomain(td1) // true
//	id.IsInTrustDomain(td2) // false
func (i *IdentityCredential) IsInTrustDomain(td *TrustDomain) bool {
	return i.trustDomain.Equals(td)
}

// IsZero checks if this IdentityCredential is uninitialized or invalid.
//
// Returns true if:
//   - Instance is nil
//   - Trust domain is nil (should never happen with NewIdentityCredentialFromComponents)
//   - URI is empty (should never happen with proper construction)
//
// Use this to detect uninitialized values or programming errors.
//
// Example:
//
//	var id *IdentityCredential
//	id.IsZero() // true
//
//	id = NewIdentityCredentialFromComponents(td, "/workload")
//	id.IsZero() // false
func (i *IdentityCredential) IsZero() bool {
	return i == nil || i.trustDomain == nil || i.uri == ""
}

// Key returns a string suitable for use as a map key or set member.
//
// The key is the canonical URI, which ensures proper deduplication and lookup
// semantics in collections.
//
// Example:
//
//	cache := make(map[string]*SomeData)
//	cache[id.Key()] = data
//	data, ok := cache[id.Key()]
func (i *IdentityCredential) Key() string {
	return i.uri
}

// MarshalJSON implements json.Marshaler for JSON serialization.
//
// The identity is serialized as its canonical URI string:
//
//	{"identity": "spiffe://example.org/workload"}
//
// Returns error if the identity is zero-value (nil or invalid).
//
// Example:
//
//	data, err := json.Marshal(id)
//	// data: "spiffe://example.org/workload"
func (i *IdentityCredential) MarshalJSON() ([]byte, error) {
	if i.IsZero() {
		return nil, fmt.Errorf("%w: cannot marshal zero-value identity credential", ErrInvalidIdentityCredential)
	}
	return json.Marshal(i.uri)
}

// UnmarshalJSON implements json.Unmarshaler for JSON deserialization.
//
// This method returns ErrNotSupported to enforce that parsing must go through
// IdentityCredentialParser adapters which use go-spiffe SDK for proper validation.
//
// Rationale:
//   - Keeps domain layer free of SPIFFE parsing logic
//   - Ensures all parsing uses SDK validation (DNS, normalization, security)
//   - Prevents creating invalid IdentityCredential instances via JSON
//
// To deserialize, unmarshal to a string then use IdentityCredentialParser:
//
//	var uriString string
//	json.Unmarshal(data, &uriString)
//	id, err := parser.ParseFromString(ctx, uriString)
func (i *IdentityCredential) UnmarshalJSON(data []byte) error {
	return fmt.Errorf("%w: unmarshaling requires IdentityCredentialParser adapter", ErrNotSupported)
}

// MarshalText implements encoding.TextMarshaler for text serialization.
//
// The identity is serialized as its canonical URI string.
// This is used by various Go encoding packages (YAML, TOML, etc.).
//
// Returns error if the identity is zero-value (nil or invalid).
//
// Example:
//
//	text, err := id.MarshalText()
//	// text: []byte("spiffe://example.org/workload")
func (i *IdentityCredential) MarshalText() ([]byte, error) {
	if i.IsZero() {
		return nil, fmt.Errorf("%w: cannot marshal zero-value identity credential", ErrInvalidIdentityCredential)
	}
	return []byte(i.uri), nil
}

// UnmarshalText implements encoding.TextUnmarshaler for text deserialization.
//
// This method returns ErrNotSupported for the same reasons as UnmarshalJSON:
// parsing must go through IdentityCredentialParser adapters.
//
// To deserialize, use IdentityCredentialParser:
//
//	id, err := parser.ParseFromString(ctx, string(text))
func (i *IdentityCredential) UnmarshalText(text []byte) error {
	return fmt.Errorf("%w: unmarshaling requires IdentityCredentialParser adapter", ErrNotSupported)
}
