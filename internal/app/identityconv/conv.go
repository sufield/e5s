package identityconv

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/pocket/hexagon/spire/internal/domain"
	"github.com/pocket/hexagon/spire/internal/ports"
)

// ValidateProcessIdentity returns domain sentinel errors callers can branch on.
func ValidateProcessIdentity(p ports.ProcessIdentity) error {
	if p.PID < 0 || p.UID < 0 || p.GID < 0 {
		return fmt.Errorf("%w: negative pid/uid/gid", domain.ErrInvalidProcessIdentity)
	}
	if strings.TrimSpace(p.Path) == "" {
		return fmt.Errorf("%w: empty path", domain.ErrInvalidProcessIdentity)
	}
	return nil
}

// ToWorkload converts a ports DTO to a domain entity with light normalization.
func ToWorkload(p ports.ProcessIdentity) *domain.Workload {
	path := strings.TrimSpace(p.Path)
	if path != "" {
		path = filepath.Clean(path)
	}
	return domain.NewWorkload(p.PID, p.UID, p.GID, path)
}

// DeriveIdentityName is optional sugar for UI/logs (last path segment or TD).
func DeriveIdentityName(cred *domain.IdentityCredential) string {
	if cred == nil {
		return "unknown"
	}
	path := strings.Trim(cred.Path(), "/")
	if path == "" {
		return cred.TrustDomain().String()
	}
	parts := strings.Split(path, "/")
	return parts[len(parts)-1]
}
