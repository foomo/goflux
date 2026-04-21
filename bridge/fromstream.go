package bridge

import (
	"context"

	"github.com/foomo/goflow"
	"github.com/foomo/goflux"
)

// FromStream consumes a [goflow.Stream] of messages and publishes each one via
// the provided [goflux.Publisher]. The original message nats is used for
// publishing.
func FromStream[T any](stream goflow.Stream[goflux.Message[T]], pub goflux.Publisher[T]) error {
	return stream.ForEach(func(ctx context.Context, msg goflux.Message[T]) error {
		return pub.Publish(ctx, msg.Subject, msg.Payload)
	})
}
