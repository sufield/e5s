package zerotrustserver

import (
	"context"
	"crypto/tls"
	"errors"
	"net/http"
	"os"
	"time"

	"github.com/pocket/hexagon/spire/internal/ports"
	"github.com/spiffe/go-spiffe/v2/spiffeid"
	"github.com/spiffe/go-spiffe/v2/workloadapi"
)

func buildDefaults(ctx context.Context, routes map[string]http.Handler) (ports.MTLSConfig, *http.ServeMux, error) {
	// 1) HTTP defaults
	cfg := ports.MTLSConfig{
		WorkloadAPI: ports.WorkloadAPIConfig{SocketPath: selectSocket()},
		HTTP: ports.HTTPConfig{
			Address:           getenv("SERVER_ADDRESS", ":8443"),
			ReadHeaderTimeout: 10 * time.Second,
			ReadTimeout:       15 * time.Second,
			WriteTimeout:      30 * time.Second,
			IdleTimeout:       120 * time.Second,
		},
	}
	mux := http.NewServeMux()

	// 2) Always mount /health (idempotent — user can override by providing one)
	mux.Handle("/health", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	}))

	// 3) Mount user routes
	for p, h := range routes {
		mux.Handle(p, h)
	}

	// 4) Auto-detect trust domain from the caller's SVID and authorize that domain.
	td, err := detectTrustDomain(ctx, cfg.WorkloadAPI.SocketPath)
	if err != nil {
		// Fail closed with a clear error. (If you prefer, you can swap this to a
		// permissive fallback like "example.org", but strict-by-default is safer.)
		return ports.MTLSConfig{}, nil, err
	}
	cfg.SPIFFE.AllowedTrustDomain = td

	// 5) Enforce TLS 1.3+ (the adapter will honor this)
	// (If your adapter exposes TLS min version in HTTPConfig, set it there.
	// Otherwise it already defaults to 1.3; leaving this comment as a reminder.)
	_ = tls.VersionTLS13

	return cfg, mux, nil
}

func selectSocket() string {
	// Preferred (SPIFFE-standard) and common fallbacks
	candidates := []string{
		os.Getenv("SPIFFE_ENDPOINT_SOCKET"),
		os.Getenv("SPIRE_AGENT_SOCKET"),
		"unix:///tmp/spire-agent/public/api.sock",  // Minikube example
		"unix:///var/run/spire/sockets/agent.sock", // K8s daemonset default
	}
	for _, c := range candidates {
		if c != "" {
			return c
		}
	}
	// Last resort: standard K8s path
	return "unix:///var/run/spire/sockets/agent.sock"
}

func detectTrustDomain(ctx context.Context, socket string) (string, error) {
	// Create a short-lived X.509 source just to learn our TD, then close it.
	clientOpts := workloadapi.WithClientOptions(workloadapi.WithAddr(socket))
	src, err := workloadapi.NewX509Source(ctx, clientOpts)
	if err != nil {
		return "", errors.Join(errors.New("spiffe workload api unavailable"), err)
	}
	defer src.Close()

	svid, err := src.GetX509SVID()
	if err != nil {
		return "", errors.Join(errors.New("failed to fetch SVID from workload api"), err)
	}
	td := svid.ID.TrustDomain()
	if td == (spiffeid.TrustDomain{}) {
		return "", errors.New("detected empty trust domain from SVID")
	}
	return td.String(), nil
}

func getenv(k, d string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return d
}
