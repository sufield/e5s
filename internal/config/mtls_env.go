package config

import (
	"fmt"
	"os"
	"strings"
	"time"
)

// applyEnvOverrides applies strict environment variable overrides.
//
// Breaking changes from previous version:
//   - Removed: HTTP_PORT, HTTP_ENABLED, EXPECTED_SERVER_ID, ALLOWED_CLIENT_ID
//   - ALLOWED_ID and ALLOWED_IDS are mutually exclusive (error if both set)
//   - AUTH_PEER_VERIFICATION must be one of: any, trust-domain, specific-id, one-of
//   - List inputs are normalized: trimmed, deduped, empties dropped
//
// Returns error for invalid environment variable values to fail fast.
func applyEnvOverrides(cfg *MTLSConfig) error {
	// SPIRE configuration
	if v := os.Getenv("SPIRE_AGENT_SOCKET"); v != "" {
		cfg.SPIRE.SocketPath = v
	}
	if v := os.Getenv("SPIRE_TRUST_DOMAIN"); v != "" {
		cfg.SPIRE.TrustDomain = strings.TrimSpace(v)
	}
	if err := parseDurationInto("SPIRE_TIMEOUT", &cfg.SPIRE.Timeout); err != nil {
		return err
	}

	// HTTP configuration
	if v := os.Getenv("HTTP_ADDRESS"); v != "" {
		cfg.HTTP.Address = v
	}

	// HTTP durations
	if err := parseDurationInto("HTTP_TIMEOUT", &cfg.HTTP.Timeout); err != nil {
		return err
	}
	if err := parseDurationInto("HTTP_READ_HEADER_TIMEOUT", &cfg.HTTP.ReadHeaderTimeout); err != nil {
		return err
	}
	if err := parseDurationInto("HTTP_READ_TIMEOUT", &cfg.HTTP.ReadTimeout); err != nil {
		return err
	}
	if err := parseDurationInto("HTTP_WRITE_TIMEOUT", &cfg.HTTP.WriteTimeout); err != nil {
		return err
	}
	if err := parseDurationInto("HTTP_IDLE_TIMEOUT", &cfg.HTTP.IdleTimeout); err != nil {
		return err
	}

	// Auth mode (validate against allowed values)
	if v := os.Getenv("AUTH_PEER_VERIFICATION"); v != "" {
		mode := strings.ToLower(strings.TrimSpace(v))
		switch mode {
		case "any", "trust-domain", "specific-id", "one-of":
			cfg.HTTP.Auth.PeerVerification = mode
		default:
			return fmt.Errorf("invalid AUTH_PEER_VERIFICATION %q (allowed: any|trust-domain|specific-id|one-of)", v)
		}
	}
	if v := os.Getenv("AUTH_TRUST_DOMAIN"); v != "" {
		cfg.HTTP.Auth.TrustDomain = strings.TrimSpace(v)
	}

	// Allowed IDs (mutually exclusive: single ID or list, not both)
	id := strings.TrimSpace(os.Getenv("ALLOWED_ID"))
	ids := strings.TrimSpace(os.Getenv("ALLOWED_IDS"))

	if id != "" && ids != "" {
		return fmt.Errorf("ALLOWED_ID and ALLOWED_IDS are mutually exclusive; set only one")
	}

	if id != "" {
		cfg.HTTP.Auth.AllowedIDs = []string{id}
	}
	if ids != "" {
		list := splitCleanDedup(ids, ",")
		if len(list) == 0 {
			return fmt.Errorf("ALLOWED_IDS provided but resolved to empty after cleaning")
		}
		cfg.HTTP.Auth.AllowedIDs = list
	}

	return nil
}

// parseDurationInto parses a duration from an env var and stores it in target.
func parseDurationInto(env string, target *time.Duration) error {
	if v := os.Getenv(env); v != "" {
		d, err := time.ParseDuration(strings.TrimSpace(v))
		if err != nil {
			return fmt.Errorf("invalid %s %q: %w", env, v, err)
		}
		*target = d
	}
	return nil
}

// splitCleanDedup splits a string by separator, trims whitespace, removes empties, and deduplicates.
func splitCleanDedup(s, sep string) []string {
	raw := strings.Split(s, sep)
	seen := make(map[string]struct{}, len(raw))
	out := make([]string, 0, len(raw))

	for _, r := range raw {
		v := strings.TrimSpace(r)
		if v == "" {
			continue
		}
		if _, ok := seen[v]; ok {
			continue
		}
		seen[v] = struct{}{}
		out = append(out, v)
	}

	return out
}
