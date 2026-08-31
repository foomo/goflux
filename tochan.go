package goflux

import (
	"context"

	"github.com/foomo/gofuncy"
)

// ToChan bridges a [Subscriber] into a plain channel. It launches Subscribe in
// a goroutine and forwards each message (including acker) into a buffered
// channel. The returned channel closes when ctx is cancelled.
//
// ToChan does not return until the subscription is established, so a publish
// sequenced after it is not lost. When sub implements [ReadySubscriber] this is
// a guarantee; otherwise it degrades to the best-effort fallback described in
// [SubscribeWithReady].
//
// bufSize controls backpressure: a full buffer blocks the subscriber's handler
// until the consumer catches up.
func ToChan[T any](ctx context.Context, sub Subscriber[T], subject string, bufSize int) <-chan Message[T] {
	ch := make(chan Message[T], bufSize)

	gofuncy.StartWithReady(ctx, func(ctx context.Context, ready gofuncy.ReadyFunc) error {
		defer close(ch)

		return SubscribeWithReady(ctx, sub, subject, func(ctx context.Context, msg Message[T]) error {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case ch <- msg:
				return nil
			}
		}, ready)
	}, gofuncy.WithName("goflux.tochan"))

	return ch
}
