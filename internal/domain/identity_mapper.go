package domain

// IdentityMapper represents a mapping that associates an identity namespace with selectors
// It defines the conditions under which a workload qualifies for that identity
// This shifts focus to the "mapping" intent—clearly expressing how selectors map to identities.
type IdentityMapper struct {
	identityNamespace *IdentityNamespace
	selectors      *SelectorSet
	parentID       *IdentityNamespace // Parent identity namespace (e.g., agent ID)
}

// NewIdentityMapper creates a new identity mapper
// Returns ErrInvalidIdentityNamespace if identityNamespace is nil
// Returns ErrInvalidSelectors if selectors are nil or empty
func NewIdentityMapper(identityNamespace *IdentityNamespace, selectors *SelectorSet) (*IdentityMapper, error) {
	if identityNamespace == nil {
		return nil, ErrInvalidIdentityNamespace
	}
	if selectors == nil || len(selectors.All()) == 0 {
		return nil, ErrInvalidSelectors
	}

	return &IdentityMapper{
		identityNamespace: identityNamespace,
		selectors:      selectors,
	}, nil
}

// SetParentID sets the parent identity namespace (agent ID)
func (im *IdentityMapper) SetParentID(parentID *IdentityNamespace) {
	im.parentID = parentID
}

// IdentityNamespace returns the identity namespace
func (im *IdentityMapper) IdentityNamespace() *IdentityNamespace {
	return im.identityNamespace
}

// Selectors returns the selector set
func (im *IdentityMapper) Selectors() *SelectorSet {
	return im.selectors
}

// ParentID returns the parent identity namespace
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
