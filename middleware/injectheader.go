package middleware

import (
	"context"

	"github.com/foomo/goflux"
)

// InjectMessageID returns a [goflux.Middleware] that reads the message ID from the
// message's [goflux.Header] (using [goflux.MessageIDHeader] as the key) and injects it into
// the context via [goflux.WithMessageID].
//
// Push-based transports and the [goflux.Processor] do this automatically. This
// middleware is for handler chains that bypass built-in injection — for example,
// when composing handlers directly with [goflux.Subscriber.Subscribe].
func InjectMessageID[T any]() goflux.Middleware[T] {
	return func(next goflux.Handler[T]) goflux.Handler[T] {
		return func(ctx context.Context, msg goflux.Message[T]) error {
			if id := msg.Header.Get(goflux.MessageIDHeader); id != "" {
				ctx = goflux.WithMessageID(ctx, id)
			}

			return next(ctx, msg)
		}
	}
}

// ForwardMessageID returns a [goflux.Middleware] that preserves the message ID
// from the incoming context through the handler chain. Use this with pipe's
// [pipe.WithMiddleware] to forward message IDs across pipe stages.
//
// Unlike [InjectMessageID] (which reads the ID from message headers set by
// transports), ForwardMessageID reads from context — suitable for in-process
// handler chains where the ID is already in context.
func ForwardMessageID[T any]() goflux.Middleware[T] {
	return func(next goflux.Handler[T]) goflux.Handler[T] {
		return func(ctx context.Context, msg goflux.Message[T]) error {
			if id := goflux.MessageID(ctx); id != "" {
				ctx = goflux.WithMessageID(ctx, id)
			}

			return next(ctx, msg)
		}
	}
}

// InjectHeader returns a [goflux.Middleware] that injects the message's [goflux.Header] into
// the context via [goflux.WithHeader]. Downstream code can read it with
// [goflux.HeaderFromContext].
//
// Push-based transports and the [goflux.Processor] do this automatically. This
// middleware is for handler chains that bypass built-in injection.
func InjectHeader[T any]() goflux.Middleware[T] {
	return func(next goflux.Handler[T]) goflux.Handler[T] {
		return func(ctx context.Context, msg goflux.Message[T]) error {
			if msg.Header != nil {
				ctx = goflux.WithHeader(ctx, msg.Header)
			}

			return next(ctx, msg)
		}
	}
}
