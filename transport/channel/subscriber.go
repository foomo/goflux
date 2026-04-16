package channel

import (
	"context"
	"fmt"
	"log/slog"
	"sync"

	"github.com/foomo/goflux"
)

type Subscriber[T any] struct {
	bus     *Bus[T]
	bufSize int
	tel     *goflux.Telemetry
	mu      sync.RWMutex
	ch      chan goflux.Message[T]
}

func NewSubscriber[T any](bus *Bus[T], bufSize int, opts ...Option) (*Subscriber[T], error) {
	cfg := applyOpts(opts)

	s := &Subscriber[T]{bus: bus, bufSize: bufSize, tel: cfg.tel}
	if _, err := s.tel.RegisterLag("go_channel", s.Len); err != nil {
		return nil, fmt.Errorf("channel subscriber: register lag gauge: %w", err)
	}

	return s, nil
}

func (s *Subscriber[T]) Len() int64 {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.ch == nil {
		return 0
	}

	return int64(len(s.ch))
}

func (s *Subscriber[T]) Subscribe(ctx context.Context, subject string, handler goflux.Handler[T]) error {
	ch := make(chan goflux.Message[T], s.bufSize)
	s.mu.Lock()
	s.ch = ch
	s.mu.Unlock()
	s.bus.subscribe(subject, ch)

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
					slog.String("subject", subject),
					slog.Any("error", err),
				)
			}
		case <-ctx.Done():
			return nil
		}
	}
}

func (s *Subscriber[T]) Close() error { return nil }
