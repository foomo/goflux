# NATS Subjects

Package `github.com/foomo/goflux/subject/nats`

The NATS subjects package provides a type-safe builder for constructing hierarchical NATS subject strings. Instead of assembling dot-delimited strings by hand, you chain typed segments that are validated at construction time.

## Builder Chain

Every NATS subject follows a four-level hierarchy:

```
Subject  →  Domain  →  Entity  →  Event
(prefix)    (domain)   (entity)   (event)
```

Each level produces a valid NATS subject string via `String()`. The chain is immutable -- each call returns a new value.

## Subject (Prefix)

`Subject` holds zero or more leading segments -- environment, tenant, region, or any other prefix you need.

```go
func New(segments ...string) Subject
```

Zero segments are valid. `Domain` becomes the first token in the subject.

```go
// With prefix segments:
s := nats.New("prod", "acme")
s.String() // "prod.acme"

// No prefix:
s := nats.New()
s.String() // ""
```

## Domain

`Domain` represents the domain segment. Create it from a `Subject` or directly with `NewDomain`:

```go
func (p Subject) Domain(name string) Domain
func NewDomain(name string) Domain
```

```go
// From a prefix:
d := nats.New("prod").Domain("order")
d.String() // "prod.order"

// Direct -- no prefix needed:
d := nats.NewDomain("order")
d.String() // "order"
```

### Wildcard

`All()` returns a NATS `>` wildcard matching everything under the domain:

```go
d.All() // "prod.order.>"
```

## Entity

`Entity` represents the entity segment within a domain:

```go
func (d Domain) Entity(name string) Entity
```

```go
e := nats.New("prod").Domain("order").Entity("item")
e.String() // "prod.order.item"
e.All()    // "prod.order.item.>"
```

## Event

`Event` is the terminal segment -- a complete, publishable subject:

```go
func (e Entity) Event(name string) Event
```

```go
ev := nats.New("prod").Domain("order").Entity("item").Event("created")
ev.String() // "prod.order.item.created"
```

## Validation

Every segment is validated at construction time. Invalid segments cause a panic:

| Rule | Example |
|------|---------|
| Must not be empty | `""` panics |
| Must not contain `.` | `"foo.bar"` panics |
| Must not contain `*` or `>` | `"events.*"` panics |
| Must not contain whitespace | `"my event"` panics |

Panics are intentional -- subjects are typically built at init time, so invalid segments surface immediately during startup rather than silently producing broken routing at runtime.

## Usage with goflux

Subjects integrate naturally with `Publisher` and `Subscriber` since both accept a `string` subject:

```go
package main

import (
	"context"
	"log"

	"github.com/foomo/goencode/json"
	"github.com/foomo/goflux"
	gofluxnats "github.com/foomo/goflux/transport/nats"
	subjnats "github.com/foomo/goflux/subject/nats"
	"github.com/nats-io/nats.go"
)

type OrderEvent struct {
	OrderID string `json:"order_id"`
	Total   float64 `json:"total"`
}

// Define subjects once, reuse everywhere.
var (
	prefix = subjnats.New("prod", "acme")
	orders = prefix.Domain("order")
	orderItem = orders.Entity("item")

	orderItemCreated = orderItem.Event("created")
	orderItemUpdated = orderItem.Event("updated")
)

func main() {
	ctx := context.Background()

	conn, err := nats.Connect(nats.DefaultURL)
	if err != nil {
		log.Fatal(err)
	}
	defer conn.Close()

	codec := json.NewCodec[OrderEvent]()
	pub := gofluxnats.NewPublisher[OrderEvent](conn, codec)
	sub := gofluxnats.NewSubscriber[OrderEvent](conn, codec)

	// Subscribe to a single event.
	go func() {
		_ = sub.Subscribe(ctx, orderItemCreated.String(), func(ctx context.Context, msg goflux.Message[OrderEvent]) error {
			log.Printf("item created: %s", msg.Payload.OrderID)
			return nil
		})
	}()

	// Subscribe to all events under an entity.
	go func() {
		_ = sub.Subscribe(ctx, orderItem.All(), func(ctx context.Context, msg goflux.Message[OrderEvent]) error {
			log.Printf("item event on %s: %s", msg.Subject, msg.Payload.OrderID)
			return nil
		})
	}()

	// Publish.
	_ = pub.Publish(ctx, orderItemCreated.String(), OrderEvent{OrderID: "42", Total: 99.95})
}
```

## Wildcard Subscriptions

Use `All()` at the domain or entity level to subscribe to broad subject hierarchies:

```go
// All events in the order domain (items, payments, shipments, ...):
orders.All() // "prod.acme.order.>"

// All events for order items specifically:
orderItem.All() // "prod.acme.order.item.>"
```

This pairs well with fan-out patterns where a single subscriber processes events across an entire domain.

## When to Use

- **Multi-tenant systems** -- encode environment and tenant in the prefix, share domain/entity/event definitions across services
- **Consistent naming** -- the builder enforces a uniform `prefix.domain.entity.event` structure
- **Typo prevention** -- define subjects as package-level variables; the compiler catches misspellings
- **Wildcard routing** -- `All()` gives you correct `>` wildcards without manual string assembly
