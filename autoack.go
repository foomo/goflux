package goflux

import "context"

// AutoAck returns a Middleware that acknowledges messages automatically:
// nil handler error triggers Ack, non-nil error triggers Nak. Messages
// without an acker (fire-and-forget transports) are passed through as-is.
func AutoAck[T any]() Middleware[T] {
	return func(next Handler[T]) Handler[T] {
		return func(ctx context.Context, msg Message[T]) error {
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
