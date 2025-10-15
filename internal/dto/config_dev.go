//go:build dev

package dto

// WorkloadEntry is used to seed an in-memory registry (dev-only).
type WorkloadEntry struct {
	SpiffeID string `json:"spiffeId" yaml:"spiffeId"`
	Selector string `json:"selector" yaml:"selector"`
	UID      int    `json:"uid" yaml:"uid"`
}
