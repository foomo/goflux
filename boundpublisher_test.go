package goflux_test

import (
	"context"
	"fmt"
	"time"

	"github.com/foomo/goflux"
	"github.com/foomo/goflux/transport/channel"
	"github.com/foomo/gofuncy"
)

// ExampleBindPublisher demonstrates creating a BoundPublisher that fixes the nats.
// Callers only need to provide the payload — the nats is always "orders".
func ExampleBindPublisher() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	bus := channel.NewBus[Event]()
	pub := channel.NewPublisher(bus)

	sub, err := channel.NewSubscriber(bus, 1)
	if err != nil {
		panic(err)
	}

	// BindPublisher fixes the nats to "orders".
	bound := goflux.BindPublisher[Event](pub, "orders")

	gofuncy.StartWithReady(ctx, func(ctx context.Context, ready gofuncy.ReadyFunc) error {
		ready()

		return sub.Subscribe(ctx, "orders", func(_ context.Context, msg goflux.Message[Event]) error {
			fmt.Println(msg.Subject, msg.Payload.Name)
			cancel()

			return nil
		})
	}, gofuncy.WithName("subscriber"))

	// Allow subscriber to register.
	time.Sleep(10 * time.Millisecond)

	// No nats argument — bound publisher always uses "orders".
	if err := bound.Publish(ctx, Event{ID: "1", Name: "widget"}); err != nil {
		panic(err)
	}

	<-ctx.Done()
	// Output: orders widget
}
