package debug

import (
	"strings"
	"sync"
	"testing"
)

func TestFaultProfile_OneShot(t *testing.T) {
	tests := []struct {
		name     string
		setFault func(*FaultProfile)
		checkFn  func(*FaultProfile) bool
	}{
		{
			name:     "DropNextHandshake",
			setFault: func(f *FaultProfile) { f.SetDropNextHandshake(true) },
			checkFn:  func(f *FaultProfile) bool { return f.ShouldDropHandshake() },
		},
		{
			name:     "CorruptNextSPIFFEID",
			setFault: func(f *FaultProfile) { f.SetCorruptNextSPIFFEID(true) },
			checkFn:  func(f *FaultProfile) bool { return f.ShouldCorruptSPIFFEID() },
		},
		{
			name:     "ForceTrustDomainMismatch",
			setFault: func(f *FaultProfile) { f.SetForceTrustDomainMismatch(true) },
			checkFn:  func(f *FaultProfile) bool { return f.ShouldForceTrustDomainMismatch() },
		},
		{
			name:     "ForceExpiredCert",
			setFault: func(f *FaultProfile) { f.SetForceExpiredCert(true) },
			checkFn:  func(f *FaultProfile) bool { return f.ShouldForceExpiredCert() },
		},
		{
			name:     "RejectNextWorkloadLookup",
			setFault: func(f *FaultProfile) { f.SetRejectNextWorkloadLookup(true) },
			checkFn:  func(f *FaultProfile) bool { return f.ShouldRejectWorkloadLookup() },
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := &FaultProfile{}
			tt.setFault(f)

			// First check should return true
			if !tt.checkFn(f) {
				t.Errorf("First check should return true (fault enabled)")
			}

			// Second check should return false (one-shot consumed)
			if tt.checkFn(f) {
				t.Errorf("Second check should return false (fault consumed)")
			}
		})
	}
}

func TestFaultProfile_DelayValidation(t *testing.T) {
	tests := []struct {
		name      string
		delay     int
		wantErr   bool
		errContains string
	}{
		{"valid positive", 5, false, ""},
		{"valid zero", 0, false, ""},
		{"invalid negative", -1, true, "non-negative"},
		{"valid large", 999999, false, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := &FaultProfile{}
			err := f.SetDelayNextIssue(tt.delay)

			if tt.wantErr {
				if err == nil {
					t.Errorf("Expected error for delay=%d, got nil", tt.delay)
				} else if tt.errContains != "" && !strings.Contains(err.Error(), tt.errContains) {
					t.Errorf("Expected error containing %q, got: %v", tt.errContains, err)
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error for delay=%d, got: %v", tt.delay, err)
				}
			}
		})
	}
}

func TestFaultProfile_GetAndClearDelay(t *testing.T) {
	f := &FaultProfile{}

	// Set a delay
	if err := f.SetDelayNextIssue(10); err != nil {
		t.Fatalf("Failed to set delay: %v", err)
	}

	// Get and clear should return the value
	delay := f.GetAndClearDelay()
	if delay != 10 {
		t.Errorf("Expected delay=10, got %d", delay)
	}

	// Second call should return 0 (cleared)
	delay = f.GetAndClearDelay()
	if delay != 0 {
		t.Errorf("Expected delay=0 after clear, got %d", delay)
	}
}

func TestFaultProfile_Reset(t *testing.T) {
	f := &FaultProfile{}

	// Set all faults
	f.SetDropNextHandshake(true)
	f.SetCorruptNextSPIFFEID(true)
	f.SetDelayNextIssue(5)
	f.SetForceTrustDomainMismatch(true)
	f.SetForceExpiredCert(true)
	f.SetRejectNextWorkloadLookup(true)

	// Reset
	f.Reset()

	// Verify all are cleared
	if f.ShouldDropHandshake() {
		t.Error("DropNextHandshake should be cleared")
	}
	if f.ShouldCorruptSPIFFEID() {
		t.Error("CorruptNextSPIFFEID should be cleared")
	}
	if delay := f.GetAndClearDelay(); delay != 0 {
		t.Errorf("DelayNextIssueSeconds should be 0, got %d", delay)
	}
	if f.ShouldForceTrustDomainMismatch() {
		t.Error("ForceTrustDomainMismatch should be cleared")
	}
	if f.ShouldForceExpiredCert() {
		t.Error("ForceExpiredCert should be cleared")
	}
	if f.ShouldRejectWorkloadLookup() {
		t.Error("RejectNextWorkloadLookup should be cleared")
	}
}

func TestFaultProfile_Snapshot(t *testing.T) {
	f := &FaultProfile{}

	// Set some faults
	f.SetDropNextHandshake(true)
	f.SetDelayNextIssue(5)

	// Get snapshot
	snapshot := f.Snapshot()

	// Verify all expected keys are present
	expectedKeys := []string{
		"drop_next_handshake",
		"corrupt_next_spiffe_id",
		"delay_next_issue_seconds",
		"force_trust_domain_mismatch",
		"force_expired_cert",
		"reject_next_workload_lookup",
	}

	for _, key := range expectedKeys {
		if _, ok := snapshot[key]; !ok {
			t.Errorf("Snapshot missing expected key: %s", key)
		}
	}

	// Verify specific values
	if val, ok := snapshot["drop_next_handshake"].(bool); !ok || !val {
		t.Error("Snapshot should contain drop_next_handshake=true")
	}
	if val, ok := snapshot["delay_next_issue_seconds"].(int); !ok || val != 5 {
		t.Errorf("Snapshot should contain delay_next_issue_seconds=5, got %v", val)
	}
	if val, ok := snapshot["corrupt_next_spiffe_id"].(bool); !ok || val {
		t.Error("Snapshot should contain corrupt_next_spiffe_id=false")
	}
}

func TestFaultProfile_Snapshot_Concurrency(t *testing.T) {
	f := &FaultProfile{}
	var wg sync.WaitGroup

	// Concurrent writes
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			f.SetDropNextHandshake(n%2 == 0)
			f.SetDelayNextIssue(n)
		}(i)
	}

	// Concurrent snapshot reads
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			snapshot := f.Snapshot()
			// Verify snapshot has all keys (even during concurrent writes)
			if len(snapshot) != 6 {
				t.Errorf("Expected 6 keys in snapshot, got %d", len(snapshot))
			}
		}()
	}

	wg.Wait()
}

func TestFaultProfile_Concurrency(t *testing.T) {
	f := &FaultProfile{}
	var wg sync.WaitGroup

	// Concurrent writes
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			f.SetDropNextHandshake(true)
			f.SetCorruptNextSPIFFEID(true)
			f.SetDelayNextIssue(5)
			f.Reset()
		}()
	}

	// Concurrent reads
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = f.ShouldDropHandshake()
			_ = f.ShouldCorruptSPIFFEID()
			_ = f.GetAndClearDelay()
			_ = f.Snapshot()
		}()
	}

	wg.Wait()
	// If we get here without deadlock or race, the test passes
}
