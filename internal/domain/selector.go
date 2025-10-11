// Package domain models core SPIFFE concepts like selectors, identity credentials,
// and identity documents, abstracting from infrastructure dependencies.
package domain

// NOTE: This file (selector.go) is primarily used by the in-memory implementation.
// In production deployments using real SPIRE, selector matching is delegated to SPIRE Server.
// However, this file must remain included in production builds because:
// 1. The spire.SPIREClient.Attest() method returns domain.SelectorSet
// 2. Domain types (Node, IdentityMapper) reference SelectorSet
// 3. Factory interfaces (AdapterFactory.SeedRegistry) use domain.IdentityMapper
// While production code paths don't actively use selector matching logic,
// the types are part of the domain model and adapter interfaces.

import (
	"fmt"
	"strings"
)

// SelectorType represents the category of selector
type SelectorType string

const (
	SelectorTypeNode     SelectorType = "node"
	SelectorTypeWorkload SelectorType = "workload"
)

// Selector represents a key-value pair used to match workload or node attributes
// Examples: unix:uid:1001, k8s:namespace:default
type Selector struct {
	selectorType SelectorType
	key          string
	value        string
	formatted    string
}

// NewSelector creates a new selector with the given type, key, and value.
// Returns ErrEmptyKey if key is empty, ErrEmptyValue if value is empty.
//
// Example:
//   NewSelector(SelectorTypeWorkload, "uid", "1000") → "workload:uid:1000"
func NewSelector(selectorType SelectorType, key, value string) (*Selector, error) {
	if key == "" {
		return nil, fmt.Errorf("%w", ErrEmptyKey)
	}
	if value == "" {
		return nil, fmt.Errorf("%w", ErrEmptyValue)
	}

	formatted := fmt.Sprintf("%s:%s:%s", selectorType, key, value)
	return &Selector{
		selectorType: selectorType,
		key:          key,
		value:        value,
		formatted:    formatted,
	}, nil
}

// ParseSelector parses a selector from string format (key:value).
// Handles multi-colon values consistently with ParseSelectorFromString.
// Returns ErrInvalidFormat if format is invalid, ErrEmptyKey or ErrEmptyValue if components are empty.
//
// Example:
//   ParseSelector(SelectorTypeWorkload, "uid:1000") → "workload:uid:1000"
//   ParseSelector(SelectorTypeWorkload, "user:server:prod") → "workload:user:server:prod"
func ParseSelector(selectorType SelectorType, s string) (*Selector, error) {
	if s == "" {
		return nil, fmt.Errorf("%w: input string is empty", ErrInvalidFormat)
	}

	parts := strings.Split(s, ":")
	if len(parts) < 2 {
		return nil, fmt.Errorf("%w: expected key:value format, got %s", ErrInvalidFormat, s)
	}

	key := parts[0]
	// Join remaining parts for values with colons (e.g., unix:user:server-workload)
	value := strings.Join(parts[1:], ":")

	if key == "" {
		return nil, fmt.Errorf("%w", ErrEmptyKey)
	}
	if value == "" {
		return nil, fmt.Errorf("%w", ErrEmptyValue)
	}

	formatted := fmt.Sprintf("%s:%s", key, value)
	return &Selector{
		selectorType: selectorType,
		key:          key,
		value:        value,
		formatted:    formatted,
	}, nil
}

// ParseSelectorFromString parses a full selector string like "workload:uid:1001".
// Handles multi-colon values like "workload:pod:ns:default:podname".
// Returns ErrInvalidFormat if format is invalid.
//
// Example:
//   ParseSelectorFromString("workload:uid:1000") → Selector{Type: "workload", Key: "uid", Value: "1000"}
//   ParseSelectorFromString("workload:pod:ns:default:name") → Selector{Type: "workload", Key: "pod", Value: "ns:default:name"}
func ParseSelectorFromString(s string) (*Selector, error) {
	if s == "" {
		return nil, fmt.Errorf("%w: input string is empty", ErrInvalidFormat)
	}

	parts := strings.Split(s, ":")
	if len(parts) < 3 {
		return nil, fmt.Errorf("%w: expected type:key:value format, got %s", ErrInvalidFormat, s)
	}

	selectorType := SelectorType(parts[0])
	key := parts[1]
	// Join remaining parts for values with colons
	value := strings.Join(parts[2:], ":")

	return NewSelector(selectorType, key, value)
}

// String returns the selector in formatted string
func (s *Selector) String() string {
	return s.formatted
}

// Type returns the selector type
func (s *Selector) Type() SelectorType {
	return s.selectorType
}

// Key returns the selector key
func (s *Selector) Key() string {
	return s.key
}

// Value returns the selector value
func (s *Selector) Value() string {
	return s.value
}

// Equals checks if two selectors are equal using field-by-field comparison.
// Returns false if either selector is nil.
func (s *Selector) Equals(other *Selector) bool {
	if s == nil || other == nil {
		return false
	}
	return s.selectorType == other.selectorType &&
		s.key == other.key &&
		s.value == other.value
}

// SelectorSet represents a collection of unique selectors.
// Uses map internally for O(1) Contains/Add operations.
type SelectorSet struct {
	// Map key is the formatted selector string for fast lookup
	selectors map[string]*Selector
}

// NewSelectorSet creates a new selector set with the given selectors.
// Nil selectors are ignored. Duplicates are automatically deduplicated.
func NewSelectorSet(selectors ...*Selector) *SelectorSet {
	ss := &SelectorSet{selectors: make(map[string]*Selector)}
	for _, s := range selectors {
		if s != nil {
			ss.selectors[s.formatted] = s
		}
	}
	return ss
}

// Add adds a selector to the set.
// Ensures uniqueness - duplicates are not added.
// Nil selectors are ignored.
// Time complexity: O(1)
func (ss *SelectorSet) Add(selector *Selector) {
	if selector != nil {
		ss.selectors[selector.formatted] = selector
	}
}

// Contains checks if the set contains a selector.
// Returns false for nil selectors.
// Time complexity: O(1)
func (ss *SelectorSet) Contains(selector *Selector) bool {
	if selector == nil {
		return false
	}
	_, ok := ss.selectors[selector.formatted]
	return ok
}

// Len returns the number of selectors in the set.
// Time complexity: O(1)
func (ss *SelectorSet) Len() int {
	return len(ss.selectors)
}

// All returns all selectors as a slice.
// Returns a new slice to prevent external mutation (DDD immutability).
// The order of selectors is non-deterministic (map iteration order).
// Time complexity: O(n)
func (ss *SelectorSet) All() []*Selector {
	result := make([]*Selector, 0, len(ss.selectors))
	for _, s := range ss.selectors {
		result = append(result, s)
	}
	return result
}

// Strings returns all selectors as formatted strings.
// The order of strings is non-deterministic (map iteration order).
// Time complexity: O(n)
func (ss *SelectorSet) Strings() []string {
	result := make([]string, 0, len(ss.selectors))
	for formatted := range ss.selectors {
		result = append(result, formatted)
	}
	return result
}
