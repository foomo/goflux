package pipe

import (
	"context"
	"log/slog"

	"github.com/foomo/goflux"
)

// Filter decides whether a message should be forwarded.
// Returning false or a non-nil error drops the message and logs the reason.
type Filter[T any] func(ctx context.Context, msg goflux.Message[T]) (bool, error)

// DeadLetterFunc receives messages that could not be mapped or published after
// all retries are exhausted. Use it to log, alert, or forward to a DLQ.
type DeadLetterFunc[T any] func(ctx context.Context, msg goflux.Message[T], err error)

// MapFunc transforms a Message[T] into a Message[U].
// A non-nil error drops the message and routes it to the DeadLetterFunc if set.
type MapFunc[T, U any] func(ctx context.Context, msg goflux.Message[T]) (goflux.Message[U], error)

type config[T any] struct {
	filters    []Filter[T]
	deadLetter DeadLetterFunc[T]
}

// Option configures a Pipe or PipeMap call.
type Option[T any] func(*config[T])

// WithFilter registers a filter that runs before publish (or before map in
// PipeMap). Messages for which the filter returns false are silently dropped
// and logged. Filter errors are treated as false.
func WithFilter[T any](f Filter[T]) Option[T] {
	return func(c *config[T]) { c.filters = append(c.filters, f) }
}

// WithDeadLetter registers a dead-letter handler called when MapFunc returns
// an error or when the publisher fails after all retries are exhausted.
// The original Message[T] and the terminal error are passed to the handler.
func WithDeadLetter[T any](fn DeadLetterFunc[T]) Option[T] {
	return func(c *config[T]) { c.deadLetter = fn }
}

func buildConfig[T any](opts []Option[T]) *config[T] {
	cfg := &config[T]{}
	for _, o := range opts {
		o(cfg)
	}

	return cfg
}

// applyFilters returns true if the message should be dropped. Filters are
// evaluated in order; the first false or error short-circuits.
func applyFilters[T any](ctx context.Context, filters []Filter[T], msg goflux.Message[T]) bool {
	for _, f := range filters {
		ok, err := f(ctx, msg)
		if err != nil {
			slog.WarnContext(ctx, "pipe: filter error, dropping message",
				slog.String("subject", msg.Subject),
				slog.Any("error", err),
			)

			return true
		}

		if !ok {
			slog.DebugContext(ctx, "pipe: message filtered",
				slog.String("subject", msg.Subject),
			)

			return true
		}
	}

	return false
}
