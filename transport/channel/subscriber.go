package channel

import (
	"context"
	"log/slog"
	"sync"

	"github.com/foomo/goflux"
)

// Subscriber subscribes to messages published onto a [Bus].
type Subscriber[T any] struct {
	bus     *Bus[T]
	bufSize int
	tel     *goflux.Telemetry
	mu      sync.RWMutex
	ch      chan envelope[T]
}

// NewSubscriber returns a Subscriber that reads from bus, buffering up to
// bufSize messages before Subscribe applies backpressure.
func NewSubscriber[T any](bus *Bus[T], bufSize int, opts ...Option) *Subscriber[T] {
	cfg := applyOpts(opts)

	return &Subscriber[T]{bus: bus, bufSize: bufSize, tel: cfg.tel}
}

// Len reports the number of messages currently buffered, or 0 if no
// subscription is active.
func (s *Subscriber[T]) Len() int64 {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.ch == nil {
		return 0
	}

	return int64(len(s.ch))
}

// Subscribe registers handler for subject. The call blocks until ctx is
// cancelled.
func (s *Subscriber[T]) Subscribe(ctx context.Context, subject string, handler goflux.Handler[T]) error {
	return s.SubscribeWithReady(ctx, subject, handler, func() {})
}

// SubscribeWithReady behaves like [Subscriber.Subscribe] but invokes ready once
// the channel is registered on the bus, then blocks until ctx is cancelled.
// While it runs, the goflux.consumer.lag gauge reports the buffered messages
// for subject.
//
// Each handler call gets the publisher's message ID as [goflux.Message.ID] (not
// in its context), and its process span links to the publish span.
//
// Bus registration is an in-process, mutex-guarded operation, so it is already
// complete when it returns; ready exists to satisfy [goflux.ReadySubscriber] and
// let callers sequence a publish after a subscribe without knowing the
// transport.
func (s *Subscriber[T]) SubscribeWithReady(ctx context.Context, subject string, handler goflux.Handler[T], ready func()) error {
	ch := make(chan envelope[T], s.bufSize)

	// Report this subscription's own buffer: s.ch only holds the latest one.
	unregisterLag := s.tel.RegisterLag(subject, func() int64 { return int64(len(ch)) })

	s.mu.Lock()
	s.ch = ch
	s.mu.Unlock()
	s.bus.subscribe(subject, ch)

	ready()

	defer func() {
		s.bus.unsubscribe(subject, ch)
		s.mu.Lock()
		if s.ch == ch {
			s.ch = nil
		}
		s.mu.Unlock()

		unregisterLag()
	}()

	for {
		select {
		case env := <-ch:
			err := s.tel.RecordProcess(ctx, subject, system, func(ctx context.Context) error {
				return handler(ctx, env.msg)
			}, goflux.WithRemoteSpanContext(env.spanCtx), goflux.WithProcessMessageID(env.msg.ID))
			if err != nil {
				slog.ErrorContext(ctx, "channel subscriber: handler error",
					slog.String("subject", subject),
					slog.Any("error", err),
				)
			}
		case <-ctx.Done():
			return nil
		}
	}
}

// Close is a no-op; the caller owns the Bus.
func (s *Subscriber[T]) Close() error { return nil }
