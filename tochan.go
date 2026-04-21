package goflux

import (
	"context"

	"github.com/foomo/gofuncy"
)

// ToChan bridges a [Subscriber] into a plain channel. It launches Subscribe in
// a goroutine and forwards each message (including acker) into a buffered
// channel. The returned channel closes when ctx is cancelled.
//
// bufSize controls backpressure: a full buffer blocks the subscriber's handler
// until the consumer catches up.
func ToChan[T any](ctx context.Context, sub Subscriber[T], subject string, bufSize int) <-chan Message[T] {
	ch := make(chan Message[T], bufSize)

	gofuncy.Go(ctx, func(ctx context.Context) error {
		defer close(ch)

		return sub.Subscribe(ctx, subject, func(ctx context.Context, msg Message[T]) error {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case ch <- msg:
				return nil
			}
		})
	}, gofuncy.WithName("goflux.tochan"))

	return ch
}
