package goflux_test

import (
	"context"
	"fmt"
	"time"

	"github.com/foomo/goflux"
)

// ExampleThrottle demonstrates rate-limiting handler calls to at most one
// per 50ms. The first message passes immediately; the second waits.
func ExampleThrottle() {
	var received []string

	handler := goflux.Chain[Event](
		goflux.Throttle[Event](50 * time.Millisecond),
	)(func(_ context.Context, msg goflux.Message[Event]) error {
		received = append(received, msg.Payload.Name)
		return nil
	})

	ctx := context.Background()
	start := time.Now()

	_ = handler(ctx, goflux.NewMessage("events", Event{ID: "1", Name: "first"}))
	_ = handler(ctx, goflux.NewMessage("events", Event{ID: "2", Name: "second"}))

	elapsed := time.Since(start)

	fmt.Println("messages:", received)
	fmt.Println("waited:", elapsed >= 50*time.Millisecond)
	// Output:
	// messages: [first second]
	// waited: true
}
