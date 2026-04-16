# At-Least-Once Delivery

At-least-once delivery guarantees that a message is processed at least one time. If the handler fails, the message is redelivered. This pattern is supported by the JetStream transport, which uses NATS JetStream's acknowledgment protocol under the hood.

## Auto-Ack (Default)

By default, the JetStream subscriber manages acknowledgments automatically:

- Handler returns `nil` -- the message is **acked** (removed from the stream).
- Handler returns a non-nil `error` -- the message is **naked** (scheduled for redelivery).

```go
package main

import (
	"context"
	"fmt"
	"log"

	json "github.com/foomo/goencode/json/v1"
	"github.com/foomo/goflux"
	gofluxjs "github.com/foomo/goflux/transport/jetstream"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
)

type Event struct {
	ID   string
	Kind string
}

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	nc, _ := nats.Connect(nats.DefaultURL)
	defer nc.Drain()

	js, _ := jetstream.New(nc)
	stream, _ := js.CreateOrUpdateStream(ctx, jetstream.StreamConfig{
		Name:     "EVENTS",
		Subjects: []string{"events.>"},
	})
	consumer, _ := stream.CreateOrUpdateConsumer(ctx, jetstream.ConsumerConfig{
		Durable: "processor",
	})

	codec := json.NewCodec[Event]()
	sub := gofluxjs.NewSubscriber[Event](consumer, codec)

	// nil return = ack, non-nil return = nak (automatic).
	_ = sub.Subscribe(ctx, "events.>", func(ctx context.Context, msg goflux.Message[Event]) error {
		fmt.Printf("processing %s\n", msg.Payload.ID)
		return process(msg.Payload)
	})
}

func process(_ Event) error { return nil }
```

## Manual Ack

For fine-grained control, enable manual acknowledgment with `gofluxjs.WithManualAck()`. The handler is then responsible for calling one of the ack methods on every message.

```go
sub := gofluxjs.NewSubscriber[Event](consumer, codec, gofluxjs.WithManualAck())

_ = sub.Subscribe(ctx, "events.>", func(ctx context.Context, msg goflux.Message[Event]) error {
	if err := process(msg.Payload); err != nil {
		// Transient error: redeliver after a delay.
		return msg.NakWithDelay(5 * time.Second)
	}
	return msg.Ack()
})
```

### Acknowledgment Methods

| Method | Behavior |
|--------|----------|
| `msg.Ack()` | Confirm successful processing. Message is removed from the stream. |
| `msg.Nak()` | Signal failure. Message is redelivered immediately. |
| `msg.NakWithDelay(d)` | Signal failure with a redelivery delay hint. Use for transient errors (e.g., downstream timeout) to avoid tight retry loops. Falls back to `Nak()` if the transport does not support delayed redelivery. |
| `msg.Term()` | Terminate processing. Message will **not** be redelivered. Use for poison messages that can never succeed (e.g., malformed payload, business rule violation). Falls back to `Ack()` if the transport does not support terminal rejection. |

### Checking Acker Support

`msg.HasAcker()` returns `true` when the message carries acknowledgment controls (JetStream). For fire-and-forget transports (channel, NATS core) it returns `false`, and all ack methods are safe no-ops.

## AutoAck Middleware

If you switch a subscriber to manual ack but want auto-ack behavior for part of the handler chain, use the `middleware.AutoAck` middleware:

```go
import (
	"github.com/foomo/goflux"
	"github.com/foomo/goflux/middleware"
)

handler := middleware.AutoAck[Event]()(func(ctx context.Context, msg goflux.Message[Event]) error {
	// nil = ack, non-nil = nak — same as the default subscriber behavior.
	return process(msg.Payload)
})
```

`AutoAck` is a standard `Middleware[T]` and composes with `Chain`.

## When to Use

- **Business-critical events** -- order processing, payment notifications, audit logs.
- **Idempotent handlers** -- at-least-once means duplicates are possible; handlers should be idempotent.
- **Workflows with retry** -- combine `NakWithDelay` for transient failures and `Term` for dead-letter scenarios.
