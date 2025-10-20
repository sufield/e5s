package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os/signal"
	"syscall"

	"github.com/pocket/hexagon/spire/pkg/zerotrustserver"
)

// rootHandler returns "Success!" only if the request context carries an identity.
func rootHandler(w http.ResponseWriter, r *http.Request) {
	id, ok := zerotrustserver.PeerIdentity(r.Context())
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	fmt.Fprintf(w, "Success! Authenticated as: %s\n", id.SPIFFEID)
}

func main() {
	// Cancel on SIGINT/SIGTERM for graceful shutdown.
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	routes := map[string]http.Handler{
		"/": http.HandlerFunc(rootHandler),
	}

	if err := zerotrustserver.Serve(ctx, routes); err != nil {
		stop() // Ensure cleanup before exit
		//nolint:gocritic // exitAfterDefer: stop() called explicitly before Fatal
		log.Fatalf("server error: %v", err)
	}
}
