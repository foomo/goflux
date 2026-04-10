package channel

import (
	"context"

	"github.com/foomo/goflux"
)

type Publisher[T any] struct {
	bus *Bus[T]
	tel *goflux.Telemetry
}

func NewPublisher[T any](bus *Bus[T], opts ...Option) *Publisher[T] {
	cfg := applyOpts(opts)

	return &Publisher[T]{bus: bus, tel: cfg.tel}
}

func (p *Publisher[T]) Publish(ctx context.Context, subject string, v T) error {
	msg := goflux.Message[T]{Subject: subject, Payload: v}

	return p.tel.RecordPublish(ctx, subject, system, func(ctx context.Context) error {
		return p.bus.publish(ctx, subject, msg)
	})
}

func (p *Publisher[T]) Close() error { return nil }
