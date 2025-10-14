package spire

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/pocket/hexagon/spire/internal/domain"
	"github.com/pocket/hexagon/spire/internal/ports"
)

// X509Fetcher abstracts SVID fetching for testability.
type X509Fetcher interface {
	FetchX509SVID(ctx context.Context) (*domain.IdentityDocument, error)
	Close() error
}

// Options tunes the agent's refresh behavior.
type Options struct {
	// RenewWhenRemainingAtOrBelow is the fraction of lifetime remaining
	// at which to refresh (e.g., 0.20 means refresh when <=20% remains).
	// If <= 0, defaults to 0.20.
	RenewWhenRemainingAtOrBelow float64

	// NotBeforeSkew is tolerated clock skew for NotBefore being in the future.
	// If <= 0, defaults to 5m.
	NotBeforeSkew time.Duration
}

func (o *Options) normalize() {
	if o.RenewWhenRemainingAtOrBelow <= 0 || o.RenewWhenRemainingAtOrBelow >= 1 {
		o.RenewWhenRemainingAtOrBelow = 0.20
	}
	if o.NotBeforeSkew <= 0 {
		o.NotBeforeSkew = 5 * time.Minute
	}
}

// Agent delegates to external SPIRE (no local selector matching).
// Concurrency: safe for concurrent use.
type Agent struct {
	client X509Fetcher
	opts   Options

	mu            sync.RWMutex
	agentIdentity *ports.Identity // guarded by mu; never nil after ctor
}

// NewAgent constructs an Agent and seeds the expected identity credential via the parser.
// SVID is fetched lazily on first GetIdentity/FetchIdentityDocument.
func NewAgent(
	ctx context.Context,
	client X509Fetcher,
	agentSpiffeID string,
	parser ports.IdentityCredentialParser,
) (*Agent, error) {
	return NewAgentWithOptions(ctx, client, agentSpiffeID, parser, Options{})
}

// NewAgentWithOptions is like NewAgent with custom Options.
func NewAgentWithOptions(
	ctx context.Context,
	client X509Fetcher,
	agentSpiffeID string,
	parser ports.IdentityCredentialParser,
	opts Options,
) (*Agent, error) {
	if client == nil {
		return nil, fmt.Errorf("SPIRE client cannot be nil")
	}
	if parser == nil {
		return nil, fmt.Errorf("parser cannot be nil")
	}
	if agentSpiffeID == "" {
		return nil, fmt.Errorf("agent SPIFFE ID cannot be empty")
	}

	cred, err := parser.ParseFromString(ctx, agentSpiffeID)
	if err != nil {
		return nil, fmt.Errorf("parse agent SPIFFE ID: %w", err)
	}

	opts.normalize()

	return &Agent{
		client: client,
		opts:   opts,
		agentIdentity: &ports.Identity{
			IdentityCredential: cred,
			IdentityDocument:   nil, // fetched lazily
			Name:               extractNameFromCredential(cred),
		},
	}, nil
}

// GetIdentity returns the agent's identity (credential + current SVID).
// Lazily fetches and proactively refreshes when expiring soon.
func (a *Agent) GetIdentity(ctx context.Context) (*ports.Identity, error) {
	a.mu.RLock()
	current := a.agentIdentity
	need := needsRefresh(current.IdentityDocument, a.opts)
	a.mu.RUnlock()

	if need {
		a.mu.Lock()
		// Re-check under write lock to prevent stampede.
		current = a.agentIdentity
		if needsRefresh(current.IdentityDocument, a.opts) {
			doc, err := a.client.FetchX509SVID(ctx)
			if err != nil {
				a.mu.Unlock()
				return nil, fmt.Errorf("refresh agent identity: %w", err)
			}
			// Ensure fetched SVID matches expected credential.
			if !doc.IdentityCredential().Equals(current.IdentityCredential) {
				a.mu.Unlock()
				return nil, fmt.Errorf(
					"fetched document identity %s does not match expected %s",
					doc.IdentityCredential().String(),
					current.IdentityCredential.String(),
				)
			}
			a.agentIdentity = &ports.Identity{
				IdentityCredential: current.IdentityCredential,
				IdentityDocument:   doc,
				Name:               current.Name,
			}
			current = a.agentIdentity
		}
		a.mu.Unlock()
	}

	// Return a shallow copy to discourage mutation of the cached struct.
	out := *current
	return &out, nil
}

// FetchIdentityDocument fetches an SVID for THIS process via Workload API.
func (a *Agent) FetchIdentityDocument(ctx context.Context, _ ports.ProcessIdentity) (*domain.IdentityDocument, error) {
	doc, err := a.client.FetchX509SVID(ctx)
	if err != nil {
		return nil, fmt.Errorf("fetch workload SVID from SPIRE: %w", err)
	}
	return doc, nil
}

// Close releases resources. Idempotent.
func (a *Agent) Close() error {
	if a.client == nil {
		return nil
	}
	return a.client.Close()
}

// --- helpers ---

// needsRefresh decides whether to renew based on expiration, NotBefore skew, and threshold.
func needsRefresh(doc *domain.IdentityDocument, opts Options) bool {
	opts.normalize()

	if doc == nil || doc.Certificate() == nil || doc.IsExpired() {
		return true
	}

	cert := doc.Certificate()
	now := time.Now()

	// If NotBefore is significantly in the future, treat as invalid/stale.
	if !now.After(cert.NotBefore) && cert.NotBefore.Sub(now) > opts.NotBeforeSkew {
		return true
	}

	life := cert.NotAfter.Sub(cert.NotBefore)
	if life <= 0 {
		return true
	}
	remaining := cert.NotAfter.Sub(now)
	return remaining <= time.Duration(opts.RenewWhenRemainingAtOrBelow*float64(life))
}

func extractNameFromCredential(cred *domain.IdentityCredential) string {
	if cred == nil {
		return "unknown"
	}
	parts := strings.Split(strings.Trim(cred.Path(), "/"), "/")
	if len(parts) == 0 || parts[0] == "" {
		// For root IDs, return the trust domain (more helpful than "unknown").
		return cred.TrustDomain().String()
	}
	return parts[len(parts)-1]
}

// Compile-time interface compliance.
var _ ports.Agent = (*Agent)(nil)
