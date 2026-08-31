# NATS Transport

Package `github.com/foomo/goflux/transport/nats`

The NATS transport wraps a `*nats.Conn` for core NATS pub/sub and request-reply. Publishers take a `goencode.Encoder[T, []byte]`, subscribers take a `goencode.Decoder[T, []byte]`.

## Interfaces

| Interface | Implemented |
|-----------|-------------|
| `Publisher[T]` | Yes |
| `Subscriber[T]` | Yes |
| `Requester[Req, Resp]` | Yes |
| `Responder[Req, Resp]` | Yes |

## Publisher

```go
func NewPublisher[T any](conn *nats.Conn, encoder goencode.Encoder[T, []byte], opts ...Option) *Publisher[T]
```

`Publish` encodes the value with the encoder, injects OTel context and goflux headers into NATS message headers, and publishes to the subject.

`Close` calls `conn.Drain()`.

## Subscriber

```go
func NewSubscriber[T any](conn *nats.Conn, decoder goencode.Decoder[T, []byte], opts ...Option) *Subscriber[T]
```

`Subscribe` registers a callback with the NATS connection and blocks until the context is cancelled, then unsubscribes. Decode failures are logged and the message is dropped.

```go
func (s *Subscriber[T]) SubscribeWithReady(ctx context.Context, subject string, handler goflux.Handler[T], ready func()) error
```

`SubscribeWithReady` behaves like `Subscribe` but invokes `ready` once the **server** has acknowledged the subscription. This matters more here than on any other transport: `nats.Conn.Subscribe` only buffers the SUB frame client-side, so its return proves nothing about server state, and core NATS discards messages that have no matching subscriber -- silently, with no error on either side. `SubscribeWithReady` forces a `Flush` round-trip before calling `ready`.

::: warning
Use `SubscribeWithReady` whenever a publish is sequenced after a subscribe. With plain `Subscribe` the publish can reach the server first and the message is lost without a trace.
:::

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

```go
func (r *Responder[Req, Resp]) ServeWithReady(ctx context.Context, subject string, handler goflux.RequestHandler[Req, Resp], ready func()) error
```

`ServeWithReady` invokes `ready` once the server has acknowledged the responder's subscription, using the same `Flush` round-trip as the Subscriber. Without it, a request issued immediately after `Serve` starts can be dropped -- which the caller sees only as a request timeout, never naming the real cause.

`Close` calls `conn.Drain()`.

## Options

| Option | Applies to | Description |
|--------|-----------|-------------|
| `WithTelemetry(t *goflux.Telemetry)` | All | Sets the OTel telemetry instance. A default is created from OTel globals if not provided. |
| `WithQueueGroup(name string)` | Subscriber | Joins a named queue group, turning the subscription into a competing consumer. Each message is delivered to only one member of the group. |

## Behavior

- **Caller owns the connection** -- the caller is responsible for connecting and closing `*nats.Conn`. `Close()` on Publisher/Subscriber calls `conn.Drain()`.
- **Fire-and-forget** -- core NATS has no ack/nak. `Message.Acker` is nil. A message published with no matching subscriber is dropped silently, which is why readiness matters.
- **Readiness via flush** -- `SubscribeWithReady` and `ServeWithReady` issue a `Flush` round-trip so `ready` means "the server registered the subscription", not "a frame was buffered locally". `Flush` is used rather than `FlushWithContext` because the subscription's lifetime context legitimately has no deadline; the connection timeout bounds the call instead.
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

	pub := gofluxnats.NewPublisher[Event](conn, codec.Encode)
	sub := gofluxnats.NewSubscriber[Event](conn, codec.Decode)

	// Subscribe in a goroutine -- SubscribeWithReady blocks until ctx is
	// cancelled, and signals once the server has registered the subscription.
	ready := make(chan struct{})
	go func() {
		_ = sub.SubscribeWithReady(ctx, "events.created", func(ctx context.Context, msg goflux.Message[Event]) error {
			fmt.Printf("received: %s %s\n", msg.Payload.ID, msg.Payload.Name)
			return nil
		}, func() { close(ready) })
	}()

	// Wait for the subscription. Publishing before this point can be lost.
	<-ready

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
	ready := make(chan struct{})
	go func() {
		_ = responder.ServeWithReady(ctx, "orders.create", func(ctx context.Context, req OrderRequest) (OrderResponse, error) {
			return OrderResponse{OrderID: "ord-42", Status: "created"}, nil
		}, func() { close(ready) })
	}()

	// Wait for the responder. A request sent before this point is dropped and
	// surfaces only as a timeout.
	<-ready

	// Send a request.
	requester := gofluxnats.NewRequester[OrderRequest, OrderResponse](conn, reqCodec, respCodec)
	resp, err := requester.Request(ctx, "orders.create", OrderRequest{ItemID: "sku-1", Qty: 3})
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("order: %s status: %s\n", resp.OrderID, resp.Status)
}
```
