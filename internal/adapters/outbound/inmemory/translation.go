package inmemory

import (
	"github.com/pocket/hexagon/spire/internal/domain"
	"github.com/pocket/hexagon/spire/internal/ports"
)

// Anti-corruption layer: translates between domain types and adapter internal types

// domainToIdentity converts domain types to ports.Identity
func domainToIdentity(identityCredential *domain.IdentityCredential, cert *domain.IdentityDocument) *ports.Identity {
	return &ports.Identity{
		IdentityCredential: identityCredential,
		Name:              extractNameFromIdentityCredential(identityCredential),
		IdentityDocument:  cert,
	}
}
