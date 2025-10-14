//go:build dev

// Package identityconv is dev-only glue for the in-memory adapters.
//
// This package helps validate ProcessIdentity DTOs and convert them into domain.Workload.
// It exists solely for development and testing with in-memory identity providers.
//
// Production deployments use real SPIRE Workload API which attests the calling process
// directly and never manually converts ProcessIdentity to Workload. This package is
// excluded from production builds via the "dev" build tag.
//
// Build Tag: This file is only included when building with the "dev" tag:
//   go build -tags dev ...
//   go test -tags dev ...
//
// Production builds (without -tags dev) will not include this package, reducing
// the attack surface and ensuring dead code is not shipped.
package identityconv

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/pocket/hexagon/spire/internal/domain"
	"github.com/pocket/hexagon/spire/internal/ports"
)

// ValidateProcessIdentity validates a ProcessIdentity DTO.
//
// Returns domain sentinel errors that callers can branch on using errors.Is:
//   - domain.ErrInvalidProcessIdentity for invalid PIDs, UIDs, GIDs, or paths
//
// Validation rules:
//   - PID, UID, GID must be non-negative
//   - Path must be non-empty after trimming whitespace
//
// This is dev-only validation for in-memory adapters. Production SPIRE
// attestation validates process identity through OS-level mechanisms.
func ValidateProcessIdentity(p ports.ProcessIdentity) error {
	if p.PID < 0 || p.UID < 0 || p.GID < 0 {
		return fmt.Errorf("%w: negative pid/uid/gid", domain.ErrInvalidProcessIdentity)
	}
	if strings.TrimSpace(p.Path) == "" {
		return fmt.Errorf("%w: empty path", domain.ErrInvalidProcessIdentity)
	}
	return nil
}

// ToWorkload converts a ports ProcessIdentity DTO to a domain Workload entity.
//
// Performs light normalization:
//   - Trims whitespace from path
//   - Cleans path using filepath.Clean (resolves ".", "..", removes redundant separators)
//
// This conversion is dev-only glue for in-memory adapters. Production SPIRE
// attestation creates Workload entities directly from OS-level process information.
func ToWorkload(p ports.ProcessIdentity) *domain.Workload {
	path := strings.TrimSpace(p.Path)
	if path != "" {
		path = filepath.Clean(path)
	}
	return domain.NewWorkload(p.PID, p.UID, p.GID, path)
}

// DeriveIdentityName derives a human-readable name from an identity credential.
//
// Naming rules:
//   - Returns "unknown" for nil credentials
//   - Returns trust domain for root IDs (path is "/")
//   - Returns last path segment for non-root IDs
//
// Examples:
//   - nil                                    → "unknown"
//   - {TrustDomain: "example.org", Path: "/"} → "example.org"
//   - {TrustDomain: "example.org", Path: "/workload/server"} → "server"
//
// This is convenience sugar for dev UI/logs. Production systems may have
// different display name requirements.
func DeriveIdentityName(cred *domain.IdentityCredential) string {
	if cred == nil {
		return "unknown"
	}
	path := strings.Trim(cred.Path(), "/")
	if path == "" {
		return cred.TrustDomain().String()
	}
	parts := strings.Split(path, "/")
	return parts[len(parts)-1]
}
