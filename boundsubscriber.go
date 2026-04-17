package goflux

import "context"

// BoundSubscriber wraps a Subscriber with a fixed subject.
type BoundSubscriber[T any] struct {
	sub     Subscriber[T]
	subject string
}

// BindSubscriber returns a BoundSubscriber that always subscribes to the given subject.
func BindSubscriber[T any](sub Subscriber[T], subject string) *BoundSubscriber[T] {
	return &BoundSubscriber[T]{sub: sub, subject: subject}
}

// Subscribe registers handler for the bound subject. The subject parameter is
// ignored — the subject provided to [BindSubscriber] is always used.
func (b *BoundSubscriber[T]) Subscribe(ctx context.Context, _ string, handler Handler[T]) error {
	return b.sub.Subscribe(ctx, b.subject, handler)
}

// Close delegates to the underlying Subscriber.
func (b *BoundSubscriber[T]) Close() error {
	return b.sub.Close()
}
