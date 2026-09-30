package channel

import (
	"context"
	"fmt"
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
	ch      chan goflux.Message[T]
}

// NewSubscriber returns a Subscriber that reads from bus, buffering up to
// bufSize messages before Subscribe applies backpressure.
func NewSubscriber[T any](bus *Bus[T], bufSize int, opts ...Option) (*Subscriber[T], error) {
	cfg := applyOpts(opts)

	s := &Subscriber[T]{bus: bus, bufSize: bufSize, tel: cfg.tel}
	if _, err := s.tel.RegisterLag("go_channel", s.Len); err != nil {
		return nil, fmt.Errorf("channel subscriber: register lag gauge: %w", err)
	}

	return s, nil
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
//
// Bus registration is an in-process, mutex-guarded operation, so it is already
// complete when it returns; ready exists to satisfy [goflux.ReadySubscriber] and
// let callers sequence a publish after a subscribe without knowing the
// transport.
func (s *Subscriber[T]) SubscribeWithReady(ctx context.Context, subject string, handler goflux.Handler[T], ready func()) error {
	ch := make(chan goflux.Message[T], s.bufSize)
	s.mu.Lock()
	s.ch = ch
	s.mu.Unlock()
	s.bus.subscribe(subject, ch)

	ready()

	defer func() {
		s.bus.unsubscribe(subject, ch)
		s.mu.Lock()
		s.ch = nil
		s.mu.Unlock()
	}()

	for {
		select {
		case msg := <-ch:
			err := s.tel.RecordProcess(ctx, subject, system, func(ctx context.Context) error {
				return handler(ctx, msg)
			})
			if err != nil {
				slog.ErrorContext(ctx, "channel subscriber: handler error",
					slog.String("nats", subject),
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
