//go:build dev

package app_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/pocket/hexagon/spire/internal/app"
)

// Placeholder test file - tests can be added here
func TestAppPackage(t *testing.T) {
	t.Parallel()
	// Basic package test
	assert.NotNil(t, app.NewIdentityService)
}
