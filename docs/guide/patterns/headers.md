# Headers

`goflux.Header` carries metadata alongside a message's payload. It follows `http.Header` semantics: keys are case-sensitive strings, values are string slices.

## The Header Type

```go
type Header map[string][]string
```

| Method | Description |
|--------|-------------|
| `Get(key) string` | Returns the first value for the key, or `""` if absent. |
| `Set(key, value)` | Replaces any existing values for the key. |
| `Add(key, value)` | Appends a value to the key's slice. |
| `Del(key)` | Removes the key entirely. |
| `Clone() Header` | Returns a deep copy. |

## Publishing with Headers

Attach headers to outgoing messages by storing them in the context with `goflux.WithHeader`:

```go
package main

import (
	"context"

	"github.com/foomo/goencode"
	"github.com/foomo/goflux"
	gofluxnats "github.com/foomo/goflux/transport/nats"
	"github.com/nats-io/nats.go"
)

type OrderEvent struct {
	OrderID string
	Action  string
}

func main() {
	conn, _ := nats.Connect(nats.DefaultURL)
	defer conn.Drain()

	codec := goencode.NewJSONCodec[OrderEvent]()
	pub := gofluxnats.NewPublisher[OrderEvent](conn, codec)

	ctx := goflux.WithHeader(context.Background(), goflux.Header{
		"X-Tenant-ID": {"acme"},
		"X-Region":    {"eu-west-1"},
	})

	_ = pub.Publish(ctx, "orders.created", OrderEvent{
		OrderID: "ord-123",
		Action:  "created",
	})
}
```

## Reading Headers in a Handler

Headers arrive on `msg.Header`:

```go
func handler(ctx context.Context, msg goflux.Message[OrderEvent]) error {
	tenant := msg.Header.Get("X-Tenant-ID")
	region := msg.Header.Get("X-Region")

	fmt.Printf("tenant=%s region=%s order=%s\n", tenant, region, msg.Payload.OrderID)
	return nil
}
```

## Transport Behavior

Each transport handles headers differently at the wire level, but the programming model is the same.

### Channel

Headers pass through in-memory as part of the `Message` struct. No serialization or transformation occurs.

```go
bus := channel.NewBus[OrderEvent]()
pub := channel.NewPublisher[OrderEvent](bus)

ctx := goflux.WithHeader(ctx, goflux.Header{"X-Tenant-ID": {"acme"}})
_ = pub.Publish(ctx, "orders.created", event)
// Subscriber receives msg.Header with {"X-Tenant-ID": ["acme"]}
```

### NATS and JetStream

Headers from the context are merged directly into the NATS message headers. On the subscriber side, the full NATS header map is exposed as `msg.Header`.

```go
// Publisher side: goflux.Header values are added to nats.Header
// Subscriber side: msg.Header contains the full nats.Header
```

### HTTP

goflux headers are prefixed with `X-Goflux-` when sent as HTTP headers. The subscriber strips the prefix when extracting them back into `msg.Header`.

```go
// Publisher sends: X-Goflux-X-Tenant-ID: acme
// Subscriber receives: msg.Header = {"X-Tenant-ID": ["acme"]}
```

This prefixing avoids collisions with standard HTTP headers and transport-level headers (Content-Type, trace context, etc.).

## Automatic Propagation

Trace context (W3C Trace Context) and message ID (`X-Message-ID`) are propagated automatically by the transports. They do not flow through the `Header` field -- they use dedicated context keys and transport-specific mechanisms:

```go
// Message ID — set via context, not Header.
ctx = goflux.WithMessageID(ctx, "msg-abc-123")
pub.Publish(ctx, "events.created", event)

// On the subscriber side:
id := goflux.MessageID(ctx) // "msg-abc-123"
```

You do not need to manually add trace context or message IDs to the `Header` map.
