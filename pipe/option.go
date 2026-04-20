package pipe

import (
	"context"

	"github.com/foomo/goflux"
)

// Filter decides whether a message should be forwarded.
// Returning false skips the message (returns nil to transport = ack).
type Filter[T any] func(ctx context.Context, msg goflux.Message[T]) bool

// MapFunc transforms a Message[T] payload into a U value.
// A non-nil error is returned to the transport and routes to the dead-letter observer.
type MapFunc[T, U any] func(ctx context.Context, msg goflux.Message[T]) (U, error)

// FlatMapFunc expands a Message[T] into zero or more U values.
// A non-nil error is returned to the transport and routes to the dead-letter observer.
type FlatMapFunc[T, U any] func(ctx context.Context, msg goflux.Message[T]) ([]U, error)

// DeadLetterFunc is an observer called when a map/flatmap/publish operation fails.
// It receives the original message and the error for logging/alerting.
// It does NOT swallow the error — the error is still returned to the transport.
type DeadLetterFunc[T any] func(ctx context.Context, msg goflux.Message[T], err error)

// ---------------------------------------------------------------------------
// Option[T] — for New[T]
// ---------------------------------------------------------------------------

// Option configures a [New] pipe.
type Option[T any] func(*config[T])

type config[T any] struct {
	filter     Filter[T]
	deadLetter DeadLetterFunc[T]
	middleware []goflux.Middleware[T]
}

// WithFilter sets a filter that runs before publish.
// Messages for which the filter returns false are skipped (handler returns nil).
func WithFilter[T any](f Filter[T]) Option[T] {
	return func(c *config[T]) { c.filter = f }
}

// WithDeadLetter sets an observer called when publish fails.
// The observer receives the original message and the error.
func WithDeadLetter[T any](fn DeadLetterFunc[T]) Option[T] {
	return func(c *config[T]) { c.deadLetter = fn }
}

// WithMiddleware registers middleware that wraps the pipe's internal handler.
// Middleware runs before filter/map/publish — it sees the original message and
// can enrich the context that flows into subsequent stages.
func WithMiddleware[T any](mw ...goflux.Middleware[T]) Option[T] {
	return func(c *config[T]) { c.middleware = append(c.middleware, mw...) }
}

func newConfig[T any](opts []Option[T]) *config[T] {
	cfg := &config[T]{}
	for _, o := range opts {
		o(cfg)
	}

	return cfg
}

// ---------------------------------------------------------------------------
// MapOption[T, U] — for NewMap[T, U] and NewFlatMap[T, U]
// ---------------------------------------------------------------------------

// MapOption configures a [NewMap] or [NewFlatMap] pipe.
type MapOption[T, U any] func(*mapConfig[T, U])

type mapConfig[T, U any] struct {
	filter     Filter[T]
	deadLetter DeadLetterFunc[T]
	middleware []goflux.Middleware[T]
}

// WithMapFilter sets a filter that runs before map/flatmap.
func WithMapFilter[T, U any](f Filter[T]) MapOption[T, U] {
	return func(c *mapConfig[T, U]) { c.filter = f }
}

// WithMapDeadLetter sets an observer called when map/flatmap or publish fails.
func WithMapDeadLetter[T, U any](fn DeadLetterFunc[T]) MapOption[T, U] {
	return func(c *mapConfig[T, U]) { c.deadLetter = fn }
}

// WithMapMiddleware registers middleware for a map/flatmap pipe.
func WithMapMiddleware[T, U any](mw ...goflux.Middleware[T]) MapOption[T, U] {
	return func(c *mapConfig[T, U]) { c.middleware = append(c.middleware, mw...) }
}

func newMapConfig[T, U any](opts []MapOption[T, U]) *mapConfig[T, U] {
	cfg := &mapConfig[T, U]{}
	for _, o := range opts {
		o(cfg)
	}

	return cfg
}
