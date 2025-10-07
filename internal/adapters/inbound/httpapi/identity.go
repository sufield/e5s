package httpapi

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/spiffe/go-spiffe/v2/spiffeid"
)

// contextKey is the type for context keys to avoid collisions.
type contextKey string

const spiffeIDKey contextKey = "spiffe-id"

// GetSPIFFEID extracts the authenticated client SPIFFE ID from request context.
// Returns the ID and true if present, zero value and false otherwise.
//
// Example:
//
//	func handler(w http.ResponseWriter, r *http.Request) {
//	    clientID, ok := httpapi.GetSPIFFEID(r)
//	    if !ok {
//	        http.Error(w, "No client identity", http.StatusUnauthorized)
//	        return
//	    }
//	    // Use clientID for application logic
//	}
func GetSPIFFEID(r *http.Request) (spiffeid.ID, bool) {
	id, ok := r.Context().Value(spiffeIDKey).(spiffeid.ID)
	return id, ok
}

// MustGetSPIFFEID extracts the SPIFFE ID or panics.
// Use only in handlers where mTLS middleware guarantees ID presence.
//
// Example:
//
//	func handler(w http.ResponseWriter, r *http.Request) {
//	    clientID := httpapi.MustGetSPIFFEID(r) // Safe: mTLS middleware ensures ID
//	    // Use clientID
//	}
func MustGetSPIFFEID(r *http.Request) spiffeid.ID {
	id, ok := GetSPIFFEID(r)
	if !ok {
		panic("SPIFFE ID not found in request context")
	}
	return id
}

// GetTrustDomain extracts the trust domain from the client's SPIFFE ID.
// Returns the trust domain and true if present, zero value and false otherwise.
//
// Example:
//
//	td, ok := httpapi.GetTrustDomain(r)
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
//	path, ok := httpapi.GetPath(r)
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

// MatchesTrustDomain checks if the client's trust domain matches the expected value.
//
// Example:
//
//	if httpapi.MatchesTrustDomain(r, "example.org") {
//	    // Client from example.org trust domain
//	}
func MatchesTrustDomain(r *http.Request, trustDomain string) bool {
	td, ok := GetTrustDomain(r)
	if !ok {
		return false
	}
	return td.String() == trustDomain
}

// HasPathPrefix checks if the client's SPIFFE ID path starts with the given prefix.
//
// Example:
//
//	if httpapi.HasPathPrefix(r, "/service/") {
//	    // Client is a service workload
//	}
func HasPathPrefix(r *http.Request, prefix string) bool {
	path, ok := GetPath(r)
	if !ok {
		return false
	}
	return strings.HasPrefix(path, prefix)
}

// HasPathSuffix checks if the client's SPIFFE ID path ends with the given suffix.
//
// Example:
//
//	if httpapi.HasPathSuffix(r, "/admin") {
//	    // Client has admin role (application-defined)
//	}
func HasPathSuffix(r *http.Request, suffix string) bool {
	path, ok := GetPath(r)
	if !ok {
		return false
	}
	return strings.HasSuffix(path, suffix)
}

// GetPathSegments returns the path components of the SPIFFE ID as a slice.
// Empty segments are filtered out.
//
// Example:
//
//	// For spiffe://example.org/service/frontend/prod
//	segments := httpapi.GetPathSegments(r)
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
//
// Example:
//
//	if httpapi.MatchesID(r, "spiffe://example.org/service/frontend") {
//	    // Specific service identity
//	}
func MatchesID(r *http.Request, expectedID string) bool {
	id, ok := GetSPIFFEID(r)
	if !ok {
		return false
	}
	return id.String() == expectedID
}

// GetIDString returns the full SPIFFE ID as a string.
// Returns empty string if ID not present.
//
// Example:
//
//	idStr := httpapi.GetIDString(r)
//	// idStr = "spiffe://example.org/service/frontend"
func GetIDString(r *http.Request) string {
	id, ok := GetSPIFFEID(r)
	if !ok {
		return ""
	}
	return id.String()
}

// WithSPIFFEID adds a SPIFFE ID to the request context.
// This is primarily used for testing.
//
// Example:
//
//	testID := spiffeid.RequireFromString("spiffe://example.org/test")
//	req = httpapi.WithSPIFFEID(req, testID)
func WithSPIFFEID(r *http.Request, id spiffeid.ID) *http.Request {
	ctx := context.WithValue(r.Context(), spiffeIDKey, id)
	return r.WithContext(ctx)
}

// RequireAuthentication is a middleware that ensures the request has a valid SPIFFE ID.
// Returns 401 Unauthorized if no identity is present.
//
// Example:
//
//	mux.Handle("/api/", httpapi.RequireAuthentication(apiHandler))
func RequireAuthentication(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, ok := GetSPIFFEID(r); !ok {
			http.Error(w, "Authentication required", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// RequireTrustDomain is a middleware that ensures the client is from a specific trust domain.
// Returns 403 Forbidden if the trust domain doesn't match.
//
// Example:
//
//	handler := httpapi.RequireTrustDomain("example.org", apiHandler)
func RequireTrustDomain(trustDomain string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !MatchesTrustDomain(r, trustDomain) {
			http.Error(w, fmt.Sprintf("Trust domain must be %s", trustDomain), http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// RequirePathPrefix is a middleware that ensures the client's SPIFFE ID path has a specific prefix.
// Returns 403 Forbidden if the path doesn't match.
//
// Example:
//
//	// Only allow service workloads
//	handler := httpapi.RequirePathPrefix("/service/", apiHandler)
func RequirePathPrefix(prefix string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !HasPathPrefix(r, prefix) {
			http.Error(w, fmt.Sprintf("Path must start with %s", prefix), http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// LogIdentity is a middleware that logs the client's SPIFFE ID.
// Useful for debugging and auditing.
//
// Example:
//
//	mux.Handle("/api/", httpapi.LogIdentity(apiHandler))
func LogIdentity(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if id, ok := GetSPIFFEID(r); ok {
			fmt.Printf("[Identity] %s %s from %s\n", r.Method, r.URL.Path, id.String())
		} else {
			fmt.Printf("[Identity] %s %s from <unauthenticated>\n", r.Method, r.URL.Path)
		}
		next.ServeHTTP(w, r)
	})
}
