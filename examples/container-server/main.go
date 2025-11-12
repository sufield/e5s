package main

import (
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/sufield/e5s"
)

func main() {
	mode := os.Getenv("MODE") // "serve" or "single"
	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if id, ok := e5s.PeerID(r); ok {
			fmt.Fprintf(w, "Hello %s\n", id)
			return
		}
		http.Error(w, "unauthorized", http.StatusUnauthorized)
	})
	cfg := "/work/e5s-server.yaml"
	switch mode {
	case "serve":
		log.Fatal(e5s.Serve(cfg, h))
	case "single":
		log.Fatal(e5s.StartSingleThread(cfg, h))
	default:
		log.Fatal("unknown MODE")
	}
}
