package pipe_test

import (
	"context"
	"fmt"
	"time"

	"github.com/foomo/goflux"
	"github.com/foomo/goflux/pipe"
	"github.com/foomo/goflux/transport/channel"
)

// ExampleToChan demonstrates bridging a Subscriber into a plain channel.
func ExampleToChan() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	bus := channel.NewBus[Event]()
	pub := channel.NewPublisher(bus)

	sub, err := channel.NewSubscriber(bus, 1)
	if err != nil {
		panic(err)
	}

	ch := pipe.ToChan[Event](ctx, sub, "events", 4)

	time.Sleep(10 * time.Millisecond)

	_ = pub.Publish(ctx, "events", Event{ID: "1", Name: "alpha"})
	_ = pub.Publish(ctx, "events", Event{ID: "2", Name: "bravo"})

	fmt.Println((<-ch).Payload.Name)
	fmt.Println((<-ch).Payload.Name)
	// Output:
	// alpha
	// bravo
}

// ExampleBoundToChan demonstrates bridging a BoundSubscriber into a channel.
func ExampleBoundToChan() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	bus := channel.NewBus[Event]()
	pub := channel.NewPublisher(bus)

	sub, err := channel.NewSubscriber(bus, 1)
	if err != nil {
		panic(err)
	}

	boundSub := goflux.BindSubscriber[Event](sub, "events")
	ch := pipe.BoundToChan[Event](ctx, boundSub, 4)

	time.Sleep(10 * time.Millisecond)

	_ = pub.Publish(ctx, "events", Event{ID: "1", Name: "alpha"})
	_ = pub.Publish(ctx, "events", Event{ID: "2", Name: "bravo"})

	fmt.Println((<-ch).Payload.Name)
	fmt.Println((<-ch).Payload.Name)
	// Output:
	// alpha
	// bravo
}
