package goflux

import "context"

// BoundPublisher wraps a Publisher with a fixed subject.
type BoundPublisher[T any] struct {
	pub     Publisher[T]
	subject string
}

// Bind returns a BoundPublisher that always publishes to the given subject.
func Bind[T any](pub Publisher[T], subject string) *BoundPublisher[T] {
	return &BoundPublisher[T]{pub: pub, subject: subject}
}

// Publish sends v to the bound subject.
func (b *BoundPublisher[T]) Publish(ctx context.Context, v T) error {
	return b.pub.Publish(ctx, b.subject, v)
}

// Close delegates to the underlying Publisher.
func (b *BoundPublisher[T]) Close() error {
	return b.pub.Close()
}
