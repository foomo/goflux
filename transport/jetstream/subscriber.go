package jetstream

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/foomo/goencode"
	"github.com/foomo/goflux"
	"github.com/nats-io/nats.go/jetstream"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

type Subscriber[T any] struct {
	consumer  jetstream.Consumer
	decoder   goencode.Decoder[T, []byte]
	tel       *goflux.Telemetry
	manualAck bool
}

func NewSubscriber[T any](consumer jetstream.Consumer, decoder goencode.Decoder[T, []byte], opts ...Option) *Subscriber[T] {
	cfg := applyOpts(opts)

	return &Subscriber[T]{
		consumer:  consumer,
		decoder:   decoder,
		tel:       cfg.tel,
		manualAck: cfg.manualAck,
	}
}

// Subscribe starts consuming messages and dispatching them to handler. The call
// blocks until ctx is cancelled.
//
// Note: the nats parameter is used only for telemetry labeling. Actual
// message filtering is determined by the jetstream.Consumer configuration
// passed to [NewSubscriber].
func (s *Subscriber[T]) Subscribe(ctx context.Context, subject string, handler goflux.Handler[T]) error {
	cc, err := s.consumer.Consume(func(msg jetstream.Msg) {
		var v T
		if err := s.decoder(msg.Data(), &v); err != nil {
			slog.ErrorContext(ctx, "jetstream subscriber: decode failed, terminating message",
				slog.String("nats", msg.Subject()),
				slog.Any("error", err),
			)
			_ = msg.Term()

			return
		}

		// Extract the producer's span context as a link (not parent).
		// Async messaging means the consumer is temporally decoupled from
		// the producer — a span link preserves causality without implying
		// the producer is waiting for the consumer.
		remoteSpanCtx := s.tel.ExtractSpanContext(ctx, jetstreamHeaderCarrier{Headers: msg.Headers()})

		msgCtx := ctx
		if id := msg.Headers().Get(goflux.MessageIDHeader); id != "" {
			msgCtx = goflux.WithMessageID(msgCtx, id)
		}

		m := goflux.Message[T]{
			Subject: msg.Subject(),
			Payload: v,
			Header:  extractGofluxHeaders(msg.Headers()),
		}.WithAcker(&jsAcker{msg: msg})

		if err := s.tel.RecordProcess(msgCtx, subject, system, func(ctx context.Context) error {
			trace.SpanFromContext(ctx).SetAttributes(
				attribute.Int("messaging.message.body.size", len(msg.Data())),
				attribute.String("messaging.operation.type", "process"),
			)

			return handler(ctx, m)
		}, goflux.WithRemoteSpanContext(remoteSpanCtx)); err != nil {
			slog.ErrorContext(msgCtx, "jetstream subscriber: handler error",
				slog.String("nats", subject),
				slog.Any("error", err),
			)

			if !s.manualAck {
				_ = msg.Nak()
			}

			return
		}

		if !s.manualAck {
			_ = msg.Ack()
		}
	})
	if err != nil {
		return errors.Join(goflux.ErrSubscribe, goflux.ErrTransport, fmt.Errorf("jetstream: %w", err))
	}

	<-ctx.Done()

	cc.Stop()

	return nil
}

// Close is a no-op. The caller owns the jetstream.Consumer and the
// underlying *nats.Conn.
func (s *Subscriber[T]) Close() error { return nil }
