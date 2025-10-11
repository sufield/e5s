package workloadapi

import (
	"context"
	"log/slog"
	"net"

	"github.com/pocket/hexagon/spire/internal/ports"
)

// credentialsKey is an unexported type for stronger type safety in context keys.
// This prevents collisions with other packages using string-based context keys.
// See: https://go.dev/blog/context-keys
type credentialsKey struct{}

// credentialsErrorKey is an unexported type for storing credential extraction errors in context
type credentialsErrorKey struct{}

// credentialsContextKey is the singleton key for storing peer credentials in context
var credentialsContextKey = credentialsKey{}

// credentialsErrContextKey is the singleton key for storing credential extraction errors
var credentialsErrContextKey = credentialsErrorKey{}

// connWithCredentials wraps a net.Conn and stores extracted peer credentials
type connWithCredentials struct {
	net.Conn
	credentials ports.ProcessIdentity
}

// credentialsListener wraps a net.Listener and extracts credentials from accepted connections
type credentialsListener struct {
	net.Listener
	logger *slog.Logger
}

// newCredentialsListener wraps a listener to extract peer credentials on accept
func newCredentialsListener(inner net.Listener, logger *slog.Logger) net.Listener {
	return &credentialsListener{
		Listener: inner,
		logger:   logger,
	}
}

// Accept accepts a connection and immediately extracts peer credentials
func (l *credentialsListener) Accept() (net.Conn, error) {
	conn, err := l.Listener.Accept()
	if err != nil {
		return nil, err
	}

	// Extract kernel-verified credentials immediately
	credentials, err := extractCredentials(conn, l.logger)
	if err != nil {
		// Log extraction failure with context
		l.logger.Error("failed to extract peer credentials",
			"remote_addr", conn.RemoteAddr(),
			"error", err)
		// Close connection on credential extraction failure
		conn.Close()
		return nil, err
	}

	l.logger.Debug("extracted peer credentials",
		"remote_addr", conn.RemoteAddr(),
		"pid", credentials.PID,
		"uid", credentials.UID,
		"gid", credentials.GID)

	// Wrap connection with credentials
	return &connWithCredentials{
		Conn:        conn,
		credentials: credentials,
	}, nil
}

// credentialsFromContext retrieves peer credentials from request context
func credentialsFromContext(ctx context.Context) (ports.ProcessIdentity, bool) {
	creds, ok := ctx.Value(credentialsContextKey).(ports.ProcessIdentity)
	return creds, ok
}

// contextWithCredentials adds credentials to a context
func contextWithCredentials(ctx context.Context, creds ports.ProcessIdentity) context.Context {
	return context.WithValue(ctx, credentialsContextKey, creds)
}

// contextWithCredentialsError adds a credential extraction error to context
func contextWithCredentialsError(ctx context.Context, err error) context.Context {
	return context.WithValue(ctx, credentialsErrContextKey, err)
}

// credentialsErrorFromContext retrieves credential extraction error from context
func credentialsErrorFromContext(ctx context.Context) (error, bool) {
	err, ok := ctx.Value(credentialsErrContextKey).(error)
	return err, ok
}
