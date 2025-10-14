//go:build dev

package ports

// Message is demo-only: authenticated message exchange for CLI demos.
type Message struct {
	From    Identity `json:"from" yaml:"from"`
	To      Identity `json:"to" yaml:"to"`
	Content string   `json:"content" yaml:"content"`
}
