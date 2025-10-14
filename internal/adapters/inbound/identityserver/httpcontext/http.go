// Package httpcontext provides internal HTTP context utilities for SPIFFE identity handling.
// This package is an internal implementation detail used by identityserver.
// External users should use identityserver.New() and identityserver.GetIdentity() instead.
//
// All functions assume the request context has been populated with the authenticated
// client's identity by the server's TLS authentication layer.
//
// SPIFFE Path Semantics: The path component of a SPIFFE ID is opaque. Prefix/suffix
// checks are application-specific conventions, not standardized SPIFFE semantics.
package httpcontext

import (
	stdcontext "context"
	"errors"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/spiffe/go-spiffe/v2/spiffeid"
)

// Sentinel errors for identity operations.
var (
	// ErrNoSPIFFEID indicates no SPIFFE ID is present in the request context.
	ErrNoSPIFFEID = errors.New("no SPIFFE ID in request context")
)

// contextKey is the type for context keys to avoid collisions.
type contextKey string

const spiffeIDKey contextKey = "spiffe-id"

// logger is the package-level logger that can be swapped for different implementations.
// Default uses standard library log with stdout.
var logger = log.New(os.Stdout, "", log.LstdFlags)

// SetLogger sets a custom logger for identity operations.
// Pass nil to restore the default logger.
//
// Example:
//
//	customLogger := log.New(logFile, "identity: ", log.LstdFlags)
//	httpcontext.SetLogger(customLogger)
func SetLogger(l *log.Logger) {
	if l != nil {
		logger = l
	} else {
		logger = log.New(os.Stdout, "", log.LstdFlags)
	}
}

// GetSPIFFEID extracts the authenticated client SPIFFE ID from request context.
// Returns the ID and true if present, zero value and false otherwise.
// Returns false if request or context is nil.
//
// Example:
//
//	func handler(w http.ResponseWriter, r *http.Request) {
//	    clientID, ok := httpcontext.GetSPIFFEID(r)
//	    if !ok {
//	        http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
//	        return
//	    }
//	    // Use clientID for application logic
//	}
func GetSPIFFEID(r *http.Request) (spiffeid.ID, bool) {
	if r == nil {
		return spiffeid.ID{}, false
	}
	ctx := r.Context()
	if ctx == nil {
		return spiffeid.ID{}, false
	}
	id, ok := ctx.Value(spiffeIDKey).(spiffeid.ID)
	if !ok || id.IsZero() {
		return spiffeid.ID{}, false
	}
	return id, true
}

// GetSPIFFEIDOrError returns the SPIFFE ID or a typed error (no panic paths).
// This is the recommended function for production handlers.
//
// Example:
//
//	func handler(w http.ResponseWriter, r *http.Request) {
//	    clientID, err := httpcontext.GetSPIFFEIDOrError(r)
//	    if err != nil {
//	        http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
//	        return
//	    }
//	    // Use clientID
//	}
func GetSPIFFEIDOrError(r *http.Request) (spiffeid.ID, error) {
	if r == nil || r.Context() == nil {
		return spiffeid.ID{}, ErrNoSPIFFEID
	}
	id, ok := r.Context().Value(spiffeIDKey).(spiffeid.ID)
	if !ok || id.IsZero() {
		return spiffeid.ID{}, ErrNoSPIFFEID
	}
	return id, nil
}

// MustGetSPIFFEID extracts the SPIFFE ID or panics with ErrNoSPIFFEID.
// DISCOURAGED in production handlers. Use GetSPIFFEIDOrError instead.
// Safe only when mTLS middleware guarantees ID presence (e.g., tests).
//
// Example (test only):
//
//	func TestHandler(t *testing.T) {
//	    clientID := httpcontext.MustGetSPIFFEID(req) // Safe: test setup guarantees ID
//	    // Use clientID
//	}
func MustGetSPIFFEID(r *http.Request) spiffeid.ID {
	id, ok := GetSPIFFEID(r)
	if !ok {
		panic(ErrNoSPIFFEID)
	}
	return id
}

// GetTrustDomain extracts the trust domain from the client's SPIFFE ID.
// Returns the trust domain and true if present, zero value and false otherwise.
//
// Example:
//
//	td, ok := httpcontext.GetTrustDomain(r)
//	if ok && td.String() == "example.org" {
//	    // Client from expected trust domain
//	}
func GetTrustDomain(r *http.Request) (spiffeid.TrustDomain, bool) {
	id, ok := GetSPIFFEID(r)
	if !ok {
		return spiffeid.TrustDomain{}, false
	}
	return id.TrustDomain(), true
}

// GetPath extracts the path component from the client's SPIFFE ID.
// Returns the path (including leading slash) and true if present.
//
// Example:
//
//	path, ok := httpcontext.GetPath(r)
//	if ok && strings.HasPrefix(path, "/service/") {
//	    // Client is a service
//	}
func GetPath(r *http.Request) (string, bool) {
	id, ok := GetSPIFFEID(r)
	if !ok {
		return "", false
	}
	return id.Path(), true
}

// MatchesTrustDomain checks if the client's trust domain matches the expected value (string).
// For type-safe comparisons, use MatchesTrustDomainID.
//
// Example:
//
//	if httpcontext.MatchesTrustDomain(r, "example.org") {
//	    // Client from example.org trust domain
//	}
func MatchesTrustDomain(r *http.Request, trustDomain string) bool {
	td, ok := GetTrustDomain(r)
	if !ok {
		return false
	}
	return td.String() == trustDomain
}

// MatchesTrustDomainID checks if the client's trust domain matches the expected TrustDomain type.
// This is type-safe and avoids string parsing/allocation overhead.
//
// Example:
//
//	expectedTD := spiffeid.RequireTrustDomainFromString("example.org")
//	if httpcontext.MatchesTrustDomainID(r, expectedTD) {
//	    // Client from expected trust domain
//	}
func MatchesTrustDomainID(r *http.Request, td spiffeid.TrustDomain) bool {
	id, ok := GetSPIFFEID(r)
	if !ok {
		return false
	}
	return id.TrustDomain() == td
}

// HasPathPrefix checks if the client's SPIFFE ID path starts with the given prefix.
// Automatically normalizes prefix to include leading "/" if missing.
//
// Example:
//
//	if httpcontext.HasPathPrefix(r, "/service/") {
//	    // Client is a service workload
//	}
func HasPathPrefix(r *http.Request, prefix string) bool {
	path, ok := GetPath(r)
	if !ok {
		return false
	}
	// Normalize prefix to include leading slash
	if !strings.HasPrefix(prefix, "/") {
		prefix = "/" + prefix
	}
	return strings.HasPrefix(path, prefix)
}

// HasPathSuffix checks if the client's SPIFFE ID path ends with the given suffix.
//
// Example:
//
//	if httpcontext.HasPathSuffix(r, "/admin") {
//	    // Client has admin role (application-defined convention)
//	}
func HasPathSuffix(r *http.Request, suffix string) bool {
	path, ok := GetPath(r)
	if !ok {
		return false
	}
	return strings.HasSuffix(path, suffix)
}

// GetPathSegments returns the path components of the SPIFFE ID as a slice.
// Empty segments are filtered out. Returns empty slice for root path.
//
// Example:
//
//	// For spiffe://example.org/service/frontend/prod
//	segments, ok := httpcontext.GetPathSegments(r)
//	// segments = []string{"service", "frontend", "prod"}
func GetPathSegments(r *http.Request) ([]string, bool) {
	path, ok := GetPath(r)
	if !ok {
		return nil, false
	}

	// Split and filter empty segments
	segments := strings.Split(strings.Trim(path, "/"), "/")
	result := make([]string, 0, len(segments))
	for _, seg := range segments {
		if seg != "" {
			result = append(result, seg)
		}
	}

	return result, true
}

// MatchesID checks if the client's SPIFFE ID exactly matches the expected ID.
// Parses the expected ID for type safety. Returns false if parsing fails.
//
// Example:
//
//	if httpcontext.MatchesID(r, "spiffe://example.org/service/frontend") {
//	    // Specific service identity
//	}
func MatchesID(r *http.Request, expectedID string) bool {
	want, err := spiffeid.FromString(expectedID)
	if err != nil {
		return false
	}
	got, ok := GetSPIFFEID(r)
	return ok && got == want
}

// GetIDString returns the full SPIFFE ID as a string.
// Returns empty string if ID not present.
//
// Example:
//
//	idStr := httpcontext.GetIDString(r)
//	// idStr = "spiffe://example.org/service/frontend"
func GetIDString(r *http.Request) string {
	id, ok := GetSPIFFEID(r)
	if !ok {
		return ""
	}
	return id.String()
}

// WithSPIFFEID adds a SPIFFE ID to the request context.
// This is primarily used for testing. Returns nil if request is nil.
//
// Example:
//
//	testID := spiffeid.RequireFromString("spiffe://example.org/test")
//	req = httpcontext.WithSPIFFEID(req, testID)
func WithSPIFFEID(r *http.Request, id spiffeid.ID) *http.Request {
	if r == nil {
		return nil
	}
	ctx := stdcontext.WithValue(r.Context(), spiffeIDKey, id)
	return r.WithContext(ctx)
}

// RequireAuthentication is a middleware that ensures the request has a valid SPIFFE ID.
// Returns 401 Unauthorized with standard status text if no identity is present.
//
// Example:
//
//	mux.Handle("/api/", httpcontext.RequireAuthentication(apiHandler))
func RequireAuthentication(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id, ok := GetSPIFFEID(r)
		if !ok || id.IsZero() {
			http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
			return
		}
		// ID verified; pass request through
		next.ServeHTTP(w, r)
	})
}

// RequireTrustDomain is a middleware that ensures the client is from a specific trust domain.
// Returns 403 Forbidden with standard status text if the trust domain doesn't match.
//
// Example:
//
//	handler := httpcontext.RequireTrustDomain("example.org", apiHandler)
func RequireTrustDomain(trustDomain string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !MatchesTrustDomain(r, trustDomain) {
			http.Error(w, http.StatusText(http.StatusForbidden), http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// RequirePathPrefix is a middleware that ensures the client's SPIFFE ID path has a specific prefix.
// Returns 403 Forbidden with standard status text if the path doesn't match.
//
// Example:
//
//	// Only allow service workloads
//	handler := httpcontext.RequirePathPrefix("/service/", apiHandler)
func RequirePathPrefix(prefix string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !HasPathPrefix(r, prefix) {
			http.Error(w, http.StatusText(http.StatusForbidden), http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// LogIdentity is a middleware that logs the client's SPIFFE ID using the configured logger.
// Useful for debugging and auditing. Logs method, path, and identity.
//
// Example:
//
//	mux.Handle("/api/", httpcontext.LogIdentity(apiHandler))
func LogIdentity(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if id, ok := GetSPIFFEID(r); ok {
			logger.Printf("identity method=%s path=%s spiffe_id=%q", r.Method, r.URL.Path, id.String())
		} else {
			logger.Printf("identity method=%s path=%s spiffe_id=<unauthenticated>", r.Method, r.URL.Path)
		}
		next.ServeHTTP(w, r)
	})
}
