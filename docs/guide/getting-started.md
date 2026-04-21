# Getting Started

## Prerequisites

- **Go 1.22** or later

## Installation

Install the core library:

```sh
go get github.com/foomo/goflux
```

For a specific transport, add its submodule:

```sh
go get github.com/foomo/goflux/transport/channel   # in-process channels
go get github.com/foomo/goflux/transport/nats       # NATS core
go get github.com/foomo/goflux/transport/jetstream  # NATS JetStream
go get github.com/foomo/goflux/transport/http       # HTTP POST
```

## Fire-and-Forget with Channels

The channel transport has zero external dependencies -- no broker, no network. It is a good starting point and useful for tests.

```go
package main

import (
	"context"
	"fmt"
	"time"

	"github.com/foomo/goflux"
	"github.com/foomo/goflux/transport/channel"
)

type OrderEvent struct {
	OrderID string
	Status  string
}

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Create the in-process bus, publisher, and subscriber.
	bus := channel.NewBus[OrderEvent]()
	pub := channel.NewPublisher(bus)
	sub, err := channel.NewSubscriber(bus, 8) // bufSize controls backpressure
	if err != nil {
		panic(err)
	}

	// Define a handler -- same signature regardless of transport.
	handler := func(ctx context.Context, msg goflux.Message[OrderEvent]) error {
		fmt.Printf("received: nats=%s order=%s status=%s\n",
			msg.Subject, msg.Payload.OrderID, msg.Payload.Status)
		return nil
	}

	// Subscribe blocks, so run it in a goroutine.
	done := make(chan struct{})
	go func() {
		defer close(done)
		_ = sub.Subscribe(ctx, "orders", handler)
	}()

	// Give the subscriber time to register.
	time.Sleep(10 * time.Millisecond)

	// Publish a message.
	if err := pub.Publish(ctx, "orders", OrderEvent{
		OrderID: "ORD-42",
		Status:  "confirmed",
	}); err != nil {
		panic(err)
	}

	cancel()
	<-done
	// Output: received: nats=orders order=ORD-42 status=confirmed
}
```

The handler is a `goflux.Handler[OrderEvent]` -- a function with the signature `func(ctx context.Context, msg goflux.Message[T]) error`. This signature is the same for every transport.

## Swapping to NATS

The handler does not change. Only the import and constructor differ:

```go
package main

import (
	"context"
	"fmt"

	"github.com/foomo/goflux"
	gofluxnats "github.com/foomo/goflux/transport/nats"
	jsoncodec "github.com/foomo/goencode/json/v1"
	"github.com/nats-io/nats.go"
)

type OrderEvent struct {
	OrderID string `json:"order_id"`
	Status  string `json:"status"`
}

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Connect to NATS -- caller owns the connection.
	conn, err := nats.Connect(nats.DefaultURL)
	if err != nil {
		panic(err)
	}
	defer conn.Drain()

	// Create publisher and subscriber with a JSON encoder/decoder.
	codec := jsoncodec.New[OrderEvent]()
	pub := gofluxnats.NewPublisher(conn, codec.Encode)
	sub := gofluxnats.NewSubscriber(conn, codec.Decode)

	// Same handler as before -- no transport-specific code.
	handler := func(ctx context.Context, msg goflux.Message[OrderEvent]) error {
		fmt.Printf("received: %s %s\n", msg.Payload.OrderID, msg.Payload.Status)
		return nil
	}

	go func() {
		_ = sub.Subscribe(ctx, "orders", handler)
	}()

	_ = pub.Publish(ctx, "orders", OrderEvent{OrderID: "ORD-42", Status: "confirmed"})
}
```

Network transports take an `goencode.Encoder[T, []byte]` for publishers and a `goencode.Decoder[T, []byte]` for subscribers. A `goencode.Codec[T, []byte]` provides both as `.Encode` and `.Decode` method values. Encoders and decoders are function types, composable via `goencode.PipeEncoder` / `goencode.PipeDecoder`. The `goencode` library provides codecs for JSON, Protocol Buffers, and other formats.

## Other Transports

- **[JetStream](../transports/jetstream.md)** -- NATS JetStream with ack/nak/term, pull consumers, and durable subscriptions. Use when you need at-least-once delivery.
- **[HTTP](../transports/http.md)** -- HTTP POST publisher and `http.ServeMux`-based subscriber. Also supports request-reply.

## What's Next

- [Core Concepts](./core-concepts.md) -- understand every core type and the design rules
- [Fire & Forget](./patterns/fire-and-forget.md) -- the simplest messaging pattern in detail
- [At-Least-Once](./patterns/at-least-once.md) -- acknowledgment-based delivery with JetStream
