# Pull Consumer

JetStream pull consumers work through the same `Subscriber[T]` interface as push consumers. The nats.go library's `jetstream.Consumer.Consume()` method handles both push and pull delivery modes -- goflux's `Subscriber[T]` wraps this uniformly. The difference is in the JetStream consumer configuration, not the goflux API.

## How It Works

A JetStream consumer configured with `AckPolicy: jetstream.AckExplicitPolicy` operates in pull mode. The goflux `Subscriber[T]` calls `consumer.Consume()` internally, which works for both push and pull consumers. Combine this with middleware for concurrency control, auto-ack, and retry policies.

## JetStream Pull Consumer with Middleware

```go
package main

import (
	"context"
	"fmt"
	"log"
	"time"

	json "github.com/foomo/goencode/json/v1"
	"github.com/foomo/goflux"
	"github.com/foomo/goflux/middleware"
	gofluxjs "github.com/foomo/goflux/transport/jetstream"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
)

type Task struct {
	ID      string
	Payload string
}

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	nc, _ := nats.Connect(nats.DefaultURL)
	defer nc.Drain()

	js, _ := jetstream.New(nc)
	stream, _ := js.CreateOrUpdateStream(ctx, jetstream.StreamConfig{
		Name:     "TASKS",
		Subjects: []string{"tasks.>"},
	})

	// Pull consumer with explicit ack policy.
	cons, _ := stream.CreateOrUpdateConsumer(ctx, jetstream.ConsumerConfig{
		Durable:   "batch-worker",
		AckPolicy: jetstream.AckExplicitPolicy,
	})

	codec := json.NewCodec[Task]()

	// Create a subscriber from the pull consumer -- same interface as push.
	sub := gofluxjs.NewSubscriber[Task](cons, codec, gofluxjs.WithManualAck())

	// Use ToStream to bridge into goflow for stream processing.
	stream := goflux.ToStream[Task](ctx, sub, "tasks.>", 16)

	// Process with bounded concurrency via goflow.
	policy := middleware.NewRetryPolicy(5 * time.Second)
	if err := stream.Process(5, func(ctx context.Context, msg goflux.Message[Task]) error {
		fmt.Printf("processing task %s\n", msg.Payload.ID)
		err := processTask(msg.Payload)
		if err != nil {
			d := policy(err)
			switch d.Action {
			case middleware.RetryNak:
				return msg.Nak()
			case middleware.RetryNakWithDelay:
				return msg.NakWithDelay(d.Delay)
			case middleware.RetryTerm:
				return msg.Term()
			}
		}
		return msg.Ack()
	}); err != nil {
		log.Fatal(err)
	}
}

func processTask(_ Task) error { return nil }
```

## Processing Patterns

### Stream-based (goflow)

Use `ToStream` + goflow `Process` for bounded concurrency with stream operators:

```go
stream := goflux.ToStream[Task](ctx, sub, "tasks.>", 16)

stream.Process(10, func(ctx context.Context, msg goflux.Message[Task]) error {
    err := myHandler(ctx, msg)
    if err != nil {
        return msg.Nak()
    }
    return msg.Ack()
})
```

### Handler-based (middleware)

Use `Chain` + middleware for simple auto-ack or retry-aware ack:

```go
// Simple auto-ack.
handler := goflux.Chain[Task](
	middleware.AutoAck[Task](),
)(myHandler)

// Retry-aware ack with custom policy.
policy := middleware.RetryPolicy(func(err error) middleware.RetryDecision {
	if goflux.IsNonRetryable(err) {
		return middleware.RetryDecision{Action: middleware.RetryTerm}
	}
	return middleware.RetryDecision{Action: middleware.RetryNakWithDelay, Delay: 5 * time.Second}
})

handler := goflux.Chain[Task](
	middleware.RetryAck[Task](policy),
)(myHandler)
```

## Key Rules

- **Use `WithManualAck()`.** When composing ack behavior via middleware (`AutoAck` or `RetryAck`), create the subscriber with `WithManualAck()` to prevent the transport from double-acking.
- **Decode failures are terminated.** The subscriber automatically calls `Term()` on messages that fail codec decoding, preventing infinite redelivery of malformed data.
- **Caller owns the connection.** `Subscriber.Close()` is a no-op. The caller is responsible for draining the NATS connection.

## When to Use

- **Backpressure control** -- the consumer configuration controls delivery rate instead of being overwhelmed by unbounded push delivery.
- **Worker-paced consumption** -- useful when processing time varies widely and you want bounded concurrency.
- **Policy-driven retry** -- classify errors into retry/term without embedding ack logic in handlers.

For push-based delivery with automatic acknowledgment, see [at-least-once](./at-least-once).
