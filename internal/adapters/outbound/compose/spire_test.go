package compose

import (
	"context"
	"testing"

	"github.com/pocket/hexagon/spire/internal/adapters/outbound/spire"
	"github.com/pocket/hexagon/spire/internal/ports"
)

// TestSPIREAdapterFactory_InterfaceCompliance verifies the factory implements
// the production interface hierarchy correctly.
func TestSPIREAdapterFactory_InterfaceCompliance(t *testing.T) {
	// Skip actual instantiation (requires SPIRE infrastructure)
	// Just verify compile-time interface compliance
	var (
		_ ports.BaseAdapterFactory     = (*SPIREAdapterFactory)(nil)
		_ ports.ProductionAgentFactory = (*SPIREAdapterFactory)(nil)
		_ ports.CoreAdapterFactory     = (*SPIREAdapterFactory)(nil)
	)
}

// TestNewSPIREAdapterFactory_NilConfig verifies error handling for nil config.
func TestNewSPIREAdapterFactory_NilConfig(t *testing.T) {
	ctx := context.Background()
	_, err := NewSPIREAdapterFactory(ctx, nil)
	if err == nil {
		t.Fatal("expected error for nil config, got nil")
	}
	expectedMsg := "config cannot be nil"
	if err.Error() != expectedMsg {
		t.Errorf("expected error message %q, got %q", expectedMsg, err.Error())
	}
}

// TestSPIREAdapterFactory_CreateProductionAgent_EmptySpiffeID verifies validation.
func TestSPIREAdapterFactory_CreateProductionAgent_EmptySpiffeID(t *testing.T) {
	// Create a mock factory (without actual SPIRE client)
	factory := &SPIREAdapterFactory{
		config: &spire.Config{},
		client: nil, // Will cause error if method tries to use it
	}

	ctx := context.Background()
	parser := factory.CreateIdentityCredentialParser()

	_, err := factory.CreateProductionAgent(ctx, "", parser)
	if err == nil {
		t.Fatal("expected error for empty SPIFFE ID, got nil")
	}
	expectedMsg := "SPIFFE ID cannot be empty"
	if err.Error() != expectedMsg {
		t.Errorf("expected error message %q, got %q", expectedMsg, err.Error())
	}
}

// NOTE: CreateServer test removed - production SPIRE adapter no longer provides server.
// Production workloads are clients only. For server functionality, use InMemoryAdapterFactory.

// TestSPIREAdapterFactory_Close_NilClient verifies nil-safe close.
func TestSPIREAdapterFactory_Close_NilClient(t *testing.T) {
	factory := &SPIREAdapterFactory{
		config: &spire.Config{},
		client: nil, // Nil client should not panic
	}

	err := factory.Close()
	if err != nil {
		t.Errorf("expected no error for nil client close, got: %v", err)
	}
}

// TestSPIREAdapterFactory_CreateParsers verifies parser creation returns non-nil.
func TestSPIREAdapterFactory_CreateParsers(t *testing.T) {
	factory := &SPIREAdapterFactory{
		config: &spire.Config{},
		client: nil,
	}

	t.Run("CreateTrustDomainParser", func(t *testing.T) {
		parser := factory.CreateTrustDomainParser()
		if parser == nil {
			t.Error("expected non-nil trust domain parser")
		}
	})

	t.Run("CreateIdentityCredentialParser", func(t *testing.T) {
		parser := factory.CreateIdentityCredentialParser()
		if parser == nil {
			t.Error("expected non-nil identity credential parser")
		}
	})

	t.Run("CreateIdentityDocumentProvider", func(t *testing.T) {
		provider := factory.CreateIdentityDocumentProvider()
		if provider == nil {
			t.Error("expected non-nil identity document provider")
		}
	})
}
