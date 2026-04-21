# Queue Groups

Queue groups turn a set of subscribers into competing consumers: each message on a subject is delivered to exactly one member of the group. This enables horizontal scaling of message processing without duplicating work.

## How It Works

When multiple subscribers join the same queue group on the same subject, NATS distributes each incoming message to only one subscriber in the group. Subscribers that are not part of a queue group each receive every message independently.

## NATS Core

Use `gofluxnats.WithQueueGroup` to join a named queue group:

```go
package main

import (
	"context"
	"fmt"

	json "github.com/foomo/goencode/json/v1"
	"github.com/foomo/goflux"
	gofluxnats "github.com/foomo/goflux/transport/nats"
	"github.com/nats-io/nats.go"
)

type Task struct {
	ID   string
	Data string
}

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	conn, _ := nats.Connect(nats.DefaultURL)
	defer conn.Drain()

	codec := json.NewCodec[Task]()

	// Both subscribers join the "workers" queue group.
	// Each task is delivered to exactly one of them.
	sub1 := gofluxnats.NewSubscriber[Task](conn, codec.Decode, gofluxnats.WithQueueGroup("workers"))
	sub2 := gofluxnats.NewSubscriber[Task](conn, codec.Decode, gofluxnats.WithQueueGroup("workers"))

	handler := func(ctx context.Context, msg goflux.Message[Task]) error {
		fmt.Printf("processing task %s\n", msg.Payload.ID)
		return nil
	}

	go func() { _ = sub1.Subscribe(ctx, "tasks.>", handler) }()
	go func() { _ = sub2.Subscribe(ctx, "tasks.>", handler) }()

	// Publish tasks — each one goes to exactly one worker.
	pub := gofluxnats.NewPublisher[Task](conn, codec.Encode)
	for i := 0; i < 10; i++ {
		_ = pub.Publish(ctx, "tasks.process", Task{
			ID:   fmt.Sprintf("task-%d", i),
			Data: "payload",
		})
	}

	cancel()
}
```

## Multiple Instances

In a typical deployment, each service instance creates one subscriber with the same queue group name. NATS handles the distribution across all connected instances automatically:

```go
// Instance A
sub := gofluxnats.NewSubscriber[Task](conn, codec.Decode, gofluxnats.WithQueueGroup("order-processor"))
_ = sub.Subscribe(ctx, "orders.>", handler)

// Instance B (separate process, same queue group name)
sub := gofluxnats.NewSubscriber[Task](conn, codec.Decode, gofluxnats.WithQueueGroup("order-processor"))
_ = sub.Subscribe(ctx, "orders.>", handler)
```

## Scope

Queue groups are a transport-level concept, not a core goflux abstraction. The `WithQueueGroup` option is specific to the NATS transport package. Other transports achieve competing-consumer behavior through their own mechanisms (e.g., JetStream consumer groups are configured at the `jetstream.ConsumerConfig` level, not via goflux options).

## When to Use

- **Horizontal scaling** -- distribute work across N instances of the same service.
- **Load balancing** -- NATS distributes messages evenly across group members.
- **Preventing duplicate processing** -- each message is handled by exactly one consumer in the group.
