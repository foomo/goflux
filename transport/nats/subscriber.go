package nats

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/foomo/goencode"
	"github.com/foomo/goflux"
	"github.com/nats-io/nats.go"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

type Subscriber[T any] struct {
	conn       *nats.Conn
	codec      goencode.Codec[T, []byte]
	tel        *goflux.Telemetry
	queueGroup string
}

func NewSubscriber[T any](conn *nats.Conn, codec goencode.Codec[T, []byte], opts ...Option) *Subscriber[T] {
	cfg := applyOpts(opts)

	return &Subscriber[T]{conn: conn, codec: codec, tel: cfg.tel, queueGroup: cfg.queueGroup}
}

func (s *Subscriber[T]) Subscribe(ctx context.Context, subject string, handler goflux.Handler[T]) error {
	cb := func(msg *nats.Msg) {
		var v T
		if err := s.codec.Decode(msg.Data, &v); err != nil {
			slog.ErrorContext(ctx, "nats subscriber: decode failed, dropping message",
				slog.String("nats", msg.Subject),
				slog.Any("error", err),
			)

			return
		}

		// Extract the producer's span context as a link (not parent).
		// Async messaging means the consumer is temporally decoupled from
		// the producer — a span link preserves causality without implying
		// the producer is waiting for the consumer.
		remoteSpanCtx := s.tel.ExtractSpanContext(ctx, natsHeaderCarrier{Headers: msg.Header})

		msgCtx := ctx
		if id := msg.Header.Get(goflux.MessageIDHeader); id != "" {
			msgCtx = goflux.WithMessageID(msgCtx, id)
		}

		m := goflux.Message[T]{Subject: msg.Subject, Payload: v, Header: extractGofluxHeaders(msg.Header)}

		if err := s.tel.RecordProcess(msgCtx, subject, system, func(ctx context.Context) error {
			trace.SpanFromContext(ctx).SetAttributes(
				attribute.Int("messaging.message.body.size", len(msg.Data)),
				attribute.String("messaging.operation.type", "process"),
			)

			return handler(ctx, m)
		}, goflux.WithRemoteSpanContext(remoteSpanCtx)); err != nil {
			slog.ErrorContext(msgCtx, "nats subscriber: handler error",
				slog.String("nats", subject),
				slog.Any("error", err),
			)
		}
	}

	var (
		sub *nats.Subscription
		err error
	)

	if s.queueGroup != "" {
		sub, err = s.conn.QueueSubscribe(subject, s.queueGroup, cb)
	} else {
		sub, err = s.conn.Subscribe(subject, cb)
	}

	if err != nil {
		return errors.Join(goflux.ErrSubscribe, goflux.ErrTransport, fmt.Errorf("nats: %w", err))
	}

	<-ctx.Done()

	return sub.Unsubscribe()
}

// Close is a no-op. The caller owns the *nats.Conn and is responsible for
// draining or closing it.
func (s *Subscriber[T]) Close() error { return nil }
