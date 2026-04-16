package goflux

import (
	"context"

	"github.com/foomo/goflow"
)

// FromStream consumes a goflow.Stream of messages and publishes each one via
// the provided Publisher. It blocks until the stream is exhausted or the context
// is cancelled.
func FromStream[T any](ctx context.Context, stream goflow.Stream[Message[T]], pub Publisher[T]) error {
	return stream.ForEach(func(ctx context.Context, msg Message[T]) error {
		return pub.Publish(ctx, msg.Subject, msg.Payload)
	})
}
