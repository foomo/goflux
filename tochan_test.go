package goflux_test

import (
	"context"
	"fmt"
	"time"

	"github.com/foomo/goflux"
	"github.com/foomo/goflux/transport/channel"
)

// ExampleToChan demonstrates bridging a Subscriber into a plain channel.
// Messages can be consumed with a range loop instead of a callback handler.
func ExampleToChan() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	bus := channel.NewBus[Event]()
	pub := channel.NewPublisher(bus)

	sub, err := channel.NewSubscriber(bus, 1)
	if err != nil {
		panic(err)
	}

	ch := goflux.ToChan[Event](ctx, sub, "events", 4)

	// Allow subscriber to register.
	time.Sleep(10 * time.Millisecond)

	_ = pub.Publish(ctx, "events", Event{ID: "1", Name: "alpha"})
	_ = pub.Publish(ctx, "events", Event{ID: "2", Name: "bravo"})

	// Read two messages from the channel.
	fmt.Println((<-ch).Payload.Name)
	fmt.Println((<-ch).Payload.Name)
	// Output:
	// alpha
	// bravo
}
