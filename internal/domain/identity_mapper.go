package domain

// NOTE: This file (identity_mapper.go) is primarily used by the in-memory implementation.
// In production deployments using real SPIRE, identity mapping is managed by SPIRE Server's
// registration entries. However, this file must remain included in production builds because:
// 1. Factory interfaces (AdapterFactory.SeedRegistry) use *domain.IdentityMapper
// 2. The type is part of the domain model even if not actively used in production flows
// Production code should use SPIRE Server CLI for registration: spire-server entry create

// IdentityMapper represents a mapping that associates an identity credential with selectors
// It defines the conditions under which a workload qualifies for that identity
// This shifts focus to the "mapping" intent—clearly expressing how selectors map to identities.
type IdentityMapper struct {
	identityCredential *IdentityCredential
	selectors          *SelectorSet
	parentID           *IdentityCredential // Parent identity credential (e.g., agent ID)
}

// NewIdentityMapper creates a new identity mapper
// Returns ErrInvalidIdentityCredential if identityCredential is nil
// Returns ErrInvalidSelectors if selectors are nil or empty
func NewIdentityMapper(identityCredential *IdentityCredential, selectors *SelectorSet) (*IdentityMapper, error) {
	if identityCredential == nil {
		return nil, ErrInvalidIdentityCredential
	}
	if selectors == nil || len(selectors.All()) == 0 {
		return nil, ErrInvalidSelectors
	}

	return &IdentityMapper{
		identityCredential: identityCredential,
		selectors:          selectors,
	}, nil
}

// SetParentID sets the parent identity credential (agent ID)
func (im *IdentityMapper) SetParentID(parentID *IdentityCredential) {
	im.parentID = parentID
}

// IdentityCredential returns the identity credential
func (im *IdentityMapper) IdentityCredential() *IdentityCredential {
	return im.identityCredential
}

// Selectors returns the selector set
func (im *IdentityMapper) Selectors() *SelectorSet {
	return im.selectors
}

// ParentID returns the parent identity credential
func (im *IdentityMapper) ParentID() *IdentityCredential {
	return im.parentID
}

// MatchesSelectors checks if this mapper matches the given selectors
// Uses AND logic: ALL mapper selectors must be present
// in the discovered selectors for the workload to qualify for this identity.
// Example: Mapper requires [unix:uid:1000, k8s:ns:default], workload must have both.
func (im *IdentityMapper) MatchesSelectors(selectors *SelectorSet) bool {
	// All mapper selectors must be present in the discovered selectors
	for _, mapperSelector := range im.selectors.All() {
		if !selectors.Contains(mapperSelector) {
			return false // Missing required selector
		}
	}
	return true // All mapper selectors matched
}
