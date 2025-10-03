package inmemory

import (
	"github.com/pocket/hexagon/spire/internal/app/ports"
	"github.com/pocket/hexagon/spire/internal/domain"
)

// Anti-corruption layer: translates between domain types and adapter internal types

// domainToIdentity converts domain types to ports.Identity
func domainToIdentity(identityFormat *domain.IdentityNamespace, cert *domain.IdentityDocument) *ports.Identity {
	return &ports.Identity{
		IdentityNamespace:      identityFormat,
		Name:                extractNameFromIdentityNamespace(identityFormat.String()),
		IdentityDocument: cert,
	}
}

// extractNameFromIdentityNamespace extracts workload name from identity format
func extractNameFromIdentityNamespace(identityFormatStr string) string {
	// Extract name from spiffe://example.org/workload-name
	for i := len(identityFormatStr) - 1; i >= 0; i-- {
		if identityFormatStr[i] == '/' {
			return identityFormatStr[i+1:]
		}
	}
	return identityFormatStr
}
