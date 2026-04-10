package goflux_test

import (
	"context"
	"fmt"

	"github.com/foomo/goflux"
)

// ExampleChain demonstrates composing handler middlewares.
// Middlewares execute left-to-right (outermost first).
func ExampleChain() {
	var trace []string

	mwA := func(next goflux.Handler[Event]) goflux.Handler[Event] {
		return func(ctx context.Context, msg goflux.Message[Event]) error {
			trace = append(trace, "A-before")
			err := next(ctx, msg)

			trace = append(trace, "A-after")

			return err
		}
	}

	mwB := func(next goflux.Handler[Event]) goflux.Handler[Event] {
		return func(ctx context.Context, msg goflux.Message[Event]) error {
			trace = append(trace, "B-before")
			err := next(ctx, msg)

			trace = append(trace, "B-after")

			return err
		}
	}

	base := func(_ context.Context, msg goflux.Message[Event]) error {
		trace = append(trace, "handler:"+msg.Payload.Name)
		return nil
	}

	handler := goflux.Chain[Event](mwA, mwB)(base)

	msg := goflux.NewMessage("events", Event{ID: "1", Name: "hello"})
	_ = handler(context.Background(), msg)

	for _, s := range trace {
		fmt.Println(s)
	}
	// Output:
	// A-before
	// B-before
	// handler:hello
	// B-after
	// A-after
}
