package goflux

import (
	"context"
)

// Distinct returns a Middleware that deduplicates messages using a key function.
// The first message for each key passes through; subsequent messages with the
// same key are silently dropped. The seen set is not bounded — use a custom
// middleware if memory is a concern.
func Distinct[T any](key func(Message[T]) string) Middleware[T] {
	return func(next Handler[T]) Handler[T] {
		seen := make(map[string]struct{})

		return func(ctx context.Context, msg Message[T]) error {
			k := key(msg)
			if _, ok := seen[k]; ok {
				return nil
			}

			seen[k] = struct{}{}

			return next(ctx, msg)
		}
	}
}
