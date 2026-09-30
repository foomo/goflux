package channel

import (
	"context"

	"github.com/foomo/goflux"
)

// Publisher publishes messages onto a [Bus].
type Publisher[T any] struct {
	bus *Bus[T]
	tel *goflux.Telemetry
}

// NewPublisher returns a Publisher that publishes onto bus.
func NewPublisher[T any](bus *Bus[T], opts ...Option) *Publisher[T] {
	cfg := applyOpts(opts)

	return &Publisher[T]{bus: bus, tel: cfg.tel}
}

// Publish delivers v to every subscriber currently registered on subject.
func (p *Publisher[T]) Publish(ctx context.Context, subject string, v T) error {
	msg := goflux.Message[T]{Subject: subject, Payload: v, Header: goflux.HeaderFromContext(ctx)}

	return p.tel.RecordPublish(ctx, subject, system, func(ctx context.Context) error {
		return p.bus.publish(ctx, subject, msg)
	})
}

// Close is a no-op; the caller owns the Bus and any inner publishers.
func (p *Publisher[T]) Close() error { return nil }
