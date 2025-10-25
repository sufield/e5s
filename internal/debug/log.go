package debug

import (
	"fmt"
	"log"
	"sync"
)

// Logger interface for debug logging.
// Provides structured debug output that can be enabled/disabled at runtime.
//
// Example usage:
//
//	logger := debug.GetLogger()
//	logger.Debugf("Processing request for SPIFFE ID: %s", spiffeID)
//	logger.Debug("Authentication successful")
type Logger interface {
	// Debugf logs a formatted debug message
	Debugf(format string, args ...any)
	// Debug logs debug arguments
	Debug(args ...any)
}

// nopLogger does nothing (used when debug mode is disabled).
type nopLogger struct{}

func (nopLogger) Debugf(string, ...any) {}
func (nopLogger) Debug(...any)          {}

// stdLogger logs to standard logger with [DEBUG] prefix.
type stdLogger struct{}

func (stdLogger) Debugf(format string, args ...any) {
	log.Printf("[DEBUG] "+format, args...)
}

func (stdLogger) Debug(args ...any) {
	// Use Printf for consistent formatting instead of Print+append
	log.Printf("[DEBUG] %v", fmt.Sprint(args...))
}

var (
	// l is the private global debug logger (use GetLogger() to access)
	l    Logger = nopLogger{}
	once sync.Once
)

// GetLogger returns the configured debug logger.
// Always use this function to access the logger instead of storing a reference.
func GetLogger() Logger {
	return l
}

// InitLogger initializes the debug logger based on debug mode.
// Uses sync.Once to ensure initialization happens only once, even in concurrent environments.
// Call this after debug.Init() to ensure Active.Enabled is set.
//
// Example:
//
//	debug.Init()
//	debug.InitLogger()
//	logger := debug.GetLogger()
//	logger.Debugf("Application started")
func InitLogger() {
	once.Do(func() {
		if Active.Enabled {
			l = stdLogger{}
			l.Debug("Debug logging enabled")
		}
	})
}
