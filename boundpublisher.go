package goflux

import "context"

// BoundPublisher publishes to a fixed subject. No subject param needed.
type BoundPublisher[T any] interface {
	// Publish serializes v and delivers it to the bound subject.
	Publish(ctx context.Context, v T) error
	// Close releases any underlying connections.
	Close() error
}

// BindPublisher wraps a Publisher with a fixed subject.
func BindPublisher[T any](pub Publisher[T], subject string) BoundPublisher[T] {
	return &boundPublisher[T]{pub: pub, subject: subject}
}

type boundPublisher[T any] struct {
	pub     Publisher[T]
	subject string
}

func (b *boundPublisher[T]) Publish(ctx context.Context, v T) error {
	return b.pub.Publish(ctx, b.subject, v)
}

func (b *boundPublisher[T]) Close() error {
	return b.pub.Close()
}
