package goflux

import (
	"context"
	"log/slog"
)

type pipeConfig[T any] struct {
	filters    []Filter[T]
	deadLetter DeadLetterFunc[T]
}

// PipeOption configures a Pipe or PipeMap call.
type PipeOption[T any] func(*pipeConfig[T])

// WithFilter registers a filter that runs before publish (or before map in
// PipeMap). Messages for which the filter returns false are silently dropped
// and logged. Filter errors are treated as false.
func WithFilter[T any](f Filter[T]) PipeOption[T] {
	return func(c *pipeConfig[T]) { c.filters = append(c.filters, f) }
}

// WithDeadLetter registers a dead-letter handler called when MapFunc returns
// an error or when the publisher fails after all retries are exhausted.
// The original Message[T] and the terminal error are passed to the handler.
func WithDeadLetter[T any](fn DeadLetterFunc[T]) PipeOption[T] {
	return func(c *pipeConfig[T]) { c.deadLetter = fn }
}

// ---------------------------------------------------------------------------
// Pipe — same-type wiring
// ---------------------------------------------------------------------------

// Pipe returns a Handler[T] that forwards every accepted message to pub.
// Filters run first; a dropped message never reaches pub.
// A publish error is returned to the caller as-is. Acknowledgment is
// not performed — use [AutoAck] or [Processor] to manage ack/nak.
func Pipe[T any](pub Publisher[T], opts ...PipeOption[T]) Handler[T] {
	cfg := buildConfig(opts)

	return func(ctx context.Context, msg Message[T]) error {
		if dropped := applyFilters(ctx, cfg.filters, msg); dropped {
			return nil
		}

		if err := pub.Publish(ctx, msg.Subject, msg.Payload); err != nil {
			if cfg.deadLetter != nil {
				cfg.deadLetter(ctx, msg, err)
			}

			return err
		}

		return nil
	}
}

// ---------------------------------------------------------------------------
// PipeMap — type-changing wiring
// ---------------------------------------------------------------------------

// PipeMap returns a Handler[T] that maps each message from T to U before
// publishing. Filters run on T before the map. A map error routes the original
// Message[T] to the dead-letter handler (if set) and drops the message.
// A publish error after a successful map is also dead-lettered with the
// original T message.
func PipeMap[T, U any](pub Publisher[U], mapFn MapFunc[T, U], opts ...PipeOption[T]) Handler[T] {
	cfg := buildConfig(opts)

	return func(ctx context.Context, msg Message[T]) error {
		if dropped := applyFilters(ctx, cfg.filters, msg); dropped {
			return nil
		}

		mapped, err := mapFn(ctx, msg)
		if err != nil {
			slog.ErrorContext(ctx, "pipemap: map failed, dropping message",
				slog.String("subject", msg.Subject),
				slog.Any("error", err),
			)

			if cfg.deadLetter != nil {
				cfg.deadLetter(ctx, msg, err)
			}

			return nil // map errors are non-fatal to the subscriber
		}

		if err := pub.Publish(ctx, mapped.Subject, mapped.Payload); err != nil {
			if cfg.deadLetter != nil {
				cfg.deadLetter(ctx, msg, err)
			}

			return err
		}

		return nil
	}
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func buildConfig[T any](opts []PipeOption[T]) *pipeConfig[T] {
	cfg := &pipeConfig[T]{}
	for _, o := range opts {
		o(cfg)
	}

	return cfg
}

// applyFilters returns true if the message should be dropped. Filters are
// evaluated in order; the first false or error short-circuits.
func applyFilters[T any](ctx context.Context, filters []Filter[T], msg Message[T]) bool {
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
