package jetstream

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	"github.com/foomo/goencode"
	"github.com/foomo/goflux"
	"github.com/nats-io/nats.go/jetstream"
	semconv "go.opentelemetry.io/otel/semconv/v1.41.0"
	"go.opentelemetry.io/otel/trace"
)

// maxLoggedErrors caps the distinct consume error texts remembered for log
// deduplication between deliveries.
const maxLoggedErrors = 16

// Subscriber decodes and dispatches messages pulled from a JetStream
// consumer, with at-least-once delivery via ack/nak/term.
type Subscriber[T any] struct {
	consumer    jetstream.Consumer
	decoder     goencode.Decoder[T, []byte]
	tel         *goflux.Telemetry
	manualAck   bool
	consumeOpts []jetstream.PullConsumeOpt
	errHandler  jetstream.ConsumeErrHandlerFunc
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
		errHandler:  cfg.consumeErrHandler,
	}
}

// Subscribe starts consuming messages and dispatching them to handler. The call
// blocks until ctx is cancelled or consumption stops; see
// [Subscriber.SubscribeWithReady].
//
// Note: the subject parameter is used only for telemetry labeling. Actual
// message filtering is determined by the jetstream.Consumer configuration
// passed to [NewSubscriber].
func (s *Subscriber[T]) Subscribe(ctx context.Context, subject string, handler goflux.Handler[T]) error {
	return s.SubscribeWithReady(ctx, subject, handler, func() {})
}

// SubscribeWithReady behaves like [Subscriber.Subscribe] but invokes ready once
// the consumer is established, then blocks until ctx is cancelled. On ctx
// cancellation it stops the consumer and waits for the in-flight handler to
// return before returning nil, so no handler runs after it returns. A handler
// that ignores ctx therefore delays the return until it finishes.
//
// Transient consume errors, such as missing heartbeats while the consumer does
// not exist on the server (deleted, or lost in a server restart), are logged
// via slog and consumption keeps retrying every pull expiry
// ([DefaultPullExpiry]), so a recreated consumer is picked up again. Each
// distinct error is logged at warn level once; repeats are logged at debug
// level until a message is delivered.
//
// If the server reports the consumer as deleted while a pull is pending,
// nats.go stops consuming; SubscribeWithReady restarts consumption on the same
// durable consumer, so a recreated consumer is picked up in both cases. An
// ephemeral consumer cannot be recreated under its name, so its deletion
// returns an error wrapping [goflux.ErrSubscribe]. A deletion without a
// pending pull (for example during a connection outage, or while waiting to
// retry a rejected pull) is not reported and keeps retrying. If the server
// rejects a pull request (jetstream.ErrBadRequest), it logs an error and
// restarts after one pull expiry ([DefaultPullExpiry] unless set through
// [WithConsumeOpts]). It does not return, because all replicas
// would hit the same rejection and stop together. Any other stop keeps
// blocking until ctx is cancelled: the *nats.Conn being closed (the caller
// owns it; use nats.ClosedHandler to detect a connection that gave up
// reconnecting), jetstream.StopAfter, or a custom error handler stopping the
// consume context. Use [WithConsumeOpts] to tune pull timing and
// [WithConsumeErrHandler] to observe consume errors; replacing the error
// handler through [WithConsumeOpts] disables these restarts.
//
// After consumption has started, SubscribeWithReady returns an error only when
// an ephemeral consumer is deleted. Otherwise an error wrapping
// [goflux.ErrSubscribe] means Consume could not start, for example because of
// invalid options.
//
// jetstream.Consumer.Consume sets up the pull consumer synchronously, so the
// subscription exists once it returns without error. JetStream also persists
// messages, so a publish that precedes the consumer is delivered rather than
// dropped — ready is provided for interface uniformity, not to fix a loss
// window.
func (s *Subscriber[T]) SubscribeWithReady(ctx context.Context, subject string, handler goflux.Handler[T], ready func()) error {
	// Derive the consumer group name and type from cached consumer info. A
	// consumer from nats.go always caches it; ask the server once for
	// consumers that do not, such as wrappers, so a deleted ephemeral
	// consumer is never mistaken for a durable one and restarted forever.
	info := s.consumer.CachedInfo()
	if info == nil {
		var err error
		if info, err = s.consumer.Info(ctx); err != nil {
			return errors.Join(goflux.ErrSubscribe, goflux.ErrTransport, fmt.Errorf("jetstream: consumer info: %w", err))
		}
	}

	// The server treats every consumer without a durable name as ephemeral
	// and deletes it after its inactive threshold. A durable name is
	// preferred as the group name; fall back to the ephemeral name.
	ephemeral := info.Config.Durable == ""

	consumerGroup := info.Config.Durable
	if ephemeral {
		consumerGroup = info.Name
	}

	var (
		// stopErr is the terminal server error that made nats.go stop
		// consuming. nats.go reports it to the error handler before closing
		// the consume context, so it is set by the time Closed fires.
		stopErr atomic.Pointer[error]
		// logged holds the texts of consume errors logged since the last
		// delivery, so repeats are logged at debug level. A missing consumer
		// alternates between errors, so the last error alone is not enough.
		// hasLogged lets the delivery path skip the lock while it is empty.
		logMu     sync.Mutex
		logged    = map[string]struct{}{}
		hasLogged atomic.Bool
	)

	errHandler := jetstream.ConsumeErrHandler(func(consumeCtx jetstream.ConsumeContext, err error) {
		if errors.Is(err, jetstream.ErrConsumerDeleted) || errors.Is(err, jetstream.ErrBadRequest) {
			stopErr.Store(&err)
		}

		level := slog.LevelWarn
		msg := err.Error()

		logMu.Lock()
		if _, ok := logged[msg]; ok {
			level = slog.LevelDebug
		} else {
			if len(logged) >= maxLoggedErrors {
				clear(logged)
			}

			logged[msg] = struct{}{}
		}

		hasLogged.Store(true)
		logMu.Unlock()

		slog.Log(ctx, level, "jetstream subscriber: consume error",
			slog.String("subject", subject),
			slog.String("consumer", consumerGroup),
			slog.Any("error", err),
		)

		if s.errHandler != nil {
			s.errHandler(consumeCtx, err)
		}
	})

	msgHandler := func(msg jetstream.Msg) {
		if hasLogged.Load() {
			logMu.Lock()
			clear(logged)
			hasLogged.Store(false)
			logMu.Unlock()
		}

		var v T
		if err := s.decoder(msg.Data(), &v); err != nil {
			slog.ErrorContext(ctx, "jetstream subscriber: decode failed, terminating message",
				slog.String("subject", msg.Subject()),
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

		m := goflux.Message[T]{
			Subject: msg.Subject(),
			Payload: v,
			Header:  extractGofluxHeaders(msg.Headers()),
			ID:      msg.Headers().Get(goflux.MessageIDHeader),
		}.WithAcker(&jsAcker{msg: msg})

		if err := s.tel.RecordProcess(ctx, subject, system, func(ctx context.Context) error {
			sp := trace.SpanFromContext(ctx)
			sp.SetAttributes(
				semconv.MessagingMessageBodySize(len(msg.Data())),
				semconv.MessagingOperationTypeProcess,
			)

			if consumerGroup != "" {
				sp.SetAttributes(semconv.MessagingConsumerGroupName(consumerGroup))
			}

			return handler(ctx, m)
		}, goflux.WithRemoteSpanContext(remoteSpanCtx), goflux.WithProcessMessageID(m.ID)); err != nil {
			slog.ErrorContext(ctx, "jetstream subscriber: handler error",
				slog.String("subject", subject),
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
	}

	opts := append([]jetstream.PullConsumeOpt{jetstream.PullExpiry(DefaultPullExpiry), errHandler}, s.consumeOpts...)
	consume := func() (jetstream.ConsumeContext, error) {
		cc, err := s.consumer.Consume(msgHandler, opts...)
		if errors.Is(err, jetstream.ErrInvalidOption) && len(s.consumeOpts) > 0 {
			// The caller's options may conflict with DefaultPullExpiry, for
			// example a PullHeartbeat of more than half of it. Let nats.go
			// derive the expiry, and keep doing so on restarts if that works.
			fallback := append([]jetstream.PullConsumeOpt{errHandler}, s.consumeOpts...)
			if cc, err = s.consumer.Consume(msgHandler, fallback...); err == nil {
				opts = fallback
			}
		}

		if err != nil {
			return nil, errors.Join(goflux.ErrSubscribe, goflux.ErrTransport, fmt.Errorf("jetstream: %w", err))
		}

		return cc, nil
	}

	cc, err := consume()
	if err != nil {
		return err
	}

	ready()

	for {
		select {
		case <-ctx.Done():
			cc.Stop()
			<-cc.Closed()

			return nil
		case <-cc.Closed():
		}

		cause := stopErr.Swap(nil)

		switch {
		case ctx.Err() != nil:
			return nil //nolint:nilerr // ctx cancellation ends the subscription cleanly
		case cause != nil && errors.Is(*cause, jetstream.ErrConsumerDeleted) && ephemeral:
			// An ephemeral consumer cannot be recreated under the same name,
			// so restarting would retry forever. Return, so the caller can
			// create a new consumer.
			return errors.Join(goflux.ErrSubscribe, goflux.ErrTransport, fmt.Errorf("jetstream: ephemeral consumer deleted: %w", *cause))
		case cause != nil && errors.Is(*cause, jetstream.ErrConsumerDeleted):
			// nats.go stops consuming when the deletion hits a pending pull.
			// Consume makes no server call, so restarting it puts the
			// subscriber back on the retry path it takes when no pull was
			// pending: pulls fail until the consumer is recreated (for example
			// by NACK), then delivery resumes. The error handler already
			// logged the deletion at warn level, deduplicated.
			slog.DebugContext(ctx, "jetstream subscriber: consumer deleted, restarting consume",
				slog.String("subject", subject),
				slog.String("consumer", consumerGroup),
			)

		case cause != nil:
			// The server rejected a pull request (jetstream.ErrBadRequest).
			// Every replica sends the same requests, so returning would stop
			// all of them at once. Retry after a pull expiry instead, so
			// consumption resumes once the server or consumer config is fixed.
			delay := pullExpiry(opts)
			slog.ErrorContext(ctx, "jetstream subscriber: pull request rejected, restarting consume",
				slog.String("subject", subject),
				slog.String("consumer", consumerGroup),
				slog.Duration("delay", delay),
				slog.Any("error", *cause),
			)

			select {
			case <-ctx.Done():
				return nil
			case <-time.After(delay):
			}

		default:
			// The connection was closed (the caller owns it and can observe
			// that via nats.ClosedHandler), or the caller stopped consumption
			// through jetstream.StopAfter or a custom error handler. Keep the
			// contract of blocking until ctx is cancelled.
			<-ctx.Done()

			return nil
		}

		// Only the restart cases reach this point.
		if cc, err = consume(); err != nil {
			return err
		}
	}
}

// pullExpiry returns the pull expiry nats.go uses for opts: the last
// jetstream.PullExpiry option, or jetstream.DefaultExpires without one.
func pullExpiry(opts []jetstream.PullConsumeOpt) time.Duration {
	d := jetstream.DefaultExpires

	for _, o := range opts {
		if e, ok := o.(jetstream.PullExpiry); ok {
			d = time.Duration(e)
		}
	}

	return d
}

// Close is a no-op. The caller owns the jetstream.Consumer and the
// underlying *nats.Conn.
func (s *Subscriber[T]) Close() error { return nil }
