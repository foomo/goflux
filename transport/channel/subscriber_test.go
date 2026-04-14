package channel_test

import (
	"context"
	"fmt"

	"github.com/foomo/goflux"
	"github.com/foomo/goflux/transport/channel"
	"github.com/foomo/gofuncy"
)

func ExampleNewSubscriber() {
	bus := channel.NewBus[Event]()

	sub, err := channel.NewSubscriber(bus, 1)
	if err != nil {
		panic(err)
	}

	ctx, cancel := context.WithCancel(context.Background())

	gofuncy.StartWithReady(ctx, func(ctx context.Context, ready gofuncy.ReadyFunc) error {
		ready()

		return sub.Subscribe(ctx, "events", func(_ context.Context, msg goflux.Message[Event]) error {
			fmt.Println(msg.Subject, msg.Payload)
			cancel()

			return nil
		})
	}, gofuncy.WithName("subscriber"))

	pub := channel.NewPublisher(bus)
	if err := pub.Publish(ctx, "events", Event{ID: "2", Name: "bar"}); err != nil {
		panic(err)
	}

	<-ctx.Done()
	// Output: events {2 bar}
}
