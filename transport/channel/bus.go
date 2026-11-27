package channel

import (
	"context"
	"sync"

	"github.com/foomo/goflux"
	"go.opentelemetry.io/otel/trace"
)

// envelope carries a message through the bus together with the publisher's
// context values that other transports send as wire headers.
type envelope[T any] struct {
	msg     goflux.Message[T]
	spanCtx trace.SpanContext
}

// Bus is a simple in-process pub/sub broker. Subjects are matched by exact
// string equality. A Bus must be created with NewBus and is safe for
// concurrent use.
type Bus[T any] struct {
	mu          sync.RWMutex
	subscribers map[string][]chan envelope[T]
}

// NewBus creates a Bus for type T.
func NewBus[T any]() *Bus[T] {
	return &Bus[T]{subscribers: make(map[string][]chan envelope[T])}
}

func (b *Bus[T]) subscribe(subject string, ch chan envelope[T]) {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.subscribers[subject] = append(b.subscribers[subject], ch)
}

func (b *Bus[T]) unsubscribe(subject string, ch chan envelope[T]) {
	b.mu.Lock()
	defer b.mu.Unlock()

	subs := b.subscribers[subject]
	for i, s := range subs {
		if s == ch {
			b.subscribers[subject] = append(subs[:i], subs[i+1:]...)
			return
		}
	}
}

// publish blocks until every subscriber has accepted the message or ctx is
// cancelled. No messages are dropped; slow consumers apply backpressure to
// the publisher goroutine instead.
func (b *Bus[T]) publish(ctx context.Context, subject string, env envelope[T]) error {
	b.mu.RLock()
	subs := b.subscribers[subject]
	snapshot := make([]chan envelope[T], len(subs))
	copy(snapshot, subs)
	b.mu.RUnlock()

	for _, ch := range snapshot {
		select {
		case ch <- env:
		case <-ctx.Done():
			return ctx.Err()
		}
	}

	return nil
}
