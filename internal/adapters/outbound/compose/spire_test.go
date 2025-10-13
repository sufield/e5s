package compose

import (
	"context"
	"testing"

	"github.com/pocket/hexagon/spire/internal/adapters/outbound/spire"
	"github.com/pocket/hexagon/spire/internal/ports"
)

// TestSPIREAdapterFactory_InterfaceCompliance verifies the factory implements
// the interface hierarchy correctly.
func TestSPIREAdapterFactory_InterfaceCompliance(t *testing.T) {
	// Skip actual instantiation (requires SPIRE infrastructure)
	// Just verify compile-time interface compliance
	var (
		_ ports.BaseAdapterFactory = (*SPIREAdapterFactory)(nil)
		_ ports.AgentFactory       = (*SPIREAdapterFactory)(nil)
		_ ports.AdapterFactory     = (*SPIREAdapterFactory)(nil)
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

// TestNewSPIREAdapterFactory_EmptySocketPath verifies validation of required config fields.
func TestNewSPIREAdapterFactory_EmptySocketPath(t *testing.T) {
	ctx := context.Background()
	cfg := &spire.Config{
		SocketPath: "", // Empty socket path should fail
	}
	_, err := NewSPIREAdapterFactory(ctx, cfg)
	if err == nil {
		t.Fatal("expected error for empty socket path, got nil")
	}
	expectedMsg := "socket path is required"
	if err.Error() != expectedMsg {
		t.Errorf("expected error message %q, got %q", expectedMsg, err.Error())
	}
}

// TestSPIREAdapterFactory_CreateAgent_EmptySpiffeID verifies validation.
func TestSPIREAdapterFactory_CreateAgent_EmptySpiffeID(t *testing.T) {
	// Create a mock factory (without actual SPIRE client)
	factory := &SPIREAdapterFactory{
		config: &spire.Config{},
		client: nil, // Will cause error if method tries to use it
	}

	ctx := context.Background()
	parser := factory.CreateIdentityCredentialParser()

	_, err := factory.CreateAgent(ctx, "", parser)
	if err == nil {
		t.Fatal("expected error for empty SPIFFE ID, got nil")
	}
	expectedMsg := "SPIFFE ID cannot be empty"
	if err.Error() != expectedMsg {
		t.Errorf("expected error message %q, got %q", expectedMsg, err.Error())
	}
}

// TestSPIREAdapterFactory_CreateAgent_InvalidSpiffeID verifies early SPIFFE ID parsing.
func TestSPIREAdapterFactory_CreateAgent_InvalidSpiffeID(t *testing.T) {
	factory := &SPIREAdapterFactory{
		config: &spire.Config{},
		client: nil,
	}

	ctx := context.Background()
	parser := factory.CreateIdentityCredentialParser()

	// Invalid SPIFFE ID (not a valid format)
	_, err := factory.CreateAgent(ctx, "not-a-valid-spiffe-id", parser)
	if err == nil {
		t.Fatal("expected error for invalid SPIFFE ID, got nil")
	}
	// Should contain "invalid SPIFFE ID" in the error message
	if err.Error() == "" {
		t.Error("expected non-empty error message")
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

// TestSPIREAdapterFactory_Close_Idempotent verifies close is idempotent.
func TestSPIREAdapterFactory_Close_Idempotent(t *testing.T) {
	factory := &SPIREAdapterFactory{
		config: &spire.Config{},
		client: nil,
	}

	// First close
	err1 := factory.Close()
	if err1 != nil {
		t.Errorf("expected no error on first close, got: %v", err1)
	}

	// Second close (should be no-op)
	err2 := factory.Close()
	if err2 != nil {
		t.Errorf("expected no error on second close (idempotent), got: %v", err2)
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

	t.Run("CreateIdentityDocumentValidator", func(t *testing.T) {
		validator := factory.CreateIdentityDocumentValidator()
		if validator == nil {
			t.Error("expected non-nil identity document validator")
		}
	})
}
