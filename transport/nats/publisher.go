package nats

import (
	"context"
	"errors"
	"fmt"

	"github.com/foomo/goencode"
	"github.com/foomo/goflux"
	"github.com/nats-io/nats.go"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

type Publisher[T any] struct {
	conn       *nats.Conn
	serializer goencode.Codec[T]
	tel        *goflux.Telemetry
}

func NewPublisher[T any](conn *nats.Conn, serializer goencode.Codec[T], opts ...Option) *Publisher[T] {
	cfg := applyOpts(opts)

	return &Publisher[T]{conn: conn, serializer: serializer, tel: cfg.tel}
}

func (p *Publisher[T]) Publish(ctx context.Context, subject string, v T) error {
	return p.tel.RecordPublish(ctx, subject, system, func(ctx context.Context) error {
		b, err := p.serializer.Encode(v)
		if err != nil {
			return errors.Join(goflux.ErrPublish, goflux.ErrEncode, fmt.Errorf("nats: %w", err))
		}

		trace.SpanFromContext(ctx).SetAttributes(
			attribute.Int("messaging.message.body.size", len(b)),
			attribute.String("messaging.operation.type", "publish"),
		)

		msg := nats.NewMsg(subject)
		msg.Data = b
		msg.Header = make(nats.Header)

		p.tel.InjectContext(ctx, natsHeaderCarrier{Headers: msg.Header})

		if id := goflux.MessageID(ctx); id != "" {
			msg.Header.Set(goflux.MessageIDHeader, id)
		}

		if h := goflux.HeaderFromContext(ctx); h != nil {
			for k, vs := range h {
				for _, v := range vs {
					msg.Header.Add(gofluxHeaderPrefix+k, v)
				}
			}
		}

		return p.conn.PublishMsg(msg)
	})
}

// Close is a no-op. The caller owns the *nats.Conn and is responsible for
// draining or closing it.
func (p *Publisher[T]) Close() error { return nil }
