package goflux

import (
	"context"
	"sync/atomic"
)

// Take returns a Middleware that passes the first n messages through and
// silently drops all subsequent messages. The counter is safe for concurrent use.
func Take[T any](n int) Middleware[T] {
	return func(next Handler[T]) Handler[T] {
		var count atomic.Int64

		return func(ctx context.Context, msg Message[T]) error {
			if count.Add(1) > int64(n) {
				return nil
			}

			return next(ctx, msg)
		}
	}
}
