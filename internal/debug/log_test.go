package debug

import (
	"bytes"
	"log"
	"strings"
	"sync"
	"testing"
)

func TestLogger_Disabled(t *testing.T) {
	// Reset for clean test
	l = nopLogger{}
	once = sync.Once{}

	// Capture log output
	var buf bytes.Buffer
	log.SetOutput(&buf)
	defer log.SetOutput(log.Writer())

	// Logger should be no-op when disabled
	Active.Enabled = false
	InitLogger()

	logger := GetLogger()
	logger.Debug("Should not appear")
	logger.Debugf("Should not appear: %s", "test")

	if buf.Len() > 0 {
		t.Errorf("Expected no output when disabled, got: %s", buf.String())
	}
}

func TestLogger_Enabled(t *testing.T) {
	// Reset for clean test
	l = nopLogger{}
	once = sync.Once{}

	// Capture log output
	var buf bytes.Buffer
	log.SetOutput(&buf)
	defer log.SetOutput(log.Writer())

	// Enable debug mode
	Active.Enabled = true
	InitLogger()

	// Should produce output
	logger := GetLogger()
	logger.Debug("Test message")
	output := buf.String()

	if !strings.Contains(output, "[DEBUG]") {
		t.Errorf("Expected [DEBUG] prefix, got: %s", output)
	}
	if !strings.Contains(output, "Test message") {
		t.Errorf("Expected 'Test message', got: %s", output)
	}
}

func TestLogger_Debugf(t *testing.T) {
	// Reset for clean test
	l = nopLogger{}
	once = sync.Once{}

	// Capture log output
	var buf bytes.Buffer
	log.SetOutput(&buf)
	defer log.SetOutput(log.Writer())

	// Enable debug mode
	Active.Enabled = true
	InitLogger()

	// Test formatted output
	logger := GetLogger()
	logger.Debugf("User %s logged in with ID %d", "alice", 42)
	output := buf.String()

	if !strings.Contains(output, "[DEBUG]") {
		t.Errorf("Expected [DEBUG] prefix, got: %s", output)
	}
	if !strings.Contains(output, "User alice logged in with ID 42") {
		t.Errorf("Expected formatted message, got: %s", output)
	}
}

func TestLogger_OnceInitialization(t *testing.T) {
	// Reset for clean test
	l = nopLogger{}
	once = sync.Once{}

	// Capture log output
	var buf bytes.Buffer
	log.SetOutput(&buf)
	defer log.SetOutput(log.Writer())

	// Enable debug mode
	Active.Enabled = true

	// Call InitLogger multiple times
	InitLogger()
	InitLogger()
	InitLogger()

	output := buf.String()

	// Should only see one "Debug logging enabled" message
	count := strings.Count(output, "Debug logging enabled")
	if count != 1 {
		t.Errorf("Expected exactly 1 initialization message, got %d", count)
	}
}

func TestLogger_ConcurrentAccess(t *testing.T) {
	// Reset for clean test
	l = nopLogger{}
	once = sync.Once{}

	// Capture log output
	var buf bytes.Buffer
	log.SetOutput(&buf)
	defer log.SetOutput(log.Writer())

	// Enable debug mode
	Active.Enabled = true
	InitLogger()

	// Concurrent logging
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			logger := GetLogger()
			logger.Debugf("Message %d", n)
			logger.Debug("Concurrent access")
		}(i)
	}

	wg.Wait()
	// If we get here without panic, concurrent access is safe
}

func TestLogger_GetLogger(t *testing.T) {
	// Reset for clean test
	l = nopLogger{}
	once = sync.Once{}

	// Should return nopLogger initially
	logger := GetLogger()
	if _, ok := logger.(nopLogger); !ok {
		t.Errorf("Expected nopLogger, got %T", logger)
	}

	// Enable and init
	Active.Enabled = true
	InitLogger()

	// Should return stdLogger
	logger = GetLogger()
	if _, ok := logger.(stdLogger); !ok {
		t.Errorf("Expected stdLogger, got %T", logger)
	}
}


func TestLogger_DebugFormatting(t *testing.T) {
	// Reset for clean test
	l = nopLogger{}
	once = sync.Once{}

	// Capture log output
	var buf bytes.Buffer
	log.SetOutput(&buf)
	defer log.SetOutput(log.Writer())

	// Enable debug mode
	Active.Enabled = true
	InitLogger()

	// Test Debug with multiple args
	buf.Reset()
	logger := GetLogger()
	logger.Debug("Key:", "value", "number:", 42)
	output := buf.String()

	if !strings.Contains(output, "[DEBUG]") {
		t.Errorf("Expected [DEBUG] prefix, got: %s", output)
	}
	// Should contain all arguments formatted
	if !strings.Contains(output, "Key:") || !strings.Contains(output, "value") {
		t.Errorf("Expected formatted arguments, got: %s", output)
	}
}

// Benchmark logger performance
func BenchmarkLogger_Disabled(b *testing.B) {
	l = nopLogger{}
	logger := GetLogger()
	for i := 0; i < b.N; i++ {
		logger.Debugf("Benchmark message %d", i)
	}
}

func BenchmarkLogger_Enabled(b *testing.B) {
	l = stdLogger{}
	var buf bytes.Buffer
	log.SetOutput(&buf)
	defer log.SetOutput(log.Writer())

	logger := GetLogger()
	for i := 0; i < b.N; i++ {
		logger.Debugf("Benchmark message %d", i)
	}
}
