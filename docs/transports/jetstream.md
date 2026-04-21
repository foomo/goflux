# JetStream Transport

Package `github.com/foomo/goflux/transport/jetstream`

The JetStream transport provides durable, acknowledged messaging on top of NATS JetStream. It supports both push and pull consumers through the unified `Subscriber[T]` interface, with auto or manual ack.

## Interfaces

| Interface | Implemented |
|-----------|-------------|
| `Publisher[T]` | Yes |
| `Subscriber[T]` | Yes |
| `Requester[Req, Resp]` | No (use NATS core) |
| `Responder[Req, Resp]` | No (use NATS core) |

## Publisher

```go
func NewPublisher[T any](js jetstream.JetStream, encoder goencode.Encoder[T, []byte], opts ...Option) *Publisher[T]
```

`Publish` encodes the value, injects OTel context and goflux headers into NATS message headers, and publishes to the JetStream stream. The JetStream server acknowledges persistence.

`Close` is a no-op. The caller owns the `jetstream.JetStream` handle and the underlying `*nats.Conn`.

## Subscriber

```go
func NewSubscriber[T any](consumer jetstream.Consumer, decoder goencode.Decoder[T, []byte], opts ...Option) *Subscriber[T]
```

`Subscribe` starts a consumption loop using the JetStream consumer's `Consume()` method, which works for both push and pull consumers. It blocks until the context is cancelled, then stops the consume context. Decode failures cause the message to be terminated (`msg.Term()`).

`Close` is a no-op.

### Auto-ack (default)

With the default configuration, acknowledgement is handled automatically:

- Handler returns `nil` -- message is acked.
- Handler returns an error -- message is naked (redelivered).

### Manual ack

With `WithManualAck()`, the handler is responsible for calling the appropriate method on the message:

- `msg.Ack()` -- acknowledge successful processing.
- `msg.Nak()` -- negative acknowledge, triggers redelivery.
- `msg.NakWithDelay(d)` -- negative acknowledge with a delay before redelivery.
- `msg.Term()` -- terminate the message (no further redelivery).

## Stream Helper

```go
func NewStream(ctx context.Context, nc *nats.Conn, cfg jetstream.StreamConfig, opts ...jetstream.JetStreamOpt) (jetstream.JetStream, error)
```

`NewStream` is a convenience function that creates a `jetstream.JetStream` handle and idempotently creates the stream described by `cfg`. If the stream already exists, the error is suppressed. The operation has a 10-second timeout.

## Options

| Option | Applies to | Description |
|--------|-----------|-------------|
| `WithTelemetry(t *goflux.Telemetry)` | All | Sets the OTel telemetry instance. A default is created from OTel globals if not provided. |
| `WithManualAck()` | Subscriber | Disables auto-ack. The handler must call `msg.Ack()`, `msg.Nak()`, `msg.NakWithDelay()`, or `msg.Term()` on every message. |

## Behavior

- **Caller owns connections** -- the caller is responsible for the `jetstream.JetStream` handle and the `jetstream.Consumer`. `Close()` on all types is a no-op.
- **Stream and consumer configuration** -- retention policy, delivery policy, replay policy, and other JetStream settings are configured via `jetstream.StreamConfig` and `jetstream.ConsumerConfig` directly. goflux does not abstract these.
- **OTel context propagation** -- uses span links (not parent-child) because async messaging is temporally decoupled. The producer's span context is extracted and attached as a link on the consumer span.
- **Message ID** -- if `goflux.MessageID(ctx)` is set, it is propagated via the `X-Message-ID` header.

## Push Subscriber with Auto-Ack

```go
package main

import (
	"context"
	"fmt"
	"log"

	"github.com/foomo/goencode/json"
	"github.com/foomo/goflux"
	gofluxjs "github.com/foomo/goflux/transport/jetstream"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
)

type Event struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	nc, err := nats.Connect(nats.DefaultURL)
	if err != nil {
		log.Fatal(err)
	}
	defer nc.Close()

	codec := json.NewCodec[Event]()

	// Create stream (idempotent).
	js, err := gofluxjs.NewStream(ctx, nc, jetstream.StreamConfig{
		Name:     "EVENTS",
		Subjects: []string{"events.>"},
	})
	if err != nil {
		log.Fatal(err)
	}

	// Create a durable consumer.
	cons, err := js.CreateOrUpdateConsumer(ctx, "EVENTS", jetstream.ConsumerConfig{
		Durable:       "event-processor",
		FilterSubject: "events.created",
	})
	if err != nil {
		log.Fatal(err)
	}

	// Publish a message.
	pub := gofluxjs.NewPublisher[Event](js, codec.Encode)
	if err := pub.Publish(ctx, "events.created", Event{ID: "1", Name: "signup"}); err != nil {
		log.Fatal(err)
	}

	// Subscribe with auto-ack (default).
	sub := gofluxjs.NewSubscriber[Event](cons, codec.Decode)
	go func() {
		_ = sub.Subscribe(ctx, "events.created", func(ctx context.Context, msg goflux.Message[Event]) error {
			fmt.Printf("received: %s %s\n", msg.Payload.ID, msg.Payload.Name)
			return nil // auto-acked
		})
	}()
}
```

## Manual Ack with NakWithDelay

```go
sub := gofluxjs.NewSubscriber[Event](cons, codec.Decode, gofluxjs.WithManualAck())

go func() {
	_ = sub.Subscribe(ctx, "events.created", func(ctx context.Context, msg goflux.Message[Event]) error {
		if err := process(msg.Payload); err != nil {
			if isRetryable(err) {
				// Redeliver after 5 seconds.
				return msg.NakWithDelay(5 * time.Second)
			}
			// Poison message -- terminate, no further redelivery.
			return msg.Term()
		}

		return msg.Ack()
	})
}()
```

## Pull Consumer with Subscriber

JetStream pull consumers use the same `Subscriber[T]` interface. Configure the JetStream consumer with `AckPolicy: jetstream.AckExplicitPolicy` and use `WithManualAck()` combined with middleware for ack control:

```go
cons, _ := js.CreateOrUpdateConsumer(ctx, "EVENTS", jetstream.ConsumerConfig{
	Durable:   "event-puller",
	AckPolicy: jetstream.AckExplicitPolicy,
})

codec := json.NewCodec[Event]()
sub := gofluxjs.NewSubscriber[Event](cons, codec.Decode, gofluxjs.WithManualAck())

// Use ToStream + goflow for bounded concurrency.
stream := goflux.ToStream[Event](ctx, sub, "events.created", 16)

policy := middleware.NewRetryPolicy(5 * time.Second)
go func() {
	_ = stream.Process(5, func(ctx context.Context, msg goflux.Message[Event]) error {
		fmt.Printf("pulled: %s %s\n", msg.Payload.ID, msg.Payload.Name)
		err := processEvent(msg.Payload)
		if err != nil {
			d := policy(err)
			switch d.Action {
			case middleware.RetryNakWithDelay:
				return msg.NakWithDelay(d.Delay)
			case middleware.RetryTerm:
				return msg.Term()
			default:
				return msg.Nak()
			}
		}
		return msg.Ack()
	})
}()
```
