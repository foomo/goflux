package goflux

import (
	"context"
	"errors"
)

type fanoutConfig[T any] struct {
	allOrNothing bool
}

// FanOutOption configures a FanOut publisher.
type FanOutOption[T any] func(*fanoutConfig[T])

// WithFanOutAllOrNothing makes the FanOut publisher treat any single publish
// failure as a failure for the entire call. The default is best-effort: publish
// to all, join errors from any that fail.
func WithFanOutAllOrNothing[T any]() FanOutOption[T] {
	return func(c *fanoutConfig[T]) { c.allOrNothing = true }
}

// fanoutPublisher broadcasts Publish calls to N inner publishers.
type fanoutPublisher[T any] struct {
	publishers []Publisher[T]
	cfg        fanoutConfig[T]
}

// FanOut returns a Publisher[T] that forwards every Publish call to all inner
// publishers. Errors from individual publishers are joined via errors.Join.
//
// Close is a no-op — the caller owns the inner publishers and is responsible
// for closing them.
func FanOut[T any](publishers []Publisher[T], opts ...FanOutOption[T]) Publisher[T] {
	cfg := fanoutConfig[T]{}
	for _, o := range opts {
		o(&cfg)
	}

	return &fanoutPublisher[T]{publishers: publishers, cfg: cfg}
}

func (f *fanoutPublisher[T]) Publish(ctx context.Context, subject string, v T) error {
	var errs []error

	for _, pub := range f.publishers {
		if err := pub.Publish(ctx, subject, v); err != nil {
			if f.cfg.allOrNothing {
				return err
			}

			errs = append(errs, err)
		}
	}

	return errors.Join(errs...)
}

func (f *fanoutPublisher[T]) Close() error { return nil }
