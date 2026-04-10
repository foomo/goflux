package goflux

import (
	"context"
)

// Peek returns a Middleware that calls fn as a side-effect for each message
// before forwarding to the next handler. The fn function should not modify the
// message. Errors from fn are intentionally ignored — use a regular middleware
// if error handling is needed.
func Peek[T any](fn func(context.Context, Message[T])) Middleware[T] {
	return func(next Handler[T]) Handler[T] {
		return func(ctx context.Context, msg Message[T]) error {
			fn(ctx, msg)
			return next(ctx, msg)
		}
	}
}
