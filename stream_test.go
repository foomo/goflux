package goflux_test

//
// import (
// 	"context"
// 	"fmt"
// 	"time"
//
// 	"github.com/foomo/goflux"
// 	"github.com/foomo/goflow"
// 	"github.com/foomo/goflux/pkg/channel"
// )
//
// // ExampleToChan demonstrates bridging a Subscriber into a plain channel.
// func ExampleToChan() {
// 	ctx, cancel := context.WithCancel(context.Background())
// 	defer cancel()
//
// 	bus := channel.NewBus[Event]()
// 	pub := channel.NewPublisher(bus)
//
// 	sub, err := channel.NewSubscriber(bus, 1)
// 	if err != nil {
// 		panic(err)
// 	}
//
// 	// Bridge subscriber into a channel.
// 	ch := goflux.ToChan[Event](ctx, sub, "events", 10)
//
// 	// Allow goroutine inside ToChan to register on the bus.
// 	time.Sleep(10 * time.Millisecond)
//
// 	// Publish some messages, then cancel to close the channel.
// 	_ = pub.Publish(ctx, "events", Event{ID: "1", Name: "alpha"})
// 	_ = pub.Publish(ctx, "events", Event{ID: "2", Name: "beta"})
// 	_ = pub.Publish(ctx, "events", Event{ID: "3", Name: "gamma"})
//
// 	cancel()
//
// 	for e := range ch {
// 		fmt.Println(e.ID, e.Name)
// 	}
// 	// Output:
// 	// 1 alpha
// 	// 2 beta
// 	// 3 gamma
// }
//
// // ExampleToChan_withStream demonstrates the recommended pattern for bridging
// // a Subscriber into a stream.Stream using stream.FromFunc.
// func ExampleToChan_withStream() {
// 	ctx, cancel := context.WithCancel(context.Background())
// 	defer cancel()
//
// 	bus := channel.NewBus[Event]()
// 	pub := channel.NewPublisher(bus)
//
// 	sub, err := channel.NewSubscriber(bus, 1)
// 	if err != nil {
// 		panic(err)
// 	}
//
// 	// Bridge subscriber into a Stream[Event] via FromFunc.
// 	s := goflux.FromFunc(ctx, 10, func(ctx context.Context, send func(Event) error) error {
// 		return sub.Subscribe(ctx, "events", func(ctx context.Context, msg goflux.Message[Event]) error {
// 			return send(msg.Payload)
// 		})
// 	})
//
// 	// Allow goroutine inside FromFunc to register on the bus.
// 	time.Sleep(10 * time.Millisecond)
//
// 	// Publish some messages, then cancel to close the stream.
// 	_ = pub.Publish(ctx, "events", Event{ID: "1", Name: "alpha"})
// 	_ = pub.Publish(ctx, "events", Event{ID: "2", Name: "beta"})
// 	_ = pub.Publish(ctx, "events", Event{ID: "3", Name: "gamma"})
//
// 	cancel()
//
// 	// Consume via stream API.
// 	for _, e := range s.Collect() {
// 		fmt.Println(e.ID, e.Name)
// 	}
// 	// Output:
// 	// 1 alpha
// 	// 2 beta
// 	// 3 gamma
// }
