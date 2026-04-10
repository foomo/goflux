package goflux

import (
	"context"
	"sync/atomic"
)

// roundRobinPublisher distributes Publish calls across N inner publishers
// in round-robin order.
type roundRobinPublisher[T any] struct {
	publishers []Publisher[T]
	counter    atomic.Uint64
}

// RoundRobin returns a Publisher[T] that distributes each Publish call to a
// single inner publisher, cycling through them in round-robin order.
//
// Close is a no-op — the caller owns the inner publishers and is responsible
// for closing them.
func RoundRobin[T any](publishers ...Publisher[T]) Publisher[T] {
	return &roundRobinPublisher[T]{publishers: publishers}
}

func (r *roundRobinPublisher[T]) Publish(ctx context.Context, subject string, v T) error {
	n := uint64(len(r.publishers))
	idx := r.counter.Add(1) - 1

	return r.publishers[idx%n].Publish(ctx, subject, v)
}

func (r *roundRobinPublisher[T]) Close() error { return nil }
