package goflux_test

import (
	"context"
	"fmt"

	"github.com/foomo/goflux"
)

// ExampleDistinct demonstrates deduplicating messages by ID.
// The second message with ID "1" is silently dropped.
func ExampleDistinct() {
	var received []string

	handler := goflux.Chain[Event](
		goflux.Distinct[Event](func(msg goflux.Message[Event]) string {
			return msg.Payload.ID
		}),
	)(func(_ context.Context, msg goflux.Message[Event]) error {
		received = append(received, msg.Payload.Name)
		return nil
	})

	ctx := context.Background()
	_ = handler(ctx, goflux.NewMessage("events", Event{ID: "1", Name: "first"}))
	_ = handler(ctx, goflux.NewMessage("events", Event{ID: "1", Name: "duplicate"}))
	_ = handler(ctx, goflux.NewMessage("events", Event{ID: "2", Name: "second"}))

	for _, r := range received {
		fmt.Println(r)
	}
	// Output:
	// first
	// second
}
