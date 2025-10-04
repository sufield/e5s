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
		Name:                extractNameFromIdentityNamespace(identityNamespace.String()),
		IdentityDocument: cert,
	}
}

// extractNameFromIdentityNamespace extracts workload name from identity namespace
func extractNameFromIdentityNamespace(identityNamespaceStr string) string {
	// Extract name from spiffe://example.org/workload-name
	for i := len(identityNamespaceStr) - 1; i >= 0; i-- {
		if identityNamespaceStr[i] == '/' {
			return identityNamespaceStr[i+1:]
		}
	}
	return identityNamespaceStr
}
