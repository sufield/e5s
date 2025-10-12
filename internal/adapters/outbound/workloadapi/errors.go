// errors.go contains sentinel errors and configuration constants for the workloadapi package.
package workloadapi

import (
	"errors"
	"time"
)

// Configuration defaults for the Workload API client.
const (
	// DefaultTimeout is the default HTTP client timeout for Workload API requests.
	DefaultTimeout = 30 * time.Second

	// DefaultSVIDEndpoint is the default HTTP endpoint for X.509 SVID fetches.
	DefaultSVIDEndpoint = "http://unix/svid/x509"

	// MaxErrorBodySize limits how much of the error response body is read (in bytes).
	MaxErrorBodySize = 4096

	// MaxResponseBodySize limits the maximum response body size to prevent oversized payloads.
	MaxResponseBodySize = 1 << 20 // 1 MiB
)

// Note: Header constants removed — workload attestation now relies on SO_PEERCRED.
// The server extracts kernel-verified credentials automatically via Unix socket
// peer credentials. No headers are needed for attestation.

// Sentinel errors for inspectable error handling.
// These are compared using errors.Is().
var (
	// ErrInvalidSocketPath indicates the socket path is invalid or empty.
	// For Linux, absolute paths ("/tmp/...") and abstract sockets ("@spire-agent") are valid.
	ErrInvalidSocketPath = errors.New("workloadapi: invalid socket path (must start with '/' or '@')")

	// ErrInvalidArgument indicates an invalid argument was provided to a method.
	ErrInvalidArgument = errors.New("workloadapi: invalid argument")

	// ErrFetchFailed indicates that fetching an X.509 SVID from the Workload API failed.
	ErrFetchFailed = errors.New("workloadapi: failed to fetch X.509 SVID")

	// ErrInvalidResponse indicates that the server returned a malformed or unexpected response.
	ErrInvalidResponse = errors.New("workloadapi: invalid response from server")

	// ErrServerError indicates that the Workload API server returned a non-200 HTTP status.
	ErrServerError = errors.New("workloadapi: server returned error")
)
