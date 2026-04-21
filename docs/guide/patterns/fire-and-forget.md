# Fire & Forget

Fire-and-forget is the simplest messaging pattern: the publisher sends a message and moves on without waiting for acknowledgment or delivery confirmation. There are no retries and no guarantees that the message was received.

## How It Works

The publisher calls `Publish` and returns immediately after the transport accepts the message. On the subscriber side, `Message.HasAcker()` returns `false`, and calling `Ack()` or `Nak()` is a no-op.

## Supported Transports

| Transport | Notes |
|-----------|-------|
| Channel   | In-process, backpressure via blocking channel sends |
| NATS core | Network delivery, at-most-once semantics |

## Channel Transport Example

The channel transport uses an in-process `Bus` as the message broker. Messages are delivered synchronously through Go channels, so a slow subscriber applies backpressure to the publisher.

```go
package main

import (
	"context"
	"fmt"
	"log"

	"github.com/foomo/goflux"
	"github.com/foomo/goflux/transport/channel"
)

type OrderCreated struct {
	OrderID string
	Total   float64
}

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Create a typed bus — the central broker for in-process messaging.
	bus := channel.NewBus[OrderCreated]()

	// Publisher and subscriber are both bound to the same bus.
	pub := channel.NewPublisher[OrderCreated](bus)

	sub, err := channel.NewSubscriber[OrderCreated](bus, 16) // buffer size 16
	if err != nil {
		log.Fatal(err)
	}

	// Subscribe blocks until ctx is cancelled — run it in a goroutine.
	go func() {
		_ = sub.Subscribe(ctx, "orders.created", func(ctx context.Context, msg goflux.Message[OrderCreated]) error {
			fmt.Printf("received order %s (total: %.2f)\n", msg.Payload.OrderID, msg.Payload.Total)
			// HasAcker() is false — Ack/Nak are no-ops.
			return nil
		})
	}()

	// Publish a message.
	if err := pub.Publish(ctx, "orders.created", OrderCreated{
		OrderID: "ord-123",
		Total:   49.99,
	}); err != nil {
		log.Fatal(err)
	}

	cancel()
}
```

## NATS Core Example

With NATS core, messages are sent over the network but still follow at-most-once semantics. Publishers take a `goencode.Encoder`, subscribers take a `goencode.Decoder`.

```go
package main

import (
	"context"
	"fmt"
	"log"

	json "github.com/foomo/goencode/json/v1"
	"github.com/foomo/goflux"
	gofluxnats "github.com/foomo/goflux/transport/nats"
	"github.com/nats-io/nats.go"
)

type Event struct {
	Kind    string
	Message string
}

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	conn, err := nats.Connect(nats.DefaultURL)
	if err != nil {
		log.Fatal(err)
	}
	defer conn.Drain()

	codec := json.NewCodec[Event]()

	pub := gofluxnats.NewPublisher[Event](conn, codec.Encode)
	sub := gofluxnats.NewSubscriber[Event](conn, codec.Decode)

	go func() {
		_ = sub.Subscribe(ctx, "events.>", func(ctx context.Context, msg goflux.Message[Event]) error {
			fmt.Printf("[%s] %s\n", msg.Subject, msg.Payload.Message)
			return nil
		})
	}()

	_ = pub.Publish(ctx, "events.user.signup", Event{
		Kind:    "signup",
		Message: "user alice signed up",
	})

	cancel()
}
```

## When to Use

- **Testing and prototyping** -- the channel transport requires no external infrastructure.
- **In-process event buses** -- decoupling components within a single binary.
- **Acceptable loss** -- metrics, logs, or ephemeral notifications where occasional message loss is tolerable.
- **Low-latency paths** -- when the overhead of acknowledgment is not justified.

If you need delivery guarantees, use the [at-least-once](./at-least-once) pattern with JetStream instead.
