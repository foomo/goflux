package bridge_test

import (
	"context"
	"fmt"
	"time"

	"github.com/foomo/goflux/bridge"
	"github.com/foomo/goflux/transport/channel"
)

type Event struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

func ExampleToStream() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	bus := channel.NewBus[Event]()
	pub := channel.NewPublisher(bus)

	sub, _ := channel.NewSubscriber(bus, 1)

	stream := bridge.ToStream[Event](ctx, sub, "events", 4)

	go func() {
		time.Sleep(50 * time.Millisecond)

		_ = pub.Publish(ctx, "events", Event{ID: "1", Name: "alpha"})
		_ = pub.Publish(ctx, "events", Event{ID: "2", Name: "bravo"})
	}()

	ch := stream.Chan()

	fmt.Println((<-ch).Payload.Name)
	fmt.Println((<-ch).Payload.Name)
	// Output:
	// alpha
	// bravo
}
