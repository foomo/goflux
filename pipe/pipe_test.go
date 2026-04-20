package pipe_test

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/foomo/goflux"
	"github.com/foomo/goflux/pipe"
	"github.com/foomo/goflux/transport/channel"
	"github.com/foomo/gofuncy"
)

type Event struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type Summary struct {
	Label string `json:"label"`
}

// failPublisher is a Publisher that always returns the given error.
type failPublisher[T any] struct{ err error }

func (p *failPublisher[T]) Publish(context.Context, string, T) error { return p.err }
func (p *failPublisher[T]) Close() error                             { return nil }

// ExamplePipe demonstrates wiring a source subscriber to a destination
// publisher using pipe.Pipe.
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

	gofuncy.StartWithReady(ctx, func(ctx context.Context, ready gofuncy.ReadyFunc) error {
		ready()

		return dstSub.Subscribe(ctx, "events", func(_ context.Context, msg goflux.Message[Event]) error {
			fmt.Println(msg.Subject, msg.Payload)
			cancel()

			return nil
		})
	}, gofuncy.WithName("dst-subscriber"))

	gofuncy.StartWithReady(ctx, func(ctx context.Context, ready gofuncy.ReadyFunc) error {
		ready()
		return srcSub.Subscribe(ctx, "events", pipe.Pipe[Event](dstPub))
	}, gofuncy.WithName("pipe"))

	if err := srcPub.Publish(ctx, "events", Event{ID: "1", Name: "hello"}); err != nil {
		panic(err)
	}

	<-ctx.Done()
	// Output: events {1 hello}
}

// ExamplePipeMap demonstrates transforming messages from one type to another.
func ExamplePipeMap() {
	ctx, cancel := context.WithCancel(context.Background())

	srcBus := channel.NewBus[Event]()
	srcPub := channel.NewPublisher(srcBus)

	srcSub, err := channel.NewSubscriber(srcBus, 1)
	if err != nil {
		panic(err)
	}

	dstBus := channel.NewBus[Summary]()
	dstPub := channel.NewPublisher(dstBus)

	dstSub, err := channel.NewSubscriber(dstBus, 1)
	if err != nil {
		panic(err)
	}

	mapFn := func(_ context.Context, msg goflux.Message[Event]) (goflux.Message[Summary], error) {
		return goflux.NewMessage(msg.Subject, Summary{Label: msg.Payload.Name}), nil
	}

	gofuncy.StartWithReady(ctx, func(ctx context.Context, ready gofuncy.ReadyFunc) error {
		ready()

		return dstSub.Subscribe(ctx, "events", func(_ context.Context, msg goflux.Message[Summary]) error {
			fmt.Println(msg.Subject, msg.Payload)
			cancel()

			return nil
		})
	}, gofuncy.WithName("dst-subscriber"))

	gofuncy.StartWithReady(ctx, func(ctx context.Context, ready gofuncy.ReadyFunc) error {
		ready()
		return srcSub.Subscribe(ctx, "events", pipe.PipeMap[Event, Summary](dstPub, mapFn))
	}, gofuncy.WithName("pipe-map"))

	if err := srcPub.Publish(ctx, "events", Event{ID: "1", Name: "hello"}); err != nil {
		panic(err)
	}

	<-ctx.Done()
	// Output: events {hello}
}

// ExamplePipe_withFilter demonstrates filtering messages before forwarding.
func ExamplePipe_withFilter() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	srcBus := channel.NewBus[Event]()
	srcPub := channel.NewPublisher(srcBus)

	srcSub, err := channel.NewSubscriber(srcBus, 1)
	if err != nil {
		panic(err)
	}

	dstBus := channel.NewBus[Event]()
	dstPub := channel.NewPublisher(dstBus)

	dstSub, err := channel.NewSubscriber(dstBus, 1)
	if err != nil {
		panic(err)
	}

	filter := func(_ context.Context, msg goflux.Message[Event]) (bool, error) {
		return msg.Payload.ID != "skip", nil
	}

	gofuncy.StartWithReady(ctx, func(ctx context.Context, ready gofuncy.ReadyFunc) error {
		ready()

		return dstSub.Subscribe(ctx, "events", func(_ context.Context, msg goflux.Message[Event]) error {
			fmt.Println(msg.Subject, msg.Payload)
			cancel()

			return nil
		})
	}, gofuncy.WithName("dst-subscriber"))

	gofuncy.StartWithReady(ctx, func(ctx context.Context, ready gofuncy.ReadyFunc) error {
		ready()
		return srcSub.Subscribe(ctx, "events", pipe.Pipe[Event](dstPub, pipe.WithFilter[Event](filter)))
	}, gofuncy.WithName("pipe"))

	time.Sleep(10 * time.Millisecond)

	if err := srcPub.Publish(ctx, "events", Event{ID: "skip", Name: "ignored"}); err != nil {
		panic(err)
	}

	if err := srcPub.Publish(ctx, "events", Event{ID: "keep", Name: "accepted"}); err != nil {
		panic(err)
	}

	<-ctx.Done()
	// Output: events {keep accepted}
}

// ExamplePipe_withDeadLetter demonstrates the dead-letter handler on a Pipe.
func ExamplePipe_withDeadLetter() {
	ctx := context.Background()

	srcBus := channel.NewBus[Event]()
	srcPub := channel.NewPublisher(srcBus)

	srcSub, err := channel.NewSubscriber(srcBus, 1)
	if err != nil {
		panic(err)
	}

	pubErr := errors.New("publish failed")
	badPub := &failPublisher[Event]{err: pubErr}

	received := make(chan struct{})

	deadLetter := func(_ context.Context, msg goflux.Message[Event], err error) {
		fmt.Println("dead-letter:", msg.Subject, msg.Payload, err)
		close(received)
	}

	gofuncy.StartWithReady(ctx, func(ctx context.Context, ready gofuncy.ReadyFunc) error {
		ready()
		return srcSub.Subscribe(ctx, "events", pipe.Pipe[Event](badPub, pipe.WithDeadLetter[Event](deadLetter)))
	}, gofuncy.WithName("pipe"))

	if err := srcPub.Publish(ctx, "events", Event{ID: "1", Name: "hello"}); err != nil {
		panic(err)
	}

	<-received
	// Output: dead-letter: events {1 hello} publish failed
}

// ExampleBoundPipe demonstrates using BoundPipe with a BoundPublisher.
func ExampleBoundPipe() {
	ctx, cancel := context.WithCancel(context.Background())

	srcBus := channel.NewBus[Event]()
	srcPub := channel.NewPublisher(srcBus)

	srcSub, err := channel.NewSubscriber(srcBus, 1)
	if err != nil {
		panic(err)
	}

	dstBus := channel.NewBus[Event]()
	dstPub := channel.NewPublisher(dstBus)

	dstSub, err := channel.NewSubscriber(dstBus, 1)
	if err != nil {
		panic(err)
	}

	// Bind destination publisher to "output" subject.
	boundPub := goflux.BindPublisher[Event](dstPub, "output")

	gofuncy.StartWithReady(ctx, func(ctx context.Context, ready gofuncy.ReadyFunc) error {
		ready()

		return dstSub.Subscribe(ctx, "output", func(_ context.Context, msg goflux.Message[Event]) error {
			fmt.Println(msg.Subject, msg.Payload)
			cancel()

			return nil
		})
	}, gofuncy.WithName("dst-subscriber"))

	// BoundPipe: source messages forwarded to bound publisher's fixed subject.
	gofuncy.StartWithReady(ctx, func(ctx context.Context, ready gofuncy.ReadyFunc) error {
		ready()
		return srcSub.Subscribe(ctx, "events", pipe.BoundPipe[Event](boundPub))
	}, gofuncy.WithName("pipe"))

	if err := srcPub.Publish(ctx, "events", Event{ID: "1", Name: "hello"}); err != nil {
		panic(err)
	}

	<-ctx.Done()
	// Output: output {1 hello}
}
