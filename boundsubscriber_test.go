package goflux_test

import (
	"context"
	"fmt"

	"github.com/foomo/goflux"
	"github.com/foomo/goflux/transport/channel"
	"github.com/foomo/gofuncy"
)

// ExampleBindSubscriber demonstrates creating a BoundSubscriber that fixes
// the subject. Callers only need to provide the handler — the subject is
// always "orders".
func ExampleBindSubscriber() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	bus := channel.NewBus[Event]()
	pub := channel.NewPublisher(bus)

	sub := channel.NewSubscriber(bus, 1)

	// BindSubscriber fixes the subject to "orders".
	bound := goflux.BindSubscriber[Event](sub, "orders")

	gofuncy.StartWithReady(ctx, func(ctx context.Context, ready gofuncy.ReadyFunc) error {
		// No subject argument — bound subscriber always uses "orders".
		return bound.SubscribeWithReady(ctx, func(_ context.Context, msg goflux.Message[Event]) error {
			fmt.Println(msg.Subject, msg.Payload.Name)
			cancel()

			return nil
		}, ready)
	}, gofuncy.WithName("subscriber"))

	if err := pub.Publish(ctx, "orders", Event{ID: "1", Name: "widget"}); err != nil {
		panic(err)
	}

	<-ctx.Done()
	// Output: orders widget
}
