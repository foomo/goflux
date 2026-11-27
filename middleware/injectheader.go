package middleware

import (
	"context"

	"github.com/foomo/goflux"
)

// InjectMessageID returns a [goflux.Middleware] that reads the message ID from the
// message's [goflux.Header] (using [goflux.MessageIDHeader] as the key) and injects it into
// the context via [goflux.WithMessageID], so messages published from the
// handler carry it.
//
// Transports expose the publisher's message ID as [goflux.Message.ID], not in
// msg.Header. Use this middleware only when a publisher sets
// [goflux.MessageIDHeader] as a goflux header via [goflux.WithHeader]; to
// forward [goflux.Message.ID], use [ForwardMessageID].
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

// ForwardMessageID returns a [goflux.Middleware] that puts the received
// [goflux.Message.ID] into the context via [goflux.WithMessageID], so messages
// published from the handler carry the same ID. Use this with pipe's
// [pipe.WithMiddleware] to forward message IDs across pipe stages.
//
// Transports never forward the ID on their own. On JetStream the ID is also
// the Nats-Msg-Id used for deduplication: a forwarded message that lands in
// the same stream as the received one, or several messages published with
// the same forwarded ID (for example by [pipe.NewFlatMap]), are dropped as
// duplicates within the stream's Duplicates window.
//
// [pipe.WithMiddleware]: https://pkg.go.dev/github.com/foomo/goflux/pipe#WithMiddleware
// [pipe.NewFlatMap]: https://pkg.go.dev/github.com/foomo/goflux/pipe#NewFlatMap
func ForwardMessageID[T any]() goflux.Middleware[T] {
	return func(next goflux.Handler[T]) goflux.Handler[T] {
		return func(ctx context.Context, msg goflux.Message[T]) error {
			if id := msg.ID; id != "" {
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
// No transport does this automatically, so use this middleware when
// downstream code reads [goflux.HeaderFromContext].
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
