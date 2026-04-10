package goflux

import (
	"context"
)

// Process returns a Middleware that limits concurrent handler invocations to n.
// When all n slots are occupied, the handler blocks until a slot frees up or the
// context is cancelled. Each handler invocation runs synchronously — errors are
// returned to the caller as-is.
func Process[T any](n int) Middleware[T] {
	return func(next Handler[T]) Handler[T] {
		sem := make(chan struct{}, n)

		return func(ctx context.Context, msg Message[T]) error {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case sem <- struct{}{}:
			}

			defer func() { <-sem }()

			return next(ctx, msg)
		}
	}
}
