//go:build dev

package domain

// SelectorSet represents an order-preserving collection of unique selectors.
//
// Implementation:
//   - Uses map[string]struct{} for O(1) deduplication (keyed by formatted selector)
//   - Uses []Selector slice to preserve insertion order
//   - All() returns selectors in insertion order (deterministic, unlike pure map iteration)
//
// Time complexity:
//   - Add(): O(1)
//   - Contains(): O(1)
//   - All(): O(n) where n is the number of selectors
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
//   all := set.All() // Returns selectors in insertion order
type SelectorSet struct {
	// Map for O(1) deduplication check
	seen map[string]struct{}
	// Slice to preserve insertion order
	list []*Selector
}

// NewSelectorSet creates a new selector set with the given selectors.
// Nil selectors are ignored. Duplicates are automatically deduplicated.
// Selectors are added in the order provided (insertion order preserved).
//
// Time complexity: O(n) where n is the number of input selectors
//
// Example:
//   s1, _ := ParseSelectorFromString("workload:uid:1000")
//   s2, _ := ParseSelectorFromString("workload:user:app")
//   set := NewSelectorSet(s1, s2)
//   // set.Len() == 2
//   // set.All() returns [s1, s2] in that order
func NewSelectorSet(selectors ...*Selector) *SelectorSet {
	ss := &SelectorSet{
		seen: make(map[string]struct{}),
		list: make([]*Selector, 0, len(selectors)),
	}
	for _, s := range selectors {
		ss.Add(s) // Use Add to ensure deduplication and ordering
	}
	return ss
}

// Add adds a selector to the set.
// Ensures uniqueness - duplicates are ignored (no-op).
// Nil selectors are ignored.
// Preserves insertion order for All().
//
// Time complexity: O(1)
//
// Example:
//   set.Add(selector)
//   set.Add(selector) // No-op, already exists
func (ss *SelectorSet) Add(selector *Selector) {
	if selector == nil {
		return
	}

	// Check if already exists
	if _, exists := ss.seen[selector.formatted]; exists {
		return // Already in set, maintain original order
	}

	// Add to both map and list
	ss.seen[selector.formatted] = struct{}{}
	ss.list = append(ss.list, selector)
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
	_, ok := ss.seen[selector.formatted]
	return ok
}

// Len returns the number of selectors in the set.
//
// Time complexity: O(1)
//
// Example:
//   count := set.Len()
func (ss *SelectorSet) Len() int {
	return len(ss.list)
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
	return len(ss.list) == 0
}

// All returns all selectors as a slice in insertion order.
// Returns a defensive copy to prevent external mutation (DDD immutability).
//
// Order is deterministic: selectors are returned in the order they were added.
// This ensures consistent, reproducible behavior across runs.
//
// Time complexity: O(n) where n is the number of selectors
//
// Example:
//   for _, selector := range set.All() {
//       fmt.Println(selector)
//   }
func (ss *SelectorSet) All() []*Selector {
	// Return defensive copy to prevent external mutation
	result := make([]*Selector, len(ss.list))
	copy(result, ss.list)
	return result
}

// Strings returns all selectors as formatted strings in insertion order.
// Useful for logging, serialization, or displaying selector sets.
//
// Order is deterministic: selectors are returned in the order they were added.
// This ensures consistent, reproducible behavior across runs.
//
// Time complexity: O(n) where n is the number of selectors
//
// Example:
//   log.Printf("Selectors: %v", set.Strings())
//   // Output: Selectors: [workload:uid:1000 workload:user:app]
func (ss *SelectorSet) Strings() []string {
	result := make([]string, 0, len(ss.list))
	for _, s := range ss.list {
		result = append(result, s.formatted)
	}
	return result
}
