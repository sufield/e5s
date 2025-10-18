//go:build dev

// Package identityconv is dev-only glue for the in-memory adapters.
//
// This package provides helper functions for deriving human-readable names
// from identity credentials. It exists solely for development and testing.
//
// Production deployments may have different display name requirements.
// This package is excluded from production builds via the "dev" build tag.
//
// Build Tag: This file is only included when building with the "dev" tag:
//
//	go build -tags dev ...
//	go test -tags dev ...
package identityconv

import (
	"strings"

	"github.com/pocket/hexagon/spire/internal/domain"
)

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
