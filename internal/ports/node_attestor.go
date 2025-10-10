//go:build !production

package ports

import (
	"context"

	"github.com/pocket/hexagon/spire/internal/domain"
)

// NOTE: This file (node_attestor.go) is ONLY used by the in-memory implementation.
// In production deployments using real SPIRE, node attestation is handled by SPIRE Server.
// This file is excluded from production builds via build tag.

// NodeAttestor verifies node (agent host) identity and produces attestation data
// In a real SPIRE deployment, this would use platform-specific attestation (AWS IID, TPM, etc.)
// For in-memory walking skeleton, this provides hardcoded/mock attestation
type NodeAttestor interface {
	// AttestNode performs node attestation and returns the attested node with selectors
	// Returns domain.Node with selectors populated and marked as attested
	AttestNode(ctx context.Context, identityCredential *domain.IdentityCredential) (*domain.Node, error)
}
