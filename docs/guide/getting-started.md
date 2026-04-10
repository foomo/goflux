# Getting Started

## Prerequisites

- **Go 1.22** or later

## Installation

Install the core library and the channel transport:

```sh
go get github.com/foomo/goflux
go get github.com/foomo/goflux/chan
```

## Your First Example

The following program creates an in-process channel transport, publishes a single message, and prints it when received.

```go
package main

import (
	"context"
	"fmt"
	"time"

	"github.com/foomo/goflux"
	_chan "github.com/foomo/goflux/chan"
)

type OrderEvent struct {
	OrderID string
	Status  string
}

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// 1. Create a Bus -- the in-process message broker.
	bus := _chan.NewBus[OrderEvent]()

	// 2. Create a Publisher and Subscriber backed by the bus.
	pub := _chan.NewPublisher(bus)

	sub, err := _chan.NewSubscriber(bus, 1) // bufSize=1
	if err != nil {
		panic(err)
	}

	// 3. Subscribe in a goroutine -- Subscribe blocks until ctx is cancelled.
	done := make(chan struct{})
	go func() {
		_ = sub.Subscribe(ctx, "orders", func(ctx context.Context, msg goflux.Message[OrderEvent]) error {
			fmt.Printf("Received: subject=%s order=%s status=%s\n",
				msg.Subject, msg.Payload.OrderID, msg.Payload.Status)
			close(done)
			return nil
		})
	}()

	// Give the subscriber time to register on the bus.
	time.Sleep(10 * time.Millisecond)

	// 4. Publish a message.
	if err := pub.Publish(ctx, "orders", OrderEvent{
		OrderID: "ORD-42",
		Status:  "confirmed",
	}); err != nil {
		panic(err)
	}

	// 5. Wait for the handler to fire.
	<-done
	// Output: Received: subject=orders order=ORD-42 status=confirmed
}
```

## Step-by-Step Breakdown

### 1. Create the Bus

```go
bus := _chan.NewBus[OrderEvent]()
```

`Bus[T]` is the in-process message broker for the channel transport. It routes messages by subject using exact string matching. Both the publisher and subscriber reference the same bus instance.

### 2. Create Publisher and Subscriber

```go
pub := _chan.NewPublisher(bus)
sub, err := _chan.NewSubscriber(bus, 1)
```

`NewPublisher` and `NewSubscriber` return types that implement `goflux.Publisher[T]` and `goflux.Subscriber[T]`. The `bufSize` parameter on `NewSubscriber` controls backpressure: when the internal buffer is full, the publisher blocks until the subscriber catches up.

### 3. Subscribe in a Goroutine

```go
go func() {
    _ = sub.Subscribe(ctx, "orders", handler)
}()
```

::: warning
`Subscribe` **blocks** until the context is cancelled or a fatal error occurs. Always run it in a goroutine.
:::

The handler is a `goflux.Handler[T]` -- a function with the signature `func(ctx context.Context, msg goflux.Message[T]) error`. It receives fully decoded messages; no raw bytes are exposed.

### 4. Publish a Message

```go
pub.Publish(ctx, "orders", OrderEvent{OrderID: "ORD-42", Status: "confirmed"})
```

The subject `"orders"` is a plain string routing key. The payload is your typed value. With the channel transport there is no serialisation; the value is passed directly.

### 5. Receive and Process

The handler fires synchronously in the subscriber's event loop. Returning `nil` signals success; returning an error signals the transport to nack or requeue the message (behaviour is transport-specific).

## What's Next

- [Core Concepts](./core-concepts.md) -- understand `Message[T]`, `Handler[T]`, and the key rules
- [Transports](./transports.md) -- use NATS or HTTP instead of channels
- [Pipelines](./pipelines.md) -- wire subscribers to publishers with filtering and mapping
