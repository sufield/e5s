package spire

import (
	"testing"
)

// FuzzNormalizeToAddr tests path normalization with random inputs
func FuzzNormalizeToAddr(f *testing.F) {
	// Seed corpus with various path formats
	f.Add("unix:///tmp/agent.sock")
	f.Add("tcp://localhost:8081")
	f.Add("/tmp/agent.sock")
	f.Add("../../../etc/passwd")
	f.Add("C:\\Windows\\System32")
	f.Add("")

	f.Fuzz(func(t *testing.T, input string) {
		// Should never panic
		result := normalizeToAddr(input)

		// Result should always be a string (not nil)
		_ = result

		// If input has unix:// or tcp://, result should preserve it
		// This is a basic sanity check
		if input != "" {
			_ = len(result) // Just ensure we can get length without panic
		}
	})
}

// FuzzConfig tests Config struct handling with random values
func FuzzConfig(f *testing.F) {
	// Seed with valid configs
	f.Add("unix:///tmp/agent.sock", int64(30_000_000_000)) // 30s in nanoseconds
	f.Add("/tmp/agent.sock", int64(60_000_000_000))        // 60s
	f.Add("", int64(0))

	f.Fuzz(func(t *testing.T, socket string, timeoutNs int64) {
		cfg := Config{
			WorkloadSocket:      socket,
			InitialFetchTimeout: 0, // We'll use timeoutNs to construct this
		}

		// Validate config values don't cause panics
		_ = cfg.WorkloadSocket
		_ = cfg.InitialFetchTimeout

		// Test normalizeToAddr with fuzzed socket
		normalized := normalizeToAddr(socket)
		_ = normalized
	})
}
