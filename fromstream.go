package goflux

import (
	"context"

	"github.com/foomo/goflow"
)

// FromStream consumes a goflow.Stream of messages and publishes each one via
// the provided Publisher. When subject is non-empty every message is published
// to that subject; when empty the original message subject is preserved. It
// blocks until the stream is exhausted or the context is cancelled.
func FromStream[T any](stream goflow.Stream[Message[T]], pub Publisher[T], subject string) error {
	return stream.ForEach(func(ctx context.Context, msg Message[T]) error {
		s := subject
		if s == "" {
			s = msg.Subject
		}

		return pub.Publish(ctx, s, msg.Payload)
	})
}
