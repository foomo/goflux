package bridge_test

import (
	"context"
	"fmt"
	"time"

	"github.com/foomo/goflow"
	"github.com/foomo/goflux"
	"github.com/foomo/goflux/bridge"
	"github.com/foomo/goflux/transport/channel"
)

func ExampleFromStream() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	dstBus := channel.NewBus[Event]()
	dstPub := channel.NewPublisher(dstBus)
	dstSub, _ := channel.NewSubscriber(dstBus, 1)

	dstCh := goflux.ToChan[Event](ctx, dstSub, "events", 4)

	time.Sleep(50 * time.Millisecond)

	msgs := []goflux.Message[Event]{
		goflux.NewMessage("events", Event{ID: "1", Name: "alpha"}),
		goflux.NewMessage("events", Event{ID: "2", Name: "bravo"}),
	}
	stream := goflow.Of(ctx, msgs...)

	_ = bridge.FromStream(stream, dstPub)

	fmt.Println((<-dstCh).Payload.Name)
	fmt.Println((<-dstCh).Payload.Name)
	// Output:
	// alpha
	// bravo
}
