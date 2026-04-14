# Fan-Out & Fan-In

goflux provides three composition operators that combine multiple publishers or subscribers into a single interface. These are transport-agnostic -- they work with any `Publisher[T]` or `Subscriber[T]` implementation.

## FanOut

`FanOut` broadcasts every published message to all inner publishers.

```go
func FanOut[T any](publishers []Publisher[T], opts ...FanOutOption[T]) Publisher[T]
```

### Best-Effort (Default)

By default, `FanOut` publishes to every inner publisher and joins any errors. A failure in one publisher does not prevent delivery to the others.

```go
package main

import (
	"context"
	"log"

	"github.com/foomo/goencode"
	"github.com/foomo/goflux"
	gofluxnats "github.com/foomo/goflux/transport/nats"
	"github.com/nats-io/nats.go"
)

type AuditEvent struct {
	Action string
	UserID string
}

func main() {
	ctx := context.Background()

	conn, _ := nats.Connect(nats.DefaultURL)
	defer conn.Drain()

	codec := goencode.NewJSONCodec[AuditEvent]()

	pubA := gofluxnats.NewPublisher[AuditEvent](conn, codec)
	pubB := gofluxnats.NewPublisher[AuditEvent](conn, codec)

	// Broadcast to both publishers. Errors are joined.
	fan := goflux.FanOut[AuditEvent]([]goflux.Publisher[AuditEvent]{pubA, pubB})

	if err := fan.Publish(ctx, "audit.login", AuditEvent{
		Action: "login",
		UserID: "user-42",
	}); err != nil {
		log.Printf("partial failure: %v", err)
	}
}
```

### All-or-Nothing

With `WithFanOutAllOrNothing`, the first publisher error aborts the entire call immediately.

```go
fan := goflux.FanOut[AuditEvent](
	[]goflux.Publisher[AuditEvent]{pubA, pubB},
	goflux.WithFanOutAllOrNothing[AuditEvent](),
)
```

## FanIn

`FanIn` merges multiple subscribers into one. A single handler receives messages from all inner subscribers.

```go
func FanIn[T any](subscribers ...Subscriber[T]) Subscriber[T]
```

`Subscribe` blocks until all inner subscriptions complete (all contexts cancelled or all return an error).

```go
package main

import (
	"context"
	"fmt"

	"github.com/foomo/goencode"
	"github.com/foomo/goflux"
	gofluxnats "github.com/foomo/goflux/transport/nats"
	"github.com/nats-io/nats.go"
)

type Metric struct {
	Name  string
	Value float64
}

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	connA, _ := nats.Connect("nats://cluster-a:4222")
	defer connA.Drain()
	connB, _ := nats.Connect("nats://cluster-b:4222")
	defer connB.Drain()

	codec := goencode.NewJSONCodec[Metric]()

	subA := gofluxnats.NewSubscriber[Metric](connA, codec)
	subB := gofluxnats.NewSubscriber[Metric](connB, codec)

	// Merge both subscribers into one.
	merged := goflux.FanIn[Metric](subA, subB)

	// A single handler receives messages from both clusters.
	_ = merged.Subscribe(ctx, "metrics.>", func(ctx context.Context, msg goflux.Message[Metric]) error {
		fmt.Printf("%s = %.2f\n", msg.Payload.Name, msg.Payload.Value)
		return nil
	})
}
```

## RoundRobin

`RoundRobin` distributes each published message to a single inner publisher, cycling through them in order using an atomic counter.

```go
func RoundRobin[T any](publishers ...Publisher[T]) Publisher[T]
```

```go
package main

import (
	"context"
	"fmt"

	"github.com/foomo/goencode"
	"github.com/foomo/goflux"
	gofluxnats "github.com/foomo/goflux/transport/nats"
	"github.com/nats-io/nats.go"
)

type Job struct {
	ID string
}

func main() {
	ctx := context.Background()

	conn, _ := nats.Connect(nats.DefaultURL)
	defer conn.Drain()

	codec := goencode.NewJSONCodec[Job]()

	pub1 := gofluxnats.NewPublisher[Job](conn, codec)
	pub2 := gofluxnats.NewPublisher[Job](conn, codec)
	pub3 := gofluxnats.NewPublisher[Job](conn, codec)

	rr := goflux.RoundRobin[Job](pub1, pub2, pub3)

	// Messages are distributed: pub1, pub2, pub3, pub1, pub2, ...
	for i := 0; i < 6; i++ {
		_ = rr.Publish(ctx, "jobs.run", Job{ID: fmt.Sprintf("job-%d", i)})
	}
}
```

## Ownership and Close

`Close()` is a no-op on all three composed types (`FanOut`, `FanIn`, `RoundRobin`). The caller owns the inner publishers and subscribers and is responsible for closing them independently.

```go
fan := goflux.FanOut[Event]([]goflux.Publisher[Event]{pubA, pubB})
defer fan.Close() // no-op

// Caller must close the inner publishers.
defer pubA.Close()
defer pubB.Close()
```
