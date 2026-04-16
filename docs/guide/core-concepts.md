# Core Concepts

This page covers every core type in goflux and the rules that govern how they interact.

## Message[T]

`Message[T]` is the unit passed to every handler. It carries the routing key, a fully decoded payload, optional headers, and acknowledgment controls:

```go
type Message[T any] struct {
    Subject string `json:"subject"`
    Payload T      `json:"payload"`
    Header  Header `json:"header,omitempty"`
    // unexported: acker Acker
}
```

Use the constructors to create messages:

```go
msg := goflux.NewMessage("orders.created", OrderEvent{ID: "42", Total: 99.95})

// With headers:
h := goflux.Header{"X-Tenant": {"acme"}}
msg := goflux.NewMessageWithHeader("orders.created", event, h)
```

Messages expose acknowledgment methods -- `Ack()`, `Nak()`, `NakWithDelay(d)`, and `Term()`. These are no-ops on transports that do not support acknowledgments (channels, NATS core). See [Acker](#acker) below.

## Handler[T]

A handler is the callback that processes incoming messages:

```go
type Handler[T any] func(ctx context.Context, msg Message[T]) error
```

- Returning `nil` signals success.
- Returning a non-nil error signals failure. The exact consequence is transport-specific: fire-and-forget transports log and move on; JetStream transports can nak and redeliver.

```go
handler := func(ctx context.Context, msg goflux.Message[OrderEvent]) error {
    if msg.Payload.Total <= 0 {
        return fmt.Errorf("invalid order total: %f", msg.Payload.Total)
    }
    return processOrder(ctx, msg.Payload)
}
```

## Publisher[T]

A publisher sends messages to a subject:

```go
type Publisher[T any] interface {
    Publish(ctx context.Context, subject string, v T) error
    Close() error
}
```

The subject is specified at the call site, not at construction time. `Publish` serializes `v` via the transport's codec (if any) and delivers it:

```go
pub := gofluxnats.NewPublisher(conn, codec)

err := pub.Publish(ctx, "orders.created", OrderEvent{ID: "42"})
```

## Subscriber[T]

A subscriber listens on a subject and dispatches decoded messages to a handler:

```go
type Subscriber[T any] interface {
    Subscribe(ctx context.Context, subject string, handler Handler[T]) error
    Close() error
}
```

::: warning
`Subscribe` **blocks** until the context is cancelled or the transport encounters a fatal error. Always run it in a goroutine.
:::

```go
sub := gofluxnats.NewSubscriber(conn, codec)

go func() {
    if err := sub.Subscribe(ctx, "orders.created", handler); err != nil {
        slog.Error("subscriber exited", "error", err)
    }
}()
```

## Requester[Req, Resp] and Responder[Req, Resp]

Request-reply uses two paired interfaces:

```go
type Requester[Req, Resp any] interface {
    Request(ctx context.Context, subject string, req Req) (Resp, error)
    Close() error
}

type RequestHandler[Req, Resp any] func(ctx context.Context, req Req) (Resp, error)

type Responder[Req, Resp any] interface {
    Serve(ctx context.Context, subject string, handler RequestHandler[Req, Resp]) error
    Close() error
}
```

The requester sends a typed request and blocks until a typed response arrives. The responder processes incoming requests and returns responses. `Serve` blocks like `Subscribe`:

```go
// Responder side
responder := gofluxnats.NewResponder[GetOrderReq, GetOrderResp](conn, reqCodec, respCodec)

go func() {
    _ = responder.Serve(ctx, "orders.get", func(ctx context.Context, req GetOrderReq) (GetOrderResp, error) {
        order, err := db.FindOrder(ctx, req.OrderID)
        if err != nil {
            return GetOrderResp{}, err
        }
        return GetOrderResp{Order: order}, nil
    })
}()

// Requester side
requester := gofluxnats.NewRequester[GetOrderReq, GetOrderResp](conn, reqCodec, respCodec)

resp, err := requester.Request(ctx, "orders.get", GetOrderReq{OrderID: "42"})
```

## Header

`Header` carries message metadata -- trace context, message IDs, custom key-value pairs. It follows `http.Header` semantics: keys are case-sensitive, values are string slices.

```go
type Header map[string][]string
```

```go
h := make(goflux.Header)
h.Set("X-Tenant", "acme")
h.Add("X-Tag", "priority")
h.Add("X-Tag", "express")

tenant := h.Get("X-Tenant") // "acme"
h.Del("X-Tag")

copy := h.Clone() // deep copy
```

Transports propagate headers automatically. NATS maps them to NATS headers; HTTP maps them to HTTP headers.

## Acker

`Acker` is the minimal acknowledgment interface for transports that support at-least-once delivery:

```go
type Acker interface {
    Ack() error
    Nak() error
}
```

Two extended interfaces add more control:

```go
// Redeliver after a delay.
type DelayedNaker interface {
    Acker
    NakWithDelay(d time.Duration) error
}

// Permanently reject -- the message will not be redelivered.
type Terminator interface {
    Acker
    Term() error
}
```

`Message[T]` exposes these through convenience methods that gracefully degrade:

| Method | Has Acker | Has DelayedNaker | Has Terminator |
|--------|-----------|-------------------|----------------|
| `msg.Ack()` | calls `Ack()` | calls `Ack()` | calls `Ack()` |
| `msg.Nak()` | calls `Nak()` | calls `Nak()` | calls `Nak()` |
| `msg.NakWithDelay(d)` | falls back to `Nak()` | calls `NakWithDelay(d)` | falls back to `Nak()` |
| `msg.Term()` | falls back to `Ack()` | falls back to `Ack()` | calls `Term()` |

If the message has no acker at all (fire-and-forget transports), every method is a no-op.

Use `msg.HasAcker()` to check at runtime whether acknowledgment is available.

### AutoAck Middleware

For handlers that should always ack on success and nak on error, use the `AutoAck` middleware instead of manual calls:

```go
wrapped := goflux.AutoAck[OrderEvent]()(handler)
```

## Topic[T]

`Topic[T]` bundles a `Publisher[T]` and `Subscriber[T]` for services that need both:

```go
type Topic[T any] struct {
    Publisher[T]
    Subscriber[T]
}
```

```go
topic := goflux.Topic[OrderEvent]{
    Publisher:  pub,
    Subscriber: sub,
}

// Publish through the topic.
_ = topic.Publish(ctx, "orders", event)

// Subscribe through the topic.
go func() {
    _ = topic.Subscribe(ctx, "orders", handler)
}()
```

## BoundPublisher[T]

`BoundPublisher[T]` wraps a `Publisher[T]` with a fixed subject, removing the subject parameter from `Publish`:

```go
pub := gofluxnats.NewPublisher(conn, codec)
bound := goflux.Bind(pub, "orders.created")

// No subject argument needed.
err := bound.Publish(ctx, OrderEvent{ID: "42"})
```

Note that `BoundPublisher` does not implement `Publisher[T]` -- its `Publish` method has a different signature (no subject parameter).

## Middleware[T]

Middleware wraps a handler to add cross-cutting behavior:

```go
type Middleware[T any] func(Handler[T]) Handler[T]
```

Compose multiple middleware with `Chain`. The first middleware in the list is the outermost wrapper:

```go
wrapped := goflux.Chain[OrderEvent](
    middleware.AutoAck[OrderEvent](),
    middleware.InjectMessageID[OrderEvent](),
)(handler)
```

See [Middleware](/middleware/) for messaging-specific middleware (`AutoAck`, `RetryAck`, `InjectMessageID`, `InjectHeader`). For stream-processing operators (concurrency, filtering, deduplication, throttling), use [goflow](https://github.com/foomo/goflow) via `ToStream`.

## Key Design Rules

::: tip Rules to Remember
1. **Subscribe and Serve block.** Always run them in a goroutine.
2. **Caller owns connections.** Transport constructors for NATS, JetStream, and HTTP accept an existing connection. The caller connects and closes.
3. **Close semantics vary.** `Close()` on channel types is a no-op. NATS and JetStream transports call `conn.Drain()`. Check the transport documentation.
4. **No raw bytes in handlers.** `Message[T]` always carries the fully decoded payload. Decoding happens at the transport boundary.
5. **Codecs are stateless.** Share them freely across publishers and subscribers.
6. **Ack methods degrade gracefully.** Calling `Ack()` on a fire-and-forget message is a no-op, not a panic.
:::

## What's Next

- [Fire & Forget](./patterns/fire-and-forget.md) -- the simplest messaging pattern
- [At-Least-Once](./patterns/at-least-once.md) -- acknowledgment-based delivery with JetStream
- [Transports](../transports/channel.md) -- choose and configure a transport
- [Middleware](../middleware/) -- wrap handlers with cross-cutting behavior
- [Pipeline](../pipeline/) -- wire subscribers to publishers with filtering and transformation
