package goflux_test

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/foomo/goflux"
	"github.com/foomo/goflux/transport/channel"
	"github.com/foomo/gofuncy"
)

// failPublisher is a Publisher that always returns the given error.
type failPublisher[T any] struct{ err error }

func (p *failPublisher[T]) Publish(context.Context, string, T) error { return p.err }
func (p *failPublisher[T]) Close() error                             { return nil }

type Event struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type Summary struct {
	Label string `json:"label"`
}

// ExamplePipe demonstrates wiring a source subscriber to a destination
// publisher using goflux.Pipe. Every message received on "events" is forwarded
// to the destination bus on the same subject.
func ExamplePipe() {
	// Source transport.
	srcBus := channel.NewBus[Event]()
	srcPub := channel.NewPublisher(srcBus)

	srcSub, err := channel.NewSubscriber(srcBus, 1)
	if err != nil {
		panic(err)
	}

	// Destination transport.
	dstBus := channel.NewBus[Event]()
	dstPub := channel.NewPublisher(dstBus)

	dstSub, err := channel.NewSubscriber(dstBus, 1)
	if err != nil {
		panic(err)
	}

	ctx, cancel := context.WithCancel(context.Background())

	// Listen on destination — print when a message arrives.
	// Pipe preserves the original subject, so we subscribe to "events".
	gofuncy.StartWithReady(ctx, func(ctx context.Context, ready gofuncy.ReadyFunc) error {
		ready()

		return dstSub.Subscribe(ctx, "events", func(_ context.Context, msg goflux.Message[Event]) error {
			fmt.Println(msg.Subject, msg.Payload)
			cancel()

			return nil
		})
	}, gofuncy.WithName("dst-subscriber"))

	// Pipe: source subscriber forwards to destination publisher.
	gofuncy.StartWithReady(ctx, func(ctx context.Context, ready gofuncy.ReadyFunc) error {
		ready()
		return srcSub.Subscribe(ctx, "events", goflux.Pipe[Event](dstPub))
	}, gofuncy.WithName("pipe"))

	if err := srcPub.Publish(ctx, "events", Event{ID: "1", Name: "hello"}); err != nil {
		panic(err)
	}

	<-ctx.Done()
	// Output: events {1 hello}
}

// ExamplePipeMap demonstrates transforming messages from one type to another.
// Events are mapped to Summary values and published to the destination bus.
func ExamplePipeMap() {
	ctx, cancel := context.WithCancel(context.Background())

	// Source transport (Event).
	srcBus := channel.NewBus[Event]()
	srcPub := channel.NewPublisher(srcBus)

	srcSub, err := channel.NewSubscriber(srcBus, 1)
	if err != nil {
		panic(err)
	}

	// Destination transport (Summary).
	dstBus := channel.NewBus[Summary]()
	dstPub := channel.NewPublisher(dstBus)

	dstSub, err := channel.NewSubscriber(dstBus, 1)
	if err != nil {
		panic(err)
	}

	// Map function: Event → Summary, keeping the same subject.
	mapFn := func(_ context.Context, msg goflux.Message[Event]) (goflux.Message[Summary], error) {
		return goflux.NewMessage(msg.Subject, Summary{Label: msg.Payload.Name}), nil
	}

	// Listen on destination — PipeMap publishes with the subject from the MapFunc result.
	gofuncy.StartWithReady(ctx, func(ctx context.Context, ready gofuncy.ReadyFunc) error {
		ready()

		return dstSub.Subscribe(ctx, "events", func(_ context.Context, msg goflux.Message[Summary]) error {
			fmt.Println(msg.Subject, msg.Payload)
			cancel()

			return nil
		})
	}, gofuncy.WithName("dst-subscriber"))

	// PipeMap: source subscriber maps Event→Summary and forwards.
	gofuncy.StartWithReady(ctx, func(ctx context.Context, ready gofuncy.ReadyFunc) error {
		ready()
		return srcSub.Subscribe(ctx, "events", goflux.PipeMap[Event, Summary](dstPub, mapFn))
	}, gofuncy.WithName("pipe-map"))

	if err := srcPub.Publish(ctx, "events", Event{ID: "1", Name: "hello"}); err != nil {
		panic(err)
	}

	<-ctx.Done()
	// Output: events {hello}
}

// ExamplePipe_withFilter demonstrates filtering messages before they are
// forwarded. Only events whose ID is not "skip" reach the destination.
func ExamplePipe_withFilter() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Source transport.
	srcBus := channel.NewBus[Event]()
	srcPub := channel.NewPublisher(srcBus)

	srcSub, err := channel.NewSubscriber(srcBus, 1)
	if err != nil {
		panic(err)
	}

	// Destination transport.
	dstBus := channel.NewBus[Event]()
	dstPub := channel.NewPublisher(dstBus)

	dstSub, err := channel.NewSubscriber(dstBus, 1)
	if err != nil {
		panic(err)
	}

	// Filter: drop events with ID "skip".
	filter := func(_ context.Context, msg goflux.Message[Event]) (bool, error) {
		return msg.Payload.ID != "skip", nil
	}

	// Listen on destination — expect exactly one message.
	gofuncy.StartWithReady(ctx, func(ctx context.Context, ready gofuncy.ReadyFunc) error {
		ready()

		return dstSub.Subscribe(ctx, "events", func(_ context.Context, msg goflux.Message[Event]) error {
			fmt.Println(msg.Subject, msg.Payload)
			cancel()

			return nil
		})
	}, gofuncy.WithName("dst-subscriber"))

	// Pipe with filter.
	gofuncy.StartWithReady(ctx, func(ctx context.Context, ready gofuncy.ReadyFunc) error {
		ready()
		return srcSub.Subscribe(ctx, "events", goflux.Pipe[Event](dstPub, goflux.WithFilter[Event](filter)))
	}, gofuncy.WithName("pipe"))

	// Allow goroutines to register on their buses before publishing.
	time.Sleep(10 * time.Millisecond)

	// First message is filtered out.
	if err := srcPub.Publish(ctx, "events", Event{ID: "skip", Name: "ignored"}); err != nil {
		panic(err)
	}
	// Second message passes the filter.
	if err := srcPub.Publish(ctx, "events", Event{ID: "keep", Name: "accepted"}); err != nil {
		panic(err)
	}

	<-ctx.Done()
	// Output: events {keep accepted}
}

// ExamplePipeMap_withFilter demonstrates filtering messages before they are
// mapped and forwarded. The filter runs on the source type (Event); only
// passing events are transformed into Summary values.
func ExamplePipeMap_withFilter() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Source transport (Event).
	srcBus := channel.NewBus[Event]()
	srcPub := channel.NewPublisher(srcBus)

	srcSub, err := channel.NewSubscriber(srcBus, 1)
	if err != nil {
		panic(err)
	}

	// Destination transport (Summary).
	dstBus := channel.NewBus[Summary]()
	dstPub := channel.NewPublisher(dstBus)

	dstSub, err := channel.NewSubscriber(dstBus, 1)
	if err != nil {
		panic(err)
	}

	// Map function: Event → Summary.
	mapFn := func(_ context.Context, msg goflux.Message[Event]) (goflux.Message[Summary], error) {
		return goflux.NewMessage(msg.Subject, Summary{Label: msg.Payload.Name}), nil
	}

	// Filter: drop events with ID "skip".
	filter := func(_ context.Context, msg goflux.Message[Event]) (bool, error) {
		return msg.Payload.ID != "skip", nil
	}

	// Listen on destination — expect exactly one message.
	gofuncy.StartWithReady(ctx, func(ctx context.Context, ready gofuncy.ReadyFunc) error {
		ready()

		return dstSub.Subscribe(ctx, "events", func(_ context.Context, msg goflux.Message[Summary]) error {
			fmt.Println(msg.Subject, msg.Payload)
			cancel()

			return nil
		})
	}, gofuncy.WithName("dst-subscriber"))

	// PipeMap with filter.
	gofuncy.StartWithReady(ctx, func(ctx context.Context, ready gofuncy.ReadyFunc) error {
		ready()
		return srcSub.Subscribe(ctx, "events", goflux.PipeMap[Event, Summary](dstPub, mapFn, goflux.WithFilter[Event](filter)))
	}, gofuncy.WithName("pipe-map"))

	// Allow goroutines to register on their buses before publishing.
	time.Sleep(10 * time.Millisecond)

	// First message is filtered out before the map runs.
	if err := srcPub.Publish(ctx, "events", Event{ID: "skip", Name: "ignored"}); err != nil {
		panic(err)
	}
	// Second message passes the filter, gets mapped, and arrives.
	if err := srcPub.Publish(ctx, "events", Event{ID: "keep", Name: "accepted"}); err != nil {
		panic(err)
	}

	<-ctx.Done()
	// Output: events {accepted}
}

// ExamplePipe_withDeadLetter demonstrates the dead-letter handler on a Pipe.
// When the downstream publisher returns an error, the dead-letter function
// receives the original message and the error.
func ExamplePipe_withDeadLetter() {
	ctx := context.Background()

	// Source transport.
	srcBus := channel.NewBus[Event]()
	srcPub := channel.NewPublisher(srcBus)

	srcSub, err := channel.NewSubscriber(srcBus, 1)
	if err != nil {
		panic(err)
	}

	// A publisher that always fails.
	pubErr := errors.New("publish failed")
	badPub := &failPublisher[Event]{err: pubErr}

	received := make(chan struct{})

	// Dead-letter handler prints the original message and error.
	deadLetter := func(_ context.Context, msg goflux.Message[Event], err error) {
		fmt.Println("dead-letter:", msg.Subject, msg.Payload, err)
		close(received)
	}

	// Pipe with dead-letter: publish errors are routed to the handler.
	gofuncy.StartWithReady(ctx, func(ctx context.Context, ready gofuncy.ReadyFunc) error {
		ready()
		return srcSub.Subscribe(ctx, "events", goflux.Pipe[Event](badPub, goflux.WithDeadLetter[Event](deadLetter)))
	}, gofuncy.WithName("pipe"))

	if err := srcPub.Publish(ctx, "events", Event{ID: "1", Name: "hello"}); err != nil {
		panic(err)
	}

	<-received
	// Output: dead-letter: events {1 hello} publish failed
}

// ExamplePipeMap_withDeadLetter demonstrates the dead-letter handler on a
// PipeMap. When the MapFunc returns an error, the dead-letter function receives
// the original source message and the map error.
func ExamplePipeMap_withDeadLetter() {
	ctx, cancel := context.WithCancel(context.Background())

	// Source transport (Event).
	srcBus := channel.NewBus[Event]()
	srcPub := channel.NewPublisher(srcBus)

	srcSub, err := channel.NewSubscriber(srcBus, 1)
	if err != nil {
		panic(err)
	}

	// Destination transport (Summary).
	dstBus := channel.NewBus[Summary]()
	dstPub := channel.NewPublisher(dstBus)

	dstSub, err := channel.NewSubscriber(dstBus, 1)
	if err != nil {
		panic(err)
	}

	// Map function that fails for events with ID "bad".
	mapErr := errors.New("bad event")
	mapFn := func(_ context.Context, msg goflux.Message[Event]) (goflux.Message[Summary], error) {
		if msg.Payload.ID == "bad" {
			return goflux.Message[Summary]{}, mapErr
		}

		return goflux.NewMessage(msg.Subject, Summary{Label: msg.Payload.Name}), nil
	}

	// Dead-letter handler prints the original message and error.
	deadLetter := func(_ context.Context, msg goflux.Message[Event], err error) {
		fmt.Println("dead-letter:", msg.Subject, msg.Payload, err)
	}

	// Listen on destination for the valid message.
	gofuncy.StartWithReady(ctx, func(ctx context.Context, ready gofuncy.ReadyFunc) error {
		ready()

		return dstSub.Subscribe(ctx, "events", func(_ context.Context, msg goflux.Message[Summary]) error {
			fmt.Println(msg.Subject, msg.Payload)
			cancel()

			return nil
		})
	}, gofuncy.WithName("dst-subscriber"))

	// PipeMap with dead-letter.
	gofuncy.StartWithReady(ctx, func(ctx context.Context, ready gofuncy.ReadyFunc) error {
		ready()
		return srcSub.Subscribe(ctx, "events", goflux.PipeMap[Event, Summary](dstPub, mapFn, goflux.WithDeadLetter[Event](deadLetter)))
	}, gofuncy.WithName("pipe-map"))

	// First message fails the map — routed to dead-letter.
	if err := srcPub.Publish(ctx, "events", Event{ID: "bad", Name: "broken"}); err != nil {
		panic(err)
	}
	// Second message succeeds — arrives at destination.
	if err := srcPub.Publish(ctx, "events", Event{ID: "1", Name: "hello"}); err != nil {
		panic(err)
	}

	<-ctx.Done()
	// Output:
	// dead-letter: events {bad broken} bad event
	// events {hello}
}
