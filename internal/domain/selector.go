package domain

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

// NewSelector creates a new selector
// Returns ErrSelectorInvalid if key or value is empty
func NewSelector(selectorType SelectorType, key, value string) (*Selector, error) {
	if key == "" {
		return nil, fmt.Errorf("%w: key cannot be empty", ErrSelectorInvalid)
	}
	if value == "" {
		return nil, fmt.Errorf("%w: value cannot be empty", ErrSelectorInvalid)
	}

	formatted := fmt.Sprintf("%s:%s:%s", selectorType, key, value)
	return &Selector{
		selectorType: selectorType,
		key:          key,
		value:        value,
		formatted:    formatted,
	}, nil
}

// ParseSelector parses a selector from string format (key:value)
// Handles multi-colon values consistently with ParseSelectorFromString
// Returns ErrSelectorInvalid if format is invalid
func ParseSelector(selectorType SelectorType, s string) (*Selector, error) {
	parts := splitSelector(s)
	if len(parts) < 2 {
		return nil, fmt.Errorf("%w: expected key:value format, got %s", ErrSelectorInvalid, s)
	}

	key := parts[0]
	// Join remaining parts for values with colons (e.g., unix:user:server-workload)
	value := strings.Join(parts[1:], ":")

	if key == "" {
		return nil, fmt.Errorf("%w: key cannot be empty", ErrSelectorInvalid)
	}
	if value == "" {
		return nil, fmt.Errorf("%w: value cannot be empty", ErrSelectorInvalid)
	}

	formatted := fmt.Sprintf("%s:%s", key, value)
	return &Selector{
		selectorType: selectorType,
		key:          key,
		value:        value,
		formatted:    formatted,
	}, nil
}

// ParseSelectorFromString parses a full selector string like "unix:uid:1001"
// Handles multi-colon values like "k8s:pod:ns:default:podname"
// Returns ErrSelectorInvalid if format is invalid
func ParseSelectorFromString(s string) (*Selector, error) {
	parts := splitSelector(s)
	if len(parts) < 3 {
		return nil, fmt.Errorf("%w: expected type:key:value format, got %s", ErrSelectorInvalid, s)
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

// Equals checks if two selectors are equal
// Uses field-by-field comparison for robustness
func (s *Selector) Equals(other *Selector) bool {
	if other == nil {
		return false
	}
	return s.selectorType == other.selectorType &&
		s.key == other.key &&
		s.value == other.value
}

func splitSelector(s string) []string {
	result := []string{}
	current := ""
	for _, c := range s {
		if c == ':' {
			if current != "" {
				result = append(result, current)
				current = ""
			}
		} else {
			current += string(c)
		}
	}
	if current != "" {
		result = append(result, current)
	}
	return result
}

// SelectorSet represents a collection of selectors
type SelectorSet struct {
	selectors []*Selector
}

// NewSelectorSet creates a new selector set
func NewSelectorSet(selectors ...*Selector) *SelectorSet {
	return &SelectorSet{selectors: selectors}
}

// Add adds a selector to the set
// Ensures uniqueness - duplicates are not added
func (ss *SelectorSet) Add(selector *Selector) {
	if !ss.Contains(selector) {
		ss.selectors = append(ss.selectors, selector)
	}
}

// Contains checks if the set contains a selector
func (ss *SelectorSet) Contains(selector *Selector) bool {
	for _, s := range ss.selectors {
		if s.Equals(selector) {
			return true
		}
	}
	return false
}

// All returns all selectors as a copy to prevent external mutation (DDD immutability)
func (ss *SelectorSet) All() []*Selector {
	// Return a defensive copy to maintain set immutability
	result := make([]*Selector, len(ss.selectors))
	copy(result, ss.selectors)
	return result
}

// Strings returns all selectors as strings
func (ss *SelectorSet) Strings() []string {
	result := make([]string, len(ss.selectors))
	for i, s := range ss.selectors {
		result[i] = s.String()
	}
	return result
}
