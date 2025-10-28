package spire

import (
	"context"
	"errors"
	"testing"

	"github.com/sufield/e5s/internal/domain"
)

func TestIdentityCredentialParser_ParseFromString(t *testing.T) {
	parser := NewIdentityCredentialParser()
	ctx := context.Background()

	tests := []struct {
		name      string
		input     string
		wantID    string
		wantError bool
	}{
		{
			name:      "valid SPIFFE ID",
			input:     "spiffe://example.org/host",
			wantID:    "spiffe://example.org/host",
			wantError: false,
		},
		{
			name:      "valid SPIFFE ID with complex path",
			input:     "spiffe://example.org/workload/app",
			wantID:    "spiffe://example.org/workload/app",
			wantError: false,
		},
		{
			name:      "root SPIFFE ID (no path)",
			input:     "spiffe://example.org",
			wantID:    "spiffe://example.org/",
			wantError: false,
		},
		{
			name:      "whitespace trimmed",
			input:     "  spiffe://example.org/host  ",
			wantID:    "spiffe://example.org/host",
			wantError: false,
		},
		{
			name:      "whitespace only",
			input:     "   ",
			wantError: true,
		},
		{
			name:      "empty identity credential",
			input:     "",
			wantError: true,
		},
		{
			name:      "invalid scheme",
			input:     "http://example.org/host",
			wantError: true,
		},
		{
			name:      "missing trust domain",
			input:     "spiffe:///host",
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ic, err := parser.ParseFromString(ctx, tt.input)

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

			if ic == nil {
				t.Errorf("expected identity credential, got nil")
				return
			}

			if ic.String() != tt.wantID {
				t.Errorf("expected identity credential %q, got %q", tt.wantID, ic.String())
			}
		})
	}
}

func TestIdentityCredentialParser_ParseFromPath(t *testing.T) {
	parser := NewIdentityCredentialParser()
	ctx := context.Background()

	trustDomain := domain.NewTrustDomainFromName("example.org")

	tests := []struct {
		name      string
		path      string
		wantID    string
		wantError bool
	}{
		{
			name:      "path with leading slash",
			path:      "/host",
			wantID:    "spiffe://example.org/host",
			wantError: false,
		},
		{
			name:      "path without leading slash",
			path:      "host",
			wantID:    "spiffe://example.org/host",
			wantError: false,
		},
		{
			name:      "root path with slash",
			path:      "/",
			wantID:    "spiffe://example.org/",
			wantError: false,
		},
		{
			name:      "empty path (root ID)",
			path:      "",
			wantID:    "spiffe://example.org/",
			wantError: false,
		},
		{
			name:      "complex path",
			path:      "/workload/app",
			wantID:    "spiffe://example.org/workload/app",
			wantError: false,
		},
		{
			name:      "double slashes normalized",
			path:      "//svc//a",
			wantID:    "spiffe://example.org/svc/a",
			wantError: false,
		},
		{
			name:      "multiple double slashes",
			path:      "/workload//service//app",
			wantID:    "spiffe://example.org/workload/service/app",
			wantError: false,
		},
		{
			name:      "whitespace trimmed",
			path:      "  /host  ",
			wantID:    "spiffe://example.org/host",
			wantError: false,
		},
		{
			name:      "whitespace trimmed from relative path",
			path:      "  host  ",
			wantID:    "spiffe://example.org/host",
			wantError: false,
		},
		{
			name:      "deep nested path",
			path:      "/ns/production/workload/api/v1",
			wantID:    "spiffe://example.org/ns/production/workload/api/v1",
			wantError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ic, err := parser.ParseFromPath(ctx, trustDomain, tt.path)

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

			if ic == nil {
				t.Errorf("expected identity credential, got nil")
				return
			}

			if ic.String() != tt.wantID {
				t.Errorf("expected identity credential %q, got %q", tt.wantID, ic.String())
			}
		})
	}
}

func TestIdentityCredentialParser_ParseFromPath_NilTrustDomain(t *testing.T) {
	parser := NewIdentityCredentialParser()
	ctx := context.Background()

	_, err := parser.ParseFromPath(ctx, nil, "/host")
	if err == nil {
		t.Fatal("expected error for nil trust domain")
	}

	// Verify error is wrapped with domain sentinel
	if !errors.Is(err, domain.ErrInvalidIdentityCredential) {
		t.Errorf("expected error to be wrapped with domain.ErrInvalidIdentityCredential, got: %v", err)
	}
}

func TestIdentityCredentialParser_ImplementsInterface(t *testing.T) {
	// Compile-time check that IdentityCredentialParser implements the port
	var _ = NewIdentityCredentialParser()
}
