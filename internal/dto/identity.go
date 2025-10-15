package dto

import "github.com/pocket/hexagon/spire/internal/domain"

// Identity is a transport DTO for app/adapters. Optional in prod; handy in dev/CLI.
type Identity struct {
	IdentityCredential *domain.IdentityCredential `json:"identityCredential" yaml:"identityCredential"`
	Name               string                     `json:"name,omitempty" yaml:"name,omitempty"`
	IdentityDocument   *domain.IdentityDocument   `json:"identityDocument" yaml:"identityDocument"`
}
