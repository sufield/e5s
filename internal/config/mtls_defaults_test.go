package config

// Defaults Tests
//
// These tests verify that default values are correctly applied to mTLS configuration.
// Tests cover timeout defaults and other configuration defaults.
//
// Run these tests with:
//
//	go test ./internal/config/... -v -run TestDefaults
//	go test ./internal/config/... -cover

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestDefaults_AllTimeouts tests that all timeout defaults are applied
func TestDefaults_AllTimeouts(t *testing.T) {
	cfg := &MTLSConfig{}
	applyDefaults(cfg)

	assert.Equal(t, DefaultHTTPTimeout, cfg.HTTP.Timeout)
	assert.Equal(t, DefaultReadHeaderTimeout, cfg.HTTP.ReadHeaderTimeout)
	assert.Equal(t, DefaultReadTimeout, cfg.HTTP.ReadTimeout)
	assert.Equal(t, DefaultWriteTimeout, cfg.HTTP.WriteTimeout)
	assert.Equal(t, DefaultIdleTimeout, cfg.HTTP.IdleTimeout)
}
