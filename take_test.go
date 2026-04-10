package goflux_test

import (
	"context"
	"fmt"

	"github.com/foomo/goflux"
)

// ExampleTake demonstrates handling only the first 2 messages.
func ExampleTake() {
	var received []string

	handler := goflux.Chain[Event](
		goflux.Take[Event](2),
	)(func(_ context.Context, msg goflux.Message[Event]) error {
		received = append(received, msg.Payload.Name)
		return nil
	})

	ctx := context.Background()
	_ = handler(ctx, goflux.NewMessage("events", Event{ID: "1", Name: "first"}))
	_ = handler(ctx, goflux.NewMessage("events", Event{ID: "2", Name: "second"}))
	_ = handler(ctx, goflux.NewMessage("events", Event{ID: "3", Name: "third"}))

	for _, r := range received {
		fmt.Println(r)
	}
	// Output:
	// first
	// second
}
