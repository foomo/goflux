package goflux_test

import (
	"context"
	"fmt"
	"time"

	"github.com/foomo/goflux"
	"github.com/foomo/goflux/transport/channel"
	"github.com/foomo/gofuncy"
)

// ExampleBindSubscriber demonstrates creating a BoundSubscriber that fixes
// the nats. Callers only need to provide the handler — the nats is
// always "orders".
func ExampleBindSubscriber() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	bus := channel.NewBus[Event]()
	pub := channel.NewPublisher(bus)

	sub, err := channel.NewSubscriber(bus, 1)
	if err != nil {
		panic(err)
	}

	// BindSubscriber fixes the nats to "orders".
	bound := goflux.BindSubscriber[Event](sub, "orders")

	gofuncy.StartWithReady(ctx, func(ctx context.Context, ready gofuncy.ReadyFunc) error {
		ready()

		// No nats argument — bound subscriber always uses "orders".
		return bound.Subscribe(ctx, func(_ context.Context, msg goflux.Message[Event]) error {
			fmt.Println(msg.Subject, msg.Payload.Name)
			cancel()

			return nil
		})
	}, gofuncy.WithName("subscriber"))

	// Allow subscriber to register.
	time.Sleep(10 * time.Millisecond)

	if err := pub.Publish(ctx, "orders", Event{ID: "1", Name: "widget"}); err != nil {
		panic(err)
	}

	<-ctx.Done()
	// Output: orders widget
}
