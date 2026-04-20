package pipe

import (
	"context"

	"github.com/foomo/goflow"
	"github.com/foomo/goflux"
)

// ToStream bridges a Subscriber into a goflow.Stream. It launches Subscribe in
// a goroutine via ToChan and wraps the resulting channel as a Stream.
func ToStream[T any](ctx context.Context, sub goflux.Subscriber[T], subject string, bufSize int) goflow.Stream[goflux.Message[T]] {
	return goflow.From(ctx, ToChan(ctx, sub, subject, bufSize))
}

// BoundToStream bridges a BoundSubscriber into a goflow.Stream. Like ToStream
// but no subject parameter is needed.
func BoundToStream[T any](ctx context.Context, sub goflux.BoundSubscriber[T], bufSize int) goflow.Stream[goflux.Message[T]] {
	return goflow.From(ctx, BoundToChan(ctx, sub, bufSize))
}
