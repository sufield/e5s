package inmemory

import (
	"github.com/pocket/hexagon/spire/internal/app/ports"
	"github.com/pocket/hexagon/spire/internal/domain"
)

// Anti-corruption layer: translates between domain types and adapter internal types

// domainToIdentity converts domain types to ports.Identity
func domainToIdentity(identityNamespace *domain.IdentityNamespace, cert *domain.IdentityDocument) *ports.Identity {
	return &ports.Identity{
		IdentityNamespace:      identityNamespace,
		Name:                extractNameFromIdentityNamespace(identityNamespace),
		IdentityDocument: cert,
	}
}
