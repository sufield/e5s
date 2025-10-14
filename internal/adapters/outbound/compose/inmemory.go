//go:build dev

package compose

import (
	"context"
	"crypto/x509"

	"github.com/pocket/hexagon/spire/internal/adapters/outbound/inmemory"
	"github.com/pocket/hexagon/spire/internal/adapters/outbound/inmemory/attestor"
	"github.com/pocket/hexagon/spire/internal/domain"
	"github.com/pocket/hexagon/spire/internal/ports"
)

// InMemoryAdapterFactory provides in-memory implementations of all adapters
// Implements the AdapterFactory port for in-memory/walking skeleton mode
//
// NOTE: This file is excluded from production builds via build tag.
// Production deployments use SPIREAdapterFactory (defined in spire.go) instead.
type InMemoryAdapterFactory struct{}

// NewInMemoryAdapterFactory creates the factory for in-memory adapters
func NewInMemoryAdapterFactory() *InMemoryAdapterFactory {
	return &InMemoryAdapterFactory{}
}

func (f *InMemoryAdapterFactory) CreateRegistry(ctx context.Context, workloads []ports.WorkloadEntry, parser ports.IdentityCredentialParser) (*inmemory.InMemoryRegistry, error) {
	registry := inmemory.NewInMemoryRegistry()

	// Seed registry with workload configurations
	for _, workload := range workloads {
		identityCredential, err := parser.ParseFromString(ctx, workload.SpiffeID)
		if err != nil {
			return nil, err
		}

		selector, err := domain.ParseSelectorFromString(workload.Selector)
		if err != nil {
			return nil, err
		}

		selectorSet := domain.NewSelectorSet()
		selectorSet.Add(selector)

		mapper, err := domain.NewIdentityMapper(identityCredential, selectorSet)
		if err != nil {
			return nil, err
		}

		if err := registry.Seed(ctx, mapper); err != nil {
			return nil, err
		}
	}

	return registry, nil
}

func (f *InMemoryAdapterFactory) CreateTrustDomainParser() ports.TrustDomainParser {
	return inmemory.NewInMemoryTrustDomainParser()
}

func (f *InMemoryAdapterFactory) CreateIdentityCredentialParser() ports.IdentityCredentialParser {
	return inmemory.NewInMemoryIdentityCredentialParser()
}

// CreateIdentityDocumentValidator creates an identity document validator.
// In-memory implementation provides both validation and creation, so this returns
// the same provider instance that CreateIdentityDocumentProvider returns.
// This satisfies BaseAdapterFactory interface (validator only).
func (f *InMemoryAdapterFactory) CreateIdentityDocumentValidator() ports.IdentityDocumentValidator {
	return inmemory.NewInMemoryIdentityDocumentProvider()
}

// CreateIdentityDocumentProvider creates a full identity document provider.
// In-memory implementation needs creation capability for certificate generation.
// This satisfies DevelopmentAdapterFactory interface (full provider).
func (f *InMemoryAdapterFactory) CreateIdentityDocumentProvider() ports.IdentityDocumentProvider {
	return inmemory.NewInMemoryIdentityDocumentProvider()
}

func (f *InMemoryAdapterFactory) CreateTrustBundleProvider(server ports.IdentityServer) ports.TrustBundleProvider {
	if server == nil {
		return inmemory.NewInMemoryTrustBundleProvider(nil)
	}

	// Try to get bundle from InMemoryServer's internal methods (type assertion)
	// These are internal implementation details, not part of ports.IdentityServer interface
	type caBundler interface {
		GetCABundle() []*x509.Certificate
	}
	type caGetter interface {
		GetCA() *x509.Certificate
	}

	var bundle []*x509.Certificate

	// Prefer full bundle if available
	if s, ok := server.(caBundler); ok {
		b := s.GetCABundle()
		if len(b) > 0 {
			// Defensive copy to prevent mutations
			bundle = make([]*x509.Certificate, len(b))
			copy(bundle, b)
			return inmemory.NewInMemoryTrustBundleProvider(bundle)
		}
	}

	// Fallback to single CA (internal method, not in ports.IdentityServer interface)
	if s, ok := server.(caGetter); ok {
		if ca := s.GetCA(); ca != nil {
			// Defensive copy (single element slice)
			bundle = make([]*x509.Certificate, 1)
			bundle[0] = ca // Pointer copy is fine for immutable certs, but slice is copied
			return inmemory.NewInMemoryTrustBundleProvider(bundle)
		}
	}

	return inmemory.NewInMemoryTrustBundleProvider(nil)
}

// CreateServer creates an in-memory server for development/testing.
func (f *InMemoryAdapterFactory) CreateServer(ctx context.Context, trustDomain string, trustDomainParser ports.TrustDomainParser, docProvider ports.IdentityDocumentProvider) (*inmemory.InMemoryServer, error) {
	return inmemory.NewInMemoryServer(ctx, trustDomain, trustDomainParser, docProvider)
}

func (f *InMemoryAdapterFactory) CreateAttestor(workloads []ports.WorkloadEntry) *attestor.UnixWorkloadAttestor {
	a := attestor.NewUnixWorkloadAttestor()

	// Register UIDs from workload configurations
	for _, workload := range workloads {
		a.RegisterUID(workload.UID, workload.Selector)
	}

	return a
}

// CreateAgent creates an in-memory agent for development/testing.
func (f *InMemoryAdapterFactory) CreateAgent(
	ctx context.Context,
	spiffeID string,
	server *inmemory.InMemoryServer,
	registry *inmemory.InMemoryRegistry,
	attestor *attestor.UnixWorkloadAttestor,
	parser ports.IdentityCredentialParser,
	docProvider ports.IdentityDocumentProvider,
) (ports.Agent, error) {
	return inmemory.NewInMemoryAgent(ctx, spiffeID, server, registry, attestor, parser, docProvider)
}
