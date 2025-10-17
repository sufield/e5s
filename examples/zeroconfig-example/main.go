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

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	err := zerotrustserver.Serve(ctx, map[string]http.Handler{
		"/": http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			id, ok := zerotrustserver.Identity(r.Context())
			if !ok {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}
			fmt.Fprintf(w, "Success! Authenticated as: %s\n", id.SPIFFEID)
		}),
	})
	if err != nil {
		log.Fatal(err)
	}
}
