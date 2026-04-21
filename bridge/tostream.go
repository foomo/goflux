package bridge

import (
	"context"

	"github.com/foomo/goflow"
	"github.com/foomo/goflux"
)

// ToStream bridges a [goflux.Subscriber] into a [goflow.Stream]. It launches
// Subscribe in a goroutine via [goflux.ToChan] and wraps the resulting channel
// as a Stream.
func ToStream[T any](ctx context.Context, sub goflux.Subscriber[T], subject string, bufSize int) goflow.Stream[goflux.Message[T]] {
	return goflow.From(ctx, goflux.ToChan(ctx, sub, subject, bufSize))
}
