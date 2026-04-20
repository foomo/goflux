package pipe

import (
	"context"

	"github.com/foomo/goflux"
	"github.com/foomo/gofuncy"
)

// ToChan bridges a Subscriber into a plain channel. It launches Subscribe in a
// goroutine and forwards each message (including acker) into a buffered
// channel. The returned channel closes when ctx is cancelled.
//
// bufSize controls backpressure: a full buffer blocks the subscriber's handler
// until the consumer catches up.
func ToChan[T any](ctx context.Context, sub goflux.Subscriber[T], subject string, bufSize int) <-chan goflux.Message[T] {
	ch := make(chan goflux.Message[T], bufSize)

	gofuncy.Go(ctx, func(ctx context.Context) error {
		defer close(ch)

		return sub.Subscribe(ctx, subject, func(ctx context.Context, msg goflux.Message[T]) error {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case ch <- msg:
				return nil
			}
		})
	}, gofuncy.WithName("pipe.tochan"))

	return ch
}

// BoundToChan bridges a BoundSubscriber into a plain channel. Like ToChan but
// no subject parameter is needed — the bound subscriber's fixed subject is used.
func BoundToChan[T any](ctx context.Context, sub goflux.BoundSubscriber[T], bufSize int) <-chan goflux.Message[T] {
	ch := make(chan goflux.Message[T], bufSize)

	gofuncy.Go(ctx, func(ctx context.Context) error {
		defer close(ch)

		return sub.Subscribe(ctx, func(ctx context.Context, msg goflux.Message[T]) error {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case ch <- msg:
				return nil
			}
		})
	}, gofuncy.WithName("pipe.boundtochan"))

	return ch
}
