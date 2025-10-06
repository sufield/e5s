package attestor_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/pocket/hexagon/spire/internal/adapters/outbound/inmemory/attestor"
	"github.com/pocket/hexagon/spire/internal/domain"
	"github.com/pocket/hexagon/spire/internal/ports"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestUnixWorkloadAttestor_Attest_Success tests successful attestation
func TestUnixWorkloadAttestor_Attest_Success(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	attestorInst := attestor.NewUnixWorkloadAttestor()
	attestorInst.RegisterUID(1000, "unix:user:testuser")

	workload := ports.ProcessIdentity{
		PID:  1234,
		UID:  1000,
		GID:  1000,
		Path: "/bin/app",
	}

	selectors, err := attestorInst.Attest(ctx, workload)
	require.NoError(t, err)
	assert.Len(t, selectors, 3)
	assert.Contains(t, selectors, "unix:user:testuser")
	assert.Contains(t, selectors, "unix:uid:1000")
	assert.Contains(t, selectors, "unix:gid:1000")
}

// TestUnixWorkloadAttestor_Attest_MultipleUIDs tests multiple UID registrations
func TestUnixWorkloadAttestor_Attest_MultipleUIDs(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	attestorInst := attestor.NewUnixWorkloadAttestor()
	attestorInst.RegisterUID(1001, "unix:workload:server")
	attestorInst.RegisterUID(1002, "unix:workload:client")

	// Test server workload
	serverWorkload := ports.ProcessIdentity{PID: 100, UID: 1001, GID: 1001, Path: "/usr/bin/server"}
	serverSelectors, err := attestorInst.Attest(ctx, serverWorkload)
	require.NoError(t, err)
	assert.Contains(t, serverSelectors, "unix:workload:server")
	assert.Contains(t, serverSelectors, "unix:uid:1001")
	assert.Contains(t, serverSelectors, "unix:gid:1001")

	// Test client workload
	clientWorkload := ports.ProcessIdentity{PID: 200, UID: 1002, GID: 1002, Path: "/usr/bin/client"}
	clientSelectors, err := attestorInst.Attest(ctx, clientWorkload)
	require.NoError(t, err)
	assert.Contains(t, clientSelectors, "unix:workload:client")
	assert.Contains(t, clientSelectors, "unix:uid:1002")
	assert.Contains(t, clientSelectors, "unix:gid:1002")
}

// TestUnixWorkloadAttestor_Attest_UnregisteredUID tests unregistered UID error
func TestUnixWorkloadAttestor_Attest_UnregisteredUID(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	attestorInst := attestor.NewUnixWorkloadAttestor()
	attestorInst.RegisterUID(1000, "unix:user:testuser")

	// Try to attest unregistered UID
	workload := ports.ProcessIdentity{
		PID:  1234,
		UID:  9999,
		GID:  9999,
		Path: "/bin/unknown",
	}

	_, err := attestorInst.Attest(ctx, workload)
	assert.Error(t, err)
	assert.ErrorIs(t, err, domain.ErrWorkloadAttestationFailed)
	assert.Contains(t, err.Error(), "no attestation data for UID")
}

// TestUnixWorkloadAttestor_Attest_InvalidUID tests invalid UID error
func TestUnixWorkloadAttestor_Attest_InvalidUID(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	attestorInst := attestor.NewUnixWorkloadAttestor()
	attestorInst.RegisterUID(1000, "unix:user:testuser")

	// Try to attest with negative UID
	workload := ports.ProcessIdentity{
		PID:  1234,
		UID:  -1,
		GID:  1000,
		Path: "/bin/app",
	}

	_, err := attestorInst.Attest(ctx, workload)
	assert.Error(t, err)
	assert.ErrorIs(t, err, domain.ErrInvalidProcessIdentity)
	assert.Contains(t, err.Error(), "invalid UID")
}

// TestUnixWorkloadAttestor_RegisterUID tests UID registration
func TestUnixWorkloadAttestor_RegisterUID(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	attestorInst := attestor.NewUnixWorkloadAttestor()

	// Register UID
	attestorInst.RegisterUID(5000, "unix:service:myapp")

	// Verify it can be attested
	workload := ports.ProcessIdentity{UID: 5000, GID: 5000, PID: 123, Path: "/app"}
	selectors, err := attestorInst.Attest(ctx, workload)
	require.NoError(t, err)
	assert.Contains(t, selectors, "unix:service:myapp")
}

// TestUnixWorkloadAttestor_RegisterUID_Overwrite tests overwriting UID registration
func TestUnixWorkloadAttestor_RegisterUID_Overwrite(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	attestorInst := attestor.NewUnixWorkloadAttestor()

	// Register UID
	attestorInst.RegisterUID(1000, "unix:user:original")

	// Overwrite with new selector
	attestorInst.RegisterUID(1000, "unix:user:updated")

	// Verify updated selector is used
	workload := ports.ProcessIdentity{UID: 1000, GID: 1000, PID: 123, Path: "/app"}
	selectors, err := attestorInst.Attest(ctx, workload)
	require.NoError(t, err)
	assert.Contains(t, selectors, "unix:user:updated")
	assert.NotContains(t, selectors, "unix:user:original")
}

// TestUnixWorkloadAttestor_Attest_TableDriven tests various scenarios
func TestUnixWorkloadAttestor_Attest_TableDriven(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		registerUIDs   map[int]string
		workload       ports.ProcessIdentity
		expectError    bool
		expectedErr    error
		expectedCount  int
		expectSelector string
	}{
		{
			name:           "valid attestation",
			registerUIDs:   map[int]string{1000: "unix:user:app"},
			workload:       ports.ProcessIdentity{UID: 1000, GID: 1000, PID: 100, Path: "/app"},
			expectError:    false,
			expectedCount:  3,
			expectSelector: "unix:user:app",
		},
		{
			name:           "unregistered UID",
			registerUIDs:   map[int]string{1000: "unix:user:app"},
			workload:       ports.ProcessIdentity{UID: 2000, GID: 2000, PID: 200, Path: "/other"},
			expectError:    true,
			expectedErr:    domain.ErrWorkloadAttestationFailed,
			expectedCount:  0,
			expectSelector: "",
		},
		{
			name:           "negative UID",
			registerUIDs:   map[int]string{},
			workload:       ports.ProcessIdentity{UID: -5, GID: 100, PID: 100, Path: "/app"},
			expectError:    true,
			expectedErr:    domain.ErrInvalidProcessIdentity,
			expectedCount:  0,
			expectSelector: "",
		},
		{
			name:           "zero UID root",
			registerUIDs:   map[int]string{0: "unix:user:root"},
			workload:       ports.ProcessIdentity{UID: 0, GID: 0, PID: 1, Path: "/sbin/init"},
			expectError:    false,
			expectedCount:  3,
			expectSelector: "unix:user:root",
		},
		{
			name:           "high UID value",
			registerUIDs:   map[int]string{65534: "unix:user:nobody"},
			workload:       ports.ProcessIdentity{UID: 65534, GID: 65534, PID: 9999, Path: "/usr/bin/nobody"},
			expectError:    false,
			expectedCount:  3,
			expectSelector: "unix:user:nobody",
		},
		{
			name: "different GID than UID",
			registerUIDs: map[int]string{1000: "unix:user:app"},
			workload:     ports.ProcessIdentity{UID: 1000, GID: 2000, PID: 100, Path: "/app"},
			expectError:  false,
			expectedCount: 3,
			expectSelector: "unix:user:app",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			ctx := context.Background()

			attestorInst := attestor.NewUnixWorkloadAttestor()

			// Register UIDs
			for uid, selector := range tt.registerUIDs {
				attestorInst.RegisterUID(uid, selector)
			}

			// Attest workload
			selectors, err := attestorInst.Attest(ctx, tt.workload)

			if tt.expectError {
				assert.Error(t, err)
				if tt.expectedErr != nil {
					assert.ErrorIs(t, err, tt.expectedErr)
				}
			} else {
				require.NoError(t, err)
				assert.Len(t, selectors, tt.expectedCount)
				if tt.expectSelector != "" {
					assert.Contains(t, selectors, tt.expectSelector)
				}
				// Verify standard selectors always present on success
				assert.Contains(t, selectors, fmt.Sprintf("unix:uid:%d", tt.workload.UID))
				assert.Contains(t, selectors, fmt.Sprintf("unix:gid:%d", tt.workload.GID))
			}
		})
	}
}

// TestUnixWorkloadAttestor_NewUnixWorkloadAttestor tests constructor
func TestUnixWorkloadAttestor_NewUnixWorkloadAttestor(t *testing.T) {
	t.Parallel()

	attestorInst := attestor.NewUnixWorkloadAttestor()
	assert.NotNil(t, attestorInst)
}

// TestUnixWorkloadAttestor_Attest_GIDVariations tests different GID values
func TestUnixWorkloadAttestor_Attest_GIDVariations(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	attestorInst := attestor.NewUnixWorkloadAttestor()
	attestorInst.RegisterUID(1000, "unix:user:app")

	// Test various GID values
	gids := []int{0, 1000, 5000, 65534}

	for _, gid := range gids {
		workload := ports.ProcessIdentity{
			UID:  1000,
			GID:  gid,
			PID:  1234,
			Path: "/app",
		}

		selectors, err := attestorInst.Attest(ctx, workload)
		require.NoError(t, err, "Should succeed with GID %d", gid)
		assert.Contains(t, selectors, fmt.Sprintf("unix:gid:%d", gid))
	}
}

// TestUnixWorkloadAttestor_ImplementsPort verifies attestor implements WorkloadAttestor interface
func TestUnixWorkloadAttestor_ImplementsPort(t *testing.T) {
	t.Parallel()

	attestorInst := attestor.NewUnixWorkloadAttestor()
	var _ ports.WorkloadAttestor = attestorInst
}

// TestUnixWorkloadAttestor_ContextCancellation tests context cancellation handling
func TestUnixWorkloadAttestor_ContextCancellation(t *testing.T) {
	t.Parallel()

	attestorInst := attestor.NewUnixWorkloadAttestor()
	attestorInst.RegisterUID(1000, "unix:user:app")

	// Create cancelled context
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	workload := ports.ProcessIdentity{UID: 1000, GID: 1000, PID: 100, Path: "/app"}

	// Current implementation doesn't check context, but this tests future-proofing
	_, err := attestorInst.Attest(ctx, workload)
	// Should still work as current implementation doesn't use context
	// This test documents expected behavior
	assert.NoError(t, err)
}
