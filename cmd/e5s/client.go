package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"

	"github.com/sufield/e5s"
)

func clientCommand(args []string) error {
	fs := flag.NewFlagSet("client", flag.ExitOnError)

	fs.Usage = func() {
		fmt.Println(`Make mTLS requests using e5s (data-plane debugging tool)

USAGE:
    e5s client <subcommand> [flags]

SUBCOMMANDS:
    request    Send an mTLS request and display the response

EXAMPLES:
    # Make a GET request with debug output
    e5s client request \
      --config ./e5s.yaml \
      --url https://server.example.com:8443/time \
      --debug

    # Quick request (minimal output)
    e5s client request --config ./e5s.yaml --url https://localhost:8443/healthz`)
	}

	if err := fs.Parse(args); err != nil {
		return err
	}

	if fs.NArg() < 1 {
		fs.Usage()
		return fmt.Errorf("missing subcommand")
	}

	subcommand := fs.Arg(0)
	subArgs := fs.Args()[1:]

	switch subcommand {
	case "request":
		return clientRequestCommand(subArgs)
	default:
		return fmt.Errorf("unknown subcommand: %s", subcommand)
	}
}

func clientRequestCommand(args []string) error {
	fs := flag.NewFlagSet("client request", flag.ExitOnError)
	config := fs.String("config", "", "Path to e5s config file (required)")
	url := fs.String("url", "", "Server URL to request (required)")
	method := fs.String("method", "GET", "HTTP method")
	debug := fs.Bool("debug", false, "Enable debug output")
	verbose := fs.Bool("verbose", false, "Show response headers")

	fs.Usage = func() {
		fmt.Println(`Send an mTLS request using e5s client

USAGE:
    e5s client request --config <file> --url <url> [flags]

FLAGS:
    --config string   Path to e5s config file (required)
    --url string      Server URL to request (required)
    --method string   HTTP method (default: GET)
    --debug           Enable debug output (shows TLS handshake details)
    --verbose         Show response headers

EXAMPLES:
    # Simple GET request
    e5s client request \
      --config ./e5s.yaml \
      --url https://localhost:8443/time

    # Debug mode (shows config, handshake, peer ID)
    e5s client request \
      --config ./e5s-debug.yaml \
      --url https://server.example.com:1234/time \
      --debug \
      --verbose

    # POST request
    e5s client request \
      --config ./e5s.yaml \
      --url https://api.example.com/endpoint \
      --method POST

USE CASES:
    1. Debug mTLS handshake issues
    2. Verify server certificate validation
    3. Test SPIFFE ID authorization
    4. Reproduce production issues locally`)
	}

	if err := fs.Parse(args); err != nil {
		return err
	}

	// Validate required flags
	if *config == "" {
		fs.Usage()
		return fmt.Errorf("--config is required")
	}
	if *url == "" {
		fs.Usage()
		return fmt.Errorf("--url is required")
	}

	// Enable debug logging if requested
	if *debug {
		log.SetFlags(log.LstdFlags | log.Lmicroseconds | log.Lshortfile)
		os.Setenv("E5S_DEBUG", "1")
		log.Printf("DEBUG: config=%q", *config)
		log.Printf("DEBUG: url=%q", *url)
		log.Printf("DEBUG: method=%q", *method)
	}

	// Create e5s mTLS client
	if *debug {
		log.Println("→ Creating e5s mTLS client...")
	}
	client, cleanup, err := e5s.Client(*config)
	if err != nil {
		return fmt.Errorf("failed to create e5s client: %w", err)
	}
	defer func() {
		if err := cleanup(); err != nil {
			log.Printf("Warning: cleanup error: %v", err)
		}
	}()

	if *debug {
		log.Printf("→ Sending %s request to %s", *method, *url)
	}

	// Create HTTP request
	req, err := http.NewRequest(*method, *url, http.NoBody)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	// Send request
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	// Print response status
	fmt.Printf("HTTP/%d.%d %d %s\n", resp.ProtoMajor, resp.ProtoMinor, resp.StatusCode, resp.Status)

	// Print headers if verbose
	if *verbose || *debug {
		for name, values := range resp.Header {
			for _, value := range values {
				fmt.Printf("%s: %s\n", name, value)
			}
		}
		fmt.Println()
	}

	// Print response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response body: %w", err)
	}

	fmt.Print(string(body))

	// Ensure newline at end if body doesn't have one
	if len(body) > 0 && body[len(body)-1] != '\n' {
		fmt.Println()
	}

	// Print debug summary
	if *debug {
		log.Printf("✓ Request completed successfully")
		log.Printf("  Status: %d", resp.StatusCode)
		log.Printf("  Body size: %d bytes", len(body))
		if ct := resp.Header.Get("Content-Type"); ct != "" {
			log.Printf("  Content-Type: %s", ct)
		}
	}

	// Exit with non-zero if HTTP error
	if resp.StatusCode >= 400 {
		return fmt.Errorf("HTTP %d %s", resp.StatusCode, http.StatusText(resp.StatusCode))
	}

	return nil
}
