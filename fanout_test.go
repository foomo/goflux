package goflux_test

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/foomo/goflux"
)

// collectPublisher records every published message for later inspection.
type collectPublisher[T any] struct {
	mu   sync.Mutex
	msgs []T
}

func (p *collectPublisher[T]) Publish(_ context.Context, _ string, v T) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.msgs = append(p.msgs, v)

	return nil
}

func (p *collectPublisher[T]) Close() error { return nil }

func (p *collectPublisher[T]) get() []T {
	p.mu.Lock()
	defer p.mu.Unlock()

	return append([]T(nil), p.msgs...)
}

// ExampleFanOut demonstrates broadcasting a message to multiple publishers.
func ExampleFanOut() {
	ctx := context.Background()

	a := &collectPublisher[Event]{}
	b := &collectPublisher[Event]{}

	pub := goflux.FanOut[Event]([]goflux.Publisher[Event]{a, b})

	if err := pub.Publish(ctx, "events", Event{ID: "1", Name: "hello"}); err != nil {
		panic(err)
	}

	fmt.Println("a:", a.get())
	fmt.Println("b:", b.get())
	// Output:
	// a: [{1 hello}]
	// b: [{1 hello}]
}

// ExampleFanOut_bestEffort shows that best-effort mode publishes to all
// publishers even when some fail, and joins the errors.
func ExampleFanOut_bestEffort() {
	ctx := context.Background()

	good := &collectPublisher[Event]{}
	bad := &failPublisher[Event]{err: errors.New("down")}

	pub := goflux.FanOut[Event]([]goflux.Publisher[Event]{bad, good})

	err := pub.Publish(ctx, "events", Event{ID: "1", Name: "hello"})
	fmt.Println("error:", err)
	fmt.Println("good received:", good.get())
	// Output:
	// error: down
	// good received: [{1 hello}]
}

// ExampleFanOut_allOrNothing shows that all-or-nothing mode returns the
// first error immediately without publishing to remaining publishers.
func ExampleFanOut_allOrNothing() {
	ctx := context.Background()

	bad := &failPublisher[Event]{err: errors.New("down")}
	good := &collectPublisher[Event]{}

	pub := goflux.FanOut[Event](
		[]goflux.Publisher[Event]{bad, good},
		goflux.WithFanOutAllOrNothing[Event](),
	)

	err := pub.Publish(ctx, "events", Event{ID: "1", Name: "hello"})
	fmt.Println("error:", err)
	fmt.Println("good received:", good.get())
	// Output:
	// error: down
	// good received: []
}
