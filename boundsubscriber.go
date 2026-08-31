package goflux

import "context"

// BoundSubscriber subscribes to a fixed subject. No subject param needed.
type BoundSubscriber[T any] interface {
	// Subscribe registers handler for the bound subject. The call blocks until
	// ctx is canceled or the implementation encounters a fatal error.
	//
	// As with [Subscriber.Subscribe], this reports nothing about when the
	// subscription becomes live. Use SubscribeWithReady to sequence a publish
	// after a subscribe.
	Subscribe(ctx context.Context, handler Handler[T]) error
	// SubscribeWithReady behaves like Subscribe but invokes ready once the
	// subscription is established. See [ReadySubscriber] for the guarantee.
	SubscribeWithReady(ctx context.Context, handler Handler[T], ready func()) error
	// Close unsubscribes and releases resources.
	Close() error
}

// BindSubscriber wraps a Subscriber with a fixed subject. Readiness support is
// preserved: the returned BoundSubscriber delegates to sub's
// [ReadySubscriber] implementation when it has one.
func BindSubscriber[T any](sub Subscriber[T], subject string) BoundSubscriber[T] {
	return &boundSubscriber[T]{sub: sub, subject: subject}
}

type boundSubscriber[T any] struct {
	sub     Subscriber[T]
	subject string
}

func (b *boundSubscriber[T]) Subscribe(ctx context.Context, handler Handler[T]) error {
	return b.sub.Subscribe(ctx, b.subject, handler)
}

func (b *boundSubscriber[T]) SubscribeWithReady(ctx context.Context, handler Handler[T], ready func()) error {
	return SubscribeWithReady(ctx, b.sub, b.subject, handler, ready)
}

func (b *boundSubscriber[T]) Close() error {
	return b.sub.Close()
}
