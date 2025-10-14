//go:build dev

package domain

// NOTE: This file is only included in development builds (via //go:build dev tag).
// In production deployments using real SPIRE, identity mapping is managed by SPIRE Server's
// registration entries via CLI: spire-server entry create
// Production builds exclude this file entirely, as workloads only fetch identities via Workload API.

// IdentityMapper represents an immutable mapping between workload selectors and an identity.
//
// It defines the conditions under which a workload qualifies for a specific identity credential.
// The mapper uses AND semantics: ALL selectors in the mapper must be present in the workload's
// discovered selectors for the workload to qualify for this identity.
//
// Example:
//   Mapper selectors: [unix:uid:1000, k8s:namespace:prod]
//   Workload selectors: [unix:uid:1000, k8s:namespace:prod, k8s:pod:web]
//   Result: MATCH (all mapper selectors present, extra selectors are ignored)
//
// Design Philosophy:
//   - Immutable: All fields are set once at construction and never modified (except parentID)
//   - Validated: Constructor ensures identityCredential and selectors are non-nil and non-empty
//   - Nil-safe: MatchesSelectors() handles nil receivers and arguments gracefully
//   - AND semantics only: No OR logic, no priority/specificity ordering
//
// Concurrency: Safe for concurrent use (immutable value object, except SetParentID which is not thread-safe).
type IdentityMapper struct {
	identityCredential *IdentityCredential
	selectors          *SelectorSet
	parentID           *IdentityCredential // Parent identity credential (e.g., agent ID) - mutable
}

// NewIdentityMapper creates a new validated identity mapper.
//
// Validation:
//   - identityCredential must be non-nil
//   - selectors must be non-nil and non-empty
//
// Parameters:
//   - identityCredential: The SPIFFE ID that workloads will receive if they match
//   - selectors: The set of selectors that workloads must ALL have to qualify (AND logic)
//
// Returns:
//   - *IdentityMapper on success
//   - ErrInvalidIdentityCredential if identityCredential is nil
//   - ErrInvalidSelectors if selectors is nil or empty
//
// Example:
//   td := domain.NewTrustDomainFromName("example.org")
//   id := domain.NewIdentityCredentialFromComponents(td, "/workload/db")
//   selectors := domain.NewSelectorSet()
//   selectors.Add(domain.MustParseSelectorFromString("unix:uid:1000"))
//   selectors.Add(domain.MustParseSelectorFromString("k8s:namespace:prod"))
//   mapper, err := domain.NewIdentityMapper(id, selectors)
//
// Concurrency: Safe for concurrent use (pure function, no shared state).
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

// IdentityCredential returns the SPIFFE ID that workloads will receive if they match this mapper.
//
// The identity credential is treated as immutable per the domain's immutability contract.
//
// Example:
//   mapper.IdentityCredential().String() // "spiffe://example.org/workload/db"
func (im *IdentityMapper) IdentityCredential() *IdentityCredential {
	return im.identityCredential
}

// Selectors returns the set of selectors that workloads must ALL have to qualify (AND logic).
//
// The selector set is treated as immutable. Do not modify the returned set.
//
// Example:
//   for _, sel := range mapper.Selectors().All() {
//       fmt.Println(sel.Formatted()) // "unix:uid:1000", "k8s:namespace:prod", etc.
//   }
func (im *IdentityMapper) Selectors() *SelectorSet {
	return im.selectors
}

// ParentID returns the parent identity credential (typically the SPIRE agent's identity).
//
// This is optional and may be nil. In SPIRE, workload identities are typically issued by
// an agent, and the agent's identity is the parent.
//
// Example:
//   if parent := mapper.ParentID(); parent != nil {
//       fmt.Println("Parent agent:", parent.String())
//   }
func (im *IdentityMapper) ParentID() *IdentityCredential {
	return im.parentID
}

// SetParentID sets the parent identity credential (agent ID).
//
// This is the only mutable operation on IdentityMapper. It's typically called once
// during initialization to associate the mapper with its issuing agent.
//
// Note: This method is NOT thread-safe. Callers must ensure external synchronization
// if SetParentID might be called concurrently with reads.
//
// Example:
//   agentID := domain.NewIdentityCredentialFromComponents(td, "/spire/agent/k8s/node-1")
//   mapper.SetParentID(agentID)
func (im *IdentityMapper) SetParentID(parentID *IdentityCredential) {
	im.parentID = parentID
}

// MatchesSelectors checks if this mapper matches the given workload selectors using AND logic.
//
// AND Semantics:
//   ALL selectors in im.selectors must be present in the input selectors for a match.
//   Extra selectors in the input are ignored (they don't prevent a match).
//
// Nil Safety:
//   - Returns false if im is nil (allows safe method calls on nil receivers)
//   - Returns false if im.selectors is nil (defensive, should never happen with valid construction)
//   - Returns false if input selectors is nil
//
// Examples:
//   Mapper selectors: [unix:uid:1000, k8s:namespace:prod]
//
//   Input: [unix:uid:1000, k8s:namespace:prod, k8s:pod:web]
//   Result: true (all mapper selectors present, extra k8s:pod:web ignored)
//
//   Input: [unix:uid:1000]
//   Result: false (missing k8s:namespace:prod)
//
//   Input: [k8s:namespace:prod]
//   Result: false (missing unix:uid:1000)
//
//   Input: []
//   Result: false (missing both required selectors)
//
// Concurrency: Safe for concurrent use (read-only operation on immutable data).
func (im *IdentityMapper) MatchesSelectors(selectors *SelectorSet) bool {
	// Nil safety checks
	if im == nil || im.selectors == nil || selectors == nil {
		return false
	}

	// Check that ALL mapper selectors are present in the input selectors
	for _, mapperSelector := range im.selectors.All() {
		if !selectors.Contains(mapperSelector) {
			return false // Missing a required selector
		}
	}

	return true // All mapper selectors matched
}

// IsZero reports whether this mapper is uninitialized or invalid.
//
// Returns true if:
//   - Mapper is nil
//   - Identity credential is nil
//   - Selectors is nil or empty
//
// This is useful for detecting zero-value instances or programming errors.
//
// Example:
//   var mapper *IdentityMapper
//   if mapper.IsZero() {
//       // Need to create a valid mapper
//   }
//
// Concurrency: Safe for concurrent use.
func (im *IdentityMapper) IsZero() bool {
	return im == nil || im.identityCredential == nil || im.selectors == nil || len(im.selectors.All()) == 0
}
