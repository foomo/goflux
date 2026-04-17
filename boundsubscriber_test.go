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
// the subject. Callers only need to provide the handler — the subject is
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

	// BindSubscriber fixes the subject to "orders".
	bound := goflux.BindSubscriber[Event](sub, "orders")

	gofuncy.StartWithReady(ctx, func(ctx context.Context, ready gofuncy.ReadyFunc) error {
		ready()

		// The subject argument is ignored — "orders" is always used.
		return bound.Subscribe(ctx, "", func(_ context.Context, msg goflux.Message[Event]) error {
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
