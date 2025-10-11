package workloadapi

import (
	"context"
	"fmt"
	"net"
)

// credentialsConnContext is a ConnContext function for http.Server that preserves connection credentials.
// This is called by http.Server for each accepted connection, allowing us to inject the peer credentials
// (extracted by SO_PEERCRED in the listener) into the request context before any handlers run.
//
// If the connection is not properly wrapped (indicating the listener failed to extract credentials),
// an error is injected into the context to prevent silent failures downstream.
func credentialsConnContext(ctx context.Context, c net.Conn) context.Context {
	if connWithCreds, ok := c.(*connWithCredentials); ok {
		return contextWithCredentials(ctx, connWithCreds.credentials)
	}
	// Connection not wrapped - inject error for downstream detection
	return contextWithCredentialsError(ctx, fmt.Errorf(
		"connection not wrapped with credentials; listener may not be using credentialsListener or platform does not support SO_PEERCRED",
	))
}
