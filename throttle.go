package goflux

import (
	"context"
	"time"
)

// Throttle returns a Middleware that rate-limits handler invocations to at most
// one per duration d. The first message passes immediately; subsequent messages
// block until the ticker fires. Context cancellation is respected while waiting.
func Throttle[T any](d time.Duration) Middleware[T] {
	return func(next Handler[T]) Handler[T] {
		ticker := time.NewTicker(d)
		first := true

		return func(ctx context.Context, msg Message[T]) error {
			if !first {
				select {
				case <-ctx.Done():
					return ctx.Err()
				case <-ticker.C:
				}
			}

			first = false

			return next(ctx, msg)
		}
	}
}
