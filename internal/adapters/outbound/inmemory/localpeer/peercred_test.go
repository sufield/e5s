//go:build linux && dev

package localpeer

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWithCred_and_FromCtx(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		cred Cred
	}{
		{
			name: "basic credentials",
			cred: Cred{PID: 1234, UID: 1000, GID: 1000},
		},
		{
			name: "root credentials",
			cred: Cred{PID: 1, UID: 0, GID: 0},
		},
		{
			name: "high UID",
			cred: Cred{PID: 99999, UID: 65534, GID: 65534},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Store cred in context
			ctx := context.Background()
			ctx = WithCred(ctx, tt.cred)

			// Retrieve cred from context
			got, err := FromCtx(ctx)
			if err != nil {
				t.Fatalf("FromCtx() error = %v", err)
			}

			// Verify all fields match
			if got.PID != tt.cred.PID {
				t.Errorf("PID = %d, want %d", got.PID, tt.cred.PID)
			}
			if got.UID != tt.cred.UID {
				t.Errorf("UID = %d, want %d", got.UID, tt.cred.UID)
			}
			if got.GID != tt.cred.GID {
				t.Errorf("GID = %d, want %d", got.GID, tt.cred.GID)
			}
		})
	}
}

func TestFromCtx_WrongType(t *testing.T) {
	t.Parallel()

	// Store wrong type in context
	ctx := context.WithValue(context.Background(), ctxKey, "wrong type")

	_, err := FromCtx(ctx)
	if err == nil {
		t.Error("FromCtx() expected error when wrong type in context, got nil")
	}

	if !strings.Contains(err.Error(), "invalid type") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestFromCtx_NoCred(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	_, err := FromCtx(ctx)
	if err == nil {
		t.Error("FromCtx() expected error when no cred in context")
	}

	if !strings.Contains(err.Error(), "no local peer cred") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestGetExecutablePath_CurrentProcess(t *testing.T) {
	t.Parallel()

	// Get our own PID
	pid := int32(os.Getpid())

	exe, err := GetExecutablePath(pid)
	if err != nil {
		t.Fatalf("GetExecutablePath() error = %v", err)
	}

	// Use dynamic binary name match
	expected := filepath.Base(os.Args[0])
	if exe != expected {
		t.Errorf("GetExecutablePath() = %q, want %q", exe, expected)
	}

	// Should not be empty
	if exe == "" {
		t.Error("GetExecutablePath() returned empty string")
	}
}

func TestGetExecutablePath_InvalidPID(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		pid  int32
	}{
		{
			name: "non-existent PID",
			pid:  999999,
		},
		{
			name: "zero PID",
			pid:  0,
		},
		{
			name: "negative PID",
			pid:  -1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			exe, err := GetExecutablePath(tt.pid)
			if err == nil {
				t.Errorf("GetExecutablePath(%d) expected error, got nil", tt.pid)
			}

			// Should return empty string on error (not "unknown")
			if exe != "" {
				t.Errorf("GetExecutablePath(%d) = %q, want empty string on error", tt.pid, exe)
			}
		})
	}
}

func TestFormatSyntheticSPIFFEID(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		cred        Cred
		trustDomain string
		wantPrefix  string
		wantContain string
		wantErr     bool
	}{
		{
			name:        "basic user",
			cred:        Cred{PID: int32(os.Getpid()), UID: 1000, GID: 1000},
			trustDomain: "dev.local",
			wantPrefix:  "spiffe://dev.local/uid-1000/",
			wantContain: "test", // running in test binary
			wantErr:     false,
		},
		{
			name:        "root user",
			cred:        Cred{PID: int32(os.Getpid()), UID: 0, GID: 0},
			trustDomain: "dev.local",
			wantPrefix:  "spiffe://dev.local/uid-0/",
			wantContain: "test",
			wantErr:     false,
		},
		{
			name:        "custom trust domain",
			cred:        Cred{PID: int32(os.Getpid()), UID: 1234, GID: 1234},
			trustDomain: "example.org",
			wantPrefix:  "spiffe://example.org/uid-1234/",
			wantContain: "test",
			wantErr:     false,
		},
		{
			name:        "invalid PID falls back gracefully",
			cred:        Cred{PID: 999999, UID: 1000, GID: 1000},
			trustDomain: "dev.local",
			wantPrefix:  "spiffe://dev.local/uid-1000/",
			wantContain: "unknown", // can't read /proc/999999/exe
			wantErr:     false,
		},
		{
			name:        "empty trust domain",
			cred:        Cred{PID: int32(os.Getpid()), UID: 1000, GID: 1000},
			trustDomain: "",
			wantErr:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			spiffeID, err := FormatSyntheticSPIFFEID(tt.cred, tt.trustDomain)

			if tt.wantErr {
				if err == nil {
					t.Errorf("FormatSyntheticSPIFFEID() expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("FormatSyntheticSPIFFEID() error = %v", err)
			}

			// Check prefix
			if !strings.HasPrefix(spiffeID, tt.wantPrefix) {
				t.Errorf("FormatSyntheticSPIFFEID() = %q, want prefix %q", spiffeID, tt.wantPrefix)
			}

			// Check contains expected substring
			if !strings.Contains(spiffeID, tt.wantContain) {
				t.Errorf("FormatSyntheticSPIFFEID() = %q, want to contain %q", spiffeID, tt.wantContain)
			}

			// Check starts with spiffe://
			if !strings.HasPrefix(spiffeID, "spiffe://") {
				t.Errorf("FormatSyntheticSPIFFEID() = %q, want to start with 'spiffe://'", spiffeID)
			}
		})
	}
}

func TestFormatSyntheticSPIFFEID_Structure(t *testing.T) {
	t.Parallel()

	cred := Cred{
		PID: int32(os.Getpid()),
		UID: 1000,
		GID: 1000,
	}

	spiffeID, err := FormatSyntheticSPIFFEID(cred, "dev.local")
	if err != nil {
		t.Fatalf("FormatSyntheticSPIFFEID() error = %v", err)
	}

	// Verify structure: spiffe://{trust-domain}/uid-{uid}/{executable}
	parts := strings.Split(spiffeID, "/")
	if len(parts) != 5 {
		t.Errorf("SPIFFE ID structure incorrect, got %d parts: %v, want exactly 5", len(parts), parts)
	}

	// parts[0] = "spiffe:"
	// parts[1] = ""
	// parts[2] = "dev.local"
	// parts[3] = "uid-1000"
	// parts[4] = executable name

	if parts[0] != "spiffe:" {
		t.Errorf("part[0] = %q, want 'spiffe:'", parts[0])
	}
	if parts[2] != "dev.local" {
		t.Errorf("part[2] = %q, want 'dev.local'", parts[2])
	}
	if !strings.HasPrefix(parts[3], "uid-") {
		t.Errorf("part[3] = %q, want to start with 'uid-'", parts[3])
	}
	if parts[4] == "" {
		t.Error("part[4] (executable name) is empty")
	}
}
