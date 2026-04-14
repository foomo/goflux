# JetStream Transport

Package `github.com/foomo/goflux/transport/jetstream`

The JetStream transport provides durable, acknowledged messaging on top of NATS JetStream. It supports push-based subscription with auto or manual ack, and pull-based consumption via `Consumer[T]`.

## Interfaces

| Interface | Implemented |
|-----------|-------------|
| `Publisher[T]` | Yes |
| `Subscriber[T]` | Yes |
| `Consumer[T]` | Yes |
| `Requester[Req, Resp]` | No (use NATS core) |
| `Responder[Req, Resp]` | No (use NATS core) |

## Publisher

```go
func NewPublisher[T any](js jetstream.JetStream, codec goencode.Codec[T], opts ...Option) *Publisher[T]
```

`Publish` encodes the value, injects OTel context and goflux headers into NATS message headers, and publishes to the JetStream stream. The JetStream server acknowledges persistence.

`Close` is a no-op. The caller owns the `jetstream.JetStream` handle and the underlying `*nats.Conn`.

## Subscriber (push-based)

```go
func NewSubscriber[T any](consumer jetstream.Consumer, codec goencode.Codec[T], opts ...Option) *Subscriber[T]
```

`Subscribe` starts a push-based consumption loop. It blocks until the context is cancelled, then stops the consume context. Decode failures cause the message to be terminated (`msg.Term()`).

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

## Consumer (pull-based)

```go
func NewConsumer[T any](consumer jetstream.Consumer, codec goencode.Codec[T], opts ...Option) *Consumer[T]
```

`Fetch(ctx, n)` pulls up to `n` messages from the JetStream consumer. It blocks until at least one message is available or the context is cancelled. Each returned message **must** be explicitly acked. Decode failures are logged and the message is terminated.

`Close` is a no-op.

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

- **Caller owns connections** -- the caller is responsible for the `jetstream.JetStream` handle and `jetstream.Consumer`. `Close()` on all types is a no-op.
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
	pub := gofluxjs.NewPublisher[Event](js, codec)
	if err := pub.Publish(ctx, "events.created", Event{ID: "1", Name: "signup"}); err != nil {
		log.Fatal(err)
	}

	// Subscribe with auto-ack (default).
	sub := gofluxjs.NewSubscriber[Event](cons, codec)
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
sub := gofluxjs.NewSubscriber[Event](cons, codec, gofluxjs.WithManualAck())

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

## Pull Consumer with Fetch

```go
codec := json.NewCodec[Event]()
consumer := gofluxjs.NewConsumer[Event](cons, codec)

// Pull up to 10 messages.
msgs, err := consumer.Fetch(ctx, 10)
if err != nil {
	log.Fatal(err)
}

for _, msg := range msgs {
	fmt.Printf("pulled: %s %s\n", msg.Payload.ID, msg.Payload.Name)

	// Each message must be explicitly acked.
	if err := msg.Ack(); err != nil {
		log.Printf("ack failed: %v", err)
	}
}
```
