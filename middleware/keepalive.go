package middleware

import (
	"context"
	"sync"
	"time"

	"github.com/foomo/goflux"
	"github.com/foomo/gofuncy"
)

// KeepAlive returns a Middleware that calls [goflux.Message.InProgress] every
// interval while the handler runs, so long-running handlers (e.g. imports)
// are not redelivered when they exceed the transport's ack timeout. Choose an
// interval well below the timeout (e.g. JetStream AckWait / 3).
//
// The heartbeat stops before the handler's result is returned, so place
// KeepAlive inside [AutoAck] or [RetryAck] in a [goflux.Chain]. Messages
// without an acker, and non-positive intervals, are passed through.
func KeepAlive[T any](interval time.Duration) goflux.Middleware[T] {
	return func(next goflux.Handler[T]) goflux.Handler[T] {
		return func(ctx context.Context, msg goflux.Message[T]) error {
			if !msg.HasAcker() || interval <= 0 {
				return next(ctx, msg)
			}

			// stopped guards against a heartbeat racing the ack/nak issued
			// after the handler returns.
			var (
				mu      sync.Mutex
				stopped bool
			)

			stop := gofuncy.GoWithCancel(ctx, func(ctx context.Context) error {
				ticker := time.NewTicker(interval)
				defer ticker.Stop()

				for {
					select {
					case <-ctx.Done():
						return nil
					case <-ticker.C:
						mu.Lock()
						if !stopped {
							_ = msg.InProgress()
						}
						mu.Unlock()
					}
				}
			}, gofuncy.WithName("goflux.middleware.keepalive"))

			err := next(ctx, msg)

			mu.Lock()
			stopped = true
			mu.Unlock()
			stop()

			return err
		}
	}
}
