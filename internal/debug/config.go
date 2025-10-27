package debug

import (
	"os"
	"strconv"
)

// Config holds debug mode configuration
type Config struct {
	// Enabled is the global debug on/off switch
	Enabled bool

	// Mode describes the runtime environment ("debug", "staging", "production")
	Mode string

	// Stress mode uses tiny buffers, fast rotation, etc.
	Stress bool

	// SingleThreaded disables goroutines for auth path
	SingleThreaded bool

	// LocalDebugServer enables localhost debug HTTP server
	LocalDebugServer bool

	// DebugServerAddr is the address for the debug HTTP server
	DebugServerAddr string
}

// Active is the global debug configuration.
// Init() sets this once during startup. After Init() returns,
// the rest of the code must treat Active as read-only.
var Active Config

// Init initializes debug configuration from environment variables
func Init() {
	Active = Config{
		Enabled:          parseBool(os.Getenv("SPIRE_DEBUG"), false),
		Mode:             getEnvOrDefault("SPIRE_DEBUG_MODE", "debug"),
		Stress:           parseBool(os.Getenv("SPIRE_DEBUG_STRESS"), false),
		SingleThreaded:   parseBool(os.Getenv("SPIRE_DEBUG_SINGLE_THREAD"), false),
		LocalDebugServer: parseBool(os.Getenv("SPIRE_DEBUG_SERVER"), false),
		DebugServerAddr:  getEnvOrDefault("SPIRE_DEBUG_ADDR", "127.0.0.1:6060"),
	}

	// Normalize mode to allowed values (prevents typos from creating undocumented modes)
	switch Active.Mode {
	case "debug", "staging", "production":
		// allowed
	default:
		// force a known-safe mode instead of trusting garbage
		Active.Mode = "debug"
	}

	// If any debug feature is enabled, ensure global debug is on.
	// Note: This forces Active.Enabled=true even if SPIRE_DEBUG=false.
	// This is intentional. If you expose a local debug server or stress mode,
	// you are in debug whether you admit it or not.
	if Active.Stress || Active.SingleThreaded || Active.LocalDebugServer {
		Active.Enabled = true
	}
}

func parseBool(s string, defaultVal bool) bool {
	if s == "" {
		return defaultVal
	}
	val, err := strconv.ParseBool(s)
	if err != nil {
		return defaultVal
	}
	return val
}

func getEnvOrDefault(key, defaultVal string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultVal
}

// IsEnabled returns whether debug mode is enabled
func IsEnabled() bool {
	return Active.Enabled
}
