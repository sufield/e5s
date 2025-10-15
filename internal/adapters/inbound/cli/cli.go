//go:build dev

package cli

import (
	"context"
	"fmt"
	"os"

	"github.com/pocket/hexagon/spire/internal/app"
	"github.com/pocket/hexagon/spire/internal/app/identityconv"
	"github.com/pocket/hexagon/spire/internal/ports"
)

// CLI is an inbound adapter that drives the application via command-line interface
// Responsibility: ONLY I/O presentation and orchestration of use cases
// Does NOT configure or wire dependencies
type CLI struct {
	application *app.Application
}

// New creates a new CLI adapter with a bootstrapped application
func New(application *app.Application) *CLI {
	return &CLI{
		application: application,
	}
}

// Run executes the CLI adapter - handles ONLY presentation and orchestration
func (c *CLI) Run(ctx context.Context) error {
	fmt.Println("=== In-Memory SPIRE System with Hexagonal Architecture ===")
	fmt.Println()

	// Display configuration (read-only)
	fmt.Println("Configuration:")
	fmt.Printf("  Trust Domain: %s\n", c.application.Config().TrustDomain)
	fmt.Printf("  Agent SPIFFE ID: %s\n", c.application.Config().AgentSpiffeID)
	fmt.Printf("  Registered Workloads: %d\n", len(c.application.Config().Workloads))
	for _, w := range c.application.Config().Workloads {
		fmt.Printf("    - %s (UID: %d)\n", w.SpiffeID, w.UID)
	}
	fmt.Println()

	// Demonstrate workload attestation and identity document fetching
	fmt.Println("Attesting and fetching identity documents for workloads...")

	// Server workload
	serverWorkload := ports.ProcessIdentity{
		PID:  12345,
		UID:  1001,
		GID:  1001,
		Path: "/usr/bin/server",
	}
	serverDoc, err := c.application.Agent().FetchIdentityDocument(ctx, serverWorkload)
	if err != nil {
		return fmt.Errorf("failed to fetch server identity document: %w", err)
	}
	serverIdentity := ports.Identity{
		IdentityCredential: serverDoc.IdentityCredential(),
		IdentityDocument:   serverDoc,
		Name:               identityconv.DeriveIdentityName(serverDoc.IdentityCredential()),
	}
	fmt.Printf("  ✓ Server workload identity document issued: %s\n", serverIdentity.IdentityCredential.String())

	// Client workload
	clientWorkload := ports.ProcessIdentity{
		PID:  12346,
		UID:  1002,
		GID:  1002,
		Path: "/usr/bin/client",
	}
	clientDoc, err := c.application.Agent().FetchIdentityDocument(ctx, clientWorkload)
	if err != nil {
		return fmt.Errorf("failed to fetch client identity document: %w", err)
	}
	clientIdentity := ports.Identity{
		IdentityCredential: clientDoc.IdentityCredential(),
		IdentityDocument:   clientDoc,
		Name:               identityconv.DeriveIdentityName(clientDoc.IdentityCredential()),
	}
	fmt.Printf("  ✓ Client workload identity document issued: %s\n", clientIdentity.IdentityCredential.String())
	fmt.Println()

	// Execute core use case: authenticated message exchange
	fmt.Println("Performing authenticated message exchange...")

	// Client sends message to server
	msg, err := c.application.Service().ExchangeMessage(ctx, clientIdentity, serverIdentity, "Hello server")
	if err != nil {
		return fmt.Errorf("failed to exchange message: %w", err)
	}
	fmt.Printf("  [%s → %s]: %s\n", msg.From.Name, msg.To.Name, msg.Content)

	// Server sends response to client
	response, err := c.application.Service().ExchangeMessage(ctx, serverIdentity, clientIdentity, "Hello client")
	if err != nil {
		return fmt.Errorf("failed to exchange response: %w", err)
	}
	fmt.Printf("  [%s → %s]: %s\n", response.From.Name, response.To.Name, response.Content)
	fmt.Println()

	// Display summary
	currentUID := os.Getuid()
	fmt.Println("=== Summary ===")
	fmt.Printf("✓ Success! Hexagonal architecture with separated concerns:\n")
	fmt.Printf("  - ConfigLoader port: loads configuration\n")
	fmt.Printf("  - Application composer: wires all dependencies\n")
	fmt.Printf("  - CLI adapter: handles ONLY I/O presentation\n")
	fmt.Printf("  - Core domain: pure business logic\n")
	fmt.Printf("  - Current process UID: %d\n", currentUID)

	return nil
}

var _ ports.CLI = (*CLI)(nil)
