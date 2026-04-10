package channel_test

import (
	"context"
	"fmt"

	"github.com/foomo/goflux"
	"github.com/foomo/goflux/pkg/channel"
	"github.com/foomo/gofuncy"
)

func ExampleNewPublisher() {
	bus := channel.NewBus[Event]()

	sub, err := channel.NewSubscriber(bus, 1)
	if err != nil {
		panic(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	pub := channel.NewPublisher(bus)

	gofuncy.Start(ctx, func(ctx context.Context) error {
		return sub.Subscribe(ctx, "events", func(_ context.Context, msg goflux.Message[Event]) error {
			fmt.Println(msg)
			cancel()

			return nil
		})
	}, gofuncy.WithName("subscriber"))

	if err := pub.Publish(ctx, "events", Event{ID: "1", Name: "foo"}); err != nil {
		panic(err)
	}

	<-ctx.Done()
	// Output: {events {1 foo}}
}
