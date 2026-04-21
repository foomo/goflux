package goflux

import "context"

// Requester sends a typed request and waits for a typed response.
type Requester[Req, Resp any] interface {
	// Request sends req to the given nats and returns the response.
	Request(ctx context.Context, subject string, req Req) (Resp, error)
	// Close releases any underlying resources.
	Close() error
}
