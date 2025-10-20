package dto

import (
	"crypto"

	"github.com/pocket/hexagon/spire/internal/domain"
)

// Identity is a transport DTO for app/adapters. Optional in prod; handy in dev/CLI.
//
// Note: PrivateKey is stored here (in the DTO layer) rather than in the domain model.
// The domain.IdentityDocument is purely descriptive and doesn't hold sensitive key material.
// Adapters manage private keys separately and use this DTO to bundle them for transport.
//
// Concurrency: If shared across goroutines, treat PrivateKey as read-only after initialization.
// While most crypto.Signer implementations (e.g., *rsa.PrivateKey, *ecdsa.PrivateKey) are safe
// for concurrent signing operations, they should not be modified concurrently.
type Identity struct {
	IdentityCredential *domain.IdentityCredential `json:"identityCredential" yaml:"identityCredential"`
	Name               string                     `json:"name,omitempty" yaml:"name,omitempty"`
	IdentityDocument   *domain.IdentityDocument   `json:"identityDocument" yaml:"identityDocument"`
	PrivateKey         crypto.Signer              `json:"-" yaml:"-"` // Not serialized (sensitive)
}
