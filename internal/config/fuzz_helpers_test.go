package config

import (
	"os"
)

// WriteForTest writes data to a file for testing purposes
func WriteForTest(path string, data []byte) error {
	return os.WriteFile(path, data, 0o600)
}
