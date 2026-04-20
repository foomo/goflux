package pipe

import (
	"context"
	"fmt"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"

	"github.com/foomo/goflux"
)

// New returns a [goflux.Handler] that forwards every accepted message to pub.
// The handler:
//  1. Runs the middleware chain (if any)
//  2. Applies the filter (if set) — false means skip (return nil)
//  3. Forwards msg.Header into the publish context via [goflux.WithHeader]
//  4. Publishes msg.Payload to msg.Subject
//
// Publish errors are returned to the caller (transport decides retry/nak).
// The dead-letter observer is called on publish error for logging/alerting.
func New[T any](pub goflux.Publisher[T], opts ...Option[T]) goflux.Handler[T] {
	cfg := newConfig(opts)

	handler := func(ctx context.Context, msg goflux.Message[T]) error {
		span := trace.SpanFromContext(ctx)
		span.SetAttributes(attribute.String("pipe.type", "forward"))

		if cfg.filter != nil && !cfg.filter(ctx, msg) {
			span.SetAttributes(attribute.Bool("pipe.filtered", true))

			return nil
		}

		ctx = goflux.WithHeader(ctx, msg.Header)

		if err := pub.Publish(ctx, msg.Subject, msg.Payload); err != nil {
			observeDeadLetter(span, cfg.deadLetter, ctx, msg, err)
			span.AddEvent("pipe.publish_error", trace.WithAttributes(
				attribute.String("pipe.error", err.Error()),
			))

			return err
		}

		return nil
	}

	return applyMiddleware(handler, cfg.middleware)
}

// NewMap returns a [goflux.Handler] that transforms each message from T to U
// before publishing. The filter runs on the original Message[T] before the map.
//
// Map errors and publish errors are returned to the caller.
// The dead-letter observer is called on either failure.
func NewMap[T, U any](pub goflux.Publisher[U], mapFn MapFunc[T, U], opts ...MapOption[T, U]) goflux.Handler[T] {
	cfg := newMapConfig(opts)

	handler := func(ctx context.Context, msg goflux.Message[T]) error {
		span := trace.SpanFromContext(ctx)
		span.SetAttributes(attribute.String("pipe.type", "map"))

		if cfg.filter != nil && !cfg.filter(ctx, msg) {
			span.SetAttributes(attribute.Bool("pipe.filtered", true))

			return nil
		}

		mapped, err := mapFn(ctx, msg)
		if err != nil {
			observeDeadLetter(span, cfg.deadLetter, ctx, msg, err)
			span.AddEvent("pipe.map_error", trace.WithAttributes(
				attribute.String("pipe.error", err.Error()),
			))

			return fmt.Errorf("pipe map: %w", err)
		}

		ctx = goflux.WithHeader(ctx, msg.Header)

		if err := pub.Publish(ctx, msg.Subject, mapped); err != nil {
			observeDeadLetter(span, cfg.deadLetter, ctx, msg, err)
			span.AddEvent("pipe.publish_error", trace.WithAttributes(
				attribute.String("pipe.error", err.Error()),
			))

			return err
		}

		return nil
	}

	return applyMiddleware(handler, cfg.middleware)
}

// NewFlatMap returns a [goflux.Handler] that expands each message from T into
// zero or more U values, publishing each one individually.
//
// If the FlatMapFunc fails, the error is returned immediately.
// If a publish fails mid-batch, items already published are NOT rolled back —
// downstream consumers must be idempotent or deduplicate.
func NewFlatMap[T, U any](pub goflux.Publisher[U], fn FlatMapFunc[T, U], opts ...MapOption[T, U]) goflux.Handler[T] {
	cfg := newMapConfig(opts)

	handler := func(ctx context.Context, msg goflux.Message[T]) error {
		span := trace.SpanFromContext(ctx)
		span.SetAttributes(attribute.String("pipe.type", "flatmap"))

		if cfg.filter != nil && !cfg.filter(ctx, msg) {
			span.SetAttributes(attribute.Bool("pipe.filtered", true))

			return nil
		}

		items, err := fn(ctx, msg)
		if err != nil {
			observeDeadLetter(span, cfg.deadLetter, ctx, msg, err)
			span.AddEvent("pipe.map_error", trace.WithAttributes(
				attribute.String("pipe.error", err.Error()),
			))

			return fmt.Errorf("pipe flatmap: %w", err)
		}

		ctx = goflux.WithHeader(ctx, msg.Header)

		for _, item := range items {
			if err := pub.Publish(ctx, msg.Subject, item); err != nil {
				observeDeadLetter(span, cfg.deadLetter, ctx, msg, err)
				span.AddEvent("pipe.publish_error", trace.WithAttributes(
					attribute.String("pipe.error", err.Error()),
				))

				return err
			}
		}

		span.SetAttributes(attribute.Int("pipe.items_published", len(items)))

		return nil
	}

	return applyMiddleware(handler, cfg.middleware)
}

// applyMiddleware wraps handler with the middleware chain (if any).
func applyMiddleware[T any](handler goflux.Handler[T], mws []goflux.Middleware[T]) goflux.Handler[T] {
	if len(mws) == 0 {
		return handler
	}

	return goflux.Chain(mws...)(handler)
}

// observeDeadLetter calls the dead-letter observer (if set) and adds a span event.
func observeDeadLetter[T any](span trace.Span, dl DeadLetterFunc[T], ctx context.Context, msg goflux.Message[T], err error) {
	if dl != nil {
		dl(ctx, msg, err)
	}

	span.AddEvent("pipe.dead_letter", trace.WithAttributes(
		attribute.String("pipe.error", err.Error()),
	))
}
