package nats

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/foomo/goencode"
	"github.com/foomo/goflux"
	"github.com/nats-io/nats.go"
	semconv "go.opentelemetry.io/otel/semconv/v1.41.0"
	"go.opentelemetry.io/otel/trace"
)

type Subscriber[T any] struct {
	conn       *nats.Conn
	decoder    goencode.Decoder[T, []byte]
	tel        *goflux.Telemetry
	queueGroup string
}

func NewSubscriber[T any](conn *nats.Conn, decoder goencode.Decoder[T, []byte], opts ...Option) *Subscriber[T] {
	cfg := applyOpts(opts)

	return &Subscriber[T]{conn: conn, decoder: decoder, tel: cfg.tel, queueGroup: cfg.queueGroup}
}

// Subscribe registers handler for the subject. The call blocks until ctx is
// cancelled.
//
// Subscribe does not report when the subscription reaches the server. Callers
// that publish to the same subject immediately afterwards should use
// [Subscriber.SubscribeWithReady] instead — core NATS drops messages that have
// no matching subscriber, silently and without error on either side.
func (s *Subscriber[T]) Subscribe(ctx context.Context, subject string, handler goflux.Handler[T]) error {
	return s.SubscribeWithReady(ctx, subject, handler, func() {})
}

// SubscribeWithReady registers handler and invokes ready once the server has
// acknowledged the subscription, then blocks until ctx is cancelled.
//
// nats.Conn.Subscribe only buffers the SUB protocol frame client-side, so its
// return proves nothing about server-side state. A Flush round-trip forces the
// frame out and waits for the server to process it, which is what makes ready
// meaningful.
func (s *Subscriber[T]) SubscribeWithReady(ctx context.Context, subject string, handler goflux.Handler[T], ready func()) error {
	cb := func(msg *nats.Msg) {
		var v T
		if err := s.decoder(msg.Data, &v); err != nil {
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
			sp := trace.SpanFromContext(ctx)
			sp.SetAttributes(
				semconv.MessagingMessageBodySize(len(msg.Data)),
				semconv.MessagingOperationTypeProcess,
			)

			if s.queueGroup != "" {
				sp.SetAttributes(semconv.MessagingConsumerGroupName(s.queueGroup))
			}

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

	// Force the buffered SUB frame to the server and wait for it to be
	// processed. Until this returns, the subscription does not exist as far as
	// the server is concerned and matching messages are discarded.
	//
	// Flush, not FlushWithContext: the latter rejects a context without a
	// deadline, and ctx here is the subscription's lifetime context, which
	// legitimately has none. Flush bounds itself by the connection timeout.
	if err := s.conn.Flush(); err != nil {
		_ = sub.Unsubscribe()

		return errors.Join(goflux.ErrSubscribe, goflux.ErrTransport, fmt.Errorf("nats: flush: %w", err))
	}

	ready()

	<-ctx.Done()

	return sub.Unsubscribe()
}

// Close is a no-op. The caller owns the *nats.Conn and is responsible for
// draining or closing it.
func (s *Subscriber[T]) Close() error { return nil }
