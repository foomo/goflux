package goflux

import (
	"context"
	"sync/atomic"
)

// Skip returns a Middleware that drops the first n messages and passes the rest
// through to the next handler. The counter is safe for concurrent use.
func Skip[T any](n int) Middleware[T] {
	return func(next Handler[T]) Handler[T] {
		var count atomic.Int64

		return func(ctx context.Context, msg Message[T]) error {
			if count.Add(1) <= int64(n) {
				return nil
			}

			return next(ctx, msg)
		}
	}
}
