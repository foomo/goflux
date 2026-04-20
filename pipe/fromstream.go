package pipe

import (
	"context"

	"github.com/foomo/goflow"
	"github.com/foomo/goflux"
)

// FromStream consumes a goflow.Stream of messages and publishes each one via
// the provided Publisher. When subject is non-empty every message is published
// to that subject; when empty the original message subject is preserved.
func FromStream[T any](stream goflow.Stream[goflux.Message[T]], pub goflux.Publisher[T], subject string) error {
	return stream.ForEach(func(ctx context.Context, msg goflux.Message[T]) error {
		s := subject
		if s == "" {
			s = msg.Subject
		}

		return pub.Publish(ctx, s, msg.Payload)
	})
}

// BoundFromStream consumes a goflow.Stream of messages and publishes each one
// via the provided BoundPublisher. The bound publisher's fixed subject is used.
func BoundFromStream[T any](stream goflow.Stream[goflux.Message[T]], pub goflux.BoundPublisher[T]) error {
	return stream.ForEach(func(ctx context.Context, msg goflux.Message[T]) error {
		return pub.Publish(ctx, msg.Payload)
	})
}
