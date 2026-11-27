package bridge

import (
	"context"

	"github.com/foomo/goflow"
	"github.com/foomo/goflux"
)

// FromStream consumes a [goflow.Stream] of messages and publishes each one via
// the provided [goflux.Publisher]. The original message subject is used for
// publishing; the message ID ([goflux.Message.ID]) and header are not
// forwarded.
func FromStream[T any](stream goflow.Stream[goflux.Message[T]], pub goflux.Publisher[T]) error {
	return stream.ForEach(func(ctx context.Context, msg goflux.Message[T]) error {
		return pub.Publish(ctx, msg.Subject, msg.Payload)
	})
}
