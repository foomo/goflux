package jetstream

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync/atomic"

	"github.com/foomo/goencode"
	"github.com/foomo/goflux"
	"github.com/nats-io/nats.go/jetstream"
	semconv "go.opentelemetry.io/otel/semconv/v1.41.0"
	"go.opentelemetry.io/otel/trace"
)

// Subscriber decodes and dispatches messages pulled from a JetStream
// consumer, with at-least-once delivery via ack/nak/term.
type Subscriber[T any] struct {
	consumer    jetstream.Consumer
	decoder     goencode.Decoder[T, []byte]
	tel         *goflux.Telemetry
	manualAck   bool
	consumeOpts []jetstream.PullConsumeOpt
}

// NewSubscriber returns a Subscriber that pulls from consumer, decoding each
// message with decoder. By default the handler's error decides ack/nak; pass
// [WithManualAck] to ack/nak from within the handler via [goflux.Message.Ack]
// and friends.
func NewSubscriber[T any](consumer jetstream.Consumer, decoder goencode.Decoder[T, []byte], opts ...Option) *Subscriber[T] {
	cfg := applyOpts(opts)

	return &Subscriber[T]{
		consumer:    consumer,
		decoder:     decoder,
		tel:         cfg.tel,
		manualAck:   cfg.manualAck,
		consumeOpts: cfg.consumeOpts,
	}
}

// Subscribe starts consuming messages and dispatching them to handler. The call
// blocks until ctx is cancelled or consumption stops; see
// [Subscriber.SubscribeWithReady].
//
// Note: the nats parameter is used only for telemetry labeling. Actual
// message filtering is determined by the jetstream.Consumer configuration
// passed to [NewSubscriber].
func (s *Subscriber[T]) Subscribe(ctx context.Context, subject string, handler goflux.Handler[T]) error {
	return s.SubscribeWithReady(ctx, subject, handler, func() {})
}

// SubscribeWithReady behaves like [Subscriber.Subscribe] but invokes ready once
// the consumer is established, then blocks until ctx is cancelled (returning
// nil) or consumption stops on its own (returning an error).
//
// Consume errors, such as missing heartbeats while the consumer does not exist
// on the server (deleted, or lost in a server restart), are logged via slog and
// consumption keeps retrying every pull expiry ([DefaultPullExpiry]), so a
// recreated consumer is picked up again. Terminal errors, such as the
// connection being closed or the server reporting the consumer as deleted
// while a pull is pending, stop consumption; SubscribeWithReady then returns
// an error wrapping [goflux.ErrSubscribe] instead of blocking silently. Use
// [WithConsumeOpts] to tune pull timing or replace the error handler.
//
// jetstream.Consumer.Consume sets up the pull consumer synchronously, so the
// subscription exists once it returns without error. JetStream also persists
// messages, so a publish that precedes the consumer is delivered rather than
// dropped — ready is provided for interface uniformity, not to fix a loss
// window.
func (s *Subscriber[T]) SubscribeWithReady(ctx context.Context, subject string, handler goflux.Handler[T], ready func()) error {
	// Derive the consumer group name from cached consumer info (no network
	// call). A durable name is preferred; fall back to the ephemeral name.
	consumerGroup := ""
	if info := s.consumer.CachedInfo(); info != nil {
		consumerGroup = info.Config.Durable
		if consumerGroup == "" {
			consumerGroup = info.Name
		}
	}

	var lastErr atomic.Pointer[error]

	opts := append([]jetstream.PullConsumeOpt{
		jetstream.PullExpiry(DefaultPullExpiry),
		jetstream.ConsumeErrHandler(func(_ jetstream.ConsumeContext, err error) {
			lastErr.Store(&err)
			slog.WarnContext(ctx, "jetstream subscriber: consume error",
				slog.String("nats", subject),
				slog.String("consumer", consumerGroup),
				slog.Any("error", err),
			)
		}),
	}, s.consumeOpts...)

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
			sp := trace.SpanFromContext(ctx)
			sp.SetAttributes(
				semconv.MessagingMessageBodySize(len(msg.Data())),
				semconv.MessagingOperationTypeProcess,
			)

			if consumerGroup != "" {
				sp.SetAttributes(semconv.MessagingConsumerGroupName(consumerGroup))
			}

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
	}, opts...)
	if err != nil {
		return errors.Join(goflux.ErrSubscribe, goflux.ErrTransport, fmt.Errorf("jetstream: %w", err))
	}

	ready()

	select {
	case <-ctx.Done():
		cc.Stop()

		return nil
	case <-cc.Closed():
		// Best effort: nats.go may close the consume context before the
		// error handler has recorded the cause.
		cause := errors.New("unknown cause")
		if err := lastErr.Load(); err != nil {
			cause = *err
		}

		return errors.Join(goflux.ErrSubscribe, goflux.ErrTransport, fmt.Errorf("jetstream: consume stopped: %w", cause))
	}
}

// Close is a no-op. The caller owns the jetstream.Consumer and the
// underlying *nats.Conn.
func (s *Subscriber[T]) Close() error { return nil }
