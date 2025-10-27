//go:build !linux || !dev

package localpeer

import (
	"context"
	"errors"
	"fmt"
	"net"
)

// Cred represents Unix domain socket peer credentials
// This stub version is used on non-Linux systems or production builds
type Cred struct {
	PID int32  `json:"pid,omitempty"`
	UID uint32 `json:"uid,omitempty"`
	GID uint32 `json:"gid,omitempty"`
}

// ctxKeyType is the type for context keys to avoid collisions
type ctxKeyType string

// ctxKey is the context key for storing peer credentials
const ctxKey ctxKeyType = "localpeer.Cred"

// ErrNoCred is returned when no peer credential is found in context
var ErrNoCred = errors.New("no local peer cred")

// WithCred stores peer credentials in the context (stub version)
// If ctx is nil, returns a new background context with the credentials
func WithCred(ctx context.Context, c Cred) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	return context.WithValue(ctx, ctxKey, c)
}

// FromCtx retrieves peer credentials from the context (stub version)
func FromCtx(ctx context.Context) (Cred, error) {
	v := ctx.Value(ctxKey)
	if v == nil {
		return Cred{}, fmt.Errorf("no local peer cred in context: %w", ErrNoCred)
	}
	c, ok := v.(Cred)
	if !ok {
		return Cred{}, fmt.Errorf("invalid type in context: expected Cred, got %T", v)
	}
	return c, nil
}

// GetPeerCred is not implemented on non-Linux systems
func GetPeerCred(ctx context.Context, conn *net.UnixConn) (Cred, error) {
	return Cred{}, fmt.Errorf("stub: SO_PEERCRED is only available on Linux in dev mode")
}

// GetExecutablePath is not implemented on non-Linux systems
func GetExecutablePath(ctx context.Context, pid int32) (string, error) {
	return "", fmt.Errorf("stub: /proc filesystem is only available on Linux")
}

// FormatSyntheticSPIFFEID is not implemented on non-Linux systems
func FormatSyntheticSPIFFEID(cred Cred, trustDomain string) (string, error) {
	return "", fmt.Errorf("stub: synthetic SPIFFE IDs are only available on Linux in dev mode")
}
