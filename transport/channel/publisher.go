package channel

import (
	"context"

	"github.com/foomo/goflux"
	"go.opentelemetry.io/otel/trace"
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

// Publish delivers v to every subscriber currently registered on subject. The
// publish span and the message ID from [goflux.WithMessageID] travel with the
// message, like the trace and message ID headers of network transports.
func (p *Publisher[T]) Publish(ctx context.Context, subject string, v T) error {
	msg := goflux.Message[T]{
		Subject: subject,
		Payload: v,
		Header:  goflux.HeaderFromContext(ctx),
		ID:      goflux.MessageID(ctx),
	}

	return p.tel.RecordPublish(ctx, subject, system, func(ctx context.Context) error {
		return p.bus.publish(ctx, subject, envelope[T]{
			msg:     msg,
			spanCtx: trace.SpanContextFromContext(ctx),
		})
	})
}

// Close is a no-op; the caller owns the Bus and any inner publishers.
func (p *Publisher[T]) Close() error { return nil }
