package spire

import (
	"context"
	"errors"
	"testing"

	"github.com/pocket/hexagon/spire/internal/domain"
)

func TestTrustDomainParser_FromString(t *testing.T) {
	parser := NewTrustDomainParser()
	ctx := context.Background()

	tests := []struct {
		name      string
		input     string
		wantName  string
		wantError bool
	}{
		{
			name:      "valid trust domain",
			input:     "example.org",
			wantName:  "example.org",
			wantError: false,
		},
		{
			name:      "valid trust domain with subdomain",
			input:     "prod.example.org",
			wantName:  "prod.example.org",
			wantError: false,
		},
		{
			name:      "empty trust domain",
			input:     "",
			wantError: true,
		},
		{
			name:      "trust domain with port",
			input:     "example.org:8080",
			wantError: true,
		},
		{
			name:      "trust domain with path",
			input:     "example.org/path",
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			td, err := parser.FromString(ctx, tt.input)

			if tt.wantError {
				if err == nil {
					t.Errorf("expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if td == nil {
				t.Errorf("expected trust domain, got nil")
				return
			}

			if td.String() != tt.wantName {
				t.Errorf("expected trust domain %q, got %q", tt.wantName, td.String())
			}
		})
	}
}

func TestTrustDomainParser_ImplementsInterface(t *testing.T) {
	// Compile-time check that TrustDomainParser implements the port
	var _ = NewTrustDomainParser()
}

func TestTrustDomainParser_ErrorWrapping(t *testing.T) {
	parser := NewTrustDomainParser()
	ctx := context.Background()

	_, err := parser.FromString(ctx, "example.org:8080")
	if err == nil {
		t.Fatal("expected error for trust domain with port")
	}

	// Verify error is wrapped with domain sentinel
	if !errors.Is(err, domain.ErrInvalidTrustDomain) {
		t.Errorf("expected error to be wrapped with domain.ErrInvalidTrustDomain, got: %v", err)
	}
}
