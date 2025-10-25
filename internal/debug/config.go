package debug

import (
	"os"
	"strconv"
)

// Config holds debug mode configuration
type Config struct {
	// Enabled is the global debug on/off switch
	Enabled bool

	// Stress mode uses tiny buffers, fast rotation, etc.
	Stress bool

	// SingleThreaded disables goroutines for auth path
	SingleThreaded bool

	// LocalDebugServer enables localhost debug HTTP server
	LocalDebugServer bool

	// DebugServerAddr is the address for the debug HTTP server
	DebugServerAddr string
}

// Active is the global debug configuration
var Active Config

// Init initializes debug configuration from environment variables
func Init() {
	Active = Config{
		Enabled:          parseBool(os.Getenv("SPIRE_DEBUG"), false),
		Stress:           parseBool(os.Getenv("SPIRE_DEBUG_STRESS"), false),
		SingleThreaded:   parseBool(os.Getenv("SPIRE_DEBUG_SINGLE_THREAD"), false),
		LocalDebugServer: parseBool(os.Getenv("SPIRE_DEBUG_SERVER"), false),
		DebugServerAddr:  getEnvOrDefault("SPIRE_DEBUG_ADDR", "127.0.0.1:6060"),
	}

	// If any debug feature is enabled, ensure global debug is on
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
