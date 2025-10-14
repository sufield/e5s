//go:build dev

package ports

import "context"

// Service represents core application use cases (business logic).
// Dev-only: This demonstrates identity-based operations in a hexagonal architecture.
//
// This interface shows how domain logic remains independent of infrastructure concerns
// (HTTP, gRPC, CLI, databases, etc.).
//
// Usage:
//
//	service := app.NewIdentityService(agent, registry)
//	msg, err := service.ExchangeMessage(ctx, alice, bob, "hello")
//
// Note: This demonstrates how authenticated identities flow through business logic.
// In production, extend this with domain-specific use cases (policy enforcement,
// audit logging, authorization checks, etc.).
type Service interface {
	// ExchangeMessage performs an authenticated message exchange between two identities.
	//
	// This demonstrates a typical identity-based operation where the service:
	// 1. Validates both identities have valid credentials
	// 2. Checks identity credentials are not expired
	// 3. Executes the business operation (message exchange)
	//
	// This is pure domain logic with no infrastructure dependencies.
	// Implementation can enforce policies, audit exchanges, rate limit, etc.
	//
	// Parameters:
	//   - ctx: Request context for timeout and cancellation
	//   - from: Sender's authenticated identity
	//   - to: Recipient's authenticated identity
	//   - content: Message content
	//
	// Returns:
	//   - *Message: The created message with metadata
	//   - error: Domain error if validation or business rules fail
	//
	// Errors:
	//   - ErrInvalidIdentityCredential: Identity lacks valid credential
	//   - ErrIdentityDocumentExpired: Identity credential expired
	ExchangeMessage(ctx context.Context, from Identity, to Identity, content string) (*Message, error)
}
