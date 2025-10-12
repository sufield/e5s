// Package domain models core SPIFFE concepts like selectors, identity credentials,
// and identity documents, abstracting from infrastructure dependencies.
package domain

// NOTE: Selector types and matching logic are primarily used by in-memory implementations.
// In production deployments using real SPIRE, selector matching is delegated to SPIRE Server.
// However, these types must remain in production builds because:
// 1. Domain types (Node, IdentityMapper) reference SelectorSet
// 2. Factory interfaces (AdapterFactory.SeedRegistry) use domain.IdentityMapper
// 3. In-memory attestors return selectors for development/testing
// While production code paths don't actively use selector matching logic,
// the types are part of the domain model and adapter interfaces.

import (
	"fmt"
	"strings"
)

// Selector represents a key-value pair used to match workload or node attributes.
// Selectors are immutable after creation and are used by SPIRE for workload attestation.
//
// Format: type:key:value
//
// Examples:
//   - unix:uid:1001 (workload selector for UID)
//   - k8s:namespace:default (workload selector for K8s namespace)
//   - k8s:pod:ns:default:name (multi-colon value support)
//   - aws:instance-type:t2.micro (node selector for instance type)
//
// SPIRE uses selectors to map workload/node properties to SPIFFE identities.
// During attestation, SPIRE Agent collects selectors about the workload,
// then SPIRE Server matches them against registration entries to determine
// which identity (SPIFFE ID) should be issued.
type Selector struct {
	selectorType SelectorType
	key          string
	value        string
	formatted    string // Pre-computed string representation for performance
}

// NewSelector creates a new selector with validation.
//
// Parameters:
//   - selectorType: The category of selector (node or workload)
//   - key: The selector attribute name (e.g., "uid", "namespace")
//   - value: The selector attribute value (e.g., "1000", "default")
//
// Returns:
//   - ErrEmptyKey if key is empty
//   - ErrEmptyValue if value is empty
//
// Example:
//   selector, err := NewSelector(SelectorTypeWorkload, "uid", "1000")
//   if err != nil {
//       return err
//   }
//   // selector.String() == "workload:uid:1000"
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

// ParseSelector parses a selector from "key:value" format with explicit type.
// Handles multi-colon values consistently (e.g., "user:server:prod" → value="server:prod").
//
// This is useful when the selector type is known from context (e.g., parsing
// workload-specific configuration where all selectors are workload-type).
//
// Format: key:value (type provided separately)
//
// Returns:
//   - ErrInvalidFormat if format is invalid
//   - ErrEmptyKey if key is empty
//   - ErrEmptyValue if value is empty
//
// Example:
//   selector, err := ParseSelector(SelectorTypeWorkload, "uid:1000")
//   // selector: workload:uid:1000
//
//   selector, err := ParseSelector(SelectorTypeWorkload, "user:server:prod")
//   // selector: workload:user:server:prod (value="server:prod")
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

// ParseSelectorFromString parses a full selector string in "type:key:value" format.
// Handles multi-colon values correctly (e.g., "workload:pod:ns:default:podname").
//
// This is the primary parsing function when receiving selector strings from
// external sources (config files, SPIRE Server, etc.) where the full format
// including type is present.
//
// Format: type:key:value
//
// Returns:
//   - ErrInvalidFormat if format is invalid
//   - ErrEmptyKey if key is empty
//   - ErrEmptyValue if value is empty
//
// Example:
//   selector, err := ParseSelectorFromString("workload:uid:1000")
//   // selector.Type() == SelectorTypeWorkload
//   // selector.Key() == "uid"
//   // selector.Value() == "1000"
//
//   selector, err := ParseSelectorFromString("workload:pod:ns:default:name")
//   // selector.Value() == "ns:default:name" (multi-colon support)
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

// String returns the selector in formatted string representation.
// The format is always "type:key:value" for consistency.
//
// Example:
//   selector.String() // "workload:uid:1000"
func (s *Selector) String() string {
	return s.formatted
}

// Type returns the selector type (node or workload).
//
// Example:
//   if selector.Type() == SelectorTypeWorkload {
//       // Process workload selector
//   }
func (s *Selector) Type() SelectorType {
	return s.selectorType
}

// Key returns the selector key (e.g., "uid", "namespace", "pod").
//
// Example:
//   switch selector.Key() {
//   case "uid":
//       // Process UID selector
//   case "namespace":
//       // Process namespace selector
//   }
func (s *Selector) Key() string {
	return s.key
}

// Value returns the selector value (e.g., "1000", "default").
// For selectors with multi-colon values, this returns the full value
// with colons preserved (e.g., "ns:default:podname").
//
// Example:
//   uid := selector.Value() // "1000"
func (s *Selector) Value() string {
	return s.value
}

// Equals performs field-by-field comparison of two selectors.
// Returns false if either selector is nil.
//
// Two selectors are considered equal if they have the same type, key, and value.
// This is used for deduplication in SelectorSet and for matching operations.
//
// Example:
//   s1, _ := ParseSelectorFromString("workload:uid:1000")
//   s2, _ := ParseSelectorFromString("workload:uid:1000")
//   s1.Equals(s2) // true
//
//   s3, _ := ParseSelectorFromString("workload:uid:1001")
//   s1.Equals(s3) // false
func (s *Selector) Equals(other *Selector) bool {
	if s == nil || other == nil {
		return false
	}
	return s.selectorType == other.selectorType &&
		s.key == other.key &&
		s.value == other.value
}
