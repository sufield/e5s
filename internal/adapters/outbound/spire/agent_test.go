package spire

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"errors"
	"math/big"
	"net/url"
	"sync"
	"testing"
	"time"

	"github.com/pocket/hexagon/spire/internal/domain"
	"github.com/pocket/hexagon/spire/internal/ports"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockX509Fetcher implements X509Fetcher for testing
type mockX509Fetcher struct {
	mu         sync.Mutex
	svid       *domain.IdentityDocument
	err        error
	callCnt    int
	closed     bool
	fetchDelay time.Duration // Optional delay to simulate network latency
}

func (m *mockX509Fetcher) FetchX509SVID(ctx context.Context) (*domain.IdentityDocument, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.fetchDelay > 0 {
		time.Sleep(m.fetchDelay)
	}

	m.callCnt++
	if m.err != nil {
		return nil, m.err
	}
	return m.svid, nil
}

func (m *mockX509Fetcher) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.closed = true
	return nil
}

func (m *mockX509Fetcher) getCallCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.callCnt
}

func (m *mockX509Fetcher) isClosed() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.closed
}

// mockCredentialParser implements ports.IdentityCredentialParser for testing
type mockCredentialParser struct{}

func (m *mockCredentialParser) ParseFromString(ctx context.Context, id string) (*domain.IdentityCredential, error) {
	if id == "" {
		return nil, domain.ErrInvalidIdentityCredential
	}
	// Simple parsing for tests
	td := domain.NewTrustDomainFromName("example.org")
	return domain.NewIdentityCredentialFromComponents(td, "/agent"), nil
}

func (m *mockCredentialParser) ParseFromPath(ctx context.Context, trustDomain *domain.TrustDomain, path string) (*domain.IdentityCredential, error) {
	return domain.NewIdentityCredentialFromComponents(trustDomain, path), nil
}

// Helper: Create test certificate with specific validity period
func createTestCert(t *testing.T, spiffeID string, notBefore, notAfter time.Time) (*x509.Certificate, *rsa.PrivateKey) {
	t.Helper()

	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	spiffeURI, err := url.Parse(spiffeID)
	require.NoError(t, err)

	template := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: spiffeID},
		URIs:         []*url.URL{spiffeURI},
		NotBefore:    notBefore,
		NotAfter:     notAfter,
		KeyUsage:     x509.KeyUsageDigitalSignature,
	}

	certDER, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	require.NoError(t, err)

	cert, err := x509.ParseCertificate(certDER)
	require.NoError(t, err)

	return cert, key
}

func TestNewAgent_Success(t *testing.T) {
	ctx := context.Background()
	fetcher := &mockX509Fetcher{}
	parser := &mockCredentialParser{}

	agent, err := NewAgent(ctx, fetcher, "spiffe://example.org/agent", parser)

	require.NoError(t, err)
	require.NotNil(t, agent)
	assert.NotNil(t, agent.agentIdentity)
	assert.Equal(t, 0, fetcher.getCallCount(), "Constructor should not fetch (lazy initialization)")
}

func TestNewAgent_NilClient(t *testing.T) {
	ctx := context.Background()
	parser := &mockCredentialParser{}

	agent, err := NewAgent(ctx, nil, "spiffe://example.org/agent", parser)

	assert.Error(t, err)
	assert.Nil(t, agent)
	assert.Contains(t, err.Error(), "SPIRE client cannot be nil")
}

func TestNewAgent_NilParser(t *testing.T) {
	ctx := context.Background()
	fetcher := &mockX509Fetcher{}

	agent, err := NewAgent(ctx, fetcher, "spiffe://example.org/agent", nil)

	assert.Error(t, err)
	assert.Nil(t, agent)
	assert.Contains(t, err.Error(), "parser cannot be nil")
}

func TestNewAgent_EmptySpiffeID(t *testing.T) {
	ctx := context.Background()
	fetcher := &mockX509Fetcher{}
	parser := &mockCredentialParser{}

	agent, err := NewAgent(ctx, fetcher, "", parser)

	assert.Error(t, err)
	assert.Nil(t, agent)
	assert.Contains(t, err.Error(), "agent SPIFFE ID cannot be empty")
}

func TestGetIdentity_LazyFetch(t *testing.T) {
	ctx := context.Background()

	// Create mock SVID with long validity
	cert, key := createTestCert(t, "spiffe://example.org/agent",
		time.Now().Add(-1*time.Hour),
		time.Now().Add(24*time.Hour))

	td := domain.NewTrustDomainFromName("example.org")
	cred := domain.NewIdentityCredentialFromComponents(td, "/agent")
	doc, err := domain.NewIdentityDocumentFromComponents(cred, cert, key, []*x509.Certificate{cert})
	require.NoError(t, err)

	fetcher := &mockX509Fetcher{svid: doc}
	parser := &mockCredentialParser{}

	agent, err := NewAgent(ctx, fetcher, "spiffe://example.org/agent", parser)
	require.NoError(t, err)

	// First call should fetch
	identity, err := agent.GetIdentity(ctx)
	require.NoError(t, err)
	require.NotNil(t, identity)
	assert.Equal(t, 1, fetcher.getCallCount(), "First GetIdentity should fetch")

	// Second call should use cached (not expired)
	identity2, err := agent.GetIdentity(ctx)
	require.NoError(t, err)
	require.NotNil(t, identity2)
	assert.Equal(t, 1, fetcher.getCallCount(), "Second GetIdentity should use cache")
}

func TestGetIdentity_RenewsExpiringSoon(t *testing.T) {
	ctx := context.Background()

	// Create cert that expires in 1 hour (will be "expiring soon")
	// With 20% threshold, renewal happens when < 12 minutes remain
	cert1, key1 := createTestCert(t, "spiffe://example.org/agent",
		time.Now().Add(-23*time.Hour),
		time.Now().Add(1*time.Hour)) // Total lifetime: 24h, remaining: 1h (4.17%)

	td := domain.NewTrustDomainFromName("example.org")
	cred := domain.NewIdentityCredentialFromComponents(td, "/agent")
	doc1, err := domain.NewIdentityDocumentFromComponents(cred, cert1, key1, []*x509.Certificate{cert1})
	require.NoError(t, err)

	// Create fresh cert for renewal
	cert2, key2 := createTestCert(t, "spiffe://example.org/agent",
		time.Now().Add(-1*time.Hour),
		time.Now().Add(24*time.Hour))
	doc2, err := domain.NewIdentityDocumentFromComponents(cred, cert2, key2, []*x509.Certificate{cert2})
	require.NoError(t, err)

	fetcher := &mockX509Fetcher{svid: doc1}
	parser := &mockCredentialParser{}

	agent, err := NewAgent(ctx, fetcher, "spiffe://example.org/agent", parser)
	require.NoError(t, err)

	// First call fetches doc1
	identity1, err := agent.GetIdentity(ctx)
	require.NoError(t, err)
	assert.Equal(t, 1, fetcher.getCallCount())

	// Update fetcher to return fresh doc2
	fetcher.mu.Lock()
	fetcher.svid = doc2
	fetcher.mu.Unlock()

	// Second call should renew (doc1 expires soon)
	identity2, err := agent.GetIdentity(ctx)
	require.NoError(t, err)
	assert.Equal(t, 2, fetcher.getCallCount(), "Should renew expiring document")
	// identity1 and identity2 are both *domain.IdentityDocument now, compare them directly
	assert.NotEqual(t, identity1, identity2, "Should have different document instances after renewal")
}

func TestGetIdentity_FetchFailure(t *testing.T) {
	ctx := context.Background()

	fetchErr := errors.New("SPIRE unavailable")
	fetcher := &mockX509Fetcher{err: fetchErr}
	parser := &mockCredentialParser{}

	agent, err := NewAgent(ctx, fetcher, "spiffe://example.org/agent", parser)
	require.NoError(t, err)

	// Should fail on first fetch
	identity, err := agent.GetIdentity(ctx)
	assert.Error(t, err)
	assert.Nil(t, identity)
	assert.Contains(t, err.Error(), "refresh agent identity")
}

func TestGetIdentity_DefensiveCopy(t *testing.T) {
	ctx := context.Background()

	cert, key := createTestCert(t, "spiffe://example.org/agent",
		time.Now().Add(-1*time.Hour),
		time.Now().Add(24*time.Hour))

	td := domain.NewTrustDomainFromName("example.org")
	cred := domain.NewIdentityCredentialFromComponents(td, "/agent")
	doc, err := domain.NewIdentityDocumentFromComponents(cred, cert, key, []*x509.Certificate{cert})
	require.NoError(t, err)

	fetcher := &mockX509Fetcher{svid: doc}
	parser := &mockCredentialParser{}

	agent, err := NewAgent(ctx, fetcher, "spiffe://example.org/agent", parser)
	require.NoError(t, err)

	// Get identity twice
	identity1, err := agent.GetIdentity(ctx)
	require.NoError(t, err)

	identity2, err := agent.GetIdentity(ctx)
	require.NoError(t, err)

	// GetIdentity now returns the same *domain.IdentityDocument (immutable)
	assert.Same(t, identity1, identity2, "Returns same immutable document")

	// Verify content is correct
	assert.Equal(t, identity1.IdentityCredential().String(), identity2.IdentityCredential().String())
}

func TestGetIdentity_Concurrency(t *testing.T) {
	ctx := context.Background()

	cert, key := createTestCert(t, "spiffe://example.org/agent",
		time.Now().Add(-1*time.Hour),
		time.Now().Add(24*time.Hour))

	td := domain.NewTrustDomainFromName("example.org")
	cred := domain.NewIdentityCredentialFromComponents(td, "/agent")
	doc, err := domain.NewIdentityDocumentFromComponents(cred, cert, key, []*x509.Certificate{cert})
	require.NoError(t, err)

	fetcher := &mockX509Fetcher{svid: doc, fetchDelay: 10 * time.Millisecond}
	parser := &mockCredentialParser{}

	agent, err := NewAgent(ctx, fetcher, "spiffe://example.org/agent", parser)
	require.NoError(t, err)

	// Run concurrent GetIdentity calls
	const numGoroutines = 10
	var wg sync.WaitGroup
	errors := make(chan error, numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := agent.GetIdentity(ctx)
			if err != nil {
				errors <- err
			}
		}()
	}

	wg.Wait()
	close(errors)

	// No errors should occur
	for err := range errors {
		t.Errorf("Concurrent GetIdentity failed: %v", err)
	}

	// All goroutines may fetch initially (thundering herd on first call)
	// This is acceptable - production code should call GetIdentity once at startup
	// to avoid this. The important thing is no data races occurred.
	assert.LessOrEqual(t, fetcher.getCallCount(), numGoroutines, "Should not fetch more than number of goroutines")
}

func TestGetIdentity_PreventsRefreshStampede(t *testing.T) {
	ctx := context.Background()

	// Create cert expiring soon (triggers refresh)
	cert1, key1 := createTestCert(t, "spiffe://example.org/agent",
		time.Now().Add(-23*time.Hour),
		time.Now().Add(1*time.Hour)) // 4.17% remaining - triggers refresh

	td := domain.NewTrustDomainFromName("example.org")
	cred := domain.NewIdentityCredentialFromComponents(td, "/agent")
	doc1, err := domain.NewIdentityDocumentFromComponents(cred, cert1, key1, []*x509.Certificate{cert1})
	require.NoError(t, err)

	// Create fresh cert for renewal
	cert2, key2 := createTestCert(t, "spiffe://example.org/agent",
		time.Now().Add(-1*time.Hour),
		time.Now().Add(24*time.Hour))
	doc2, err := domain.NewIdentityDocumentFromComponents(cred, cert2, key2, []*x509.Certificate{cert2})
	require.NoError(t, err)

	// Use long delay to simulate slow SPIRE server
	fetcher := &mockX509Fetcher{svid: doc1, fetchDelay: 50 * time.Millisecond}
	parser := &mockCredentialParser{}

	agent, err := NewAgent(ctx, fetcher, "spiffe://example.org/agent", parser)
	require.NoError(t, err)

	// First call to load initial doc1 (expiring soon)
	_, err = agent.GetIdentity(ctx)
	require.NoError(t, err)
	assert.Equal(t, 1, fetcher.getCallCount())

	// Update fetcher to return fresh doc2
	fetcher.mu.Lock()
	fetcher.svid = doc2
	fetcher.mu.Unlock()

	// Now run concurrent GetIdentity calls while doc needs refresh
	// Without stampede prevention, all goroutines would fetch concurrently
	// With stampede prevention (double-check locking), only one goroutine should fetch
	const numGoroutines = 10
	var wg sync.WaitGroup
	errors := make(chan error, numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := agent.GetIdentity(ctx)
			if err != nil {
				errors <- err
			}
		}()
	}

	wg.Wait()
	close(errors)

	// No errors should occur
	for err := range errors {
		t.Errorf("Concurrent GetIdentity failed: %v", err)
	}

	// With stampede prevention, should see exactly 2 fetches total:
	// 1. Initial fetch
	// 2. One refresh (other goroutines wait for the refresh to complete)
	callCount := fetcher.getCallCount()
	assert.Equal(t, 2, callCount, "Should prevent stampede: exactly 2 fetches (initial + one refresh)")
}

func TestFetchIdentityDocument_Success(t *testing.T) {
	ctx := context.Background()

	cert, key := createTestCert(t, "spiffe://example.org/workload",
		time.Now().Add(-1*time.Hour),
		time.Now().Add(24*time.Hour))

	td := domain.NewTrustDomainFromName("example.org")
	cred := domain.NewIdentityCredentialFromComponents(td, "/workload")
	doc, err := domain.NewIdentityDocumentFromComponents(cred, cert, key, []*x509.Certificate{cert})
	require.NoError(t, err)

	fetcher := &mockX509Fetcher{svid: doc}
	parser := &mockCredentialParser{}

	agent, err := NewAgent(ctx, fetcher, "spiffe://example.org/agent", parser)
	require.NoError(t, err)

	// FetchIdentityDocument ignores workload parameter
	workload := domain.NewWorkload(1234, 1000, 1000, "/usr/bin/workload")
	identity, err := agent.FetchIdentityDocument(ctx, workload)

	require.NoError(t, err)
	require.NotNil(t, identity)
	// identity is now *domain.IdentityDocument, not *ports.Identity
	assert.NotNil(t, identity.IdentityCredential())
	assert.Equal(t, "spiffe://example.org/workload", identity.IdentityCredential().String())
	assert.Equal(t, 1, fetcher.getCallCount())
}

func TestFetchIdentityDocument_FetchFailure(t *testing.T) {
	ctx := context.Background()

	fetchErr := errors.New("Workload API unavailable")
	fetcher := &mockX509Fetcher{err: fetchErr}
	parser := &mockCredentialParser{}

	agent, err := NewAgent(ctx, fetcher, "spiffe://example.org/agent", parser)
	require.NoError(t, err)

	workload := domain.NewWorkload(1234, 1000, 1000, "/usr/bin/workload")
	identity, err := agent.FetchIdentityDocument(ctx, workload)

	assert.Error(t, err)
	assert.Nil(t, identity)
	assert.Contains(t, err.Error(), "fetch workload SVID")
}

func TestExtractNameFromCredential(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		expected string
	}{
		{
			name:     "simple path",
			path:     "/workload",
			expected: "workload",
		},
		{
			name:     "kubernetes-style path",
			path:     "/ns/prod/sa/api",
			expected: "api",
		},
		{
			name:     "agent path",
			path:     "/agent",
			expected: "agent",
		},
		{
			name:     "nested path",
			path:     "/service/backend/v1",
			expected: "v1",
		},
		{
			name:     "trailing slash",
			path:     "/workload/",
			expected: "workload",
		},
		{
			name:     "empty path (root ID)",
			path:     "",
			expected: "example.org",
		},
		{
			name:     "root path",
			path:     "/",
			expected: "example.org",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			td := domain.NewTrustDomainFromName("example.org")
			cred := domain.NewIdentityCredentialFromComponents(td, tt.path)
			name := extractNameFromCredential(cred)
			assert.Equal(t, tt.expected, name)
		})
	}
}

func TestExtractNameFromCredential_Nil(t *testing.T) {
	name := extractNameFromCredential(nil)
	assert.Equal(t, "unknown", name)
}

func TestAgent_Close(t *testing.T) {
	ctx := context.Background()
	fetcher := &mockX509Fetcher{}
	parser := &mockCredentialParser{}

	agent, err := NewAgent(ctx, fetcher, "spiffe://example.org/agent", parser)
	require.NoError(t, err)

	// Close should succeed
	err = agent.Close()
	assert.NoError(t, err)
	assert.True(t, fetcher.isClosed(), "Close should close underlying client")

	// Multiple closes should be safe
	err = agent.Close()
	assert.NoError(t, err)
}

func TestAgent_InterfaceCompliance(t *testing.T) {
	// Compile-time checks (already in agent.go)
	var _ ports.Agent = (*Agent)(nil)
	var _ X509Fetcher = (*mockX509Fetcher)(nil)
}

func TestNeedsRefresh_Nil(t *testing.T) {
	assert.True(t, needsRefresh(nil, Options{}), "nil document should be considered expiring")
}

func TestNeedsRefresh_Expired(t *testing.T) {
	cert, key := createTestCert(t, "spiffe://example.org/test",
		time.Now().Add(-2*time.Hour),
		time.Now().Add(-1*time.Hour)) // Expired

	td := domain.NewTrustDomainFromName("example.org")
	cred := domain.NewIdentityCredentialFromComponents(td, "/test")
	doc, err := domain.NewIdentityDocumentFromComponents(cred, cert, key, []*x509.Certificate{cert})
	require.NoError(t, err)

	assert.True(t, needsRefresh(doc, Options{}), "Expired document should be considered expiring")
}

func TestNeedsRefresh_NearExpiry(t *testing.T) {
	// Cert valid for 24 hours, with 1 hour remaining (4.17% of lifetime)
	// Threshold is 20%, so this should trigger renewal
	cert, key := createTestCert(t, "spiffe://example.org/test",
		time.Now().Add(-23*time.Hour),
		time.Now().Add(1*time.Hour))

	td := domain.NewTrustDomainFromName("example.org")
	cred := domain.NewIdentityCredentialFromComponents(td, "/test")
	doc, err := domain.NewIdentityDocumentFromComponents(cred, cert, key, []*x509.Certificate{cert})
	require.NoError(t, err)

	assert.True(t, needsRefresh(doc, Options{}), "Document with <20% lifetime remaining should be expiring")
}

func TestNeedsRefresh_Fresh(t *testing.T) {
	// Cert valid for 24 hours, with 20 hours remaining (83% of lifetime)
	cert, key := createTestCert(t, "spiffe://example.org/test",
		time.Now().Add(-4*time.Hour),
		time.Now().Add(20*time.Hour))

	td := domain.NewTrustDomainFromName("example.org")
	cred := domain.NewIdentityCredentialFromComponents(td, "/test")
	doc, err := domain.NewIdentityDocumentFromComponents(cred, cert, key, []*x509.Certificate{cert})
	require.NoError(t, err)

	assert.False(t, needsRefresh(doc, Options{}), "Fresh document with >20% lifetime should not be expiring")
}

func TestNeedsRefresh_ExactThreshold(t *testing.T) {
	// Cert valid for 24 hours, with exactly 20% remaining (4.8 hours)
	cert, key := createTestCert(t, "spiffe://example.org/test",
		time.Now().Add(-19*time.Hour-12*time.Minute),
		time.Now().Add(4*time.Hour+48*time.Minute))

	td := domain.NewTrustDomainFromName("example.org")
	cred := domain.NewIdentityCredentialFromComponents(td, "/test")
	doc, err := domain.NewIdentityDocumentFromComponents(cred, cert, key, []*x509.Certificate{cert})
	require.NoError(t, err)

	// At exactly 20%, should trigger renewal (<=)
	assert.True(t, needsRefresh(doc, Options{}), "Document at exactly 20% threshold should be expiring")
}

func TestNeedsRefresh_ClockSkewWithinTolerance(t *testing.T) {
	// Cert valid in near future (3 minutes from now) - within 5 minute tolerance
	cert, key := createTestCert(t, "spiffe://example.org/test",
		time.Now().Add(3*time.Minute),
		time.Now().Add(24*time.Hour))

	td := domain.NewTrustDomainFromName("example.org")
	cred := domain.NewIdentityCredentialFromComponents(td, "/test")
	doc, err := domain.NewIdentityDocumentFromComponents(cred, cert, key, []*x509.Certificate{cert})
	require.NoError(t, err)

	// Within 5 minute skew tolerance, should not trigger refresh
	assert.False(t, needsRefresh(doc, Options{}), "Cert with NotBefore within 5min should be acceptable")
}

func TestNeedsRefresh_ClockSkewBeyondTolerance(t *testing.T) {
	// Cert valid far in future (10 minutes from now) - beyond 5 minute tolerance
	cert, key := createTestCert(t, "spiffe://example.org/test",
		time.Now().Add(10*time.Minute),
		time.Now().Add(24*time.Hour))

	td := domain.NewTrustDomainFromName("example.org")
	cred := domain.NewIdentityCredentialFromComponents(td, "/test")
	doc, err := domain.NewIdentityDocumentFromComponents(cred, cert, key, []*x509.Certificate{cert})
	require.NoError(t, err)

	// Beyond 5 minute skew tolerance, should trigger refresh (suspicious)
	assert.True(t, needsRefresh(doc, Options{}), "Cert with NotBefore >5min in future should trigger refresh")
}

func TestNeedsRefresh_ZeroLifetime(t *testing.T) {
	// Malformed cert: NotBefore == NotAfter (zero lifetime)
	now := time.Now()
	cert, key := createTestCert(t, "spiffe://example.org/test", now, now)

	td := domain.NewTrustDomainFromName("example.org")
	cred := domain.NewIdentityCredentialFromComponents(td, "/test")
	doc, err := domain.NewIdentityDocumentFromComponents(cred, cert, key, []*x509.Certificate{cert})
	require.NoError(t, err)

	assert.True(t, needsRefresh(doc, Options{}), "Cert with zero lifetime should trigger refresh")
}

func TestNeedsRefresh_NegativeLifetime(t *testing.T) {
	// Malformed cert: NotAfter before NotBefore (negative lifetime)
	cert, key := createTestCert(t, "spiffe://example.org/test",
		time.Now().Add(1*time.Hour),
		time.Now().Add(-1*time.Hour)) // NotAfter before NotBefore!

	td := domain.NewTrustDomainFromName("example.org")
	cred := domain.NewIdentityCredentialFromComponents(td, "/test")
	doc, err := domain.NewIdentityDocumentFromComponents(cred, cert, key, []*x509.Certificate{cert})
	require.NoError(t, err)

	assert.True(t, needsRefresh(doc, Options{}), "Cert with negative lifetime should trigger refresh")
}
