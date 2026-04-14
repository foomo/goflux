package goflux

import "context"

// RequestHandler processes a request and returns a response.
type RequestHandler[Req, Resp any] func(ctx context.Context, req Req) (Resp, error)

// Responder handles incoming requests and produces typed responses.
type Responder[Req, Resp any] interface {
	// Serve registers the handler for the given subject. The call blocks
	// until ctx is cancelled or a fatal error occurs.
	Serve(ctx context.Context, subject string, handler RequestHandler[Req, Resp]) error
	// Close releases any underlying resources.
	Close() error
}
