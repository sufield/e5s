package app

// NOTE: Application struct definition has been split into:
// - application_prod.go (//go:build !dev) - Production version without Registry field
// - application_dev.go (//go:build dev) - Development version with Registry field
//
// This file is intentionally empty except for this comment.
// All Application-related code is in the build-specific files.
//
// Bootstrap function is implemented in build-specific files:
// - bootstrap_dev.go (//go:build dev) - development mode with in-memory implementations
// - bootstrap_prod.go (//go:build !dev) - production mode with SPIRE infrastructure
