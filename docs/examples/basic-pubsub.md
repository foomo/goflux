---
outline: deep
---

# Basic Pub/Sub

The simplest way to get started with goflux is the `chan/` transport. It moves messages between goroutines using Go channels -- no network, no codec, no external dependencies.

## Publish and Subscribe

Create a `Bus`, a `Publisher`, and a `Subscriber` that share the same bus. `Subscribe` blocks until its context is cancelled, so run it in a separate goroutine.

```go
package main

import (
	"context"
	"fmt"
	"time"

	"github.com/foomo/goflux"
	_chan "github.com/foomo/goflux/chan"
)

type Event struct {
	ID   string
	Name string
}

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Shared in-process bus.
	bus := _chan.NewBus[Event]()

	// Publisher and subscriber both bind to the same bus.
	pub := _chan.NewPublisher(bus)

	sub, err := _chan.NewSubscriber(bus, 1) // buffer size 1
	if err != nil {
		panic(err)
	}

	done := make(chan struct{})

	// Subscribe blocks — run in a goroutine.
	go func() {
		_ = sub.Subscribe(ctx, "events", func(_ context.Context, msg goflux.Message[Event]) error {
			fmt.Println(msg.Subject, msg.Payload)
			close(done)
			return nil
		})
	}()

	// Small delay so the subscriber registers before we publish.
	time.Sleep(10 * time.Millisecond)

	if err := pub.Publish(ctx, "events", Event{ID: "1", Name: "hello"}); err != nil {
		panic(err)
	}

	<-done
	// Output: events {1 hello}
}
```

Key points:

- `Bus[T]` is the shared message registry -- pass it to both the publisher and subscriber.
- The buffer size passed to `NewSubscriber` controls backpressure. A full buffer blocks the publisher (no drops by design).
- `Subscribe` blocks until `ctx` is cancelled. Always run it in a goroutine.
- No codec is needed -- the `chan/` transport passes values in-process by value.

## Pipe: Forwarding Between Buses

`goflux.Pipe` returns a `Handler[T]` that forwards every received message to a destination publisher. This wires a source subscriber directly to a destination publisher.

```go
package main

import (
	"context"
	"fmt"
	"time"

	"github.com/foomo/goflux"
	_chan "github.com/foomo/goflux/chan"
)

type Event struct {
	ID   string
	Name string
}

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Source transport.
	srcBus := _chan.NewBus[Event]()
	srcPub := _chan.NewPublisher(srcBus)

	srcSub, err := _chan.NewSubscriber(srcBus, 1)
	if err != nil {
		panic(err)
	}

	// Destination transport.
	dstBus := _chan.NewBus[Event]()
	dstPub := _chan.NewPublisher(dstBus)

	dstSub, err := _chan.NewSubscriber(dstBus, 1)
	if err != nil {
		panic(err)
	}

	done := make(chan struct{})

	// Listen on the destination bus.
	go func() {
		_ = dstSub.Subscribe(ctx, "events", func(_ context.Context, msg goflux.Message[Event]) error {
			fmt.Println(msg.Subject, msg.Payload)
			close(done)
			return nil
		})
	}()

	// Pipe: source subscriber forwards to destination publisher.
	go func() {
		_ = srcSub.Subscribe(ctx, "events", goflux.Pipe[Event](dstPub))
	}()

	time.Sleep(10 * time.Millisecond)

	if err := srcPub.Publish(ctx, "events", Event{ID: "1", Name: "hello"}); err != nil {
		panic(err)
	}

	<-done
	// Output: events {1 hello}
}
```

## PipeMap: Type Transformation

`goflux.PipeMap` transforms messages from one type to another before forwarding. The `MapFunc` receives a `Message[T]` and returns a `Message[U]`.

```go
package main

import (
	"context"
	"fmt"
	"time"

	"github.com/foomo/goflux"
	_chan "github.com/foomo/goflux/chan"
)

type Event struct {
	ID   string
	Name string
}

type Summary struct {
	Label string
}

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Source transport (Event).
	srcBus := _chan.NewBus[Event]()
	srcPub := _chan.NewPublisher(srcBus)

	srcSub, err := _chan.NewSubscriber(srcBus, 1)
	if err != nil {
		panic(err)
	}

	// Destination transport (Summary).
	dstBus := _chan.NewBus[Summary]()
	dstPub := _chan.NewPublisher(dstBus)

	dstSub, err := _chan.NewSubscriber(dstBus, 1)
	if err != nil {
		panic(err)
	}

	// Map function: Event -> Summary.
	mapFn := func(_ context.Context, msg goflux.Message[Event]) (goflux.Message[Summary], error) {
		return goflux.NewMessage(msg.Subject, Summary{Label: msg.Payload.Name}), nil
	}

	done := make(chan struct{})

	// Listen on the destination bus.
	go func() {
		_ = dstSub.Subscribe(ctx, "events", func(_ context.Context, msg goflux.Message[Summary]) error {
			fmt.Println(msg.Subject, msg.Payload)
			close(done)
			return nil
		})
	}()

	// PipeMap: source subscriber maps Event -> Summary and forwards.
	go func() {
		_ = srcSub.Subscribe(ctx, "events", goflux.PipeMap[Event, Summary](dstPub, mapFn))
	}()

	time.Sleep(10 * time.Millisecond)

	if err := srcPub.Publish(ctx, "events", Event{ID: "1", Name: "hello"}); err != nil {
		panic(err)
	}

	<-done
	// Output: events {hello}
}
```

## Pipe with Filter

Add a filter to drop messages before they reach the destination. Filters return `(bool, error)` -- `false` or an error drops the message.

```go
// Filter: drop events with ID "skip".
filter := func(_ context.Context, msg goflux.Message[Event]) (bool, error) {
	return msg.Payload.ID != "skip", nil
}

handler := goflux.Pipe[Event](dstPub, goflux.WithFilter[Event](filter))
```

## Pipe with Dead Letter

When the downstream publisher fails, the dead-letter function receives the original message and the error.

```go
deadLetter := func(_ context.Context, msg goflux.Message[Event], err error) {
	fmt.Println("dead-letter:", msg.Subject, msg.Payload, err)
}

handler := goflux.Pipe[Event](pub, goflux.WithDeadLetter[Event](deadLetter))
```

This works the same way with `PipeMap` -- if the map function returns an error, the original `Message[T]` is routed to the dead-letter handler.
