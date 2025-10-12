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
// Production implementation: SPIREClient via Workload API.
// Test implementation: mock for unit tests.
type X509Fetcher interface {
	FetchX509SVID(ctx context.Context) (*domain.IdentityDocument, error)
	Close() error
}

// Agent implements the ports.Agent interface by delegating to external SPIRE.
// This agent does NOT do local selector matching or attestation.
// It fully delegates to SPIRE Server's registration entries and workload attestation.
//
// Concurrency: All methods are safe for concurrent use. GetIdentity uses
// read-write locking to prevent data races during identity refresh.
type Agent struct {
	client           X509Fetcher
	credentialParser ports.IdentityCredentialParser
	agentSpiffeID    string

	mu            sync.RWMutex
	agentIdentity *ports.Identity // Guarded by mu
}

// NewAgent creates a new SPIRE agent that fully delegates to external SPIRE.
//
// Lifecycle Note: Construction is cheap (no network I/O). The first call to
// GetIdentity() or FetchIdentityDocument() will perform the initial SVID fetch.
// This prevents construction failures when SPIRE is temporarily unavailable.
//
// Parameters:
//   - ctx: Used only for initial validation (parsing SPIFFE ID)
//   - client: X509Fetcher implementation (typically SPIREClient)
//   - agentSpiffeID: Expected SPIFFE ID for this agent (e.g., "spiffe://example.org/agent")
//   - parser: IdentityCredentialParser for SPIFFE ID validation
//
// Returns error if:
//   - client or parser is nil
//   - agentSpiffeID is empty or invalid format
func NewAgent(
	ctx context.Context,
	client X509Fetcher,
	agentSpiffeID string,
	parser ports.IdentityCredentialParser,
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

	// Parse and validate agent SPIFFE ID (fast, no network)
	agentCredential, err := parser.ParseFromString(ctx, agentSpiffeID)
	if err != nil {
		return nil, fmt.Errorf("parse agent SPIFFE ID: %w", err)
	}

	return &Agent{
		client:           client,
		credentialParser: parser,
		agentSpiffeID:    agentSpiffeID,
		agentIdentity: &ports.Identity{
			IdentityCredential: agentCredential,
			IdentityDocument:   nil, // Fetched lazily on first GetIdentity()
			Name:               extractNameFromCredential(agentCredential),
		},
	}, nil
}

// GetIdentity returns the agent's own identity.
//
// Concurrency: Safe for concurrent use. Returns a defensive copy to prevent
// external mutations of internal state.
//
// Lifecycle: First call performs initial SVID fetch. Subsequent calls return
// cached identity, refreshing only when the document expires soon (within 20%
// of remaining lifetime).
//
// Renewal Strategy: Documents are renewed proactively before expiration to
// avoid returning expired documents under load. A document is considered
// "expiring soon" when 80% of its lifetime has passed.
//
// Returns error if:
//   - Initial fetch fails (first call only)
//   - Refresh fails for expiring document
func (a *Agent) GetIdentity(ctx context.Context) (*ports.Identity, error) {
	// Fast path: check if refresh needed (read lock)
	a.mu.RLock()
	current := a.agentIdentity
	a.mu.RUnlock()

	// Check if we need to fetch/refresh
	needsRefresh := current == nil ||
		current.IdentityDocument == nil ||
		expiresSoon(current.IdentityDocument)

	if needsRefresh {
		// Fetch fresh document
		doc, err := a.client.FetchX509SVID(ctx)
		if err != nil {
			return nil, fmt.Errorf("refresh agent identity: %w", err)
		}

		// Sanity check: Verify fetched document matches expected credential
		// This catches configuration mismatches (e.g., wrong SPIFFE ID registered)
		if !doc.IdentityCredential().Equals(current.IdentityCredential) {
			return nil, fmt.Errorf("fetched document identity %s does not match expected %s",
				doc.IdentityCredential().String(),
				current.IdentityCredential.String())
		}

		// Update cached identity (write lock)
		next := &ports.Identity{
			IdentityCredential: current.IdentityCredential,
			IdentityDocument:   doc,
			Name:               current.Name,
		}
		a.mu.Lock()
		a.agentIdentity = next
		a.mu.Unlock()
		current = next
	}

	// Return defensive copy to prevent caller mutations
	out := *current
	return &out, nil
}

// expiresSoon returns true if the document is expired or will expire within
// 20% of its remaining lifetime. This provides a buffer for renewal before
// actual expiration.
//
// Example: If a cert is valid for 24 hours, it will be renewed after 19.2 hours
// (80% elapsed), leaving a 4.8-hour buffer before expiration.
func expiresSoon(doc *domain.IdentityDocument) bool {
	if doc == nil {
		return true
	}

	// Check if already expired (fast path)
	if doc.IsExpired() {
		return true
	}

	// Calculate renewal threshold (20% of lifetime remaining)
	cert := doc.Certificate()
	if cert == nil {
		return true
	}

	total := cert.NotAfter.Sub(cert.NotBefore)
	remaining := time.Until(cert.NotAfter)

	// Renew when 20% or less of lifetime remains
	return remaining <= total/5
}

// FetchIdentityDocument fetches an identity document for THIS process.
//
// IMPORTANT: SPIRE Workload API can ONLY fetch SVIDs for the calling process
// (authenticated via Unix domain socket peer credentials). It CANNOT fetch
// SVIDs for arbitrary processes. The workload parameter is accepted for
// interface compatibility but is IGNORED in production mode.
//
// PRODUCTION MODE: Fully delegates to SPIRE Agent/Server
// This does NOT do local attestation or selector matching. SPIRE handles:
//  1. Workload attestation (SPIRE Agent extracts selectors from calling process)
//  2. Selector matching against registration entries (SPIRE Server)
//  3. SVID issuance for the matched identity (SPIRE Server)
//  4. SVID delivery via Workload API (SPIRE Agent)
//
// Parameters:
//   - ctx: Context for timeout/cancellation
//   - workload: IGNORED in production (Workload API only attests caller)
//
// Returns:
//   - Identity with SPIRE-issued SVID for THIS process
//   - Error if Workload API unavailable or no registration entry matches
func (a *Agent) FetchIdentityDocument(ctx context.Context, _ ports.ProcessIdentity) (*ports.Identity, error) {
	// Fetch X.509 SVID from SPIRE Workload API
	// The Workload API will:
	//   1. Authenticate the calling process (Unix domain socket peer credentials)
	//   2. Request attestation from SPIRE Server (extract platform selectors)
	//   3. Match selectors against registration entries (SPIRE Server)
	//   4. Issue and return the appropriate SVID
	doc, err := a.client.FetchX509SVID(ctx)
	if err != nil {
		return nil, fmt.Errorf("fetch workload SVID from SPIRE: %w", err)
	}

	// Extract the identity credential from the document
	credential := doc.IdentityCredential()

	// Create identity with the SPIRE-issued document
	identity := &ports.Identity{
		IdentityCredential: credential,
		IdentityDocument:   doc,
		Name:               extractNameFromCredential(credential),
	}

	return identity, nil
}

// extractNameFromCredential extracts a human-readable name from identity credential.
// Uses the last path segment for readability (e.g., "/ns/prod/sa/api" → "api").
//
// Examples:
//   - "spiffe://example.org/workload" → "workload"
//   - "spiffe://example.org/ns/prod/sa/api" → "api"
//   - "spiffe://example.org/agent" → "agent"
func extractNameFromCredential(credential *domain.IdentityCredential) string {
	if credential == nil {
		return "unknown"
	}

	// Split path by "/" and get last non-empty segment
	path := credential.Path()
	segments := strings.Split(strings.Trim(path, "/"), "/")

	if len(segments) == 0 || segments[0] == "" {
		return "unknown"
	}

	// Return last segment (most specific identifier)
	return segments[len(segments)-1]
}

// Close releases resources held by the agent.
// Safe to call multiple times.
func (a *Agent) Close() error {
	if a.client == nil {
		return nil
	}
	return a.client.Close()
}

// Compile-time interface compliance checks
var _ ports.Agent = (*Agent)(nil)
var _ X509Fetcher = (*SPIREClient)(nil)
