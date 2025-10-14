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
	"sync/atomic"

	"github.com/spiffe/go-spiffe/v2/spiffeid"
)

// Sentinel errors for identity operations.
var (
	// ErrNoSPIFFEID indicates no SPIFFE ID is present in the request context.
	ErrNoSPIFFEID = errors.New("no SPIFFE ID in request context")
)

// spiffeIDKeyType is a zero-sized private type for context keys.
// This prevents collisions with other packages using context.
type spiffeIDKeyType struct{}

var spiffeIDKey spiffeIDKeyType

// plog holds the package-level logger atomically (thread-safe swapping).
var plog atomic.Value // *log.Logger

// redactID controls whether SPIFFE IDs are redacted in logs (default: false).
var redactID atomic.Bool

func init() {
	plog.Store(log.New(os.Stdout, "", log.LstdFlags))
	redactID.Store(false)
}

// SetLogger sets a custom logger for identity operations.
// Pass nil to restore the default logger. Thread-safe.
//
// Example:
//
//	customLogger := log.New(logFile, "identity: ", log.LstdFlags)
//	httpcontext.SetLogger(customLogger)
func SetLogger(l *log.Logger) {
	if l == nil {
		l = log.New(os.Stdout, "", log.LstdFlags)
	}
	plog.Store(l)
}

// SetRedactIdentity controls whether SPIFFE IDs are redacted in logs.
// Enable for production environments where IDs may be sensitive. Thread-safe.
//
// Example:
//
//	httpcontext.SetRedactIdentity(true) // Logs show "[redacted]" instead of actual IDs
func SetRedactIdentity(on bool) {
	redactID.Store(on)
}

// GetSPIFFEID extracts the authenticated client SPIFFE ID from request context.
// Returns the ID and true if present, zero value and false otherwise.
// Returns false if request or context is nil.
//
// This is the fast-path variant for hot paths. For production handlers,
// use GetSPIFFEIDOrError for clearer error handling.
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
	if r == nil || r.Context() == nil {
		return spiffeid.ID{}, false
	}
	id, ok := r.Context().Value(spiffeIDKey).(spiffeid.ID)
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
	id, ok := GetSPIFFEID(r)
	if !ok {
		return spiffeid.ID{}, ErrNoSPIFFEID
	}
	return id, nil
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

// MatchesIDParsed checks if the client's SPIFFE ID exactly matches the expected ID.
// Use this when you have a pre-parsed spiffeid.ID to avoid per-request parsing overhead.
//
// Example:
//
//	expectedID := spiffeid.RequireFromString("spiffe://example.org/service/frontend")
//	if httpcontext.MatchesIDParsed(r, expectedID) {
//	    // Specific service identity
//	}
func MatchesIDParsed(r *http.Request, want spiffeid.ID) bool {
	got, ok := GetSPIFFEID(r)
	return ok && got == want
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
	if prefix != "" && !strings.HasPrefix(prefix, "/") {
		prefix = "/" + prefix
	}
	return strings.HasPrefix(path, prefix)
}

// HasPathSuffix checks if the client's SPIFFE ID path ends with the given suffix.
// Automatically normalizes suffix to include leading "/" if missing (for consistency).
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
	// Normalize suffix to include leading slash for consistency with prefix
	if suffix != "" && !strings.HasPrefix(suffix, "/") {
		suffix = "/" + suffix
	}
	return strings.HasSuffix(path, suffix)
}

// Authenticated is a middleware that ensures the request has a valid SPIFFE ID.
// Returns 401 Unauthorized with standard status text if no identity is present.
// This is the canonical authentication middleware - use it as the base for all auth checks.
//
// Example:
//
//	mux.Handle("/api/", httpcontext.Authenticated(apiHandler))
func Authenticated(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, err := GetSPIFFEIDOrError(r); err != nil {
			http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// RequireTrustDomainID is a middleware that ensures the client is from a specific trust domain.
// Returns 403 Forbidden with standard status text if the trust domain doesn't match.
// Use type-safe spiffeid.TrustDomain to avoid parsing overhead and string typos.
//
// Example:
//
//	expectedTD := spiffeid.RequireTrustDomainFromString("example.org")
//	handler := httpcontext.RequireTrustDomainID(expectedTD, apiHandler)
func RequireTrustDomainID(td spiffeid.TrustDomain, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !MatchesTrustDomainID(r, td) {
			http.Error(w, http.StatusText(http.StatusForbidden), http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// RequireAnyTrustDomain is a middleware that ensures the client is from one of the allowed trust domains.
// Returns 401 if no identity present, 403 if identity from disallowed trust domain.
// Pre-computes a lookup set for O(1) trust domain checks.
//
// Example:
//
//	allowed := []spiffeid.TrustDomain{
//	    spiffeid.RequireTrustDomainFromString("example.org"),
//	    spiffeid.RequireTrustDomainFromString("partner.com"),
//	}
//	handler := httpcontext.RequireAnyTrustDomain(allowed, apiHandler)
func RequireAnyTrustDomain(allowed []spiffeid.TrustDomain, next http.Handler) http.Handler {
	// Pre-compute lookup set at middleware creation time
	set := make(map[string]struct{}, len(allowed))
	for _, td := range allowed {
		set[td.String()] = struct{}{}
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		td, ok := GetTrustDomain(r)
		if !ok {
			http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
			return
		}
		if _, ok := set[td.String()]; !ok {
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
// Respects the redaction setting (SetRedactIdentity). Useful for debugging and auditing.
// Logs method, path, and identity in structured key-value format.
//
// Example:
//
//	mux.Handle("/api/", httpcontext.LogIdentity(apiHandler))
func LogIdentity(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		idStr := "<unauthenticated>"
		if id, ok := GetSPIFFEID(r); ok {
			if redactID.Load() {
				idStr = "[redacted]"
			} else {
				idStr = id.String()
			}
		}
		plog.Load().(*log.Logger).Printf("identity method=%s path=%s spiffe_id=%q", r.Method, r.URL.Path, idStr)
		next.ServeHTTP(w, r)
	})
}
