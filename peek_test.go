package goflux_test

import (
	"context"
	"fmt"

	"github.com/foomo/goflux"
)

// ExamplePeek demonstrates observing messages as a side-effect without
// modifying them.
func ExamplePeek() {
	var (
		peeked  []string
		handled []string
	)

	handler := goflux.Chain[Event](
		goflux.Peek[Event](func(_ context.Context, msg goflux.Message[Event]) {
			peeked = append(peeked, "peek:"+msg.Payload.Name)
		}),
	)(func(_ context.Context, msg goflux.Message[Event]) error {
		handled = append(handled, "handle:"+msg.Payload.Name)
		return nil
	})

	ctx := context.Background()
	_ = handler(ctx, goflux.NewMessage("events", Event{ID: "1", Name: "hello"}))

	fmt.Println(peeked)
	fmt.Println(handled)
	// Output:
	// [peek:hello]
	// [handle:hello]
}
