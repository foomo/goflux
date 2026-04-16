package goflux

import (
	"context"

	"github.com/foomo/goflow"
)

// ToStream bridges a Subscriber into a goflow.Stream. It launches Subscribe in
// a goroutine via ToChan and wraps the resulting channel as a Stream.
//
// bufSize controls backpressure: a full buffer blocks the subscriber's handler
// until the consumer catches up.
func ToStream[T any](ctx context.Context, sub Subscriber[T], subject string, bufSize int) goflow.Stream[Message[T]] {
	return goflow.From(ctx, ToChan(ctx, sub, subject, bufSize))
}
