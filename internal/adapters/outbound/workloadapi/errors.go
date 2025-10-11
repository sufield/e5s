// errors.go contains sentinel errors and configuration constants for the workloadapi package.
package workloadapi

import (
	"errors"
	"time"
)

// Constants for configuration defaults
const (
	// DefaultTimeout is the default HTTP client timeout for Workload API requests
	DefaultTimeout = 30 * time.Second

	// DefaultSVIDEndpoint is the default HTTP endpoint for X.509 SVID fetches
	DefaultSVIDEndpoint = "http://unix/svid/x509"

	// MaxErrorBodySize limits how much of error response body we read
	MaxErrorBodySize = 4096
)

// Header constants removed - workload attestation now uses SO_PEERCRED
// The server extracts kernel-verified credentials automatically via Unix socket peer credentials.
// No headers are needed or sent by the client for attestation.

// Sentinel errors for inspectable error handling
var (
	// ErrInvalidSocketPath indicates the socket path is invalid or empty
	ErrInvalidSocketPath = errors.New("socket path must be an absolute path starting with '/'")

	// ErrInvalidArgument indicates an invalid argument was provided to a method
	ErrInvalidArgument = errors.New("invalid argument")

	// ErrFetchFailed indicates the SVID fetch operation failed
	ErrFetchFailed = errors.New("failed to fetch X.509 SVID from Workload API")

	// ErrInvalidResponse indicates the server returned an invalid or malformed response
	ErrInvalidResponse = errors.New("invalid response from Workload API server")

	// ErrServerError indicates the server returned an error status code
	ErrServerError = errors.New("workload API server returned error")
)
