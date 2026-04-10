package goflux

import (
	"context"

	"github.com/foomo/gofuncy"
)

// ToChan bridges a Subscriber into a plain channel. It launches Subscribe in a
// goroutine and forwards each message payload into a buffered channel. The
// returned channel closes when ctx is cancelled.
//
// bufSize controls backpressure: a full buffer blocks the subscriber's handler
// until the consumer catches up.
//
// To convert to a stream.Stream, use stream.FromFunc:
//
//	s := stream.FromFunc(ctx, bufSize, func(ctx context.Context, send func(T) error) error {
//	    return sub.Subscribe(ctx, subject, func(ctx context.Context, msg goflux.Message[T]) error {
//	        return send(msg.Payload)
//	    })
//	})
func ToChan[T any](ctx context.Context, sub Subscriber[T], subject string, bufSize int) <-chan T {
	ch := make(chan T, bufSize)

	gofuncy.Go(ctx, func(ctx context.Context) error {
		defer close(ch)

		return sub.Subscribe(ctx, subject, func(ctx context.Context, msg Message[T]) error {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case ch <- msg.Payload:
				return nil
			}
		})
	}, gofuncy.WithName("goflux.tochan"))

	return ch
}
