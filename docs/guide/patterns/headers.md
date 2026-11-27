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

::: code-group

<div class="vp-block active" data-title="Diagram"><img src="../../diagrams/headers.svg" alt="The publisher reads X-Tenant-ID from the context header; NATS and HTTP carry it as X-Goflux-X-Tenant-ID; the subscriber strips the prefix and the handler reads X-Tenant-ID from msg.Header"></div>

```go [Code]
package main

import (
	"context"

	json "github.com/foomo/goencode/json/v1"
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

	codec := json.NewCodec[OrderEvent]()
	pub := gofluxnats.NewPublisher[OrderEvent](conn, codec.Encode)

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

:::

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

goflux headers are sent as NATS headers with the `X-Goflux-` prefix, like on HTTP. The subscriber extracts only the prefixed keys, strips the prefix, and exposes them as `msg.Header`. Trace context and message ID headers are not part of `msg.Header`.

```go
// Publisher sends: X-Goflux-X-Tenant-ID: acme
// Subscriber receives: msg.Header = {"X-Tenant-ID": ["acme"]}
```

### HTTP

goflux headers are prefixed with `X-Goflux-` when sent as HTTP headers. The subscriber strips the prefix when extracting them back into `msg.Header`.

```go
// Publisher sends: X-Goflux-X-Tenant-ID: acme (canonicalized to X-Goflux-X-Tenant-Id)
// Subscriber receives: msg.Header = {"X-Tenant-Id": ["acme"]}
// msg.Header.Get("X-Tenant-ID") still returns "acme" (case-insensitive fallback)
```

HTTP changes the case of header keys, so read headers with `Get` instead of indexing the map.

This prefixing avoids collisions with standard HTTP headers and transport-level headers (Content-Type, trace context, etc.).

## Automatic Propagation

Trace context (W3C Trace Context) and message ID (`X-Message-ID`) are sent automatically by all transports (the in-process channel transport passes them with the message instead of as headers). They do not flow through the `Header` field -- they use dedicated mechanisms:

```go
// Message ID — set via context, not Header.
ctx = goflux.WithMessageID(ctx, "msg-abc-123")
pub.Publish(ctx, "events.created", event)

// On the subscriber side:
id := msg.ID // "msg-abc-123"
```

Trace context flows on to messages published from the handler; the message ID does not (see [Message ID](/integrations/opentelemetry#message-id)).

You do not need to manually add trace context or message IDs to the `Header` map.
