//go:build !linux || !dev

package localpeer

import (
	"context"
	"fmt"
	"net"
)

// Cred represents Unix domain socket peer credentials
// This stub version is used on non-Linux systems or production builds
type Cred struct {
	PID int32  `json:"pid"`
	UID uint32 `json:"uid"`
	GID uint32 `json:"gid"`
}

const ctxKey = "localpeer.Cred"

// WithCred stores peer credentials in the context (stub version)
func WithCred(ctx context.Context, c Cred) context.Context {
	return context.WithValue(ctx, ctxKey, c)
}

// FromCtx retrieves peer credentials from the context (stub version)
func FromCtx(ctx context.Context) (Cred, error) {
	v := ctx.Value(ctxKey)
	if v == nil {
		return Cred{}, fmt.Errorf("no local peer cred in context")
	}
	c, ok := v.(Cred)
	if !ok {
		return Cred{}, fmt.Errorf("invalid type in context: expected Cred, got %T", v)
	}
	return c, nil
}

// GetPeerCred is not implemented on non-Linux systems
func GetPeerCred(conn *net.UnixConn) (Cred, error) {
	return Cred{}, fmt.Errorf("SO_PEERCRED is only available on Linux in dev mode")
}

// GetExecutablePath is not implemented on non-Linux systems
func GetExecutablePath(pid int32) (string, error) {
	return "", fmt.Errorf("/proc filesystem is only available on Linux")
}

// FormatSyntheticSPIFFEID is not implemented on non-Linux systems
func FormatSyntheticSPIFFEID(cred Cred, trustDomain string) (string, error) {
	return "", fmt.Errorf("synthetic SPIFFE IDs are only available on Linux in dev mode")
}
