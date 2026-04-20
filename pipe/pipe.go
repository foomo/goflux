package pipe

import (
	"context"
	"log/slog"

	"github.com/foomo/goflux"
)

// Pipe returns a Handler[T] that forwards every accepted message to pub.
// Filters run first; a dropped message never reaches pub.
// A publish error is returned to the caller as-is.
func Pipe[T any](pub goflux.Publisher[T], opts ...Option[T]) goflux.Handler[T] {
	cfg := buildConfig(opts)

	return func(ctx context.Context, msg goflux.Message[T]) error {
		if dropped := applyFilters(ctx, cfg.filters, msg); dropped {
			return nil
		}

		ctx = goflux.WithHeader(ctx, msg.Header)

		if err := pub.Publish(ctx, msg.Subject, msg.Payload); err != nil {
			if cfg.deadLetter != nil {
				cfg.deadLetter(ctx, msg, err)
			}

			return err
		}

		return nil
	}
}

// PipeMap returns a Handler[T] that maps each message from T to U before
// publishing. Filters run on T before the map. A map error routes the original
// Message[T] to the dead-letter handler (if set) and drops the message.
func PipeMap[T, U any](pub goflux.Publisher[U], mapFn MapFunc[T, U], opts ...Option[T]) goflux.Handler[T] {
	cfg := buildConfig(opts)

	return func(ctx context.Context, msg goflux.Message[T]) error {
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

		ctx = goflux.WithHeader(ctx, msg.Header)

		if err := pub.Publish(ctx, mapped.Subject, mapped.Payload); err != nil {
			if cfg.deadLetter != nil {
				cfg.deadLetter(ctx, msg, err)
			}

			return err
		}

		return nil
	}
}

// BoundPipe returns a Handler[T] that forwards every accepted message to a
// BoundPublisher. The bound publisher's fixed subject is used for publishing.
func BoundPipe[T any](pub goflux.BoundPublisher[T], opts ...Option[T]) goflux.Handler[T] {
	cfg := buildConfig(opts)

	return func(ctx context.Context, msg goflux.Message[T]) error {
		if dropped := applyFilters(ctx, cfg.filters, msg); dropped {
			return nil
		}

		ctx = goflux.WithHeader(ctx, msg.Header)

		if err := pub.Publish(ctx, msg.Payload); err != nil {
			if cfg.deadLetter != nil {
				cfg.deadLetter(ctx, msg, err)
			}

			return err
		}

		return nil
	}
}

// BoundPipeMap returns a Handler[T] that maps each message from T to U before
// publishing to a BoundPublisher[U].
func BoundPipeMap[T, U any](pub goflux.BoundPublisher[U], mapFn MapFunc[T, U], opts ...Option[T]) goflux.Handler[T] {
	cfg := buildConfig(opts)

	return func(ctx context.Context, msg goflux.Message[T]) error {
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

			return nil
		}

		ctx = goflux.WithHeader(ctx, msg.Header)

		if err := pub.Publish(ctx, mapped.Payload); err != nil {
			if cfg.deadLetter != nil {
				cfg.deadLetter(ctx, msg, err)
			}

			return err
		}

		return nil
	}
}
