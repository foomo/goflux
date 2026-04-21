package jetstream

import (
	"context"
	"errors"
	"fmt"

	"github.com/foomo/goencode"
	"github.com/foomo/goflux"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

type Publisher[T any] struct {
	js    jetstream.JetStream
	codec goencode.Codec[T, []byte]
	tel   *goflux.Telemetry
}

func NewPublisher[T any](js jetstream.JetStream, codec goencode.Codec[T, []byte], opts ...Option) *Publisher[T] {
	cfg := applyOpts(opts)

	return &Publisher[T]{js: js, codec: codec, tel: cfg.tel}
}

func (p *Publisher[T]) Publish(ctx context.Context, subject string, v T) error {
	return p.tel.RecordPublish(ctx, subject, system, func(ctx context.Context) error {
		b, err := p.codec.Encode(v)
		if err != nil {
			return errors.Join(goflux.ErrPublish, goflux.ErrEncode, fmt.Errorf("jetstream: %w", err))
		}

		trace.SpanFromContext(ctx).SetAttributes(
			attribute.Int("messaging.message.body.size", len(b)),
			attribute.String("messaging.operation.type", "publish"),
		)

		msg := nats.NewMsg(subject)
		msg.Data = b
		msg.Header = make(nats.Header)

		p.tel.InjectContext(ctx, jetstreamHeaderCarrier{Headers: msg.Header})

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

		_, err = p.js.PublishMsg(ctx, msg)

		return err
	})
}

// Close is a no-op. The caller owns the jetstream.JetStream handle and the
// underlying *nats.Conn.
func (p *Publisher[T]) Close() error { return nil }
