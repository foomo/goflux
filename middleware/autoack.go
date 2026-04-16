package middleware

import (
	"context"

	"github.com/foomo/goflux"
)

// AutoAck returns a Middleware that acknowledges messages automatically:
// nil handler error triggers Ack, non-nil error triggers Nak. Messages
// without an acker (fire-and-forget transports) are passed through as-is.
func AutoAck[T any]() goflux.Middleware[T] {
	return func(next goflux.Handler[T]) goflux.Handler[T] {
		return func(ctx context.Context, msg goflux.Message[T]) error {
			err := next(ctx, msg)
			if !msg.HasAcker() {
				return err
			}

			if err != nil {
				_ = msg.Nak()

				return err
			}

			return msg.Ack()
		}
	}
}
