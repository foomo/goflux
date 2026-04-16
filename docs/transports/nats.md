# NATS Transport

Package `github.com/foomo/goflux/transport/nats`

The NATS transport wraps a `*nats.Conn` for core NATS pub/sub and request-reply. It requires a `goencode.Codec[T]` for message serialization.

## Interfaces

| Interface | Implemented |
|-----------|-------------|
| `Publisher[T]` | Yes |
| `Subscriber[T]` | Yes |
| `Requester[Req, Resp]` | Yes |
| `Responder[Req, Resp]` | Yes |

## Publisher

```go
func NewPublisher[T any](conn *nats.Conn, codec goencode.Codec[T], opts ...Option) *Publisher[T]
```

`Publish` encodes the value with the codec, injects OTel context and goflux headers into NATS message headers, and publishes to the subject.

`Close` calls `conn.Drain()`.

## Subscriber

```go
func NewSubscriber[T any](conn *nats.Conn, codec goencode.Codec[T], opts ...Option) *Subscriber[T]
```

`Subscribe` registers a callback with the NATS connection and blocks until the context is cancelled, then unsubscribes. Decode failures are logged and the message is dropped.

`Close` calls `conn.Drain()`.

## Requester

```go
func NewRequester[Req, Resp any](
    conn *nats.Conn,
    reqCodec goencode.Codec[Req],
    respCodec goencode.Codec[Resp],
    opts ...Option,
) *Requester[Req, Resp]
```

`Request` encodes the request, publishes it to the subject using NATS request-reply, waits for a response, decodes it, and returns the result. The call respects context cancellation.

`Close` calls `conn.Drain()`.

## Responder

```go
func NewResponder[Req, Resp any](
    conn *nats.Conn,
    reqCodec goencode.Codec[Req],
    respCodec goencode.Codec[Resp],
    opts ...Option,
) *Responder[Req, Resp]
```

`Serve` subscribes to the subject, decodes incoming requests, passes them to the `goflux.RequestHandler[Req, Resp]`, encodes the response, and sends it back via NATS reply. The call blocks until the context is cancelled.

`Close` calls `conn.Drain()`.

## Options

| Option | Applies to | Description |
|--------|-----------|-------------|
| `WithTelemetry(t *goflux.Telemetry)` | All | Sets the OTel telemetry instance. A default is created from OTel globals if not provided. |
| `WithQueueGroup(name string)` | Subscriber | Joins a named queue group, turning the subscription into a competing consumer. Each message is delivered to only one member of the group. |

## Behavior

- **Caller owns the connection** -- the caller is responsible for connecting and closing `*nats.Conn`. `Close()` on Publisher/Subscriber calls `conn.Drain()`.
- **Fire-and-forget** -- core NATS has no ack/nak. `Message.Acker` is nil.
- **OTel context propagation** -- uses span links (not parent-child) because async messaging is temporally decoupled. The producer's span context is extracted via `ExtractSpanContext` and attached as a link on the consumer span.
- **Header carrier** -- a custom `natsHeaderCarrier` preserves raw key casing (unlike `http.Header` which canonicalizes keys), ensuring W3C TraceContext lowercase keys survive the round-trip.
- **Message ID** -- if `goflux.MessageID(ctx)` is set, it is propagated via the `X-Message-ID` header.

## Pub/Sub Example

```go
package main

import (
	"context"
	"fmt"
	"log"

	"github.com/foomo/goencode/json"
	"github.com/foomo/goflux"
	gofluxnats "github.com/foomo/goflux/transport/nats"
	"github.com/nats-io/nats.go"
)

type Event struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	conn, err := nats.Connect(nats.DefaultURL)
	if err != nil {
		log.Fatal(err)
	}
	defer conn.Close()

	codec := json.NewCodec[Event]()

	pub := gofluxnats.NewPublisher[Event](conn, codec)
	sub := gofluxnats.NewSubscriber[Event](conn, codec)

	// Subscribe in a goroutine -- Subscribe blocks until ctx is cancelled.
	go func() {
		_ = sub.Subscribe(ctx, "events.created", func(ctx context.Context, msg goflux.Message[Event]) error {
			fmt.Printf("received: %s %s\n", msg.Payload.ID, msg.Payload.Name)
			return nil
		})
	}()

	// Publish a message.
	if err := pub.Publish(ctx, "events.created", Event{ID: "1", Name: "signup"}); err != nil {
		log.Fatal(err)
	}
}
```

## Request-Reply Example

```go
package main

import (
	"context"
	"fmt"
	"log"

	"github.com/foomo/goencode/json"
	gofluxnats "github.com/foomo/goflux/transport/nats"
	"github.com/nats-io/nats.go"
)

type OrderRequest struct {
	ItemID string `json:"item_id"`
	Qty    int    `json:"qty"`
}

type OrderResponse struct {
	OrderID string `json:"order_id"`
	Status  string `json:"status"`
}

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	conn, err := nats.Connect(nats.DefaultURL)
	if err != nil {
		log.Fatal(err)
	}
	defer conn.Close()

	reqCodec := json.NewCodec[OrderRequest]()
	respCodec := json.NewCodec[OrderResponse]()

	// Start responder in a goroutine.
	responder := gofluxnats.NewResponder[OrderRequest, OrderResponse](conn, reqCodec, respCodec)
	go func() {
		_ = responder.Serve(ctx, "orders.create", func(ctx context.Context, req OrderRequest) (OrderResponse, error) {
			return OrderResponse{OrderID: "ord-42", Status: "created"}, nil
		})
	}()

	// Send a request.
	requester := gofluxnats.NewRequester[OrderRequest, OrderResponse](conn, reqCodec, respCodec)
	resp, err := requester.Request(ctx, "orders.create", OrderRequest{ItemID: "sku-1", Qty: 3})
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("order: %s status: %s\n", resp.OrderID, resp.Status)
}
```
