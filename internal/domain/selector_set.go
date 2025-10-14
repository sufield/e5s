//go:build dev

package domain

// SelectorSet represents a collection of unique selectors.
// Provides O(1) add/contains operations using map-based storage.
//
// Thread-safety: SelectorSet is NOT thread-safe. Callers must synchronize access.
//
// NOTE: This file is only included in development builds (via //go:build dev tag).
// In production deployments, SPIRE Server manages selector matching.
// Production builds exclude this file entirely.
//
// Example:
//   set := NewSelectorSet()
//   set.Add(selector1)
//   set.Add(selector2)
//   if set.Contains(selector1) {
//       // Process matching selector
//   }
type SelectorSet struct {
	// Map key is the formatted selector string for O(1) lookup
	selectors map[string]*Selector
}

// NewSelectorSet creates a new selector set with the given selectors.
// Nil selectors are ignored. Duplicates are automatically deduplicated.
//
// Time complexity: O(n) where n is the number of input selectors
//
// Example:
//   s1, _ := ParseSelectorFromString("workload:uid:1000")
//   s2, _ := ParseSelectorFromString("workload:user:app")
//   set := NewSelectorSet(s1, s2)
//   // set.Len() == 2
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
// Ensures uniqueness - duplicates overwrite existing entries.
// Nil selectors are ignored.
//
// Time complexity: O(1)
//
// Example:
//   set.Add(selector)
//   set.Add(selector) // No-op, already exists
func (ss *SelectorSet) Add(selector *Selector) {
	if selector != nil {
		ss.selectors[selector.formatted] = selector
	}
}

// Contains checks if the set contains a selector.
// Returns false for nil selectors.
//
// Time complexity: O(1)
//
// Example:
//   if set.Contains(selector) {
//       // Selector found
//   }
func (ss *SelectorSet) Contains(selector *Selector) bool {
	if selector == nil {
		return false
	}
	_, ok := ss.selectors[selector.formatted]
	return ok
}

// Len returns the number of selectors in the set.
//
// Time complexity: O(1)
//
// Example:
//   count := set.Len()
func (ss *SelectorSet) Len() int {
	return len(ss.selectors)
}

// IsEmpty returns true if the set contains no selectors.
// More expressive than checking Len() == 0.
//
// Time complexity: O(1)
//
// Example:
//   if set.IsEmpty() {
//       return ErrNoSelectors
//   }
func (ss *SelectorSet) IsEmpty() bool {
	return len(ss.selectors) == 0
}

// All returns all selectors as a slice.
// Returns a new slice to prevent external mutation (DDD immutability).
//
// The order of selectors is non-deterministic due to map iteration order.
// If you need deterministic ordering, sort the result after calling All().
//
// Time complexity: O(n) where n is the number of selectors
//
// Example:
//   for _, selector := range set.All() {
//       fmt.Println(selector)
//   }
func (ss *SelectorSet) All() []*Selector {
	result := make([]*Selector, 0, len(ss.selectors))
	for _, s := range ss.selectors {
		result = append(result, s)
	}
	return result
}

// Strings returns all selectors as formatted strings.
// Useful for logging, serialization, or displaying selector sets.
//
// The order of strings is non-deterministic due to map iteration order.
// If you need deterministic ordering, sort the result after calling Strings().
//
// Time complexity: O(n) where n is the number of selectors
//
// Example:
//   log.Printf("Selectors: %v", set.Strings())
//   // Output: Selectors: [workload:uid:1000 workload:user:app]
func (ss *SelectorSet) Strings() []string {
	result := make([]string, 0, len(ss.selectors))
	for formatted := range ss.selectors {
		result = append(result, formatted)
	}
	return result
}
