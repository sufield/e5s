package domain

// IdentityMapper represents a mapping that associates an identity format with selectors
// It defines the conditions under which a workload qualifies for that identity
// This shifts focus to the "mapping" intent—clearly expressing how selectors map to identities.
type IdentityMapper struct {
	identityFormat *IdentityNamespace
	selectors      *SelectorSet
	parentID       *IdentityNamespace // Parent identity format (e.g., agent ID)
}

// NewIdentityMapper creates a new identity mapper
// Returns ErrInvalidIdentityNamespace if identityFormat is nil
// Returns ErrInvalidSelectors if selectors are nil or empty
func NewIdentityMapper(identityFormat *IdentityNamespace, selectors *SelectorSet) (*IdentityMapper, error) {
	if identityFormat == nil {
		return nil, ErrInvalidIdentityNamespace
	}
	if selectors == nil || len(selectors.All()) == 0 {
		return nil, ErrInvalidSelectors
	}

	return &IdentityMapper{
		identityFormat: identityFormat,
		selectors:      selectors,
	}, nil
}

// SetParentID sets the parent identity format (agent ID)
func (im *IdentityMapper) SetParentID(parentID *IdentityNamespace) {
	im.parentID = parentID
}

// IdentityNamespace returns the identity format
func (im *IdentityMapper) IdentityNamespace() *IdentityNamespace {
	return im.identityFormat
}

// Selectors returns the selector set
func (im *IdentityMapper) Selectors() *SelectorSet {
	return im.selectors
}

// ParentID returns the parent identity format
func (im *IdentityMapper) ParentID() *IdentityNamespace {
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
