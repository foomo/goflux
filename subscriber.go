package goflux

import (
	"context"
)

// Subscriber listens on one or more subjects and dispatches decoded messages
// to a Handler.
type Subscriber[T any] interface {
	// Subscribe registers handler for the subject. The call blocks until ctx is
	// canceled or the implementation encounters a fatal error.
	//
	// Subscribe gives no guarantee about when the subscription becomes visible
	// to the broker. Because it blocks, callers must run it in a goroutine and
	// therefore cannot observe registration. A caller that publishes to the
	// same subject immediately after starting Subscribe may lose messages:
	// core NATS silently drops messages that have no matching subscriber.
	// Use [ReadySubscriber] when the publish must not be lost.
	Subscribe(ctx context.Context, subject string, handler Handler[T]) error
	// Close unsubscribes and releases resources.
	Close() error
}

// ReadySubscriber is an optional interface for subscribers that can signal when
// their subscription is established and guaranteed to receive messages.
//
// It exists because [Subscriber.Subscribe] blocks for the lifetime of the
// subscription, leaving no return value that could mean "registered". Without a
// readiness signal, a subscribe-then-publish sequence is a race: the publish can
// reach the broker before the subscription does.
//
// All goflux transports implement this interface. Prefer it over Subscribe
// whenever a publish is sequenced after a subscribe.
type ReadySubscriber[T any] interface {
	Subscriber[T]
	// SubscribeWithReady behaves like [Subscriber.Subscribe] but invokes ready
	// once the subscription is established and certain to receive subsequently
	// published messages. For network transports this means the broker has
	// acknowledged the subscription, not merely that a frame was buffered
	// locally.
	//
	// ready is called at most once, and never after SubscribeWithReady returns.
	// If registration fails, SubscribeWithReady returns the error without
	// calling ready. ready must not block.
	SubscribeWithReady(ctx context.Context, subject string, handler Handler[T], ready func()) error
}

// SubscribeWithReady subscribes via [ReadySubscriber.SubscribeWithReady] when
// sub supports it, and otherwise falls back to [Subscriber.Subscribe], invoking
// ready immediately before subscribing.
//
// The fallback is best-effort and inherently racy — it exists so third-party
// Subscriber implementations keep working, not to provide a guarantee. All
// goflux transports take the non-racy path.
func SubscribeWithReady[T any](ctx context.Context, sub Subscriber[T], subject string, handler Handler[T], ready func()) error {
	if rs, ok := sub.(ReadySubscriber[T]); ok {
		return rs.SubscribeWithReady(ctx, subject, handler, ready)
	}

	ready()

	return sub.Subscribe(ctx, subject, handler)
}
