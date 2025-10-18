//go:build dev

package domain

// SelectorType represents the category of selector (node vs workload).
// Used to distinguish between node-level and workload-level attestation in SPIRE.
//
// NOTE: This file is only included in development builds (via //go:build dev tag).
// Production builds exclude this file entirely.
type SelectorType string

const (
	// SelectorTypeNode represents node-level selectors for node attestation.
	// Node selectors describe properties of the compute node/instance itself.
	//
	// Examples:
	//   - k8s:cluster:production
	//   - aws:instance-type:t2.micro
	//   - aws:region:us-west-2
	SelectorTypeNode SelectorType = "node"

	// SelectorTypeWorkload represents workload-level selectors for workload attestation.
	// Workload selectors describe properties of processes/containers running on nodes.
	//
	// Examples:
	//   - unix:uid:1000
	//   - unix:user:app-server
	//   - k8s:namespace:default
	//   - k8s:pod:name:frontend-7d5f4c8b9-xk2lp
	SelectorTypeWorkload SelectorType = "workload"
)

// IsValid returns true if the selector type is a recognized value.
// Use this to validate user input or parsed selector types.
//
// Example:
//
//	t := SelectorType("workload")
//	if !t.IsValid() {
//	    return fmt.Errorf("invalid selector type: %s", t)
//	}
func (t SelectorType) IsValid() bool {
	return t == SelectorTypeNode || t == SelectorTypeWorkload
}

// String returns the string representation of the selector type.
func (t SelectorType) String() string {
	return string(t)
}
