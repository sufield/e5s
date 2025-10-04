package ports

import (
	"context"
)

// Driving port: how clients call the app
type Service interface {
	// ExchangeMessage performs an authenticated message exchange
	ExchangeMessage(ctx context.Context, from Identity, to Identity, content string) (*Message, error)
}
