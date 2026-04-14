# Pull Consumer

The pull consumer pattern gives the application explicit control over when and how many messages are fetched. Instead of the broker pushing messages to a callback, the consumer calls `Fetch` to pull a batch on demand. Every fetched message must be explicitly acknowledged.

## The Consumer Interface

```go
type Consumer[T any] interface {
	Fetch(ctx context.Context, n int) ([]Message[T], error)
	Close() error
}
```

`Fetch` retrieves up to `n` messages. It blocks until at least one message is available or `ctx` is cancelled. The returned messages carry an acker -- you must call `Ack()`, `Nak()`, or `Term()` on each one.

## JetStream Implementation

Only the JetStream transport implements `Consumer[T]`. It wraps a `jetstream.Consumer` and decodes each message through a `goencode.Codec`.

```go
package main

import (
	"context"
	"log"

	"github.com/foomo/goencode"
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
	consumer, _ := stream.CreateOrUpdateConsumer(ctx, jetstream.ConsumerConfig{
		Durable: "batch-worker",
	})

	codec := goencode.NewJSONCodec[Task]()
	pull := gofluxjs.NewConsumer[Task](consumer, codec)

	// Fetch up to 10 messages at a time.
	msgs, err := pull.Fetch(ctx, 10)
	if err != nil {
		log.Fatal(err)
	}

	for _, msg := range msgs {
		if err := processTask(msg.Payload); err != nil {
			_ = msg.Nak() // redeliver
			continue
		}
		_ = msg.Ack() // done
	}
}

func processTask(_ Task) error { return nil }
```

## Batch Processing Loop

A typical worker loops over `Fetch` until the context is cancelled:

```go
func runWorker(ctx context.Context, pull goflux.Consumer[Task]) error {
	for {
		msgs, err := pull.Fetch(ctx, 50)
		if err != nil {
			return err // context cancelled or fatal error
		}

		for _, msg := range msgs {
			if err := processTask(msg.Payload); err != nil {
				if isTransient(err) {
					_ = msg.NakWithDelay(5 * time.Second)
				} else {
					_ = msg.Term() // poison message, do not redeliver
				}
				continue
			}
			_ = msg.Ack()
		}
	}
}
```

## Key Rules

- **Always ack.** Every message returned by `Fetch` must be explicitly acknowledged. Unacked messages will eventually be redelivered by JetStream after the ack-wait timeout.
- **Decode failures are terminated.** The consumer automatically calls `Term()` on messages that fail codec decoding, preventing infinite redelivery of malformed data.
- **Caller owns the connection.** `Consumer.Close()` is a no-op. The caller is responsible for draining the NATS connection.

## When to Use

- **Batch processing** -- aggregate N messages before writing to a database or sending a bulk API call.
- **Backpressure control** -- the consumer decides its own pace instead of being overwhelmed by push delivery.
- **Worker-paced consumption** -- useful when processing time varies widely and you want to avoid buffering messages in memory.

For push-based delivery with automatic acknowledgment, see [at-least-once](./at-least-once).
